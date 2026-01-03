package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Config holds the application configuration
type Config struct {
	GeminiAPIKey string `json:"gemini_api_key"`
	Hotkey       string `json:"hotkey"`        // e.g., "cmd+shift+v"
	WhisperModel string `json:"whisper_model"` // tiny, base, small
	Mode         string `json:"mode"`          // casual, formal
	mu           sync.RWMutex
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
			Hotkey:       "cmd+shift+v",
			WhisperModel: "base",
			Mode:         "casual",
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

	return json.Unmarshal(data, c)
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
func (c *Config) GetGeminiAPIKey() string {
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

// GetHotkey returns the configured hotkey
func (c *Config) GetHotkey() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Hotkey
}

// SetHotkey sets the hotkey
func (c *Config) SetHotkey(hotkey string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Hotkey = hotkey
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
