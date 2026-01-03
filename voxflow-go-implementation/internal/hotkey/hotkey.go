package hotkey

import (
	"fmt"
	"strings"
	"sync"

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

// Callback is called when hotkey is pressed
type Callback func(state State)

// Manager handles global hotkey registration and state
type Manager struct {
	state    State
	hk       *hotkey.Hotkey
	callback Callback
	mu       sync.RWMutex
	running  bool
}

// NewManager creates a new hotkey manager
func NewManager(callback Callback) *Manager {
	return &Manager{
		state:    StateIdle,
		callback: callback,
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
		"9": hotkey.Key9,
		"space": hotkey.KeySpace,
		"return": hotkey.KeyReturn, "enter": hotkey.KeyReturn,
		"escape": hotkey.KeyEscape, "esc": hotkey.KeyEscape,
		"tab": hotkey.KeyTab,
	}

	if key, ok := keyMap[keyStr]; ok {
		return key, nil
	}
	return 0, fmt.Errorf("unknown key: %s", keyStr)
}

// Register registers the global hotkey
func (m *Manager) Register(hotkeyStr string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mods, key, err := parseHotkey(hotkeyStr)
	if err != nil {
		return err
	}

	m.hk = hotkey.New(mods, key)
	return nil
}

// Start begins listening for hotkey in a goroutine
// Must be called from main goroutine via mainthread.Init
func (m *Manager) Start() error {
	m.mu.Lock()
	if m.hk == nil {
		m.mu.Unlock()
		return fmt.Errorf("hotkey not registered")
	}
	m.running = true
	m.mu.Unlock()

	go mainthread.Init(func() {
		if err := m.hk.Register(); err != nil {
			fmt.Printf("Failed to register hotkey: %v\n", err)
			return
		}
		defer m.hk.Unregister()

		for range m.hk.Keydown() {
			m.mu.Lock()
			if !m.running {
				m.mu.Unlock()
				return
			}

			// Toggle state
			switch m.state {
			case StateIdle:
				m.state = StateRecording
			case StateRecording:
				m.state = StateProcessing
			case StateProcessing:
				// Do nothing while processing
				m.mu.Unlock()
				continue
			}

			currentState := m.state
			callback := m.callback
			m.mu.Unlock()

			if callback != nil {
				callback(currentState)
			}
		}
	})

	return nil
}

// Stop stops listening for hotkey
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = false
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
}
