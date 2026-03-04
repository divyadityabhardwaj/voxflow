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
}

type refineResponse struct {
	Text    string `json:"text"`
	Refused bool   `json:"refused"`
}

func (c *Client) RefineText(rawText string, model string, mode string) (string, error) {
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
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1/chat/completions", strings.TrimRight(c.baseURL, "/"))
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("local model error (status %d): %s", resp.StatusCode, string(body))
	}

	var localResp response
	if err := json.Unmarshal(body, &localResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(localResp.Choices) == 0 {
		return "", fmt.Errorf("no response generated")
	}

	result := localResp.Choices[0].Message.Content

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
			return rawText, nil
		}
		return refineResp.Text, nil
	}

	return cleanResult, nil
}
