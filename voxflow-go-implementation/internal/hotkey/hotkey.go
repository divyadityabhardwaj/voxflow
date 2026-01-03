package hotkey

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.design/x/hotkey"
	"golang.design/x/hotkey/mainthread"
)

// State represents the current app state
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

// TriggerType represents what triggered the recording
type TriggerType int

const (
	TriggerNone TriggerType = iota
	TriggerHandsFree
	TriggerPushToTalk
)

// Callback is called when state changes
type Callback func(state State)

// reconfigRequest holds new hotkey configuration
type reconfigRequest struct {
	handsFreeStr string
	pttStr       string
	result       chan error
}

// Manager handles global hotkey registration and state
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

// NewManager creates a new hotkey manager
func NewManager(callback Callback) *Manager {
	return &Manager{
		state:      StateIdle,
		callback:   callback,
		reconfigCh: make(chan reconfigRequest), // Unbuffered for synchronous update
	}
}

// parseHotkey converts a string like "cmd+shift+v" to hotkey modifiers and key
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

// parseKey converts a key string to a hotkey.Key
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

// Update updates the registered hotkeys (called from app.go)
// This sends a request to the main loop and waits for the result
func (m *Manager) Update(handsFreeStr, pttStr string) error {
	// Create request with result channel
	req := reconfigRequest{
		handsFreeStr: handsFreeStr,
		pttStr:       pttStr,
		result:       make(chan error, 1),
	}

	// Send request to the main loop (don't hold any locks here!)
	select {
	case m.reconfigCh <- req:
		// Wait for result
		return <-req.result
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout waiting for hotkey update")
	}
}

// Start begins listening using mainthread
// This should only be called ONCE at app startup
func (m *Manager) Start(handsFreeStr, pttStr string) error {
	m.mu.Lock()
	m.running = true
	m.mu.Unlock()

	go mainthread.Init(func() {
		// Initial Registration
		if handsFreeStr != "" {
			mods, key, err := parseHotkey(handsFreeStr)
			if err == nil {
				m.handsFreeHK = hotkey.New(mods, key)
				if err := m.handsFreeHK.Register(); err != nil {
					fmt.Printf("Failed to register initial hands-free: %v\n", err)
				}
			}
		}
		if pttStr != "" {
			mods, key, err := parseHotkey(pttStr)
			if err == nil {
				m.pushToTalkHK = hotkey.New(mods, key)
				if err := m.pushToTalkHK.Register(); err != nil {
					fmt.Printf("Failed to register initial ptt: %v\n", err)
				}
			}
		}

		// Event Loop - runs FOREVER
		for {
			// Get current hotkey references (no lock needed for reading pointers in this context)
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
				// Reconfigure request received
				err := m.handleReconfigure(req.handsFreeStr, req.pttStr)
				req.result <- err
				continue

			case _, ok := <-hfDown:
				if !ok {
					continue
				}
				m.handleHandsFree()

			case _, ok := <-pttDown:
				if !ok {
					continue
				}
				m.handlePushToTalkDown()

			case _, ok := <-pttUp:
				if !ok {
					continue
				}
				m.handlePushToTalkUp()

			case <-time.After(100 * time.Millisecond):
				// Heartbeat to allow reconfigure checks
			}
		}
	})

	return nil
}

// handleReconfigure performs the actual hotkey swap (called from main loop)
func (m *Manager) handleReconfigure(handsFreeStr, pttStr string) error {
	// Unregister old hotkeys
	if m.handsFreeHK != nil {
		m.handsFreeHK.Unregister()
		m.handsFreeHK = nil
	}
	if m.pushToTalkHK != nil {
		m.pushToTalkHK.Unregister()
		m.pushToTalkHK = nil
	}

	// Parse and register new hands-free
	if handsFreeStr != "" {
		mods, key, err := parseHotkey(handsFreeStr)
		if err != nil {
			return fmt.Errorf("invalid hands-free hotkey: %w", err)
		}
		m.handsFreeHK = hotkey.New(mods, key)
		if err := m.handsFreeHK.Register(); err != nil {
			return fmt.Errorf("failed to register hands-free: %w", err)
		}
	}

	// Parse and register new PTT
	if pttStr != "" {
		mods, key, err := parseHotkey(pttStr)
		if err != nil {
			// Cleanup partial registration
			if m.handsFreeHK != nil {
				m.handsFreeHK.Unregister()
				m.handsFreeHK = nil
			}
			return fmt.Errorf("invalid ptt hotkey: %w", err)
		}
		m.pushToTalkHK = hotkey.New(mods, key)
		if err := m.pushToTalkHK.Register(); err != nil {
			// Cleanup partial registration
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
	fmt.Println("[Hotkey] HandsFree triggered!")
	m.mu.Lock()

	if !m.running {
		fmt.Println("[Hotkey] HandsFree ignored - not running")
		m.mu.Unlock()
		return
	}

	fmt.Printf("[Hotkey] HandsFree current state: %s\n", m.state)
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
		// Do nothing
	}

	callback := m.callback
	m.mu.Unlock()

	// Call callback OUTSIDE of lock to avoid deadlock
	if shouldCallback && callback != nil {
		fmt.Printf("[Hotkey] HandsFree calling callback with state: %s\n", newState)
		callback(newState)
	}
}

func (m *Manager) handlePushToTalkDown() {
	fmt.Println("[Hotkey] PushToTalk DOWN triggered!")
	m.mu.Lock()

	if !m.running {
		fmt.Println("[Hotkey] PushToTalk DOWN ignored - not running")
		m.mu.Unlock()
		return
	}

	fmt.Printf("[Hotkey] PushToTalk DOWN current state: %s\n", m.state)
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
		fmt.Printf("[Hotkey] PushToTalk DOWN calling callback with state: %s\n", newState)
		callback(newState)
	}
}

func (m *Manager) handlePushToTalkUp() {
	fmt.Println("[Hotkey] PushToTalk UP triggered!")
	m.mu.Lock()

	if !m.running {
		fmt.Println("[Hotkey] PushToTalk UP ignored - not running")
		m.mu.Unlock()
		return
	}

	fmt.Printf("[Hotkey] PushToTalk UP current state: %s, trigger: %d\n", m.state, m.activeTrigger)
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
		fmt.Printf("[Hotkey] PushToTalk UP calling callback with state: %s\n", newState)
		callback(newState)
	}
}

// Stop stops listening for hotkey
func (m *Manager) Stop() {
	m.mu.Lock()
	m.running = false
	m.mu.Unlock()
	// Note: We don't actually stop the mainthread.Init loop because
	// doing so would terminate the app. We just set running=false.
}

// GetState returns the current state
func (m *Manager) GetState() State {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

// SetState sets the current state
func (m *Manager) SetState(state State) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = state
	if state == StateIdle {
		m.activeTrigger = TriggerNone
	}
}
