package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Config holds the application configuration
type Config struct {
	GeminiAPIKey     string `json:"gemini_api_key"`
	HandsFreeHotkey  string `json:"hands_free_hotkey"`   // e.g., "cmd+shift+space"
	PushToTalkHotkey string `json:"push_to_talk_hotkey"` // e.g., "cmd+shift+p"
	Hotkey           string `json:"hotkey,omitempty"`    // Legacy field, kept for migration
	WhisperModel     string `json:"whisper_model"`       // tiny, base, small
	Mode             string `json:"mode"`                // casual, formal
	MiniModeX        int    `json:"mini_mode_x"`         // Saved X position of mini pill
	MiniModeY        int    `json:"mini_mode_y"`         // Saved Y position of mini pill
	mu               sync.RWMutex
}

var (
	instance *Config
	once     sync.Once
)

// GetConfigDir returns the application config directory
func GetConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	configDir := filepath.Join(homeDir, ".voxflow")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", err
	}
	return configDir, nil
}

// GetConfigPath returns the path to the config file
func GetConfigPath() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "config.json"), nil
}

// GetInstance returns the singleton config instance
func GetInstance() *Config {
	once.Do(func() {
		instance = &Config{
			HandsFreeHotkey:  "cmd+shift+space",
			PushToTalkHotkey: "cmd+shift+p",
			WhisperModel:     "base",
			Mode:             "casual",
		}
		instance.Load()
	})
	return instance
}

// Load reads the config from disk
func (c *Config) Load() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	configPath, err := GetConfigPath()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No config file yet, use defaults
		}
		return err
	}

	err = json.Unmarshal(data, c)
	if err != nil {
		return err
	}

	// Migration: If legacy Hotkey exists but HandsFreeHotkey is empty, use legacy
	if c.Hotkey != "" && c.HandsFreeHotkey == "" {
		c.HandsFreeHotkey = c.Hotkey
	}

	// Ensure defaults
	if c.HandsFreeHotkey == "" {
		c.HandsFreeHotkey = "cmd+shift+space"
	}
	if c.PushToTalkHotkey == "" {
		c.PushToTalkHotkey = "cmd+shift+p"
	}
	if c.WhisperModel == "" {
		c.WhisperModel = "base"
	}
	if c.Mode == "" {
		c.Mode = "casual"
	}

	// Check environment variable first for API key
	if apiKey := os.Getenv("GEMINI_API_KEY"); apiKey != "" {
		c.GeminiAPIKey = apiKey
	}

	return nil
}

// Save writes the config to disk
func (c *Config) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	configPath, err := GetConfigPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// GetGeminiAPIKey returns the Gemini API key
// Checks environment variable first, then config file
func (c *Config) GetGeminiAPIKey() string {
	// Check environment variable first
	if envKey := os.Getenv("GEMINI_API_KEY"); envKey != "" {
		return envKey
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.GeminiAPIKey
}

// SetGeminiAPIKey sets the Gemini API key
func (c *Config) SetGeminiAPIKey(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.GeminiAPIKey = key
}

// GetHandsFreeHotkey returns the hands-free hotkey
func (c *Config) GetHandsFreeHotkey() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.HandsFreeHotkey == "" {
		return "cmd+shift+space"
	}
	return c.HandsFreeHotkey
}

// SetHandsFreeHotkey sets the hands-free hotkey
func (c *Config) SetHandsFreeHotkey(hotkey string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.HandsFreeHotkey = hotkey
}

// GetPushToTalkHotkey returns the push-to-talk hotkey
func (c *Config) GetPushToTalkHotkey() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.PushToTalkHotkey == "" {
		return "cmd+shift+p"
	}
	return c.PushToTalkHotkey
}

// SetPushToTalkHotkey sets the push-to-talk hotkey
func (c *Config) SetPushToTalkHotkey(hotkey string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.PushToTalkHotkey = hotkey
}

// GetHotkey returns the configured hotkey (legacy)
func (c *Config) GetHotkey() string {
	return c.GetHandsFreeHotkey()
}

// SetHotkey sets the hotkey (legacy, maps to hands-free)
func (c *Config) SetHotkey(hotkey string) {
	c.SetHandsFreeHotkey(hotkey)
}

// GetWhisperModel returns the Whisper model size
func (c *Config) GetWhisperModel() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.WhisperModel
}

// SetWhisperModel sets the Whisper model size
func (c *Config) SetWhisperModel(model string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.WhisperModel = model
}

// GetMode returns the transcription mode
func (c *Config) GetMode() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Mode
}

// SetMode sets the transcription mode
func (c *Config) SetMode(mode string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Mode = mode
}

// GetMiniModePosition returns the saved mini mode position
func (c *Config) GetMiniModePosition() (int, int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.MiniModeX, c.MiniModeY
}

// SetMiniModePosition sets the saved mini mode position
func (c *Config) SetMiniModePosition(x, y int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.MiniModeX = x
	c.MiniModeY = y
}
