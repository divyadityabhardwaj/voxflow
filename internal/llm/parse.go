package llm

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode"
)

type RefineResponse struct {
	Text    string `json:"text"`
	Refused bool   `json:"refused"`
	OkToGo  bool   `json:"ok_to_go"`
}

// When parsed is false the caller must paste rawText: whatever the model said
// instead of JSON is chatter, never the user's text.
func ParseRefineResponse(raw string, rawText string) (result string, okToGo bool, parsed bool) {
	clean := StripCodeFences(stripThink(raw))
	if i, j := strings.Index(clean, "{"), strings.LastIndex(clean, "}"); i >= 0 && j > i {
		clean = clean[i : j+1]
	}

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
	if resp.Text != "" && !plausibleRefinement(rawText, resp.Text) {
		return rawText, false, true
	}
	return resp.Text, false, true
}

var thinkBlock = regexp.MustCompile(`(?s)<think>.*?</think>`)

func stripThink(text string) string {
	text = strings.TrimSpace(thinkBlock.ReplaceAllString(text, ""))
	if strings.HasPrefix(text, "<think>") {
		return "" // cut off mid-reasoning
	}
	return text
}

// Rejects replies that answer, obey or pad the dictation instead of cleaning it.
// Newlines are fine: list formatting legitimately adds them.
func plausibleRefinement(raw, refined string) bool {
	if len(refined) > 2*len(raw)+80 {
		return false
	}
	words := wordTokens(refined)
	// Too few words for a ratio to mean anything ("Don't", "25%").
	if len(words) <= 3 {
		return true
	}
	known := make(map[string]bool)
	for _, w := range wordTokens(raw) {
		known[w] = true
	}
	hits, counted := 0, 0
	for _, w := range words {
		// Spoken numbers come back as digits, so digits never match raw text.
		if isNumeric(w) {
			continue
		}
		counted++
		if known[w] {
			hits++
		}
	}
	return hits*2 >= counted
}

func isNumeric(w string) bool {
	return strings.IndexFunc(w, func(r rune) bool { return !unicode.IsNumber(r) }) < 0
}

func wordTokens(s string) []string {
	var out []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			out = append(out, string(cur))
			cur = cur[:0]
		}
	}
	for _, r := range strings.ToLower(s) {
		switch {
		// Scripts written without spaces: one character per token, or a single
		// corrected character would make the whole sentence one unmatched word.
		case unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Thai):
			flush()
			out = append(out, string(r))
		case unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r):
			cur = append(cur, r)
		default:
			flush()
		}
	}
	flush()
	return out
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
