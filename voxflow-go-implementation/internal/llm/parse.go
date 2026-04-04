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
