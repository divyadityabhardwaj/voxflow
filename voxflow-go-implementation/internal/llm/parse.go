package llm

import (
	"encoding/json"
	"strings"
)

// RefineResponse is the structured JSON output expected from LLM refinement.
type RefineResponse struct {
	Text    string `json:"text"`
	Refused bool   `json:"refused"`
	OkToGo  bool   `json:"ok_to_go"`
}

// ParseRefineResponse strips markdown code fences from raw LLM output and
// attempts to parse it as a RefineResponse.
//
// Returns:
//   - result: the refined text (or rawText if refused/ok_to_go)
//   - okToGo: true means caller should use the original rawText
//   - parsed: true means the response was successfully parsed as JSON
func ParseRefineResponse(raw string, rawText string) (result string, okToGo bool, parsed bool) {
	clean := StripCodeFences(raw)

	var resp RefineResponse
	if err := json.Unmarshal([]byte(clean), &resp); err != nil {
		return "", false, false
	}

	if resp.Refused {
		return rawText, false, true
	}
	if resp.OkToGo {
		return rawText, true, true
	}
	return resp.Text, false, true
}

// UnparsedFallback picks the text to use when the LLM reply was not valid JSON.
// A reply that starts with "{" is a truncated or malformed JSON object; pasting
// it would inject a JSON fragment into the user's app, so use the raw transcript.
// Anything else is treated as the model answering in plain text.
func UnparsedFallback(raw, rawText string) string {
	clean := StripCodeFences(raw)
	if clean == "" || strings.HasPrefix(clean, "{") {
		return rawText
	}
	return clean
}

// RefineMaxTokens sizes the output cap to the input so long dictations are not
// cut off mid-JSON. ~2 chars per token leaves 2x headroom over typical English.
func RefineMaxTokens(rawText string) int {
	n := len(rawText)/2 + 256
	if n < 768 {
		n = 768
	}
	return n
}

// StripCodeFences removes markdown code block wrappers from text.
func StripCodeFences(text string) string {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```json") {
		text = strings.TrimPrefix(text, "```json")
		text = strings.TrimSpace(text)
		text = strings.TrimSuffix(text, "```")
	} else if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```")
		text = strings.TrimSpace(text)
		text = strings.TrimSuffix(text, "```")
	}
	return strings.TrimSpace(text)
}
