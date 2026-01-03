package gemini

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	baseURL = "https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash:generateContent"
)

// Client handles communication with the Gemini API
type Client struct {
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new Gemini client
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetAPIKey updates the API key
func (c *Client) SetAPIKey(apiKey string) {
	c.apiKey = apiKey
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
	Candidates []Candidate `json:"candidates"`
	Error      *APIError   `json:"error,omitempty"`
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

// RefineText sends raw transcription to Gemini for refinement
func (c *Client) RefineText(rawText string, mode string) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("API key not set")
	}

	// Build the system prompt based on mode
	systemPrompt := buildSystemPrompt(mode)

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
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build URL with API key
	url := fmt.Sprintf("%s?key=%s", baseURL, c.apiKey)

	// Make HTTP request
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Parse response
	var geminiResp Response
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for API error
	if geminiResp.Error != nil {
		return "", fmt.Errorf("API error: %s (code: %d)", geminiResp.Error.Message, geminiResp.Error.Code)
	}

	// Extract the refined text
	if len(geminiResp.Candidates) == 0 || geminiResp.Candidates[0].Content == nil {
		return "", fmt.Errorf("no response generated")
	}

	if len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response")
	}

	return geminiResp.Candidates[0].Content.Parts[0].Text, nil
}

// buildSystemPrompt creates the appropriate prompt based on mode
func buildSystemPrompt(mode string) string {
	baseInstructions := `You are a transcription refinement assistant. Clean up the following transcription:
- Remove filler words (um, uh, ah, like, you know, etc.)
- Fix grammar and punctuation
- Correct obvious speech-to-text errors
- If the user says "make it a list" or "bullet points", format as a list
- If the user says "new paragraph" or "paragraph break", insert a paragraph break
- If the user says "period" or "full stop", insert a period
- If the user says "comma", insert a comma
- If the user says "question mark", insert a question mark
- If the user says "exclamation mark" or "exclamation point", insert an exclamation mark

Return ONLY the refined text, nothing else. Do not add explanations or commentary.`

	switch mode {
	case "formal":
		return baseInstructions + `

Additional instructions for formal mode:
- Use formal language and professional tone
- Expand contractions (don't → do not, can't → cannot)
- Use complete sentences
- Maintain professional vocabulary`

	case "casual":
		fallthrough
	default:
		return baseInstructions + `

Additional instructions for casual mode:
- Keep it conversational and natural
- Contractions are fine
- Maintain the speaker's natural speaking style where appropriate`
	}
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

	url := fmt.Sprintf("%s?key=%s", baseURL, c.apiKey)

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
