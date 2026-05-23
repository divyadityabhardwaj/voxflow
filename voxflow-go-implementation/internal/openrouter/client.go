package openrouter

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"voxflow/internal/llm"
)

const (
	baseAPIURL = "https://openrouter.ai/api/v1"
)

// ModelDescriptions contains descriptions for popular models.
var ModelDescriptions = map[string]string{
	"qwen/qwen3-235b-a22b:free":           "Qwen 3 - Excellent reasoning (262K context)",
	"deepseek/deepseek-chat-v3-0324:free": "DeepSeek V3 - Strong open model (128K context)",
	"meta-llama/llama-4-maverick:free":    "Llama 4 Maverick - Meta's latest (128K context)",
	"nvidia/nemotron-3-nano-30b-a3b:free": "Nemotron 3 Nano - NVIDIA's efficient (256K)",
	"google/gemma-3-4b-it:free":           "Gemma 3 4B - Good balance of speed/quality",
	"mistralai/mistral-7b-instruct:free":  "Mistral 7B - Reliable open model",
}

// FallbackFreeModels is used when the API call to list free models fails.
var FallbackFreeModels = []string{
	"qwen/qwen3-235b-a22b:free",
	"deepseek/deepseek-chat-v3-0324:free",
	"meta-llama/llama-4-maverick:free",
	"nvidia/nemotron-3-nano-30b-a3b:free",
	"google/gemma-3-4b-it:free",
	"mistralai/mistral-7b-instruct:free",
}

// Client handles communication with the OpenRouter API.
type Client struct {
	apiKey   string
	openai   *llm.OpenAIClient
	models   []string
	modelsMu sync.Mutex
}

// NewClient creates a new OpenRouter client.
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		openai: llm.NewOpenAIClient(baseAPIURL, apiKey, map[string]string{
			"HTTP-Referer": "https://voxflow.app",
			"X-Title":      "Voxflow",
		}),
	}
}

// SetAPIKey updates the API key and clears the cached model list, since a new
// key may have access to a different set of models.
func (c *Client) SetAPIKey(apiKey string) {
	c.apiKey = apiKey
	c.openai.APIKey = apiKey
	c.modelsMu.Lock()
	c.models = nil
	c.modelsMu.Unlock()
}

// GetFreeModels fetches the currently available free models from OpenRouter.
// Results are cached; call SetAPIKey to reset the cache. Falls back to
// FallbackFreeModels on error.
func (c *Client) GetFreeModels() ([]string, error) {
	c.modelsMu.Lock()
	if c.models != nil {
		defer c.modelsMu.Unlock()
		return c.models, nil
	}
	c.modelsMu.Unlock()

	url := fmt.Sprintf("%s/models?free=true", baseAPIURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	req.Header.Set("HTTP-Referer", "https://voxflow.app")
	req.Header.Set("X-Title", "Voxflow")

	resp, err := c.openai.HTTPClient.Do(req)
	if err != nil {
		return FallbackFreeModels, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return FallbackFreeModels, fmt.Errorf("API error (status %d)", resp.StatusCode)
	}

	var modelsResp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return FallbackFreeModels, fmt.Errorf("failed to parse models: %w", err)
	}

	var freeModels []string
	for _, model := range modelsResp.Data {
		if strings.Contains(model.ID, ":free") {
			freeModels = append(freeModels, model.ID)
		}
	}

	if len(freeModels) == 0 {
		return FallbackFreeModels, nil
	}

	c.modelsMu.Lock()
	c.models = freeModels
	c.modelsMu.Unlock()

	return freeModels, nil
}

// GetModelDescription returns a human-readable description for a model ID.
func GetModelDescription(model string) string {
	if desc, ok := ModelDescriptions[model]; ok {
		return desc
	}
	return "Free model on OpenRouter"
}

// RefineText sends rawText to the OpenRouter model for transcription cleanup.
// Delegates to the shared OpenAIClient after an API key guard.
func (c *Client) RefineText(rawText, model string) (string, int, bool, error) {
	if c.apiKey == "" {
		return "", 0, false, fmt.Errorf("API key not set")
	}
	return c.openai.RefineText(rawText, model)
}

// CheckModel runs a latency probe against the given OpenRouter model and returns
// (latencyMs, tokensPerSecond, error).
func (c *Client) CheckModel(model string) (int64, float64, error) {
	if c.apiKey == "" {
		return 0, 0, fmt.Errorf("API key not set")
	}
	return c.openai.CheckModel(model)
}

// RetryWithInstruction re-processes text with a custom instruction using OpenRouter.
func (c *Client) RetryWithInstruction(text, instruction, model string) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("API key not set")
	}
	return c.openai.RetryWithInstruction(text, instruction, model)
}
