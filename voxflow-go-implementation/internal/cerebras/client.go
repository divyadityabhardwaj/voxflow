package cerebras

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
	"voxflow/internal/llm"
)

const (
	baseAPIURL = "https://api.cerebras.ai/v1"
)

// ModelDescriptions contains descriptions for popular Cerebras models
var ModelDescriptions = map[string]string{
	"llama3.1-8b":  "Llama 3.1 8B (Fastest)",
	"llama3.1-70b": "Llama 3.1 70B (High Quality)",
}

// AvailableModels is a list of Cerebras models to choose from
var AvailableModels = []string{
	"llama3.1-8b",
	"llama3.1-70b",
}

// Client handles communication with the Cerebras API
type Client struct {
	apiKey     string
	httpClient *http.Client
	models     []string
	modelsMu   sync.Mutex
}

// NewClient creates a new Cerebras client
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// SetAPIKey updates the API key
func (c *Client) SetAPIKey(apiKey string) {
	c.apiKey = apiKey
}

// Request represents a Cerebras API request
type Request struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Response represents a Cerebras API response
type Response struct {
	ID      string   `json:"id"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Usage represents token usage statistics
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Choice represents a generated choice
type Choice struct {
	Message Message `json:"message"`
}

// ModelsResponse represents the response from the /models endpoint
type ModelsResponse struct {
	Data []ModelInfo `json:"data"`
}

// ModelInfo represents a model from the API
type ModelInfo struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}

// GetModels fetches available models from the Cerebras API using the user's API key
func (c *Client) GetModels() ([]string, error) {
	if c.apiKey == "" {
		return AvailableModels, fmt.Errorf("API key not set")
	}

	c.modelsMu.Lock()
	if c.models != nil {
		c.modelsMu.Unlock()
		return c.models, nil
	}
	c.modelsMu.Unlock()

	url := fmt.Sprintf("%s/models", baseAPIURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return AvailableModels, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return AvailableModels, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return AvailableModels, fmt.Errorf("API error (status %d)", resp.StatusCode)
	}

	var modelsResp ModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return AvailableModels, fmt.Errorf("failed to parse models: %w", err)
	}

	var models []string
	for _, model := range modelsResp.Data {
		// Only include chat/language models, skip embedding/tool models
		if strings.Contains(model.ID, "embedding") ||
			strings.Contains(model.ID, "tool-use") {
			continue
		}
		models = append(models, model.ID)
	}

	if len(models) == 0 {
		return AvailableModels, nil
	}

	sort.Strings(models)

	c.modelsMu.Lock()
	c.models = models
	c.modelsMu.Unlock()

	return models, nil
}

// ClearModelsCache clears the cached models list (call when API key changes)
func (c *Client) ClearModelsCache() {
	c.modelsMu.Lock()
	c.models = nil
	c.modelsMu.Unlock()
}

// GetModelDescription returns the description for a model
func GetModelDescription(model string) string {
	if desc, ok := ModelDescriptions[model]; ok {
		return desc
	}
	return "Cerebras Model"
}

// RefineText sends raw transcription to Cerebras for refinement
func (c *Client) RefineText(rawText string, model string, mode string) (string, int, bool, error) {
	if c.apiKey == "" {
		return "", 0, false, fmt.Errorf("API key not set")
	}

	systemPrompt := llm.BuildSystemPrompt(mode)

	req := Request{
		Model: model,
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: "Transcription to refine:\n" + rawText},
		},
		Temperature: 0.3,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", 0, false, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/chat/completions", baseAPIURL)

	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return "", 0, false, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", 0, false, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, false, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", 0, false, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var cerebrasResp Response
	if err := json.Unmarshal(respBody, &cerebrasResp); err != nil {
		return "", 0, false, fmt.Errorf("failed to parse response: %w, response: %s", err, string(respBody))
	}

	if len(cerebrasResp.Choices) == 0 {
		return "", 0, false, fmt.Errorf("no response generated")
	}

	result := cerebrasResp.Choices[0].Message.Content
	tokenCount := cerebrasResp.Usage.CompletionTokens

	// Debug logging
	fmt.Printf("[Cerebras] Raw output (%d chars), Tokens: %d:\n%s\n", len(result), tokenCount, result)

	// Parse structured response
	refined, okToGo, parsed := llm.ParseRefineResponse(result, rawText)
	if !parsed {
		return llm.StripCodeFences(result), tokenCount, false, nil
	}
	return refined, tokenCount, okToGo, nil
}

// CheckModel tests a model and returns latency in milliseconds and tokens per second
func (c *Client) CheckModel(model string) (int64, float64, error) {
	if c.apiKey == "" {
		return 0, 0, fmt.Errorf("API key not set")
	}

	req := Request{
		Model: model,
		Messages: []Message{
			{Role: "user", Content: llm.LatencyTestText},
		},
		Temperature: 0.3,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/chat/completions", baseAPIURL)

	startTime := time.Now()

	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return 0, 0, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

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

	var cerebrasResp Response
	if err := json.Unmarshal(respBody, &cerebrasResp); err != nil {
		return latency, 0, nil // Return latency even if TPS fails
	}

	tokenCount := cerebrasResp.Usage.CompletionTokens
	var tps float64 = 0
	if latency > 0 && tokenCount > 0 {
		tps = float64(tokenCount) / (float64(latency) / 1000.0)
	}

	return latency, tps, nil
}
