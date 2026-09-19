package main

import (
	"fmt"
	"slices"
	"voxflow/internal/config"
	"voxflow/internal/llm"
	"voxflow/internal/logger"
	"voxflow/internal/whisper"
)

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
	Vocabulary          string `json:"vocabulary"`
}

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
		Vocabulary:          a.config.GetVocabulary(),
	}
}

func (a *App) SetAPIKey(key string) error {
	a.config.SetGeminiAPIKey(key)
	a.geminiClient.SetAPIKey(key)
	_ = config.ClearModelCache("gemini")
	go a.ensureValidModel("gemini")
	return a.config.Save()
}

func (a *App) ensureValidModel(provider string) {
	var (
		models       []string
		err          error
		current, def string
		set          func(string)
	)
	switch provider {
	case "gemini":
		if a.config.GetGeminiAPIKey() == "" {
			return
		}
		models, err = a.GetGeminiModels()
		current, def = a.config.GetGeminiModel(), config.DefaultGeminiModel
		set = func(m string) { a.config.SetGeminiModel(m); a.geminiClient.SetModel(m) }
	case "openrouter":
		models, err = a.GetOpenRouterModels()
		current, def, set = a.config.GetOpenRouterModel(), config.DefaultOpenRouterModel, a.config.SetOpenRouterModel
	case "groq":
		if a.config.GetGroqAPIKey() == "" {
			return
		}
		models, err = a.GetGroqModels()
		current, def, set = a.config.GetGroqModel(), config.DefaultGroqModel, a.config.SetGroqModel
	case "cerebras":
		if a.config.GetCerebrasAPIKey() == "" {
			return
		}
		models, err = a.GetCerebrasModels()
		current, def, set = a.config.GetCerebrasModel(), config.DefaultCerebrasModel, a.config.SetCerebrasModel
	default:
		return
	}
	if err != nil || len(models) == 0 || slices.Contains(models, current) {
		return
	}
	pick := models[0]
	if slices.Contains(models, def) {
		pick = def
	}
	logger.Warnf("[LLM] %s no longer lists %q, switching to %q", provider, current, pick)
	set(pick)
	if err := a.config.Save(); err != nil {
		logger.Errorf("[LLM] Failed to save model switch: %v", err)
	}
}

func (a *App) reloadHotkeys() error {
	hf := a.config.GetHandsFreeHotkey()
	ptt := a.config.GetPushToTalkHotkey()

	if a.hotkeyManager != nil {
		logger.Infof("Updating hotkeys: HF=%s, PTT=%s", hf, ptt)
		return a.hotkeyManager.Update(hf, ptt)
	}
	return fmt.Errorf("hotkey manager not initialized")
}

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

func (a *App) SetWhisperModel(model string) error {
	a.config.SetWhisperModel(model)
	err := a.config.Save()
	if err != nil {
		return err
	}

	a.modelReady.Store(false)
	go a.checkModelStatus()
	return nil
}

func (a *App) GetAllModels() ([]whisper.ModelInfo, error) {
	return a.whisperService.GetAllModels()
}

func (a *App) SetGeminiModel(model string) error {
	a.config.SetGeminiModel(model)
	a.geminiClient.SetModel(model)
	return a.config.Save()
}

type CheckResult struct {
	LatencyMs int64   `json:"latency"`
	TPS       float64 `json:"tps"`
}

func (a *App) GetGeminiModels() ([]string, error) {
	if cached, ok := config.LoadModelCache("gemini"); ok {
		return cached, nil
	}
	models, err := a.geminiClient.ListModels()
	if err == nil && len(models) > 0 {
		_ = config.SaveModelCache("gemini", models)
	}
	return models, err
}

func (a *App) CheckGeminiModel(model string) (*CheckResult, error) {
	latency, tps, err := a.geminiClient.CheckModel(model)
	if err != nil {
		return nil, err
	}
	return &CheckResult{LatencyMs: latency, TPS: tps}, nil
}

func (a *App) GetOpenRouterModels() ([]string, error) {
	if cached, ok := config.LoadModelCache("openrouter"); ok {
		return cached, nil
	}
	models, err := a.openRouterClient.GetFreeModels()
	if err == nil && len(models) > 0 {
		_ = config.SaveModelCache("openrouter", models)
	}
	return models, err
}

func (a *App) CheckOpenRouterModel(model string) (*CheckResult, error) {
	latency, tps, err := a.openRouterClient.CheckModel(model)
	if err != nil {
		return nil, err
	}
	return &CheckResult{LatencyMs: latency, TPS: tps}, nil
}

func (a *App) SetOpenRouterAPIKey(key string) error {
	a.config.SetOpenRouterAPIKey(key)
	a.openRouterClient.SetAPIKey(key)
	_ = config.ClearModelCache("openrouter")
	go a.ensureValidModel("openrouter")
	return a.config.Save()
}

func (a *App) SetLLMProvider(provider string) error {
	a.config.SetLLMProvider(provider)
	go a.ensureValidModel(provider)
	return a.config.Save()
}

func (a *App) SetOpenRouterModel(model string) error {
	a.config.SetOpenRouterModel(model)
	return a.config.Save()
}

func (a *App) GetGroqModels() ([]string, error) {
	if cached, ok := config.LoadModelCache("groq"); ok {
		return cached, nil
	}
	models, err := a.groqClient.GetModels()
	if err == nil && len(models) > 0 {
		_ = config.SaveModelCache("groq", models)
	}
	return models, err
}

func (a *App) CheckGroqModel(model string) (*CheckResult, error) {
	latency, tps, err := a.groqClient.CheckModel(model)
	if err != nil {
		return nil, err
	}
	return &CheckResult{LatencyMs: latency, TPS: tps}, nil
}

func (a *App) SetGroqAPIKey(key string) error {
	a.config.SetGroqAPIKey(key)
	a.groqClient.SetAPIKey(key)
	a.groqClient.ClearModelsCache()
	_ = config.ClearModelCache("groq")
	go a.ensureValidModel("groq")
	return a.config.Save()
}

func (a *App) SetGroqModel(model string) error {
	a.config.SetGroqModel(model)
	return a.config.Save()
}

func (a *App) GetCerebrasModels() ([]string, error) {
	if cached, ok := config.LoadModelCache("cerebras"); ok {
		return cached, nil
	}
	models, err := a.cerebrasClient.GetModels()
	if err == nil && len(models) > 0 {
		_ = config.SaveModelCache("cerebras", models)
	}
	return models, err
}

func (a *App) CheckCerebrasModel(model string) (*CheckResult, error) {
	latency, tps, err := a.cerebrasClient.CheckModel(model)
	if err != nil {
		return nil, err
	}
	return &CheckResult{LatencyMs: latency, TPS: tps}, nil
}

func (a *App) CheckLocalModel(model string) (*CheckResult, error) {
	latency, tps, err := a.localClient.CheckModel(model)
	if err != nil {
		return nil, err
	}
	return &CheckResult{LatencyMs: latency, TPS: tps}, nil
}

// Also reinitialises the local HTTP client.
func (a *App) SetLocalURL(url string) error {
	a.config.SetLocalURL(url)
	a.localClient.SetBaseURL(url)
	return a.config.Save()
}

func (a *App) SetLocalModel(model string) error {
	a.config.SetLocalModel(model)
	return a.config.Save()
}

func (a *App) SetCerebrasAPIKey(key string) error {
	a.config.SetCerebrasAPIKey(key)
	a.cerebrasClient.SetAPIKey(key)
	a.cerebrasClient.ClearModelsCache()
	_ = config.ClearModelCache("cerebras")
	go a.ensureValidModel("cerebras")
	return a.config.Save()
}

func (a *App) SetCerebrasModel(model string) error {
	a.config.SetCerebrasModel(model)
	return a.config.Save()
}

// mode: "refine", "raw", or "copy-only".
func (a *App) SetRefinementMode(mode string) error {
	a.config.SetRefinementMode(mode)
	return a.config.Save()
}

// Pushes custom terms to Whisper and the LLM prompt.
func (a *App) SetVocabulary(v string) error {
	a.config.SetVocabulary(v)
	a.whisperService.SetPrompt(v)
	llm.SetVocabulary(v)
	return a.config.Save()
}

// lang "auto" = detection.
func (a *App) SetWhisperLanguage(lang string) error {
	a.config.SetWhisperLanguage(lang)
	a.whisperService.SetLanguage(lang)
	return a.config.Save()
}

func (a *App) SetMuteSystemAudio(val bool) error {
	a.config.SetMuteSystemAudio(val)
	return a.config.Save()
}
