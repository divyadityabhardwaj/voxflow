package llm

import (
	"encoding/json"
	"strings"
)

type RefineResponse struct {
	Text    string `json:"text"`
	Refused bool   `json:"refused"`
	OkToGo  bool   `json:"ok_to_go"`
}

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

// Leading "{" means malformed JSON — paste raw transcript, not a JSON fragment.
func UnparsedFallback(raw, rawText string) string {
	clean := StripCodeFences(raw)
	if clean == "" || strings.HasPrefix(clean, "{") {
		return rawText
	}
	return clean
}

// ~2 chars/token so long dictations aren't cut mid-JSON.
func RefineMaxTokens(rawText string) int {
	n := len(rawText)/2 + 256
	if n < 768 {
		n = 768
	}
	return n
}

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
