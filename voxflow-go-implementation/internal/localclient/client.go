package localclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"voxflow/internal/llm"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

type request struct {
	Model       string    `json:"model"`
	Messages    []message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type response struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
	Usage usage `json:"usage"`
}

type usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type refineResponse struct {
	Text    string `json:"text"`
	Refused bool   `json:"refused"`
}

func (c *Client) RefineText(rawText string, model string, mode string) (string, int, error) {
	systemPrompt := llm.BuildSystemPrompt(mode)
	req := request{
		Model: model,
		Messages: []message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: "Transcription to refine:\n" + rawText},
		},
		Temperature: 0.3,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return "", 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1/chat/completions", strings.TrimRight(c.baseURL, "/"))
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return "", 0, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", 0, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("local model error (status %d): %s", resp.StatusCode, string(body))
	}

	var localResp response
	if err := json.Unmarshal(body, &localResp); err != nil {
		return "", 0, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(localResp.Choices) == 0 {
		return "", 0, fmt.Errorf("no response generated")
	}

	result := localResp.Choices[0].Message.Content
	tokenCount := localResp.Usage.CompletionTokens

	// Debug logging
	fmt.Printf("[LocalClient] Raw output (%d chars), Tokens: %d:\n%s\n", len(result), tokenCount, result)

	cleanResult := strings.TrimSpace(result)
	if strings.HasPrefix(cleanResult, "```json") {
		cleanResult = strings.TrimPrefix(cleanResult, "```json")
		cleanResult = strings.TrimSuffix(strings.TrimSpace(cleanResult), "```")
		cleanResult = strings.TrimSpace(cleanResult)
	} else if strings.HasPrefix(cleanResult, "```") {
		cleanResult = strings.TrimPrefix(cleanResult, "```")
		cleanResult = strings.TrimSuffix(strings.TrimSpace(cleanResult), "```")
		cleanResult = strings.TrimSpace(cleanResult)
	}

	var refineResp refineResponse
	if err := json.Unmarshal([]byte(cleanResult), &refineResp); err == nil {
		if refineResp.Refused {
			return rawText, tokenCount, nil
		}
		return refineResp.Text, tokenCount, nil
	}

	return cleanResult, tokenCount, nil
}

// CheckModel tests a model and returns latency in milliseconds and tokens per second
func (c *Client) CheckModel(model string) (int64, float64, error) {
	req := request{
		Model: model,
		Messages: []message{
			{Role: "user", Content: llm.LatencyTestText},
		},
		Temperature: 0.3,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1/chat/completions", strings.TrimRight(c.baseURL, "/"))

	startTime := time.Now()

	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return 0, 0, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	latency := time.Since(startTime).Milliseconds()

	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("API error (status %d)", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to read response: %w", err)
	}

	var localResp response
	if err := json.Unmarshal(respBody, &localResp); err != nil {
		return latency, 0, nil // Return latency even if TPS fails
	}

	tokenCount := localResp.Usage.CompletionTokens
	var tps float64 = 0
	if latency > 0 && tokenCount > 0 {
		tps = float64(tokenCount) / (float64(latency) / 1000.0)
	}

	return latency, tps, nil
}
