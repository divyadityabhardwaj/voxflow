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
}

// Choice represents a generated choice
type Choice struct {
	Message Message `json:"message"`
}

// RefineResponse represents the structured output from refinement
type RefineResponse struct {
	Text    string `json:"text"`
	Refused bool   `json:"refused"`
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
func (c *Client) RefineText(rawText string, model string, mode string) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("API key not set")
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
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/chat/completions", baseAPIURL)

	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var cerebrasResp Response
	if err := json.Unmarshal(respBody, &cerebrasResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w, response: %s", err, string(respBody))
	}

	if len(cerebrasResp.Choices) == 0 {
		return "", fmt.Errorf("no response generated")
	}

	result := cerebrasResp.Choices[0].Message.Content

	// Debug logging
	fmt.Printf("[Cerebras] Raw output (%d chars):\n%s\n", len(result), result)

	cleanResult := result
	if strings.HasPrefix(cleanResult, "```json") {
		cleanResult = strings.TrimPrefix(cleanResult, "```json")
		cleanResult = strings.TrimSuffix(strings.TrimSpace(cleanResult), "```")
		cleanResult = strings.TrimSpace(cleanResult)
	} else if strings.HasPrefix(cleanResult, "```") {
		cleanResult = strings.TrimPrefix(cleanResult, "```")
		cleanResult = strings.TrimSuffix(strings.TrimSpace(cleanResult), "```")
		cleanResult = strings.TrimSpace(cleanResult)
	}

	var refineResp RefineResponse
	if err := json.Unmarshal([]byte(cleanResult), &refineResp); err == nil {
		if refineResp.Refused {
			return rawText, nil
		}
		return refineResp.Text, nil
	}

	return cleanResult, nil
}

// CheckModel tests a model and returns latency in milliseconds
func (c *Client) CheckModel(model string) (int64, error) {
	if c.apiKey == "" {
		return 0, fmt.Errorf("API key not set")
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
		return 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/chat/completions", baseAPIURL)

	startTime := time.Now()

	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return 0, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	latency := time.Since(startTime).Milliseconds()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("API error (status %d)", resp.StatusCode)
	}

	return latency, nil
}
