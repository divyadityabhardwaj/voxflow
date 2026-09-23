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

// OpenRouter's app attribution.
var attributionHeaders = map[string]string{
	"HTTP-Referer": "https://github.com/divyadityabhardwaj/voxflow",
	"X-Title":      "VoxFlow",
}

var FallbackFreeModels = []string{
	"google/gemma-4-31b-it:free",
	"google/gemma-4-26b-a4b-it:free",
	"nvidia/nemotron-3-super-120b-a12b:free",
	"z-ai/glm-5.2:free",
}

type Client struct {
	apiKey   string
	openai   *llm.OpenAIClient
	models   []string
	modelsMu sync.Mutex
}

func NewClient(apiKey string) *Client {
	c := &Client{
		apiKey: apiKey,
		openai: llm.NewOpenAIClient(baseAPIURL, apiKey, attributionHeaders),
	}
	c.openai.DisableReasoning = true
	return c
}

// New key may list different models; clears cache.
func (c *Client) SetAPIKey(apiKey string) {
	c.apiKey = apiKey
	c.openai.APIKey = apiKey
	c.modelsMu.Lock()
	c.models = nil
	c.modelsMu.Unlock()
}

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
	for k, v := range attributionHeaders {
		req.Header.Set(k, v)
	}

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
		// Reasoning-first models spend the whole output budget thinking; a dictation
		// cleanup call needs a plain instruct model.
		if strings.HasSuffix(model.ID, ":free") &&
			!strings.Contains(model.ID, "reasoning") &&
			!strings.Contains(model.ID, "thinking") {
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

func (c *Client) RefineText(rawText, model string) (string, int, bool, error) {
	if c.apiKey == "" {
		return "", 0, false, fmt.Errorf("API key not set")
	}
	return c.openai.RefineText(rawText, model)
}

func (c *Client) CheckModel(model string) (int64, float64, error) {
	if c.apiKey == "" {
		return 0, 0, fmt.Errorf("API key not set")
	}
	return c.openai.CheckModel(model)
}

func (c *Client) RetryWithInstruction(text, instruction, model string) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("API key not set")
	}
	return c.openai.RetryWithInstruction(text, instruction, model)
}

func (c *Client) Prewarm(model string) {
	if c.apiKey != "" {
		c.openai.Prewarm(model)
	}
}
