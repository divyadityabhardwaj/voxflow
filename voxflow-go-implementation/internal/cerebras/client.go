package cerebras

import (
	"fmt"
	"strings"
	"sync"
	"voxflow/internal/llm"
)

const (
	baseAPIURL = "https://api.cerebras.ai/v1"
)

// ModelDescriptions contains descriptions for popular Cerebras models.
var ModelDescriptions = map[string]string{
	"llama3.1-8b":  "Llama 3.1 8B (Fastest)",
	"llama3.1-70b": "Llama 3.1 70B (High Quality)",
}

// AvailableModels is a static fallback list used when the API is unreachable.
var AvailableModels = []string{
	"llama3.1-8b",
	"llama3.1-70b",
}

// Client handles communication with the Cerebras API.
type Client struct {
	apiKey   string
	openai   *llm.OpenAIClient
	models   []string
	modelsMu sync.Mutex
}

// NewClient creates a new Cerebras client.
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
	return "Cerebras Model"
}

// GetModels fetches available chat/language models from the Cerebras API.
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
		return !strings.Contains(id, "embedding") &&
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

// RefineText sends rawText to the Cerebras model for transcription cleanup.
// Delegates to the shared OpenAIClient after an API key guard.
func (c *Client) RefineText(rawText, model string) (string, int, bool, error) {
	if c.apiKey == "" {
		return "", 0, false, fmt.Errorf("API key not set")
	}
	return c.openai.RefineText(rawText, model)
}

// CheckModel runs a latency probe against the given Cerebras model and returns
// (latencyMs, tokensPerSecond, error).
func (c *Client) CheckModel(model string) (int64, float64, error) {
	if c.apiKey == "" {
		return 0, 0, fmt.Errorf("API key not set")
	}
	return c.openai.CheckModel(model)
}

// RetryWithInstruction re-processes text with a custom instruction using Cerebras.
func (c *Client) RetryWithInstruction(text, instruction, model string) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("API key not set")
	}
	return c.openai.RetryWithInstruction(text, instruction, model)
}

// Prewarm initiates background connection pre-warming.
func (c *Client) Prewarm(model string) {
	if c.apiKey != "" {
		c.openai.Prewarm(model)
	}
}
