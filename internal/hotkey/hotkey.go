package hotkey

import (
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

type reconfigRequest struct {
	handsFreeStr string
	pttStr       string
	result       chan error
}

type Manager struct {
	state         State
	handsFreeHK   *hotkey.Hotkey
	pushToTalkHK  *hotkey.Hotkey
	callback      Callback
	mu            sync.RWMutex
	running       bool
	activeTrigger TriggerType
	reconfigCh    chan reconfigRequest
}

func NewManager(callback Callback) *Manager {
	return &Manager{
		state:      StateIdle,
		callback:   callback,
		reconfigCh: make(chan reconfigRequest),
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

func (m *Manager) Update(handsFreeStr, pttStr string) error {
	req := reconfigRequest{
		handsFreeStr: handsFreeStr,
		pttStr:       pttStr,
		result:       make(chan error, 1),
	}

	select { // reconfig runs on hotkey goroutine — no locks here
	case m.reconfigCh <- req:
		return <-req.result
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout waiting for hotkey update")
	}
}

// Start registers hotkeys. No mainthread.Init — Wails owns Cocoa; a second loop caused ~100% idle CPU. Idempotent via m.running.
func (m *Manager) Start(handsFreeStr, pttStr string) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return nil
	}
	m.running = true
	m.mu.Unlock()

	if handsFreeStr != "" {
		mods, key, err := parseHotkey(handsFreeStr)
		if err == nil {
			m.handsFreeHK = hotkey.New(mods, key)
			if err := m.handsFreeHK.Register(); err != nil {
				logger.Errorf("Failed to register initial hands-free: %v", err)
				m.handsFreeHK = nil
			}
		}
	}
	if pttStr != "" {
		mods, key, err := parseHotkey(pttStr)
		if err == nil {
			m.pushToTalkHK = hotkey.New(mods, key)
			if err := m.pushToTalkHK.Register(); err != nil {
				logger.Errorf("Failed to register initial ptt: %v", err)
				m.pushToTalkHK = nil
			}
		}
	}

	go func() {
		for {
			hf := m.handsFreeHK
			ptt := m.pushToTalkHK

			var hfDown <-chan hotkey.Event
			var pttDown, pttUp <-chan hotkey.Event

			if hf != nil {
				hfDown = hf.Keydown()
			}
			if ptt != nil {
				pttDown = ptt.Keydown()
				pttUp = ptt.Keyup()
			}

			select {
			case req := <-m.reconfigCh:
				err := m.handleReconfigure(req.handsFreeStr, req.pttStr)
				req.result <- err
				continue

			case _, ok := <-hfDown:
				if !ok {
					m.handsFreeHK = nil
					continue
				}
				m.handleHandsFree()

			case _, ok := <-pttDown:
				if !ok {
					m.pushToTalkHK = nil
					continue
				}
				m.handlePushToTalkDown()

			case _, ok := <-pttUp:
				if !ok {
					m.pushToTalkHK = nil
					continue
				}
				m.handlePushToTalkUp()
			}
		}
	}()

	return nil
}

func (m *Manager) handleReconfigure(handsFreeStr, pttStr string) error {
	if m.handsFreeHK != nil {
		m.handsFreeHK.Unregister()
		m.handsFreeHK = nil
	}
	if m.pushToTalkHK != nil {
		m.pushToTalkHK.Unregister()
		m.pushToTalkHK = nil
	}

	if handsFreeStr != "" {
		mods, key, err := parseHotkey(handsFreeStr)
		if err != nil {
			return fmt.Errorf("invalid hands-free hotkey: %w", err)
		}
		m.handsFreeHK = hotkey.New(mods, key)
		if err := m.handsFreeHK.Register(); err != nil {
			m.handsFreeHK = nil
			return fmt.Errorf("failed to register hands-free: %w", err)
		}
	}

	if pttStr != "" {
		mods, key, err := parseHotkey(pttStr)
		if err != nil {
			if m.handsFreeHK != nil {
				m.handsFreeHK.Unregister()
				m.handsFreeHK = nil
			}
			return fmt.Errorf("invalid ptt hotkey: %w", err)
		}
		m.pushToTalkHK = hotkey.New(mods, key)
		if err := m.pushToTalkHK.Register(); err != nil {
			m.pushToTalkHK = nil
			if m.handsFreeHK != nil {
				m.handsFreeHK.Unregister()
				m.handsFreeHK = nil
			}
			return fmt.Errorf("failed to register ptt: %w", err)
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
