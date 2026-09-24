package orchestrator

import (
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode"

	"voxflow/internal/audio"
)

var whisperNoiseMarkerRe = regexp.MustCompile(
	`(?i)[\[(](audio|music|applause|noise|silence|laughter|blank_audio|blank audio|no speech|no_speech|sigh|cough|gasp|snort|groan|grunt|whispering|bell|buzz|chuckle|throat-clearing)[\])]` +
		`|\[[A-Z_]{2,}(?:[\s_][A-Z_]{2,})*\]`,
)

var soundDescriptionRe = regexp.MustCompile(`\[[^\]]*\]|\([^)]*\)|\*[^*]*\*|[♪♫]`)

// Whisper's stock output for silence and room noise, learned from subtitled video.
// Keys are in normalizeForMatch form.
var silenceHallucinations = map[string]bool{
	"you":                                  true,
	"bye":                                  true,
	"bye bye":                              true,
	"thanks":                               true,
	"thank you":                            true,
	"thank you very much":                  true,
	"thank you so much":                    true,
	"thanks for watching":                  true,
	"thank you for watching":               true,
	"thank you so much for watching":       true,
	"thanks for listening":                 true,
	"thank you for listening":              true,
	"please subscribe":                     true,
	"see you next time":                    true,
	"subtitles by the amara org community": true,
	"ご視聴ありがとうございました": true,
}

// Whisper echoes the vocabulary prompt on silent chunks.
const maxVocabularyEchoWords = 4

type streamChunk struct {
	Start    time.Duration
	Duration time.Duration
	Text     string
}

// cleanWhisperText strips noise markers and drops text that holds no words.
func cleanWhisperText(text string) string {
	text = whisperNoiseMarkerRe.ReplaceAllString(text, " ")
	text = strings.Join(strings.Fields(text), " ")
	if normalizeForMatch(soundDescriptionRe.ReplaceAllString(text, " ")) == "" {
		return ""
	}
	return text
}

// cleanWhisperChunk also drops Whisper's stock silence phrases and echoes of the
// vocabulary prompt, but only when the chunk's own audio has no speech energy,
// so a spoken "Thank you." or "Kubernetes" survives.
func cleanWhisperChunk(text, vocabulary string, samples []int16) string {
	text = cleanWhisperText(text)
	if text == "" || audio.HasActivity(samples) {
		return text
	}
	if isSilenceHallucination(text) || isVocabularyEcho(text, vocabulary) {
		return ""
	}
	return text
}

func isVocabularyEcho(text, vocabulary string) bool {
	words := strings.Fields(normalizeForMatch(text))
	if len(words) > maxVocabularyEchoWords {
		return false
	}
	vocab := strings.Fields(normalizeForMatch(vocabulary))
	for _, w := range words {
		if !slices.Contains(vocab, w) {
			return false
		}
	}
	return true
}

func isSilenceHallucination(text string) bool {
	return silenceHallucinations[normalizeForMatch(soundDescriptionRe.ReplaceAllString(text, " "))]
}

// normalizeForMatch lowercases text and reduces it to space-separated letter/digit runs.
func normalizeForMatch(s string) string {
	return strings.Join(strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}), " ")
}

// Chunks are disjoint audio (the recorder cuts without overlap), so a word on both
// sides of a boundary was said twice and is kept.
func mergeStreamingChunks(chunks []streamChunk) string {
	ordered := make([]streamChunk, len(chunks))
	copy(ordered, chunks)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].Start < ordered[j].Start
	})

	texts := make([]string, 0, len(ordered))
	for _, c := range ordered {
		if c.Text != "" {
			texts = append(texts, c.Text)
		}
	}
	return strings.Join(texts, " ")
}
