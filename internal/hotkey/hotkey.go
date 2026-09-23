package hotkey

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"voxflow/internal/logger"

	"golang.design/x/hotkey"
)

type State int

const (
	StateIdle State = iota
	StateRecording
	StateProcessing
)

func (s State) String() string {
	switch s {
	case StateIdle:
		return "Idle"
	case StateRecording:
		return "Recording"
	case StateProcessing:
		return "Processing"
	default:
		return "Unknown"
	}
}

type TriggerType int

const (
	TriggerNone TriggerType = iota
	TriggerHandsFree
	TriggerPushToTalk
)

type Callback func(state State)

type keyEvent int

const (
	handsFreeDown keyEvent = iota
	handsFreeUp
	pushToTalkDown
	pushToTalkUp
)

type request struct {
	fn     func() error
	result chan error
}

type Manager struct {
	state         State
	callback      Callback
	mu            sync.RWMutex
	running       bool
	activeTrigger TriggerType

	// Owned by the loop goroutine.
	handsFreeHK  *hotkey.Hotkey
	pushToTalkHK *hotkey.Hotkey
	handsFreeStr string
	pttStr       string
	suspended    bool

	requests chan request
	// One channel for every key event, so a down queued behind a slow callback
	// is still handled before its up.
	keyEvents chan keyEvent
}

func NewManager(callback Callback) *Manager {
	return &Manager{
		state:     StateIdle,
		callback:  callback,
		requests:  make(chan request),
		keyEvents: make(chan keyEvent, 64),
	}
}

func parseHotkey(hotkeyStr string) ([]hotkey.Modifier, hotkey.Key, error) {
	parts := strings.Split(strings.ToLower(hotkeyStr), "+")
	if len(parts) < 2 {
		return nil, 0, fmt.Errorf("invalid hotkey format: %s", hotkeyStr)
	}

	var mods []hotkey.Modifier
	for _, part := range parts[:len(parts)-1] {
		switch part {
		case "cmd", "command", "super":
			mods = append(mods, hotkey.ModCmd)
		case "ctrl", "control":
			mods = append(mods, hotkey.ModCtrl)
		case "shift":
			mods = append(mods, hotkey.ModShift)
		case "alt", "option", "opt":
			mods = append(mods, hotkey.ModOption)
		default:
			return nil, 0, fmt.Errorf("unknown modifier: %s", part)
		}
	}

	keyStr := parts[len(parts)-1]
	key, err := parseKey(keyStr)
	if err != nil {
		return nil, 0, err
	}

	return mods, key, nil
}

func parseKey(keyStr string) (hotkey.Key, error) {
	keyMap := map[string]hotkey.Key{
		"a": hotkey.KeyA, "b": hotkey.KeyB, "c": hotkey.KeyC,
		"d": hotkey.KeyD, "e": hotkey.KeyE, "f": hotkey.KeyF,
		"g": hotkey.KeyG, "h": hotkey.KeyH, "i": hotkey.KeyI,
		"j": hotkey.KeyJ, "k": hotkey.KeyK, "l": hotkey.KeyL,
		"m": hotkey.KeyM, "n": hotkey.KeyN, "o": hotkey.KeyO,
		"p": hotkey.KeyP, "q": hotkey.KeyQ, "r": hotkey.KeyR,
		"s": hotkey.KeyS, "t": hotkey.KeyT, "u": hotkey.KeyU,
		"v": hotkey.KeyV, "w": hotkey.KeyW, "x": hotkey.KeyX,
		"y": hotkey.KeyY, "z": hotkey.KeyZ,
		"0": hotkey.Key0, "1": hotkey.Key1, "2": hotkey.Key2,
		"3": hotkey.Key3, "4": hotkey.Key4, "5": hotkey.Key5,
		"6": hotkey.Key6, "7": hotkey.Key7, "8": hotkey.Key8,
		"9":      hotkey.Key9,
		"space":  hotkey.KeySpace,
		"return": hotkey.KeyReturn, "enter": hotkey.KeyReturn,
		"escape": hotkey.KeyEscape, "esc": hotkey.KeyEscape,
		"tab": hotkey.KeyTab,
	}

	if key, ok := keyMap[keyStr]; ok {
		return key, nil
	}
	return 0, fmt.Errorf("unknown key: %s", keyStr)
}

// Update re-registers both hotkeys. While suspended it only validates them;
// they are registered when the suspension ends.
func (m *Manager) Update(handsFreeStr, pttStr string) error {
	return m.do(func() error {
		m.handsFreeStr, m.pttStr = handsFreeStr, pttStr
		if m.suspended {
			return validate(handsFreeStr, pttStr)
		}
		return m.rebind()
	})
}

// Suspend unregisters the global hotkeys so their combos reach the app window
// (e.g. while recording a new shortcut), and registers them again when false.
func (m *Manager) Suspend(suspend bool) error {
	return m.do(func() error {
		if suspend == m.suspended {
			return nil
		}
		m.suspended = suspend
		return m.rebind()
	})
}

func (m *Manager) do(fn func() error) error {
	req := request{fn: fn, result: make(chan error, 1)}
	select { // runs on the hotkey goroutine — no locks here
	case m.requests <- req:
		return <-req.result
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout waiting for hotkey update")
	}
}

// Start registers hotkeys. No mainthread.Init — Wails owns Cocoa; a second loop caused ~100% idle CPU. Idempotent via m.running.
// The listener runs even when a registration fails; the error names which one.
func (m *Manager) Start(handsFreeStr, pttStr string) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return nil
	}
	m.running = true
	m.mu.Unlock()

	m.handsFreeStr, m.pttStr = handsFreeStr, pttStr
	err := m.rebind()
	go m.loop()
	return err
}

func (m *Manager) loop() {
	for {
		select {
		case req := <-m.requests:
			req.result <- req.fn()
		case ev := <-m.keyEvents:
			switch ev {
			case handsFreeDown:
				m.handleHandsFree()
			case pushToTalkDown:
				m.handlePushToTalkDown()
			case pushToTalkUp:
				m.handlePushToTalkUp()
			}
		}
	}
}

// rebind replaces both registrations. Each hotkey registers on its own, so one
// bad combo doesn't take the other down.
func (m *Manager) rebind() error {
	for _, hk := range []*hotkey.Hotkey{m.handsFreeHK, m.pushToTalkHK} {
		if hk != nil {
			hk.Unregister()
		}
	}
	m.handsFreeHK, m.pushToTalkHK = nil, nil
	if m.suspended {
		return nil
	}

	var errs []error
	var err error
	if m.handsFreeStr != "" {
		if m.handsFreeHK, err = m.bind(m.handsFreeStr, handsFreeDown, handsFreeUp); err != nil {
			errs = append(errs, fmt.Errorf("hands-free hotkey %s: %w", m.handsFreeStr, err))
		}
	}
	if m.pttStr != "" {
		if m.pushToTalkHK, err = m.bind(m.pttStr, pushToTalkDown, pushToTalkUp); err != nil {
			errs = append(errs, fmt.Errorf("push-to-talk hotkey %s: %w", m.pttStr, err))
		}
	}
	return errors.Join(errs...)
}

func (m *Manager) bind(hotkeyStr string, down, up keyEvent) (*hotkey.Hotkey, error) {
	mods, key, err := parseHotkey(hotkeyStr)
	if err != nil {
		return nil, err
	}
	hk := hotkey.New(mods, key)
	if err := hk.Register(); err != nil {
		return nil, err
	}
	// The library delivers down and up on separate channels; merging them as
	// they arrive keeps their order. Both goroutines end on Unregister.
	go m.forward(hk.Keydown(), down)
	go m.forward(hk.Keyup(), up)
	return hk, nil
}

func (m *Manager) forward(events <-chan hotkey.Event, ev keyEvent) {
	for range events {
		m.keyEvents <- ev
	}
}

func validate(hotkeyStrs ...string) error {
	for _, s := range hotkeyStrs {
		if s == "" {
			continue
		}
		if _, _, err := parseHotkey(s); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) handleHandsFree() {
	logger.Debugf("[Hotkey] HandsFree triggered!")
	m.mu.Lock()

	if !m.running {
		logger.Debugf("[Hotkey] HandsFree ignored - not running")
		m.mu.Unlock()
		return
	}

	logger.Debugf("[Hotkey] HandsFree current state: %s", m.state)
	var newState State
	var shouldCallback bool

	switch m.state {
	case StateIdle:
		m.state = StateRecording
		m.activeTrigger = TriggerHandsFree
		newState = m.state
		shouldCallback = true
	case StateRecording:
		if m.activeTrigger == TriggerHandsFree || m.activeTrigger == TriggerNone {
			m.state = StateProcessing
			m.activeTrigger = TriggerNone
			newState = m.state
			shouldCallback = true
		}
	case StateProcessing:
	}

	callback := m.callback
	m.mu.Unlock()

	if shouldCallback && callback != nil { // outside lock — callback may re-enter
		logger.Debugf("[Hotkey] HandsFree calling callback with state: %s", newState)
		callback(newState)
	}
}

func (m *Manager) handlePushToTalkDown() {
	logger.Debugf("[Hotkey] PushToTalk DOWN triggered!")
	m.mu.Lock()

	if !m.running {
		logger.Debugf("[Hotkey] PushToTalk DOWN ignored - not running")
		m.mu.Unlock()
		return
	}

	logger.Debugf("[Hotkey] PushToTalk DOWN current state: %s", m.state)
	var newState State
	var shouldCallback bool

	if m.state == StateIdle {
		m.state = StateRecording
		m.activeTrigger = TriggerPushToTalk
		newState = m.state
		shouldCallback = true
	}

	callback := m.callback
	m.mu.Unlock()

	if shouldCallback && callback != nil {
		logger.Debugf("[Hotkey] PushToTalk DOWN calling callback with state: %s", newState)
		callback(newState)
	}
}

func (m *Manager) handlePushToTalkUp() {
	logger.Debugf("[Hotkey] PushToTalk UP triggered!")
	m.mu.Lock()

	if !m.running {
		logger.Debugf("[Hotkey] PushToTalk UP ignored - not running")
		m.mu.Unlock()
		return
	}

	logger.Debugf("[Hotkey] PushToTalk UP current state: %s, trigger: %d", m.state, m.activeTrigger)
	var newState State
	var shouldCallback bool

	if m.state == StateRecording && m.activeTrigger == TriggerPushToTalk {
		m.state = StateProcessing
		m.activeTrigger = TriggerNone
		newState = m.state
		shouldCallback = true
	}

	callback := m.callback
	m.mu.Unlock()

	if shouldCallback && callback != nil {
		logger.Debugf("[Hotkey] PushToTalk UP calling callback with state: %s", newState)
		callback(newState)
	}
}

func (m *Manager) Stop() {
	m.mu.Lock()
	m.running = false
	m.mu.Unlock()
}

func (m *Manager) GetState() State {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

func (m *Manager) SetState(state State) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = state
	if state == StateIdle {
		m.activeTrigger = TriggerNone
	}
}
