package gemini

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
	"voxflow/internal/llm"
	"voxflow/internal/logger"
)

const (
	baseAPIURL = "https://generativelanguage.googleapis.com/v1beta"
)

// Client handles communication with the Gemini API
type Client struct {
	apiKey     string
	modelName  string
	httpClient *http.Client
	models     []string
	modelsMu   sync.Mutex
}

// newTunedTransport returns an http.Transport optimised for low-latency API
// calls.
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

// NewClient creates a new Gemini client
func NewClient(apiKey string, modelName string) *Client {
	if modelName == "" {
		modelName = "gemini-2.0-flash-lite"
	}
	return &Client{
		apiKey:    apiKey,
		modelName: modelName,
		httpClient: &http.Client{
			Timeout:   15 * time.Second,
			Transport: newTunedTransport(),
		},
	}
}

// SetAPIKey updates the API key
func (c *Client) SetAPIKey(apiKey string) {
	c.apiKey = apiKey
	// Clear cached models since new key may have different access
	c.modelsMu.Lock()
	c.models = nil
	c.modelsMu.Unlock()
}

// SetModel updates the model name
func (c *Client) SetModel(modelName string) {
	c.modelName = modelName
}

// Request represents a Gemini API request
type Request struct {
	Contents          []Content        `json:"contents"`
	SystemInstruction *Content         `json:"systemInstruction,omitempty"`
	GenerationConfig  GenerationConfig `json:"generationConfig,omitempty"`
}

// Content represents a message content
type Content struct {
	Parts []Part `json:"parts"`
	Role  string `json:"role,omitempty"`
}

// Part represents a part of the content
type Part struct {
	Text string `json:"text"`
}

// GenerationConfig holds generation parameters
type GenerationConfig struct {
	Temperature     float64 `json:"temperature,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
}

// Response represents a Gemini API response
type Response struct {
	Candidates    []Candidate    `json:"candidates"`
	UsageMetadata *UsageMetadata `json:"usageMetadata,omitempty"`
	Error         *APIError      `json:"error,omitempty"`
}

// UsageMetadata represents token usage statistics
type UsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// Candidate represents a generated candidate
type Candidate struct {
	Content *Content `json:"content"`
}

// APIError represents an API error
type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

// RefineText sends raw transcription to Gemini for refinement.
// If ok_to_go is true, the caller should use rawText (second return is true).
// RefineText satisfies llm.Refiner. If model is non-empty it overrides the
// client's configured model for this call only.
func (c *Client) RefineText(rawText, model string) (string, int, bool, error) {
	logger.Debugf("[Gemini] Refining text: %d chars", len(rawText))
	if c.apiKey == "" {
		return "", 0, false, fmt.Errorf("API key not set")
	}

	activeModel := c.modelName
	if model != "" {
		activeModel = model
	}

	systemPrompt := llm.BuildSystemPrompt()

	// Create the request with proper system instruction separation
	req := Request{
		SystemInstruction: &Content{
			Parts: []Part{{Text: systemPrompt}},
		},
		Contents: []Content{
			{
				Role:  "user",
				Parts: []Part{{Text: rawText}},
			},
		},
		GenerationConfig: GenerationConfig{
			Temperature:     0.2,  // Lower temperature for more consistent output
			MaxOutputTokens: 1024, // Reduced for typical voice transcription length
		},
	}

	// Marshal request
	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", 0, false, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build URL with API key
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", baseAPIURL, activeModel, c.apiKey)

	// Make HTTP request
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return "", 0, false, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", 0, false, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, false, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse response
	var geminiResp Response
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return "", 0, false, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for API error
	if geminiResp.Error != nil {
		return "", 0, false, fmt.Errorf("API error: %s (code: %d)", geminiResp.Error.Message, geminiResp.Error.Code)
	}

	// Extract the refined text
	if len(geminiResp.Candidates) == 0 || geminiResp.Candidates[0].Content == nil {
		return "", 0, false, fmt.Errorf("no response generated")
	}

	if len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", 0, false, fmt.Errorf("empty response")
	}

	result := geminiResp.Candidates[0].Content.Parts[0].Text

	// Debug logging
	logger.Debugf("[Gemini] Raw output: %d chars", len(result))

	// Extract token count
	var tokenCount int
	if geminiResp.UsageMetadata != nil {
		tokenCount = geminiResp.UsageMetadata.CandidatesTokenCount
	}

	// Parse structured response
	refined, okToGo, parsed := llm.ParseRefineResponse(result, rawText)
	if !parsed {
		logger.Warnf("[Gemini] Warning: Response was not valid JSON, using as plain text")
		return llm.StripCodeFences(result), tokenCount, false, nil
	}
	return refined, tokenCount, okToGo, nil
}

// RetryWithInstruction re-processes text with a custom instruction
func (c *Client) RetryWithInstruction(text, instruction, model string) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("API key not set")
	}

	activeModel := c.modelName
	if model != "" {
		activeModel = model
	}

	prompt := fmt.Sprintf(`Apply the following instruction to the text:
Instruction: %s

Text:
%s

Return ONLY the modified text, nothing else.`, instruction, text)

	req := Request{
		Contents: []Content{
			{
				Parts: []Part{
					{Text: prompt},
				},
			},
		},
		GenerationConfig: GenerationConfig{
			Temperature:     0.3,
			MaxOutputTokens: 2048,
		},
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", baseAPIURL, activeModel, c.apiKey)

	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var geminiResp Response
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if geminiResp.Error != nil {
		return "", fmt.Errorf("API error: %s", geminiResp.Error.Message)
	}

	if len(geminiResp.Candidates) == 0 || geminiResp.Candidates[0].Content == nil {
		return "", fmt.Errorf("no response generated")
	}

	if len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response")
	}

	return geminiResp.Candidates[0].Content.Parts[0].Text, nil
}

// ModelListResponse represents the response from listing models
type ModelListResponse struct {
	Models []Model   `json:"models"`
	Error  *APIError `json:"error,omitempty"`
}

// Model represents a Gemini model
type Model struct {
	Name                       string   `json:"name"`
	DisplayName                string   `json:"displayName"`
	Description                string   `json:"description"`
	InputTokenLimit            int      `json:"inputTokenLimit"`
	OutputTokenLimit           int      `json:"outputTokenLimit"`
	SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
}

// ListModels returns a list of available Gemini models
func (c *Client) ListModels() ([]string, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("API key not set")
	}

	c.modelsMu.Lock()
	if c.models != nil {
		c.modelsMu.Unlock()
		return c.models, nil
	}
	c.modelsMu.Unlock()

	url := fmt.Sprintf("%s/models?key=%s", baseAPIURL, c.apiKey)

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var listResp ModelListResponse
	if err := json.Unmarshal(respBody, &listResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if listResp.Error != nil {
		return nil, fmt.Errorf("API error: %s (code: %d)", listResp.Error.Message, listResp.Error.Code)
	}

	var models []string
	for _, m := range listResp.Models {
		// Filter for models that support generateContent
		isContentGen := false
		for _, method := range m.SupportedGenerationMethods {
			if method == "generateContent" {
				isContentGen = true
				break
			}
		}

		// Only include base models (not tuned), starting with "models/gemini"
		if isContentGen && strings.HasPrefix(m.Name, "models/gemini") {
			// Strip "models/" prefix for cleaner display/usage
			name := strings.TrimPrefix(m.Name, "models/")
			models = append(models, name)
		}
	}

	c.modelsMu.Lock()
	c.models = models
	c.modelsMu.Unlock()

	return models, nil
}

// CheckModel tests a model and returns latency in milliseconds and tokens per second
func (c *Client) CheckModel(modelName string) (int64, float64, error) {
	if c.apiKey == "" {
		return 0, 0, fmt.Errorf("API key not set")
	}

	req := Request{
		Contents: []Content{
			{
				Parts: []Part{
					{Text: llm.LatencyTestText},
				},
			},
		},
		GenerationConfig: GenerationConfig{
			Temperature: 0.3,
			// No max tokens cap - let it generate full response for accurate latency test
		},
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", baseAPIURL, modelName, c.apiKey)

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
		respBody, _ := io.ReadAll(resp.Body)
		return 0, 0, fmt.Errorf("API error: %s (status: %d)", string(respBody), resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to read response: %w", err)
	}

	var geminiResp Response
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return latency, 0, nil // Return latency even if TPS fails
	}

	tokenCount := 0
	if geminiResp.UsageMetadata != nil {
		tokenCount = geminiResp.UsageMetadata.CandidatesTokenCount
	}

	var tps float64 = 0
	if latency > 0 && tokenCount > 0 {
		tps = float64(tokenCount) / (float64(latency) / 1000.0)
	}

	return latency, tps, nil
}

// Prewarm initiates a background HEAD request to baseAPIURL to warm up the
// TLS/HTTP2 connection.
func (c *Client) Prewarm(model string) {
	if c.apiKey == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "HEAD", baseAPIURL, nil)
		if err != nil {
			return
		}
		resp, err := c.httpClient.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}()
}
