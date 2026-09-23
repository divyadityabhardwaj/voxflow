package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

type Client struct {
	apiKey     string
	modelName  string
	httpClient *http.Client
	models     []string
	modelsMu   sync.Mutex
}

func NewClient(apiKey string, modelName string) *Client {
	return &Client{
		apiKey:    apiKey,
		modelName: modelName,
		httpClient: &http.Client{
			Timeout:   15 * time.Second,
			Transport: llm.NewTransport(),
		},
	}
}

func (c *Client) SetAPIKey(apiKey string) {
	c.apiKey = apiKey
	c.modelsMu.Lock()
	c.models = nil
	c.modelsMu.Unlock()
}

func (c *Client) SetModel(modelName string) {
	c.modelName = modelName
}

// API key in header, not query string (proxy/access logs).
func (c *Client) request(method, url string, body []byte) func() (*http.Request, error) {
	return func() (*http.Request, error) {
		var r io.Reader
		if body != nil {
			r = bytes.NewReader(body)
		}
		req, err := http.NewRequest(method, url, r)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-goog-api-key", c.apiKey)
		return req, nil
	}
}

type Request struct {
	Contents          []Content        `json:"contents"`
	SystemInstruction *Content         `json:"systemInstruction,omitempty"`
	GenerationConfig  GenerationConfig `json:"generationConfig,omitempty"`
}

type Content struct {
	Parts []Part `json:"parts"`
	Role  string `json:"role,omitempty"`
}

type Part struct {
	Text string `json:"text"`
}

type GenerationConfig struct {
	Temperature     float64 `json:"temperature,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
}

type Response struct {
	Candidates    []Candidate    `json:"candidates"`
	UsageMetadata *UsageMetadata `json:"usageMetadata,omitempty"`
}

type UsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

type Candidate struct {
	Content *Content `json:"content"`
}

// model non-empty overrides the client default for this call. ok_to_go => use rawText.
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

	req := Request{
		SystemInstruction: &Content{
			Parts: []Part{{Text: systemPrompt}},
		},
		Contents: []Content{
			{
				Role:  "user",
				Parts: []Part{{Text: "<transcription>\n" + rawText + "\n</transcription>"}},
			},
		},
		GenerationConfig: GenerationConfig{
			Temperature:     0.2, // Lower temperature for more consistent output
			MaxOutputTokens: llm.RefineMaxTokens(rawText),
		},
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", 0, false, fmt.Errorf("failed to marshal request: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), llm.RefineBudget(rawText))
	defer cancel()

	url := fmt.Sprintf("%s/models/%s:generateContent", baseAPIURL, activeModel)
	respBody, status, err := llm.DoWithRetryContext(ctx, c.httpClient, 1, c.request("POST", url, reqBody))
	if err != nil {
		return "", 0, false, err
	}
	if status != http.StatusOK {
		return "", 0, false, llm.StatusError(status, respBody)
	}

	var geminiResp Response
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return "", 0, false, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 || geminiResp.Candidates[0].Content == nil {
		return "", 0, false, fmt.Errorf("no response generated")
	}

	if len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", 0, false, fmt.Errorf("empty response")
	}

	result := geminiResp.Candidates[0].Content.Parts[0].Text

	logger.Debugf("[Gemini] Raw output: %d chars", len(result))

	var tokenCount int
	if geminiResp.UsageMetadata != nil {
		tokenCount = geminiResp.UsageMetadata.CandidatesTokenCount
	}

	refined, okToGo, parsed := llm.ParseRefineResponse(result, rawText)
	if !parsed {
		logger.Warnf("[Gemini] Warning: Response was not valid JSON")
		return rawText, tokenCount, false, nil
	}
	return refined, tokenCount, okToGo, nil
}

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

	url := fmt.Sprintf("%s/models/%s:generateContent", baseAPIURL, activeModel)
	respBody, status, err := llm.DoWithRetry(c.httpClient, c.request("POST", url, reqBody))
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", llm.StatusError(status, respBody)
	}

	var geminiResp Response
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 || geminiResp.Candidates[0].Content == nil {
		return "", fmt.Errorf("no response generated")
	}

	if len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response")
	}

	return geminiResp.Candidates[0].Content.Parts[0].Text, nil
}

type ModelListResponse struct {
	Models []Model `json:"models"`
}

type Model struct {
	Name                       string   `json:"name"`
	DisplayName                string   `json:"displayName"`
	Description                string   `json:"description"`
	InputTokenLimit            int      `json:"inputTokenLimit"`
	OutputTokenLimit           int      `json:"outputTokenLimit"`
	SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
}

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

	respBody, status, err := llm.DoWithRetry(c.httpClient, c.request("GET", baseAPIURL+"/models", nil))
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, llm.StatusError(status, respBody)
	}

	var listResp ModelListResponse
	if err := json.Unmarshal(respBody, &listResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	var models []string
	for _, m := range listResp.Models {
		isContentGen := false
		for _, method := range m.SupportedGenerationMethods {
			if method == "generateContent" {
				isContentGen = true
				break
			}
		}

		if isContentGen && strings.HasPrefix(m.Name, "models/gemini") {
			name := strings.TrimPrefix(m.Name, "models/")
			models = append(models, name)
		}
	}

	c.modelsMu.Lock()
	c.models = models
	c.modelsMu.Unlock()

	return models, nil
}

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
		},
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/models/%s:generateContent", baseAPIURL, modelName)

	startTime := time.Now()

	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return 0, 0, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	latency := time.Since(startTime).Milliseconds()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return 0, 0, llm.StatusError(resp.StatusCode, respBody)
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
