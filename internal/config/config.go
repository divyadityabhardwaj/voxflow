package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Default refinement models. Cloud catalogs churn, so ensureValidModel in the app
// swaps these out when a provider stops listing them.
const (
	DefaultGeminiModel     = "gemini-2.5-flash-lite"
	DefaultOpenRouterModel = "google/gemma-4-31b-it:free"
	DefaultGroqModel       = "openai/gpt-oss-20b"
	DefaultCerebrasModel   = "llama3.1-8b"
)

type Config struct {
	GeminiAPIKey     string `json:"gemini_api_key"`
	OpenRouterAPIKey string `json:"openrouter_api_key"`
	HandsFreeHotkey  string `json:"hands_free_hotkey"`   // e.g., "cmd+shift+space"
	PushToTalkHotkey string `json:"push_to_talk_hotkey"` // e.g., "cmd+shift+p"
	Hotkey           string `json:"hotkey,omitempty"`    // Legacy field, kept for migration
	WhisperModel     string `json:"whisper_model"`       // tiny, base, small
	WhisperLanguage  string `json:"whisper_language"`    // fixed language for transcription (en)
	WhisperThreads   int    `json:"whisper_threads"`     // 0 = whisper default
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

	Vocabulary          string             `json:"vocabulary,omitempty"` // comma-separated terms fed to Whisper and the LLM
	RefinementMode      string             `json:"refinement_mode"`      // "refine", "raw", "copy-only"
	MuteSystemAudio     *bool              `json:"mute_system_audio,omitempty"`
	AppRules            map[string]AppRule `json:"app_rules,omitempty"`
	OnboardingCompleted bool               `json:"onboarding_completed"`
	mu                  sync.RWMutex
	saveMu              sync.Mutex // serialises Save: concurrent writers would share one .tmp
}

// AppRule holds per-application overrides for refinement and injection behavior.
type AppRule struct {
	// RefinementMode overrides the global mode: "refine", "raw", or "copy-only".
	RefinementMode string `json:"refinement_mode,omitempty"`
	// InjectMethod is "paste" (default), "clipboard" (copy only, no Cmd+V),
	// or "type" (keystrokes, for apps that remap Cmd+V).
	InjectMethod string `json:"inject_method,omitempty"`
}

var (
	instance *Config
	once     sync.Once
)

func GetConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	configDir := filepath.Join(homeDir, ".voxflow")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return "", err
	}
	return configDir, nil
}

func GetConfigPath() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "config.json"), nil
}

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
		c.GeminiModel = DefaultGeminiModel
	}
	if c.LLMProvider == "" {
		c.LLMProvider = "gemini"
	}
	if c.OpenRouterModel == "" {
		c.OpenRouterModel = DefaultOpenRouterModel
	}
	if c.GroqModel == "" {
		c.GroqModel = DefaultGroqModel
	}
	if c.CerebrasModel == "" {
		c.CerebrasModel = DefaultCerebrasModel
	}
	if c.LocalURL == "" {
		c.LocalURL = "http://localhost:11434"
	}
	if c.RefinementMode == "" {
		c.RefinementMode = "refine"
	}
	if c.AppRules == nil {
		c.AppRules = make(map[string]AppRule)
	}

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

func (c *Config) Save() error {
	c.saveMu.Lock()
	defer c.saveMu.Unlock()
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

	// Write-then-rename so a crash mid-write never leaves a truncated config (API keys live here).
	tmp := configPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, configPath)
}

func (c *Config) GetGeminiAPIKey() string {
	if envKey := os.Getenv("GEMINI_API_KEY"); envKey != "" {
		return envKey
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.GeminiAPIKey
}

func (c *Config) SetGeminiAPIKey(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.GeminiAPIKey = key
}

func (c *Config) GetHandsFreeHotkey() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.HandsFreeHotkey == "" {
		return "cmd+shift+space"
	}
	return c.HandsFreeHotkey
}

func (c *Config) SetHandsFreeHotkey(hotkey string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.HandsFreeHotkey = hotkey
}

func (c *Config) GetPushToTalkHotkey() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.PushToTalkHotkey == "" {
		return "cmd+shift+p"
	}
	return c.PushToTalkHotkey
}

func (c *Config) SetPushToTalkHotkey(hotkey string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.PushToTalkHotkey = hotkey
}

func (c *Config) GetHotkey() string {
	return c.GetHandsFreeHotkey()
}

func (c *Config) SetHotkey(hotkey string) {
	c.SetHandsFreeHotkey(hotkey)
}

func (c *Config) GetWhisperModel() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.WhisperModel
}

func (c *Config) SetWhisperModel(model string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.WhisperModel = model
}

func (c *Config) GetWhisperLanguage() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.WhisperLanguage == "" {
		return "en"
	}
	return c.WhisperLanguage
}

func (c *Config) SetWhisperLanguage(language string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if language == "" {
		language = "en"
	}
	c.WhisperLanguage = language
}

func (c *Config) GetWhisperThreads() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.WhisperThreads < 0 {
		return 0
	}
	return c.WhisperThreads
}

func (c *Config) SetWhisperThreads(threads int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if threads < 0 {
		threads = 0
	}
	c.WhisperThreads = threads
}

func (c *Config) GetMiniModePosition() (int, int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.MiniModeX, c.MiniModeY
}

func (c *Config) SetMiniModePosition(x, y int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.MiniModeX = x
	c.MiniModeY = y
}

func (c *Config) GetMaximizedWindowPosition() (int, int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.MaximizedX, c.MaximizedY
}

func (c *Config) SetMaximizedWindowPosition(x, y int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.MaximizedX = x
	c.MaximizedY = y
}

func (c *Config) GetMaximizedWindowSize() (int, int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.MaximizedW, c.MaximizedH
}

func (c *Config) SetMaximizedWindowSize(w, h int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.MaximizedW = w
	c.MaximizedH = h
}

func (c *Config) GetGeminiModel() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.GeminiModel == "" {
		return DefaultGeminiModel
	}
	return c.GeminiModel
}

func (c *Config) SetGeminiModel(model string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.GeminiModel = model
}

func (c *Config) GetOpenRouterAPIKey() string {
	if envKey := os.Getenv("OPENROUTER_API_KEY"); envKey != "" {
		return envKey
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.OpenRouterAPIKey
}

func (c *Config) SetOpenRouterAPIKey(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.OpenRouterAPIKey = key
}

func (c *Config) GetLLMProvider() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.LLMProvider == "" {
		return "gemini" // Default to Gemini
	}
	return c.LLMProvider
}

func (c *Config) SetLLMProvider(provider string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.LLMProvider = provider
}

func (c *Config) GetOpenRouterModel() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.OpenRouterModel == "" {
		return DefaultOpenRouterModel
	}
	return c.OpenRouterModel
}

func (c *Config) SetOpenRouterModel(model string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.OpenRouterModel = model
}

func (c *Config) GetGroqAPIKey() string {
	if envKey := os.Getenv("GROQ_API_KEY"); envKey != "" {
		return envKey
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.GroqAPIKey
}

func (c *Config) SetGroqAPIKey(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.GroqAPIKey = key
}

func (c *Config) GetGroqModel() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.GroqModel == "" {
		return DefaultGroqModel
	}
	return c.GroqModel
}

func (c *Config) SetGroqModel(model string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.GroqModel = model
}

func (c *Config) GetCerebrasAPIKey() string {
	if envKey := os.Getenv("CEREBRAS_API_KEY"); envKey != "" {
		return envKey
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.CerebrasAPIKey
}

func (c *Config) SetCerebrasAPIKey(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.CerebrasAPIKey = key
}

func (c *Config) GetCerebrasModel() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.CerebrasModel == "" {
		return DefaultCerebrasModel
	}
	return c.CerebrasModel
}

func (c *Config) SetCerebrasModel(model string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.CerebrasModel = model
}

func (c *Config) GetLocalModel() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.LocalModel
}

func (c *Config) SetLocalModel(model string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.LocalModel = model
}

func (c *Config) GetLocalURL() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.LocalURL == "" {
		return "http://localhost:11434"
	}
	return c.LocalURL
}

func (c *Config) SetLocalURL(url string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.LocalURL = url
}

func (c *Config) GetVocabulary() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Vocabulary
}

func (c *Config) SetVocabulary(v string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Vocabulary = v
}

func (c *Config) GetRefinementMode() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.RefinementMode == "" {
		return "refine"
	}
	return c.RefinementMode
}

func (c *Config) SetRefinementMode(mode string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.RefinementMode = mode
}

func (c *Config) GetMuteSystemAudio() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.MuteSystemAudio == nil {
		return true // Default is true
	}
	return *c.MuteSystemAudio
}

func (c *Config) SetMuteSystemAudio(val bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.MuteSystemAudio = &val
}

func (c *Config) GetOnboardingCompleted() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.OnboardingCompleted
}

func (c *Config) SetOnboardingCompleted(done bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.OnboardingCompleted = done
}

func (c *Config) GetAppRules() map[string]AppRule {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[string]AppRule, len(c.AppRules))
	for k, v := range c.AppRules {
		out[k] = v
	}
	return out
}

func (c *Config) SetAppRule(bundleID string, rule AppRule) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.AppRules == nil {
		c.AppRules = make(map[string]AppRule)
	}
	c.AppRules[bundleID] = rule
}

func (c *Config) RemoveAppRule(bundleID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.AppRules, bundleID)
}

func (c *Config) ResolveRefinementMode(bundleID string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if bundleID != "" {
		if rule, ok := c.AppRules[bundleID]; ok && rule.RefinementMode != "" {
			return rule.RefinementMode
		}
	}
	if c.RefinementMode == "" {
		return "refine"
	}
	return c.RefinementMode
}

func (c *Config) InjectMethodFor(bundleID string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if bundleID != "" {
		switch m := c.AppRules[bundleID].InjectMethod; m {
		case "clipboard", "type":
			return m
		}
	}
	return "paste"
}

type CachedModelList struct {
	Models    []string  `json:"models"`
	Timestamp time.Time `json:"timestamp"`
}

type ModelCache map[string]CachedModelList

const modelCacheTTL = 24 * time.Hour

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
