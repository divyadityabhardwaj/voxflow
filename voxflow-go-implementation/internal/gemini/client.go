package gemini

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

// NewClient creates a new Gemini client
func NewClient(apiKey string, modelName string) *Client {
	if modelName == "" {
		modelName = "gemini-1.5-flash"
	}
	return &Client{
		apiKey:    apiKey,
		modelName: modelName,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
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
	Contents         []Content        `json:"contents"`
	GenerationConfig GenerationConfig `json:"generationConfig,omitempty"`
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

// RefineResponse represents the structured output from refinement
type RefineResponse struct {
	Text    string `json:"text"`
	Refused bool   `json:"refused"`
}

// RefineText sends raw transcription to Gemini for refinement
func (c *Client) RefineText(rawText string, mode string) (string, int, error) {
	fmt.Printf("[Gemini] Refining text: %s\n", rawText)
	if c.apiKey == "" {
		return "", 0, fmt.Errorf("API key not set")
	}

	// Build the system prompt based on mode
	systemPrompt := llm.BuildSystemPrompt(mode)

	// Create the request
	req := Request{
		Contents: []Content{
			{
				Parts: []Part{
					{Text: systemPrompt + "\n\nTranscription to refine:\n" + rawText},
				},
			},
		},
		GenerationConfig: GenerationConfig{
			Temperature:     0.3, // Lower temperature for more consistent output
			MaxOutputTokens: 2048,
		},
	}

	// Marshal request
	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build URL with API key
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", baseAPIURL, c.modelName, c.apiKey)

	// Make HTTP request
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return "", 0, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", 0, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse response
	var geminiResp Response
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return "", 0, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for API error
	if geminiResp.Error != nil {
		return "", 0, fmt.Errorf("API error: %s (code: %d)", geminiResp.Error.Message, geminiResp.Error.Code)
	}

	// Extract the refined text
	if len(geminiResp.Candidates) == 0 || geminiResp.Candidates[0].Content == nil {
		return "", 0, fmt.Errorf("no response generated")
	}

	if len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", 0, fmt.Errorf("empty response")
	}

	result := geminiResp.Candidates[0].Content.Parts[0].Text

	// Debug logging
	fmt.Printf("[Gemini] Raw output (%d chars):\n%s\n", len(result), result)

	// Extract token count
	var tokenCount int
	if geminiResp.UsageMetadata != nil {
		tokenCount = geminiResp.UsageMetadata.CandidatesTokenCount
	}

	// Clean up result - remove markdown code blocks if present
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

	// Try to parse as JSON response
	var refineResp RefineResponse
	if err := json.Unmarshal([]byte(cleanResult), &refineResp); err == nil {
		// Successfully parsed JSON
		if refineResp.Refused {
			fmt.Printf("[Gemini] Content was refused, using raw text instead\n")
			return rawText, tokenCount, nil
		}
		// Return the text (even if empty - that's what Gemini gave us)
		return refineResp.Text, tokenCount, nil
	}

	// If JSON parsing failed, Gemini returned plain text (old behavior)
	fmt.Printf("[Gemini] Warning: Response was not valid JSON, using as plain text\n")
	return cleanResult, tokenCount, nil
}

// RetryWithInstruction re-processes text with a custom instruction
func (c *Client) RetryWithInstruction(text string, instruction string) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("API key not set")
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

	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", baseAPIURL, c.modelName, c.apiKey)

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
	Models []Model `json:"models"`
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

// CheckModel tests a model and returns latency in milliseconds
func (c *Client) CheckModel(modelName string) (int64, error) {
	if c.apiKey == "" {
		return 0, fmt.Errorf("API key not set")
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
		return 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", baseAPIURL, modelName, c.apiKey)

	startTime := time.Now()

	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return 0, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	latency := time.Since(startTime).Milliseconds()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("API error: %s (status: %d)", string(respBody), resp.StatusCode)
	}

	return latency, nil
}
