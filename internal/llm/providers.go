package llm

import (
	"strings"
	"voxflow/internal/config"
)

type Provider struct {
	ID, Name     string
	BaseURL      string // empty for local: the user's server URL + /v1
	DefaultModel string
	NeedsKey     bool
	Local        bool // user-run server: slow cold starts, gets warmed up
	Headers      map[string]string
	// ModelsQuery is appended to /models; OpenModels lists without a key.
	ModelsQuery    string
	OpenModels     bool
	ModelPrefix    string // stripped from listed ids
	KeepModel      func(id string) bool
	FallbackModels []string
	// Sends {"reasoning":{"enabled":false}}; only OpenRouter accepts it.
	DisableReasoning bool
}

func excludes(subs ...string) func(string) bool {
	return func(id string) bool {
		for _, s := range subs {
			if strings.Contains(id, s) {
				return false
			}
		}
		return true
	}
}

var Providers = []Provider{
	{
		ID: "gemini", Name: "Gemini",
		BaseURL:      "https://generativelanguage.googleapis.com/v1beta/openai",
		DefaultModel: config.DefaultGeminiModel,
		NeedsKey:     true,
		ModelPrefix:  "models/",
		KeepModel: func(id string) bool {
			// Only text chat models: the list also carries TTS, image, live-audio and embedding models.
			return strings.HasPrefix(id, "gemini") && excludes("tts", "image", "embedding", "live", "audio",
				"transcribe", "computer-use", "robotics", "customtools", "translate", "thinking")(id)
		},
	},
	{
		ID: "openrouter", Name: "OpenRouter",
		BaseURL:      "https://openrouter.ai/api/v1",
		DefaultModel: config.DefaultOpenRouterModel,
		NeedsKey:     true,
		// OpenRouter's app attribution.
		Headers: map[string]string{
			"HTTP-Referer": "https://github.com/divyadityabhardwaj/voxflow",
			"X-Title":      "VoxFlow",
		},
		ModelsQuery: "?free=true",
		OpenModels:  true,
		// Reasoning-first models spend the whole output budget thinking; a dictation
		// cleanup call needs a plain instruct model.
		KeepModel: func(id string) bool {
			return strings.HasSuffix(id, ":free") && excludes("reasoning", "thinking")(id)
		},
		FallbackModels: []string{
			"google/gemma-4-31b-it:free",
			"google/gemma-4-26b-a4b-it:free",
			"nvidia/nemotron-3-super-120b-a12b:free",
			"z-ai/glm-5.2:free",
		},
		DisableReasoning: true,
	},
	{
		ID: "groq", Name: "Groq",
		BaseURL:        "https://api.groq.com/openai/v1",
		DefaultModel:   config.DefaultGroqModel,
		NeedsKey:       true,
		KeepModel:      excludes("whisper", "tts", "embedding", "guard", "tool-use"),
		FallbackModels: []string{"openai/gpt-oss-20b", "openai/gpt-oss-120b"},
	},
	{
		ID: "cerebras", Name: "Cerebras",
		BaseURL:        "https://api.cerebras.ai/v1",
		DefaultModel:   config.DefaultCerebrasModel,
		NeedsKey:       true,
		KeepModel:      excludes("embedding", "tool-use"),
		FallbackModels: []string{"llama3.1-8b", "llama3.1-70b"},
	},
	{ID: "local", Name: "Local", Local: true},
}

// Unknown ids resolve to Gemini, the default provider.
func ProviderByID(id string) Provider {
	for _, p := range Providers {
		if p.ID == id {
			return p
		}
	}
	return Providers[0]
}
