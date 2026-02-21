package groq

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"voxflow/internal/llm"
)

const (
	baseAPIURL = "https://api.groq.com/openai/v1"
)

// ModelDescriptions contains descriptions for popular Groq models
var ModelDescriptions = map[string]string{
	"llama-3.1-8b-instant": "Llama 3.1 8B (Fastest)",
	"llama3-70b-8192":      "Llama 3 70B (High Quality)",
	"mixtral-8x7b-32768":   "Mixtral 8x7B (Balanced)",
	"gemma2-9b-it":         "Gemma 2 9B (Good Reasoning)",
}

// AvailableModels is a list of Groq models to choose from
var AvailableModels = []string{
	"llama-3.1-8b-instant",
	"llama3-70b-8192",
	"mixtral-8x7b-32768",
	"gemma2-9b-it",
}

// Client handles communication with the Groq API
type Client struct {
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new Groq client
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

// Request represents a Groq API request
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

// Response represents a Groq API response
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

// GetModels returns available models (hardcoded for now as Groq models are stable)
func (c *Client) GetModels() ([]string, error) {
	return AvailableModels, nil
}

// GetModelDescription returns the description for a model
func GetModelDescription(model string) string {
	if desc, ok := ModelDescriptions[model]; ok {
		return desc
	}
	return "Groq Model"
}

// RefineText sends raw transcription to Groq for refinement
func (c *Client) RefineText(rawText string, model string, mode string) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("API key not set")
	}

	systemPrompt := buildSystemPrompt(mode)

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

	var groqResp Response
	if err := json.Unmarshal(respBody, &groqResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w, response: %s", err, string(respBody))
	}

	if len(groqResp.Choices) == 0 {
		return "", fmt.Errorf("no response generated")
	}

	result := groqResp.Choices[0].Message.Content

	// Debug logging
	fmt.Printf("[Groq] Raw output (%d chars):\n%s\n", len(result), result)

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
4. NO markdown, NO code blocks, NO explanations, NO introductory sentences like "Here is the text".
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
