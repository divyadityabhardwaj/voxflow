package openrouter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
	"voxflow/internal/llm"
)

const (
	baseAPIURL = "https://openrouter.ai/api/v1"
)

// ModelDescriptions contains descriptions for popular models
var ModelDescriptions = map[string]string{
	"qwen/qwen3-235b-a22b:free":           "Qwen 3 - Excellent reasoning (262K context)",
	"deepseek/deepseek-chat-v3-0324:free": "DeepSeek V3 - Strong open model (128K context)",
	"meta-llama/llama-4-maverick:free":    "Llama 4 Maverick - Meta's latest (128K context)",
	"nvidia/nemotron-3-nano-30b-a3b:free": "Nemotron 3 Nano - NVIDIA's efficient (256K)",
	"google/gemma-3-4b-it:free":           "Gemma 3 4B - Good balance of speed/quality",
	"mistralai/mistral-7b-instruct:free":  "Mistral 7B - Reliable open model",
}

// FallbackFreeModels is used if API call fails
var FallbackFreeModels = []string{
	"qwen/qwen3-235b-a22b:free",
	"deepseek/deepseek-chat-v3-0324:free",
	"meta-llama/llama-4-maverick:free",
	"nvidia/nemotron-3-nano-30b-a3b:free",
	"google/gemma-3-4b-it:free",
	"mistralai/mistral-7b-instruct:free",
}

// Client handles communication with the OpenRouter API
type Client struct {
	apiKey     string
	httpClient *http.Client
	models     []string
	modelsMu   sync.Mutex
}

// NewClient creates a new OpenRouter client
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
	// Clear cached models since new key may have different access
	c.modelsMu.Lock()
	c.models = nil
	c.modelsMu.Unlock()
}

// Request represents an OpenRouter API request
type Request struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Response represents an OpenRouter API response
type Response struct {
	ID      string   `json:"id"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice represents a generated choice
type Choice struct {
	Message Message `json:"message"`
}

// Usage represents token usage
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// API Models Response
type ModelsResponse struct {
	Data []Model `json:"data"`
}

type Model struct {
	ID          string  `json:"id"`
	Name        string  `json:"name,omitempty"`
	Description string  `json:"description,omitempty"`
	ContextLen  int64   `json:"context_length,omitempty"`
	Pricing     Pricing `json:"pricing,omitempty"`
}

type Pricing struct {
	Prompt     string `json:"prompt,omitempty"`
	Completion string `json:"completion,omitempty"`
}

// GetFreeModels fetches available free models from OpenRouter API
func (c *Client) GetFreeModels() ([]string, error) {
	c.modelsMu.Lock()
	if c.models != nil {
		c.modelsMu.Unlock()
		return c.models, nil
	}
	c.modelsMu.Unlock()

	url := fmt.Sprintf("%s/models?free=true", baseAPIURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	req.Header.Set("HTTP-Referer", "https://voxflow.app")
	req.Header.Set("X-Title", "Voxflow")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return FallbackFreeModels, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return FallbackFreeModels, fmt.Errorf("API error (status %d)", resp.StatusCode)
	}

	var modelsResp ModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return FallbackFreeModels, fmt.Errorf("failed to parse models: %w", err)
	}

	var freeModels []string
	for _, model := range modelsResp.Data {
		// Only include text models (not image generation)
		if strings.Contains(model.ID, ":free") {
			freeModels = append(freeModels, model.ID)
		}
	}

	if len(freeModels) == 0 {
		return FallbackFreeModels, nil
	}

	c.modelsMu.Lock()
	c.models = freeModels
	c.modelsMu.Unlock()

	return freeModels, nil
}

// GetModelDescription returns the description for a model
func GetModelDescription(model string) string {
	if desc, ok := ModelDescriptions[model]; ok {
		return desc
	}
	return "Free model on OpenRouter"
}

// RefineText sends raw transcription to OpenRouter for refinement
func (c *Client) RefineText(rawText string, model string) (string, int, bool, error) {
	if c.apiKey == "" {
		return "", 0, false, fmt.Errorf("API key not set")
	}

	systemPrompt := llm.BuildSystemPrompt()

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
	httpReq.Header.Set("HTTP-Referer", "https://voxflow.app")
	httpReq.Header.Set("X-Title", "Voxflow")

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

	var openrouterResp Response
	if err := json.Unmarshal(respBody, &openrouterResp); err != nil {
		return "", 0, false, fmt.Errorf("failed to parse response: %w, response: %s", err, string(respBody))
	}

	if len(openrouterResp.Choices) == 0 {
		return "", 0, false, fmt.Errorf("no response generated")
	}

	result := openrouterResp.Choices[0].Message.Content
	tokenCount := openrouterResp.Usage.CompletionTokens

	// Debug logging (matches Gemini behavior)
	fmt.Printf("[OpenRouter] Raw output (%d chars), Tokens: %d:\n%s\n", len(result), tokenCount, result)

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
	httpReq.Header.Set("HTTP-Referer", "https://voxflow.app")
	httpReq.Header.Set("X-Title", "Voxflow")

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

	var openrouterResp Response
	if err := json.Unmarshal(respBody, &openrouterResp); err != nil {
		return latency, 0, nil // Return latency even if TPS fails
	}

	tokenCount := openrouterResp.Usage.CompletionTokens
	var tps float64 = 0
	if latency > 0 && tokenCount > 0 {
		tps = float64(tokenCount) / (float64(latency) / 1000.0)
	}

	return latency, tps, nil
}
