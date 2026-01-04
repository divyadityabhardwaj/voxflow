package gemini

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	baseURL = "https://generativelanguage.googleapis.com/v1/models/gemini-2.0-flash:generateContent"
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

// RefineResponse represents the structured output from refinement
type RefineResponse struct {
	Text    string `json:"text"`
	Refused bool   `json:"refused"`
}

// RefineText sends raw transcription to Gemini for refinement
func (c *Client) RefineText(rawText string, mode string) (string, error) {
	fmt.Printf("[Gemini] Refining text: %s\n", rawText)
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

	result := geminiResp.Candidates[0].Content.Parts[0].Text

	// Debug logging
	fmt.Printf("[Gemini] Raw output (%d chars):\n%s\n", len(result), result)

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
			return rawText, nil
		}
		// Return the text (even if empty - that's what Gemini gave us)
		return refineResp.Text, nil
	}

	// If JSON parsing failed, Gemini returned plain text (old behavior)
	fmt.Printf("[Gemini] Warning: Response was not valid JSON, using as plain text\n")
	return cleanResult, nil
}

// buildSystemPrompt creates the appropriate prompt based on mode
func buildSystemPrompt(mode string) string {
	baseInstructions := `You are an expert voice-to-text refinement assistant. Transform raw speech transcriptions into clean, polished text.

=== FILLER WORD REMOVAL ===
Remove ALL filler words and verbal tics:
- um, uh, ah, er, mm, hmm
- like, you know, I mean, so, basically, actually, literally
- kind of, sort of, right, okay, well, anyway
- "I guess", "I think" (when used as filler, not genuine expression)

=== GRAMMAR & PUNCTUATION ===
- Fix grammar mistakes and run-on sentences
- Add proper punctuation (periods, commas, apostrophes)
- Correct speech-to-text errors (homophones, mishearings)
- Capitalize proper nouns, sentence starts, "I"

=== LIST DETECTION (Format as bullet points when detected) ===
When a list is detected, format it as:
• Item one
• Item two
• Item three

Each bullet point MUST be on its own line. Do NOT put multiple bullets on one line.

Trigger phrases:
- "make it a list", "bullet points", "list format", "as a list"
- "points about", "some points", "few points", "my points"
- "here are", "the following", "these things"

Numbered indicators (convert to bullets):
- "first", "second", "third", "fourth", "fifth"
- "firstly", "secondly", "thirdly"
- "one", "two", "three" (when used as item markers)
- "point one", "point two", "number one", "number two"

=== PUNCTUATION VOICE COMMANDS ===
- "period" / "full stop" / "dot" → .
- "comma" → ,
- "question mark" → ?
- "exclamation mark" / "exclamation point" / "bang" → !
- "colon" → :
- "semicolon" / "semi colon" → ;
- "hyphen" / "dash" → -
- "open parenthesis" / "open paren" / "left paren" → (
- "close parenthesis" / "close paren" / "right paren" → )
- "open quote" / "quote" / "begin quote" → "
- "close quote" / "end quote" / "unquote" → "
- "ellipsis" / "dot dot dot" → ...
- "ampersand" / "and sign" → &
- "at sign" / "at symbol" → @
- "hashtag" / "hash" / "pound sign" → #

=== FORMATTING COMMANDS ===
- "new line" / "line break" → insert line break
- "new paragraph" / "paragraph break" / "next paragraph" → insert paragraph break
- "all caps" / "caps lock" [word] → WORD (capitalize the word)
- "bold" [word] → **word** (if markdown supported)
- "tab" / "indent" → insert tab/indent

=== EDITING COMMANDS ===
- "scratch that" / "delete that" / "never mind" → remove last sentence/phrase
- "correction" [word] → replace previous word with this one
- "go back" → context: user is correcting something

=== SPECIAL HANDLING ===
- Numbers: Keep as digits for addresses, phone numbers, dates; spell out for casual mentions
- Emails: Format properly (name at domain dot com → name@domain.com)
- URLs: Format properly (www dot example dot com → www.example.com)
- Abbreviations: Preserve common ones (etc, vs, Mr, Mrs, Dr)

=== OUTPUT FORMAT (CRITICAL) ===
You MUST respond with valid JSON in this exact format:
{"text": "your refined text here", "refused": false}

If the content contains something you cannot process due to ethical guidelines:
{"text": "", "refused": true}

Rules:
1. ALWAYS output valid JSON, nothing else
2. The "text" field contains the refined transcription
3. Set "refused" to true ONLY if you cannot process the content
4. NO markdown, NO code blocks, NO explanations
5. Preserve the speaker's intent and meaning
6. When in doubt, keep the original phrasing`

	switch mode {
	case "formal":
		return baseInstructions + `

=== FORMAL MODE ===
- Use professional, polished language
- Expand contractions: don't → do not, can't → cannot, won't → will not
- Use complete, well-structured sentences
- Avoid slang and colloquialisms
- Suitable for: business emails, reports, official documents`

	case "casual":
		fallthrough
	default:
		return baseInstructions + `

=== CASUAL MODE ===
- Keep conversational, natural tone
- Contractions are fine (don't, can't, won't)
- Maintain speaker's personality and style
- Light editing - don't over-formalize
- Suitable for: messages, notes, personal writing`
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
