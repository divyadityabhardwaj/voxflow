package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"
)

const (
	retryMaxAttempts = 3
	retryBaseDelay   = time.Second
	retryMaxDelay    = 30 * time.Second
)

func isRetryableStatus(code int) bool {
	return code == http.StatusTooManyRequests ||
		code == http.StatusBadGateway ||
		code == http.StatusServiceUnavailable
}

// chatRequest is the OpenAI-compatible /chat/completions request body.
type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
}

// chatMessage is a single message in a chat conversation.
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatResponse is the OpenAI-compatible /chat/completions response.
type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// modelsResponse is the OpenAI-compatible /models response.
type modelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// OpenAIClient implements the common OpenAI /chat/completions HTTP protocol.
// Any provider that exposes an OpenAI-compatible API can embed or reference
// this client to avoid duplicating HTTP boilerplate.
type OpenAIClient struct {
	BaseURL      string
	APIKey       string
	ExtraHeaders map[string]string
	HTTPClient   *http.Client
}

// NewOpenAIClient creates a new OpenAIClient with a 60-second HTTP timeout.
func NewOpenAIClient(baseURL, apiKey string, extraHeaders map[string]string) *OpenAIClient {
	return &OpenAIClient{
		BaseURL:      baseURL,
		APIKey:       apiKey,
		ExtraHeaders: extraHeaders,
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// doPost sends reqBody to url with retry/backoff on transient failures (429/502/503
// and network errors). Returns the raw response body on success.
func (c *OpenAIClient) doPost(reqBody []byte, url string) ([]byte, int, error) {
	delay := retryBaseDelay
	var lastStatus int
	for attempt := 0; attempt <= retryMaxAttempts; attempt++ {
		httpReq, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}
		c.applyHeaders(httpReq)

		resp, err := c.HTTPClient.Do(httpReq)
		if err != nil {
			if attempt == retryMaxAttempts {
				return nil, 0, fmt.Errorf("failed to send request: %w", err)
			}
			time.Sleep(delay)
			delay = min(delay*2, retryMaxDelay)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, resp.StatusCode, fmt.Errorf("failed to read response: %w", err)
		}

		lastStatus = resp.StatusCode
		if !isRetryableStatus(resp.StatusCode) || attempt == retryMaxAttempts {
			return body, lastStatus, nil
		}

		time.Sleep(delay)
		delay = min(delay*2, retryMaxDelay)
	}
	return nil, lastStatus, nil
}

// doGet sends an HTTP GET to url with the same retry/backoff strategy as doPost.
func (c *OpenAIClient) doGet(url string) ([]byte, int, error) {
	delay := retryBaseDelay
	var lastStatus int
	for attempt := 0; attempt <= retryMaxAttempts; attempt++ {
		httpReq, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}
		c.applyHeaders(httpReq)

		resp, err := c.HTTPClient.Do(httpReq)
		if err != nil {
			if attempt == retryMaxAttempts {
				return nil, 0, fmt.Errorf("failed to send request: %w", err)
			}
			time.Sleep(delay)
			delay = min(delay*2, retryMaxDelay)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, resp.StatusCode, fmt.Errorf("failed to read response: %w", err)
		}

		lastStatus = resp.StatusCode
		if !isRetryableStatus(resp.StatusCode) || attempt == retryMaxAttempts {
			return body, lastStatus, nil
		}

		time.Sleep(delay)
		delay = min(delay*2, retryMaxDelay)
	}
	return nil, lastStatus, nil
}

// applyHeaders sets Content-Type, Authorization (when APIKey is set), and any
// extra provider-specific headers on the given request.
func (c *OpenAIClient) applyHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	for k, v := range c.ExtraHeaders {
		req.Header.Set(k, v)
	}
}

// RefineText sends rawText to the model for transcription cleanup.
// Returns (polishedText, completionTokenCount, okToGo, error).
// okToGo == true means the LLM signalled the input was already clean.
func (c *OpenAIClient) RefineText(rawText, model string) (string, int, bool, error) {
	req := chatRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "system", Content: BuildSystemPrompt()},
			{Role: "user", Content: "Transcription to refine:\n" + rawText},
		},
		Temperature: 0.3,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", 0, false, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/chat/completions", c.BaseURL)
	respBody, statusCode, err := c.doPost(reqBody, url)
	if err != nil {
		return "", 0, false, err
	}

	if statusCode != http.StatusOK {
		return "", 0, false, fmt.Errorf("API error (status %d): %s", statusCode, string(respBody))
	}

	var apiResp chatResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return "", 0, false, fmt.Errorf("failed to parse response: %w, response: %s", err, string(respBody))
	}

	if len(apiResp.Choices) == 0 {
		return "", 0, false, fmt.Errorf("no response generated")
	}

	result := apiResp.Choices[0].Message.Content
	tokenCount := apiResp.Usage.CompletionTokens

	refined, okToGo, parsed := ParseRefineResponse(result, rawText)
	if !parsed {
		return StripCodeFences(result), tokenCount, false, nil
	}
	return refined, tokenCount, okToGo, nil
}

// RetryWithInstruction re-processes text with a custom instruction.
func (c *OpenAIClient) RetryWithInstruction(text, instruction, model string) (string, error) {
	prompt := fmt.Sprintf(`Apply the following instruction to the text:
Instruction: %s

Text:
%s

Return ONLY the modified text, nothing else.`, instruction, text)

	req := chatRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "user", Content: prompt},
		},
		Temperature: 0.3,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/chat/completions", c.BaseURL)
	respBody, statusCode, err := c.doPost(reqBody, url)
	if err != nil {
		return "", err
	}

	if statusCode != http.StatusOK {
		return "", fmt.Errorf("API error (status %d): %s", statusCode, string(respBody))
	}

	var apiResp chatResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w, response: %s", err, string(respBody))
	}

	if len(apiResp.Choices) == 0 {
		return "", fmt.Errorf("no response generated")
	}

	return StripCodeFences(apiResp.Choices[0].Message.Content), nil
}

// CheckModel runs a latency probe against the provider and returns
// (latencyMs, tokensPerSecond, error).
func (c *OpenAIClient) CheckModel(model string) (int64, float64, error) {
	req := chatRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "user", Content: LatencyTestText},
		},
		Temperature: 0.3,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/chat/completions", c.BaseURL)
	startTime := time.Now()

	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return 0, 0, fmt.Errorf("failed to create request: %w", err)
	}
	c.applyHeaders(httpReq)

	resp, err := c.HTTPClient.Do(httpReq)
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

	var apiResp chatResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return latency, 0, nil // return latency even if TPS calculation fails
	}

	tokenCount := apiResp.Usage.CompletionTokens
	var tps float64
	if latency > 0 && tokenCount > 0 {
		tps = float64(tokenCount) / (float64(latency) / 1000.0)
	}

	return latency, tps, nil
}

// GetModels fetches the model list from {BaseURL}/models.
// The filter func (if non-nil) is called for each model ID; returning false
// excludes that model from the result. The returned slice is sorted
// alphabetically.
func (c *OpenAIClient) GetModels(filter func(id string) bool) ([]string, error) {
	url := fmt.Sprintf("%s/models", c.BaseURL)

	respBody, statusCode, err := c.doGet(url)
	if err != nil {
		return nil, err
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", statusCode, string(respBody))
	}

	var modResp modelsResponse
	if err := json.Unmarshal(respBody, &modResp); err != nil {
		return nil, fmt.Errorf("failed to parse models: %w", err)
	}

	var models []string
	for _, m := range modResp.Data {
		if filter == nil || filter(m.ID) {
			models = append(models, m.ID)
		}
	}

	sort.Strings(models)
	return models, nil
}
