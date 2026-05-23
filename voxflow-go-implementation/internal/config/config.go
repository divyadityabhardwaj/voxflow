package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Config holds the application configuration
type Config struct {
	GeminiAPIKey     string `json:"gemini_api_key"`
	OpenRouterAPIKey string `json:"openrouter_api_key"`
	HandsFreeHotkey  string `json:"hands_free_hotkey"`   // e.g., "cmd+shift+space"
	PushToTalkHotkey string `json:"push_to_talk_hotkey"` // e.g., "cmd+shift+p"
	Hotkey           string `json:"hotkey,omitempty"`    // Legacy field, kept for migration
	WhisperModel     string `json:"whisper_model"`       // tiny, base, small
	WhisperLanguage  string `json:"whisper_language"`    // fixed language for transcription (en)
	WhisperThreads   int    `json:"whisper_threads"`     // 0 = auto
	WhisperProfile   string `json:"whisper_profile"`     // machine+model profile key for autotuned threads
	MiniModeX        int    `json:"mini_mode_x"`         // Saved X position of mini pill
	MiniModeY        int    `json:"mini_mode_y"`         // Saved Y position of mini pill
	MaximizedX       int    `json:"maximized_x"`         // Saved X position of maximized window
	MaximizedY       int    `json:"maximized_y"`         // Saved Y position of maximized window
	MaximizedW       int    `json:"maximized_w"`         // Saved width of maximized window
	MaximizedH       int    `json:"maximized_h"`         // Saved height of maximized window
	GeminiModel      string `json:"gemini_model"`        // Saved Gemini model to use
	LLMProvider      string `json:"llm_provider"`        // "gemini", "openrouter", "groq", "cerebras"
	OpenRouterModel  string `json:"openrouter_model"`    // Saved OpenRouter model to use
	GroqAPIKey       string `json:"groq_api_key"`
	GroqModel        string `json:"groq_model"`
	CerebrasAPIKey   string `json:"cerebras_api_key"`
	CerebrasModel    string `json:"cerebras_model"`

	LocalModel string `json:"local_model"` // Free-form model name sent to the local server
	LocalURL   string `json:"local_url"`   // Base URL of the local OpenAI-compatible server

	RefinementMode  string `json:"refinement_mode"` // "refine", "raw", "copy-only"
	MuteSystemAudio *bool  `json:"mute_system_audio,omitempty"`
	mu              sync.RWMutex
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
			WhisperLanguage:  "en",
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
	if c.WhisperLanguage == "" {
		c.WhisperLanguage = "en"
	}
	if c.WhisperThreads < 0 {
		c.WhisperThreads = 0
	}
	if c.GeminiModel == "" {
		c.GeminiModel = "gemini-1.5-flash"
	}
	if c.LLMProvider == "" {
		c.LLMProvider = "gemini"
	}
	if c.OpenRouterModel == "" {
		c.OpenRouterModel = "qwen/qwen3-235b-a22b:free"
	}
	if c.GroqModel == "" {
		c.GroqModel = "llama-3.1-8b-instant"
	}
	if c.CerebrasModel == "" {
		c.CerebrasModel = "llama3.1-8b"
	}
	if c.LocalURL == "" {
		c.LocalURL = "http://localhost:11434"
	}
	if c.RefinementMode == "" {
		c.RefinementMode = "refine"
	}

	// Check environment variable first for API key
	if apiKey := os.Getenv("GEMINI_API_KEY"); apiKey != "" {
		c.GeminiAPIKey = apiKey
	}
	if apiKey := os.Getenv("OPENROUTER_API_KEY"); apiKey != "" {
		c.OpenRouterAPIKey = apiKey
	}
	if apiKey := os.Getenv("GROQ_API_KEY"); apiKey != "" {
		c.GroqAPIKey = apiKey
	}
	if apiKey := os.Getenv("CEREBRAS_API_KEY"); apiKey != "" {
		c.CerebrasAPIKey = apiKey
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

	return os.WriteFile(configPath, data, 0600)
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

// GetWhisperLanguage returns the fixed language used by Whisper.
func (c *Config) GetWhisperLanguage() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.WhisperLanguage == "" {
		return "en"
	}
	return c.WhisperLanguage
}

// SetWhisperLanguage sets the fixed language used by Whisper.
func (c *Config) SetWhisperLanguage(language string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if language == "" {
		language = "en"
	}
	c.WhisperLanguage = language
}

// GetWhisperThreads returns the configured Whisper thread count (0 = auto).
func (c *Config) GetWhisperThreads() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.WhisperThreads < 0 {
		return 0
	}
	return c.WhisperThreads
}

// SetWhisperThreads sets the Whisper thread count (0 = auto).
func (c *Config) SetWhisperThreads(threads int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if threads < 0 {
		threads = 0
	}
	c.WhisperThreads = threads
}

// GetWhisperProfile returns the profile key used for thread autotune cache.
func (c *Config) GetWhisperProfile() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.WhisperProfile
}

// SetWhisperProfile sets the profile key used for thread autotune cache.
func (c *Config) SetWhisperProfile(profile string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.WhisperProfile = profile
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

// GetMaximizedWindowPosition returns the saved maximized window position
func (c *Config) GetMaximizedWindowPosition() (int, int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.MaximizedX, c.MaximizedY
}

// SetMaximizedWindowPosition sets the saved maximized window position
func (c *Config) SetMaximizedWindowPosition(x, y int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.MaximizedX = x
	c.MaximizedY = y
}

// GetMaximizedWindowSize returns the saved maximized window size
func (c *Config) GetMaximizedWindowSize() (int, int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.MaximizedW, c.MaximizedH
}

// SetMaximizedWindowSize sets the saved maximized window size
func (c *Config) SetMaximizedWindowSize(w, h int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.MaximizedW = w
	c.MaximizedH = h
}

// GetGeminiModel returns the configured Gemini model
func (c *Config) GetGeminiModel() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.GeminiModel == "" {
		return "gemini-1.5-flash"
	}
	return c.GeminiModel
}

// SetGeminiModel sets the configured Gemini model
func (c *Config) SetGeminiModel(model string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.GeminiModel = model
}

// GetOpenRouterAPIKey returns the OpenRouter API key
func (c *Config) GetOpenRouterAPIKey() string {
	if envKey := os.Getenv("OPENROUTER_API_KEY"); envKey != "" {
		return envKey
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.OpenRouterAPIKey
}

// SetOpenRouterAPIKey sets the OpenRouter API key
func (c *Config) SetOpenRouterAPIKey(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.OpenRouterAPIKey = key
}

// GetLLMProvider returns the LLM provider (gemini or openrouter)
func (c *Config) GetLLMProvider() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.LLMProvider == "" {
		return "gemini" // Default to Gemini
	}
	return c.LLMProvider
}

// SetLLMProvider sets the LLM provider
func (c *Config) SetLLMProvider(provider string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.LLMProvider = provider
}

// GetOpenRouterModel returns the configured OpenRouter model
func (c *Config) GetOpenRouterModel() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.OpenRouterModel == "" {
		return "qwen/qwen3-235b-a22b:free" // Default to Qwen
	}
	return c.OpenRouterModel
}

// SetOpenRouterModel sets the OpenRouter model
func (c *Config) SetOpenRouterModel(model string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.OpenRouterModel = model
}

// GetGroqAPIKey returns the Groq API key
func (c *Config) GetGroqAPIKey() string {
	if envKey := os.Getenv("GROQ_API_KEY"); envKey != "" {
		return envKey
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.GroqAPIKey
}

// SetGroqAPIKey sets the Groq API key
func (c *Config) SetGroqAPIKey(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.GroqAPIKey = key
}

// GetGroqModel returns the configured Groq model
func (c *Config) GetGroqModel() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.GroqModel == "" {
		return "llama-3.1-8b-instant"
	}
	return c.GroqModel
}

// SetGroqModel sets the Groq model
func (c *Config) SetGroqModel(model string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.GroqModel = model
}

// GetCerebrasAPIKey returns the Cerebras API key
func (c *Config) GetCerebrasAPIKey() string {
	if envKey := os.Getenv("CEREBRAS_API_KEY"); envKey != "" {
		return envKey
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.CerebrasAPIKey
}

// SetCerebrasAPIKey sets the Cerebras API key
func (c *Config) SetCerebrasAPIKey(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.CerebrasAPIKey = key
}

// GetCerebrasModel returns the configured Cerebras model
func (c *Config) GetCerebrasModel() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.CerebrasModel == "" {
		return "llama3.1-8b"
	}
	return c.CerebrasModel
}

// SetCerebrasModel sets the Cerebras model
func (c *Config) SetCerebrasModel(model string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.CerebrasModel = model
}

// GetLocalModel returns the user-supplied local model name (e.g. "qwen3:8b").
func (c *Config) GetLocalModel() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.LocalModel
}

// SetLocalModel sets the local model name.
func (c *Config) SetLocalModel(model string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.LocalModel = model
}

// GetLocalURL returns the base URL of the local OpenAI-compatible server.
func (c *Config) GetLocalURL() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.LocalURL == "" {
		return "http://localhost:11434"
	}
	return c.LocalURL
}

// SetLocalURL sets the base URL of the local OpenAI-compatible server.
func (c *Config) SetLocalURL(url string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.LocalURL = url
}

// GetRefinementMode returns the configured refinement mode ("refine", "raw", "copy-only").
func (c *Config) GetRefinementMode() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.RefinementMode == "" {
		return "refine"
	}
	return c.RefinementMode
}

// SetRefinementMode sets the refinement mode.
func (c *Config) SetRefinementMode(mode string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.RefinementMode = mode
}

// GetMuteSystemAudio returns whether the system audio should be muted during recording (defaults to true).
func (c *Config) GetMuteSystemAudio() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.MuteSystemAudio == nil {
		return true // Default is true
	}
	return *c.MuteSystemAudio
}

// SetMuteSystemAudio sets whether system audio should be muted during recording.
func (c *Config) SetMuteSystemAudio(val bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.MuteSystemAudio = &val
}

// CachedModelList holds a list of models and the time they were fetched.
type CachedModelList struct {
	Models    []string  `json:"models"`
	Timestamp time.Time `json:"timestamp"`
}

// ModelCache maps provider names to their cached model lists.
type ModelCache map[string]CachedModelList

const modelCacheTTL = 24 * time.Hour

// LoadModelCache reads the model list for the given provider from disk if it hasn't expired.
func LoadModelCache(provider string) ([]string, bool) {
	configDir, err := GetConfigDir()
	if err != nil {
		return nil, false
	}
	cachePath := filepath.Join(configDir, "models_cache.json")
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, false
	}
	var cache ModelCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, false
	}
	cached, ok := cache[provider]
	if !ok {
		return nil, false
	}
	if time.Since(cached.Timestamp) > modelCacheTTL {
		return nil, false
	}
	return cached.Models, true
}

// SaveModelCache persists a list of models for the given provider to disk.
func SaveModelCache(provider string, models []string) error {
	configDir, err := GetConfigDir()
	if err != nil {
		return err
	}
	cachePath := filepath.Join(configDir, "models_cache.json")
	cache := make(ModelCache)
	if data, err := os.ReadFile(cachePath); err == nil {
		_ = json.Unmarshal(data, &cache)
	}
	cache[provider] = CachedModelList{
		Models:    models,
		Timestamp: time.Now(),
	}
	data, err := json.Marshal(cache)
	if err != nil {
		return err
	}
	return os.WriteFile(cachePath, data, 0644)
}

// ClearModelCache invalidates the cached model list for a given provider.
func ClearModelCache(provider string) error {
	configDir, err := GetConfigDir()
	if err != nil {
		return err
	}
	cachePath := filepath.Join(configDir, "models_cache.json")
	cache := make(ModelCache)
	if data, err := os.ReadFile(cachePath); err == nil {
		_ = json.Unmarshal(data, &cache)
	}
	delete(cache, provider)
	data, err := json.Marshal(cache)
	if err != nil {
		return err
	}
	return os.WriteFile(cachePath, data, 0644)
}

