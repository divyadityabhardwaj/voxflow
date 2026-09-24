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
	InputDevice         string `json:"input_device"`
}

func (a *App) GetConfig() *ConfigResponse {
	return &ConfigResponse{
		Hotkey:              a.config.GetHotkey(),
		HandsFreeHotkey:     a.config.GetHandsFreeHotkey(),
		PushToTalkHotkey:    a.config.GetPushToTalkHotkey(),
		WhisperModel:        a.config.GetWhisperModel(),
		WhisperLanguage:     a.config.GetWhisperLanguage(),
		WhisperThreads:      a.config.GetWhisperThreads(),
		GeminiModel:         a.config.GetModel("gemini"),
		APIKeySet:           a.config.GetAPIKey("gemini") != "",
		LLMProvider:         a.config.GetLLMProvider(),
		OpenRouterModel:     a.config.GetModel("openrouter"),
		OpenRouterAPIKeySet: a.config.GetAPIKey("openrouter") != "",
		GroqModel:           a.config.GetModel("groq"),
		GroqAPIKeySet:       a.config.GetAPIKey("groq") != "",
		CerebrasModel:       a.config.GetModel("cerebras"),
		CerebrasAPIKeySet:   a.config.GetAPIKey("cerebras") != "",
		LocalModel:          a.config.GetModel("local"),
		LocalURL:            a.config.GetLocalURL(),
		RefinementMode:      a.config.GetRefinementMode(),
		MuteSystemAudio:     a.config.GetMuteSystemAudio(),
		Vocabulary:          a.config.GetVocabulary(),
		InputDevice:         a.config.GetInputDevice(),
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

// SuspendHotkeys releases the global shortcuts while the frontend records a new one.
func (a *App) SuspendHotkeys(suspend bool) error {
	return a.hotkeyManager.Suspend(suspend)
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

type ProviderInfo struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	NeedsKey     bool   `json:"needs_key"`
	KeySet       bool   `json:"key_set"`
	Model        string `json:"model"`
	DefaultModel string `json:"default_model"`
	Local        bool   `json:"local"`
}

func (a *App) GetProviders() []ProviderInfo {
	out := make([]ProviderInfo, 0, len(llm.Providers))
	for _, p := range llm.Providers {
		out = append(out, ProviderInfo{
			ID:           p.ID,
			Name:         p.Name,
			NeedsKey:     p.NeedsKey,
			KeySet:       a.config.GetAPIKey(p.ID) != "",
			Model:        a.config.GetModel(p.ID),
			DefaultModel: p.DefaultModel,
			Local:        p.Local,
		})
	}
	return out
}

func knownProvider(id string) error {
	if llm.ProviderByID(id).ID != id {
		return fmt.Errorf("unknown provider %q", id)
	}
	return nil
}

func (a *App) SetProvider(id string) error {
	if err := knownProvider(id); err != nil {
		return err
	}
	a.config.SetLLMProvider(id)
	go a.ensureValidModel(id)
	return a.config.Save()
}

func (a *App) SetProviderAPIKey(id, key string) error {
	if err := knownProvider(id); err != nil {
		return err
	}
	a.config.SetAPIKey(id, key)
	a.llmClient(id).APIKey = a.config.GetAPIKey(id)
	_ = config.ClearModelCache(id)
	go a.ensureValidModel(id)
	return a.config.Save()
}

func (a *App) SetProviderModel(id, model string) error {
	if err := knownProvider(id); err != nil {
		return err
	}
	a.config.SetModel(id, model)
	return a.config.Save()
}

type CheckResult struct {
	LatencyMs int64   `json:"latency"`
	TPS       float64 `json:"tps"`
}

func (a *App) GetProviderModels(id string) ([]string, error) {
	if err := knownProvider(id); err != nil {
		return nil, err
	}
	if cached, ok := config.LoadModelCache(id); ok {
		return cached, nil
	}
	models, err := a.llmClient(id).GetModels()
	if err == nil && len(models) > 0 {
		_ = config.SaveModelCache(id, models)
	}
	return models, err
}

func (a *App) CheckProviderModel(id, model string) (*CheckResult, error) {
	if err := knownProvider(id); err != nil {
		return nil, err
	}
	latency, tps, err := a.llmClient(id).CheckModel(model)
	if err != nil {
		return nil, err
	}
	return &CheckResult{LatencyMs: latency, TPS: tps}, nil
}

// Swaps out a model the provider no longer lists.
func (a *App) ensureValidModel(provider string) {
	p := llm.ProviderByID(provider)
	if p.Local || (!p.OpenModels && a.config.GetAPIKey(p.ID) == "") {
		return
	}
	models, err := a.GetProviderModels(p.ID)
	current := a.config.GetModel(p.ID)
	if err != nil || len(models) == 0 || slices.Contains(models, current) {
		return
	}
	pick := models[0]
	if slices.Contains(models, p.DefaultModel) {
		pick = p.DefaultModel
	}
	logger.Warnf("[LLM] %s no longer lists %q, switching to %q", p.ID, current, pick)
	a.config.SetModel(p.ID, pick)
	if err := a.config.Save(); err != nil {
		logger.Errorf("[LLM] Failed to save model switch: %v", err)
	}
}

func (a *App) SetLocalURL(url string) error {
	a.config.SetLocalURL(url)
	a.llmClient("local").SetServerURL(url)
	_ = config.ClearModelCache("local")
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

// mode: "refine", "raw", or "copy-only".
func (a *App) SetRefinementMode(mode string) error {
	a.config.SetRefinementMode(mode)
	return a.config.Save()
}
