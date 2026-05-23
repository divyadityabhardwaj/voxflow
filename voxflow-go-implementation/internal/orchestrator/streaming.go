package orchestrator

import (
	"regexp"
	"sort"
	"strings"
	"time"
)

var whisperNoiseMarkerRe = regexp.MustCompile(
	`(?i)(^|\s)[\[(](audio|music|applause|noise|silence|laughter)[\])](?:\s|$)` +
		`|(^|\s)\[[A-Z]{2,}(?:\s[A-Z]{2,})+\](?:\s|$)`,
)

type streamChunk struct {
	Start time.Duration
	Text  string
}

func cleanWhisperText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	text = whisperNoiseMarkerRe.ReplaceAllString(text, " ")
	text = strings.Join(strings.Fields(text), " ")
	return strings.TrimSpace(text)
}

// mergeStreamingChunks combines chunks, removing overlapping/duplicate words.
func mergeStreamingChunks(chunks []streamChunk) string {
	if len(chunks) == 0 {
		return ""
	}

	ordered := make([]streamChunk, len(chunks))
	copy(ordered, chunks)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].Start < ordered[j].Start
	})

	if len(ordered) == 1 {
		return ordered[0].Text
	}

	result := ordered[0].Text
	for i := 1; i < len(ordered); i++ {
		current := ordered[i].Text
		if result != "" && current != "" {
			resultWords := strings.Fields(result)
			nextWords := strings.Fields(current)
			if len(resultWords) > 0 && len(nextWords) > 0 {
				lastWord := resultWords[len(resultWords)-1]
				firstWord := nextWords[0]
				if strings.EqualFold(lastWord, firstWord) {
					result += " " + strings.Join(nextWords[1:], " ")
				} else {
					result += " " + current
				}
			} else {
				result += " " + current
			}
		} else if current != "" {
			result += current
		}
	}
	return strings.TrimSpace(result)
}
