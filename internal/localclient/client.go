package localclient

import (
	"strings"
	"voxflow/internal/llm"
)

type Client struct {
	openai *llm.OpenAIClient
}

// baseURL is server root (no trailing slash); /v1 is appended.
func NewClient(baseURL string) *Client {
	return &Client{
		openai: llm.NewOpenAIClient(strings.TrimRight(baseURL, "/")+"/v1", "", nil),
	}
}

func (c *Client) SetBaseURL(baseURL string) {
	c.openai.BaseURL = strings.TrimRight(baseURL, "/") + "/v1"
}

func (c *Client) RefineText(rawText, model string) (string, int, bool, error) {
	return c.openai.RefineText(rawText, model)
}

func (c *Client) CheckModel(model string) (int64, float64, error) {
	return c.openai.CheckModel(model)
}

func (c *Client) RetryWithInstruction(text, instruction, model string) (string, error) {
	return c.openai.RetryWithInstruction(text, instruction, model)
}

func (c *Client) Prewarm(model string) {
	c.openai.Prewarm(model)
}
