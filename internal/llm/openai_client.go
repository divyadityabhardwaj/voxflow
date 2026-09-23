package llm

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"time"
)

const (
	retryMaxAttempts = 3
	retryBaseDelay   = 250 * time.Millisecond
	retryMaxDelay    = 5 * time.Second
)

func isRetryableStatus(code int) bool {
	return code == http.StatusTooManyRequests ||
		code == http.StatusBadGateway ||
		code == http.StatusServiceUnavailable
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	// OpenRouter-only: turn off hybrid-thinking so the reply is just the text.
	Reasoning *reasoningOpts `json:"reasoning,omitempty"`
}

type reasoningOpts struct {
	Enabled bool `json:"enabled"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

type modelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

type OpenAIClient struct {
	BaseURL      string
	APIKey       string
	ExtraHeaders map[string]string
	HTTPClient   *http.Client
	// DisableReasoning sends {"reasoning":{"enabled":false}}; only OpenRouter accepts it.
	DisableReasoning bool
}

func (c *OpenAIClient) reasoning() *reasoningOpts {
	if c.DisableReasoning {
		return &reasoningOpts{Enabled: false}
	}
	return nil
}

func newTunedTransport() *http.Transport {
	return &http.Transport{
		TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12},
		ForceAttemptHTTP2:   true,
		MaxIdleConnsPerHost: 4,
		MaxIdleConns:        8,
		IdleConnTimeout:     120 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}
}

func NewOpenAIClient(baseURL, apiKey string, extraHeaders map[string]string) *OpenAIClient {
	return &OpenAIClient{
		BaseURL:      baseURL,
		APIKey:       apiKey,
		ExtraHeaders: extraHeaders,
		HTTPClient: &http.Client{
			Timeout:   15 * time.Second,
			Transport: newTunedTransport(),
		},
	}
}

func DoWithRetry(hc *http.Client, newReq func() (*http.Request, error)) ([]byte, int, error) {
	delay := retryBaseDelay
	var lastStatus int
	for attempt := 0; attempt <= retryMaxAttempts; attempt++ {
		req, err := newReq()
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := hc.Do(req)
		if err != nil {
			// A timeout already cost the full client deadline; retrying would keep
			// the pipeline in "Refining" for a minute on a dead network.
			var ne net.Error
			if attempt == retryMaxAttempts || (errors.As(err, &ne) && ne.Timeout()) {
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

func (c *OpenAIClient) doPost(reqBody []byte, url string) ([]byte, int, error) {
	return DoWithRetry(c.HTTPClient, func() (*http.Request, error) {
		req, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
		if err != nil {
			return nil, err
		}
		c.applyHeaders(req)
		return req, nil
	})
}

func (c *OpenAIClient) doGet(url string) ([]byte, int, error) {
	return DoWithRetry(c.HTTPClient, func() (*http.Request, error) {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		c.applyHeaders(req)
		return req, nil
	})
}

func (c *OpenAIClient) applyHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	for k, v := range c.ExtraHeaders {
		req.Header.Set(k, v)
	}
}

func (c *OpenAIClient) RefineText(rawText, model string) (string, int, bool, error) {
	req := chatRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "system", Content: BuildSystemPrompt()},
			{Role: "user", Content: "Transcription to refine:\n<transcription>\n" + rawText + "\n</transcription>"},
		},
		Temperature: 0.3,
		MaxTokens:   RefineMaxTokens(rawText),
		Reasoning:   c.reasoning(),
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
		return rawText, tokenCount, false, nil
	}
	return refined, tokenCount, okToGo, nil
}

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
		Reasoning:   c.reasoning(),
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

	return StripCodeFences(stripThink(apiResp.Choices[0].Message.Content)), nil
}

func (c *OpenAIClient) CheckModel(model string) (int64, float64, error) {
	req := chatRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "user", Content: LatencyTestText},
		},
		Temperature: 0.3,
		Reasoning:   c.reasoning(),
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

func (c *OpenAIClient) Prewarm(model string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "HEAD", c.BaseURL, nil)
		if err != nil {
			return
		}
		c.applyHeaders(req)

		resp, err := c.HTTPClient.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}()
}
