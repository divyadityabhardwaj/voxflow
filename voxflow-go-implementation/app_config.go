package main

import (
	"fmt"
	"voxflow/internal/cerebras"
	"voxflow/internal/groq"
	"voxflow/internal/logger"
	"voxflow/internal/openrouter"
	"voxflow/internal/whisper"
)

// ConfigResponse represents the strongly-typed configuration payload exposed to the frontend.
type ConfigResponse struct {
	Hotkey              string `json:"hotkey"`
	HandsFreeHotkey     string `json:"hands_free_hotkey"`
	PushToTalkHotkey    string `json:"push_to_talk_hotkey"`
	WhisperModel        string `json:"whisper_model"`
	WhisperLanguage     string `json:"whisper_language"`
	WhisperThreads      int    `json:"whisper_threads"`
	GeminiModel         string `json:"gemini_model"`
	APIKeySet           bool   `json:"api_key_set"`
	LLMProvider         string `json:"llm_provider"`
	OpenRouterModel     string `json:"openrouter_model"`
	OpenRouterAPIKeySet bool   `json:"openrouter_api_key_set"`
	GroqModel           string `json:"groq_model"`
	GroqAPIKeySet       bool   `json:"groq_api_key_set"`
	CerebrasModel       string `json:"cerebras_model"`
	CerebrasAPIKeySet   bool   `json:"cerebras_api_key_set"`
	LocalModel          string `json:"local_model"`
	LocalURL            string `json:"local_url"`
	RefinementMode      string `json:"refinement_mode"`
	MuteSystemAudio     bool   `json:"mute_system_audio"`
}

// GetConfig returns the current configuration strongly typed.
func (a *App) GetConfig() *ConfigResponse {
	return &ConfigResponse{
		Hotkey:              a.config.GetHotkey(),
		HandsFreeHotkey:     a.config.GetHandsFreeHotkey(),
		PushToTalkHotkey:    a.config.GetPushToTalkHotkey(),
		WhisperModel:        a.config.GetWhisperModel(),
		WhisperLanguage:     a.config.GetWhisperLanguage(),
		WhisperThreads:      a.config.GetWhisperThreads(),
		GeminiModel:         a.config.GetGeminiModel(),
		APIKeySet:           a.config.GetGeminiAPIKey() != "",
		LLMProvider:         a.config.GetLLMProvider(),
		OpenRouterModel:     a.config.GetOpenRouterModel(),
		OpenRouterAPIKeySet: a.config.GetOpenRouterAPIKey() != "",
		GroqModel:           a.config.GetGroqModel(),
		GroqAPIKeySet:       a.config.GetGroqAPIKey() != "",
		CerebrasModel:       a.config.GetCerebrasModel(),
		CerebrasAPIKeySet:   a.config.GetCerebrasAPIKey() != "",
		LocalModel:          a.config.GetLocalModel(),
		LocalURL:            a.config.GetLocalURL(),
		RefinementMode:      a.config.GetRefinementMode(),
		MuteSystemAudio:     a.config.GetMuteSystemAudio(),
	}
}

// SetAPIKey sets the Gemini API key
func (a *App) SetAPIKey(key string) error {
	a.config.SetGeminiAPIKey(key)
	a.geminiClient.SetAPIKey(key)
	return a.config.Save()
}

// reloadHotkeys re-initializes the hotkey manager with current config
func (a *App) reloadHotkeys() error {
	hf := a.config.GetHandsFreeHotkey()
	ptt := a.config.GetPushToTalkHotkey()

	if a.hotkeyManager != nil {
		logger.Infof("Updating hotkeys: HF=%s, PTT=%s", hf, ptt)
		return a.hotkeyManager.Update(hf, ptt)
	}
	return fmt.Errorf("hotkey manager not initialized")
}

// SetHotkey sets the global hotkey (Legacy: maps to HandsFree)
func (a *App) SetHotkey(hotkeyStr string) error {
	return a.SetHandsFreeHotkey(hotkeyStr)
}

// SetHandsFreeHotkey sets the hands-free hotkey
func (a *App) SetHandsFreeHotkey(hotkeyStr string) error {
	old := a.config.GetHandsFreeHotkey()
	a.config.SetHandsFreeHotkey(hotkeyStr)

	if err := a.reloadHotkeys(); err != nil {
		logger.Errorf("Error reloading hotkeys (HF): %v", err)
		a.config.SetHandsFreeHotkey(old) // Revert on error
		a.reloadHotkeys()                // Restore state
		return err
	}

	return a.config.Save()
}

// SetPushToTalkHotkey sets the push-to-talk hotkey
func (a *App) SetPushToTalkHotkey(hotkeyStr string) error {
	old := a.config.GetPushToTalkHotkey()
	a.config.SetPushToTalkHotkey(hotkeyStr)

	if err := a.reloadHotkeys(); err != nil {
		logger.Errorf("Error reloading hotkeys (PTT): %v", err)
		a.config.SetPushToTalkHotkey(old) // Revert on error
		a.reloadHotkeys()                 // Restore state
		return err
	}

	return a.config.Save()
}

// SetWhisperModel sets the Whisper model size
func (a *App) SetWhisperModel(model string) error {
	a.config.SetWhisperModel(model)
	err := a.config.Save()
	if err != nil {
		return err
	}

	// Check if model needs to be downloaded
	a.modelReady = false
	go a.checkModelStatus()
	return nil
}

// GetAllModels returns all available models with their download status
func (a *App) GetAllModels() ([]whisper.ModelInfo, error) {
	return a.whisperService.GetAllModels()
}

// SetGeminiModel sets the Gemini model
func (a *App) SetGeminiModel(model string) error {
	a.config.SetGeminiModel(model)
	a.geminiClient.SetModel(model)
	return a.config.Save()
}

// GetGeminiModel returns the current Gemini model
func (a *App) GetGeminiModel() string {
	return a.config.GetGeminiModel()
}

// CheckResult holds the result of a model connectivity check
type CheckResult struct {
	LatencyMs int64   `json:"latency"`
	TPS       float64 `json:"tps"`
}

// GetGeminiModels returns all available Gemini models
func (a *App) GetGeminiModels() ([]string, error) {
	return a.geminiClient.ListModels()
}

// CheckGeminiModel tests a Gemini model and returns latency and TPS
func (a *App) CheckGeminiModel(model string) (*CheckResult, error) {
	latency, tps, err := a.geminiClient.CheckModel(model)
	if err != nil {
		return nil, err
	}
	return &CheckResult{LatencyMs: latency, TPS: tps}, nil
}

// GetOpenRouterModels returns all available free OpenRouter models
func (a *App) GetOpenRouterModels() ([]string, error) {
	return a.openRouterClient.GetFreeModels()
}

// GetOpenRouterModelDescriptions returns descriptions for all OpenRouter models
func (a *App) GetOpenRouterModelDescriptions() map[string]string {
	return openrouter.ModelDescriptions
}

// CheckOpenRouterModel tests an OpenRouter model and returns latency and TPS
func (a *App) CheckOpenRouterModel(model string) (*CheckResult, error) {
	latency, tps, err := a.openRouterClient.CheckModel(model)
	if err != nil {
		return nil, err
	}
	return &CheckResult{LatencyMs: latency, TPS: tps}, nil
}

// SetOpenRouterAPIKey sets the OpenRouter API key
func (a *App) SetOpenRouterAPIKey(key string) error {
	a.config.SetOpenRouterAPIKey(key)
	a.openRouterClient.SetAPIKey(key)
	return a.config.Save()
}

// SetLLMProvider sets the LLM provider (gemini or openrouter)
func (a *App) SetLLMProvider(provider string) error {
	a.config.SetLLMProvider(provider)
	a.refiner = a.activeRefiner() // swap the active refiner immediately
	return a.config.Save()
}

// GetLLMProvider returns the current LLM provider
func (a *App) GetLLMProvider() string {
	return a.config.GetLLMProvider()
}

// SetOpenRouterModel sets the OpenRouter model
func (a *App) SetOpenRouterModel(model string) error {
	a.config.SetOpenRouterModel(model)
	return a.config.Save()
}

// GetOpenRouterModel returns the current OpenRouter model
func (a *App) GetOpenRouterModel() string {
	return a.config.GetOpenRouterModel()
}

// GetGroqModels returns all available Groq models
func (a *App) GetGroqModels() ([]string, error) {
	return a.groqClient.GetModels()
}

// GetGroqModelDescriptions returns descriptions for all Groq models
func (a *App) GetGroqModelDescriptions() map[string]string {
	return groq.ModelDescriptions
}

// CheckGroqModel tests a Groq model and returns latency and TPS
func (a *App) CheckGroqModel(model string) (*CheckResult, error) {
	latency, tps, err := a.groqClient.CheckModel(model)
	if err != nil {
		return nil, err
	}
	return &CheckResult{LatencyMs: latency, TPS: tps}, nil
}

// SetGroqAPIKey sets the Groq API key
func (a *App) SetGroqAPIKey(key string) error {
	a.config.SetGroqAPIKey(key)
	a.groqClient.SetAPIKey(key)
	a.groqClient.ClearModelsCache()
	return a.config.Save()
}

// SetGroqModel sets the Groq model
func (a *App) SetGroqModel(model string) error {
	a.config.SetGroqModel(model)
	return a.config.Save()
}

// GetGroqModel returns the current Groq model
func (a *App) GetGroqModel() string {
	return a.config.GetGroqModel()
}

// GetCerebrasModels returns all available Cerebras models
func (a *App) GetCerebrasModels() ([]string, error) {
	return a.cerebrasClient.GetModels()
}

// GetCerebrasModelDescriptions returns descriptions for all Cerebras models
func (a *App) GetCerebrasModelDescriptions() map[string]string {
	return cerebras.ModelDescriptions
}

// CheckCerebrasModel tests a Cerebras model and returns latency and TPS
func (a *App) CheckCerebrasModel(model string) (*CheckResult, error) {
	latency, tps, err := a.cerebrasClient.CheckModel(model)
	if err != nil {
		return nil, err
	}
	return &CheckResult{LatencyMs: latency, TPS: tps}, nil
}

// CheckLocalModel sends a latency probe to the configured local server.
func (a *App) CheckLocalModel(model string) (*CheckResult, error) {
	latency, tps, err := a.localClient.CheckModel(model)
	if err != nil {
		return nil, err
	}
	return &CheckResult{LatencyMs: latency, TPS: tps}, nil
}

// GetLocalURL returns the base URL of the local OpenAI-compatible server.
func (a *App) GetLocalURL() string {
	return a.config.GetLocalURL()
}

// SetLocalURL updates the server URL and immediately reinitialises the local HTTP client.
func (a *App) SetLocalURL(url string) error {
	a.config.SetLocalURL(url)
	a.localClient.SetBaseURL(url)
	a.refiner = a.activeRefiner()
	return a.config.Save()
}

// GetLocalModel returns the user-configured model name.
func (a *App) GetLocalModel() string {
	return a.config.GetLocalModel()
}

// SetLocalModel sets the model name to send to the local server.
func (a *App) SetLocalModel(model string) error {
	a.config.SetLocalModel(model)
	return a.config.Save()
}

// SetCerebrasAPIKey sets the Cerebras API key
func (a *App) SetCerebrasAPIKey(key string) error {
	a.config.SetCerebrasAPIKey(key)
	a.cerebrasClient.SetAPIKey(key)
	a.cerebrasClient.ClearModelsCache()
	return a.config.Save()
}

// SetCerebrasModel sets the Cerebras model
func (a *App) SetCerebrasModel(model string) error {
	a.config.SetCerebrasModel(model)
	return a.config.Save()
}

// GetCerebrasModel returns the current Cerebras model
func (a *App) GetCerebrasModel() string {
	return a.config.GetCerebrasModel()
}

// SetRefinementMode sets the refinement mode ("refine", "raw", "copy-only")
func (a *App) SetRefinementMode(mode string) error {
	a.config.SetRefinementMode(mode)
	return a.config.Save()
}

// SetMuteSystemAudio sets whether system audio should be muted during recording
func (a *App) SetMuteSystemAudio(val bool) error {
	a.config.SetMuteSystemAudio(val)
	return a.config.Save()
}
