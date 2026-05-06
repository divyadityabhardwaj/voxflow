package localclient

import (
	"strings"
	"voxflow/internal/llm"
)

// Client handles communication with a local OpenAI-compatible server
// (e.g. Ollama, llama.cpp, LM Studio).
type Client struct {
	openai *llm.OpenAIClient
}

// NewClient creates a new local model client.
// baseURL should be the server root without a trailing slash or path suffix
// (e.g. "http://localhost:11434"). The /v1 prefix is appended automatically.
func NewClient(baseURL string) *Client {
	return &Client{
		openai: llm.NewOpenAIClient(strings.TrimRight(baseURL, "/")+"/v1", "", nil),
	}
}

// SetBaseURL updates the base URL of the local server at runtime.
// The /v1 prefix is appended automatically; any trailing slash is stripped.
func (c *Client) SetBaseURL(baseURL string) {
	c.openai.BaseURL = strings.TrimRight(baseURL, "/") + "/v1"
}

// RefineText sends rawText to the local model for transcription cleanup.
// No API key is required for local servers.
func (c *Client) RefineText(rawText, model string) (string, int, bool, error) {
	return c.openai.RefineText(rawText, model)
}

// CheckModel runs a latency probe against the local model and returns
// (latencyMs, tokensPerSecond, error).
func (c *Client) CheckModel(model string) (int64, float64, error) {
	return c.openai.CheckModel(model)
}
