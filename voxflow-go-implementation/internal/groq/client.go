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

var AvailableModels = []string{
	"openai/gpt-oss-20b",
	"openai/gpt-oss-120b",
}

type Client struct {
	apiKey   string
	openai   *llm.OpenAIClient
	models   []string
	modelsMu sync.Mutex
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		openai: llm.NewOpenAIClient(baseAPIURL, apiKey, nil),
	}
}

func (c *Client) SetAPIKey(apiKey string) {
	c.apiKey = apiKey
	c.openai.APIKey = apiKey
}

func (c *Client) ClearModelsCache() {
	c.modelsMu.Lock()
	c.models = nil
	c.modelsMu.Unlock()
}

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
