package localclient

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
	"voxflow/internal/llm"
)

// A cold local server loads the model before answering, which can take tens of seconds.
const (
	requestTimeout = 30 * time.Second
	prewarmTimeout = 60 * time.Second
)

type Client struct {
	openai *llm.OpenAIClient
}

// baseURL is server root (no trailing slash); /v1 is appended.
func NewClient(baseURL string) *Client {
	oc := llm.NewOpenAIClient(strings.TrimRight(baseURL, "/")+"/v1", "", nil)
	oc.HTTPClient.Timeout = requestTimeout
	oc.RefineTimeout = requestTimeout
	return &Client{openai: oc}
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

// Loads the model while the user is still speaking. Fire-and-forget.
func (c *Client) Prewarm(model string) {
	if model == "" {
		return
	}
	root := strings.TrimSuffix(c.openai.BaseURL, "/v1")
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), prewarmTimeout)
		defer cancel()
		if strings.Contains(root, ":11434") || strings.Contains(root, "ollama") {
			c.keepOllamaLoaded(ctx, root, model)
		}
		_ = c.openai.WarmUp(ctx, model)
	}()
}

// Ollama unloads a model after 5 idle minutes and ignores keep_alive on its /v1 API.
func (c *Client) keepOllamaLoaded(ctx context.Context, root, model string) {
	body, err := json.Marshal(map[string]string{"model": model, "keep_alive": "30m"})
	if err != nil {
		return
	}
	req, err := http.NewRequestWithContext(ctx, "POST", root+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Transport: c.openai.HTTPClient.Transport}).Do(req)
	if err != nil {
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
}
