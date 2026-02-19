package openrouter

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
	baseAPIURL = "https://openrouter.ai/api/v1"
)

// LatencyTestText is a sample transcription for testing model latency
// This simulates a realistic voice input with filler words, punctuation commands, etc.
const LatencyTestText = `um hello uh so i was thinking about the project right and like i mean we need to get things done you know
okay so here's the thing uh basically we have three main points first uh the budget right
two the timeline and three the resources basically
um so uh let's start with the budget right so basically we're looking at around maybe like five thousand dollars
but you know i mean that could change depending on what we need right
uh second point uh timeline i think we should aim for like three months you know
but actually let me think about that again i mean it might take longer
um and third resources um we need like two developers and one designer right
okay so that's the basic plan um oh wait i forgot to mention one more thing
uh the client wants it done by end of quarter right so basically that's like two months
you know i mean it's tight but doable right
um let me go back to the first point about budget actually
uh we might need extra for testing you know what i mean
so like maybe six thousand would be safer i think
okay next let's talk about timeline period
if we start next week we can finish by july fifteenth
that's like eight weeks right
but there might be delays you know
um what about the second point again oh timeline right
uh we should build in some buffer time like two weeks extra
okay moving on to resources
we need one senior developer and one junior developer
and also a ux designer right
oh wait do we need a qa person too let me think
i mean probably not for the first phase right
okay so that's three people total
um now let me summarize the main points period
one budget six thousand dollars comma two timeline ten weeks comma three resources three people
is that clear question mark
okay great um next steps we need to create a detailed plan
uh first task is to write the specification document
second task is to set up the development environment
third task is to create the prototype
um and fourth task is to get client approval right
okay so that's it for now
let me know if you have any questions
uh oh and one more thing we should schedule a meeting next tuesday
that's like at two pm right
okay sounds good
um that's all for now thanks bye`

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

// RefineResponse represents the structured output from refinement
type RefineResponse struct {
	Text    string `json:"text"`
	Refused bool   `json:"refused"`
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
	httpReq.Header.Set("HTTP-Referer", "https://voxflow.app")
	httpReq.Header.Set("X-Title", "Voxflow")

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

	var openrouterResp Response
	if err := json.Unmarshal(respBody, &openrouterResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w, response: %s", err, string(respBody))
	}

	if len(openrouterResp.Choices) == 0 {
		return "", fmt.Errorf("no response generated")
	}

	result := openrouterResp.Choices[0].Message.Content

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
			{Role: "user", Content: LatencyTestText},
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
	httpReq.Header.Set("HTTP-Referer", "https://voxflow.app")
	httpReq.Header.Set("X-Title", "Voxflow")

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
