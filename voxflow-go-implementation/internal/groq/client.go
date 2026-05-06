package groq

import (
	"fmt"
	"strings"
	"sync"
	"voxflow/internal/llm"
)

const (
	baseAPIURL = "https://api.groq.com/openai/v1"
)

// ModelDescriptions contains descriptions for popular Groq models.
var ModelDescriptions = map[string]string{
	"llama-3.1-8b-instant": "Llama 3.1 8B (Fastest)",
	"llama3-70b-8192":      "Llama 3 70B (High Quality)",
	"mixtral-8x7b-32768":   "Mixtral 8x7B (Balanced)",
	"gemma2-9b-it":         "Gemma 2 9B (Good Reasoning)",
}

// AvailableModels is a static fallback list used when the API is unreachable.
var AvailableModels = []string{
	"llama-3.1-8b-instant",
	"llama3-70b-8192",
	"mixtral-8x7b-32768",
	"gemma2-9b-it",
}

// Client handles communication with the Groq API.
type Client struct {
	apiKey   string
	openai   *llm.OpenAIClient
	models   []string
	modelsMu sync.Mutex
}

// NewClient creates a new Groq client.
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		openai: llm.NewOpenAIClient(baseAPIURL, apiKey, nil),
	}
}

// SetAPIKey updates the API key on both the wrapper and the shared HTTP client.
func (c *Client) SetAPIKey(apiKey string) {
	c.apiKey = apiKey
	c.openai.APIKey = apiKey
}

// ClearModelsCache clears the cached model list (call after an API key change).
func (c *Client) ClearModelsCache() {
	c.modelsMu.Lock()
	c.models = nil
	c.modelsMu.Unlock()
}

// GetModelDescription returns a human-readable description for a model ID.
func GetModelDescription(model string) string {
	if desc, ok := ModelDescriptions[model]; ok {
		return desc
	}
	return "Groq Model"
}

// GetModels fetches available chat/language models from the Groq API.
// Results are cached after the first successful fetch. Falls back to
// AvailableModels on error or when the API key is not set.
func (c *Client) GetModels() ([]string, error) {
	if c.apiKey == "" {
		return AvailableModels, fmt.Errorf("API key not set")
	}

	c.modelsMu.Lock()
	if c.models != nil {
		defer c.modelsMu.Unlock()
		return c.models, nil
	}
	c.modelsMu.Unlock()

	models, err := c.openai.GetModels(func(id string) bool {
		return !strings.Contains(id, "whisper") &&
			!strings.Contains(id, "tts") &&
			!strings.Contains(id, "embedding") &&
			!strings.Contains(id, "guard") &&
			!strings.Contains(id, "tool-use")
	})
	if err != nil {
		return AvailableModels, err
	}

	if len(models) == 0 {
		return AvailableModels, nil
	}

	c.modelsMu.Lock()
	c.models = models
	c.modelsMu.Unlock()

	return models, nil
}

// RefineText sends rawText to the Groq model for transcription cleanup.
// Delegates to the shared OpenAIClient after an API key guard.
func (c *Client) RefineText(rawText, model string) (string, int, bool, error) {
	if c.apiKey == "" {
		return "", 0, false, fmt.Errorf("API key not set")
	}
	return c.openai.RefineText(rawText, model)
}

// CheckModel runs a latency probe against the given Groq model and returns
// (latencyMs, tokensPerSecond, error).
func (c *Client) CheckModel(model string) (int64, float64, error) {
	if c.apiKey == "" {
		return 0, 0, fmt.Errorf("API key not set")
	}
	return c.openai.CheckModel(model)
}
