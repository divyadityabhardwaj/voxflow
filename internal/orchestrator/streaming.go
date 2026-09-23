package orchestrator

import (
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode"
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

// ponytail: Whisper echoes the vocabulary prompt on silent chunks, so a real dictation
// of only this many vocabulary terms is dropped with it; a VAD gate would fix both.
const maxVocabularyEchoWords = 4

type streamChunk struct {
	Start    time.Duration
	Duration time.Duration
	Text     string
}

// ponytail: exact whole-text match only, but a chunk that really is just "Thank you."
// (a one-word dictation, or a sign-off after a pause) is dropped too; a per-chunk VAD
// gate would remove the need for the list.
func cleanWhisperText(text string) string {
	text = whisperNoiseMarkerRe.ReplaceAllString(text, " ")
	text = strings.Join(strings.Fields(text), " ")
	if isSilenceHallucination(text) {
		return ""
	}
	return text
}

// cleanWhisperChunk is cleanWhisperText plus dropping output that only repeats
// terms from the vocabulary prompt sent with the chunk.
func cleanWhisperChunk(text, vocabulary string) string {
	text = cleanWhisperText(text)
	words := strings.Fields(normalizeForMatch(text))
	if len(words) == 0 || len(words) > maxVocabularyEchoWords {
		return text
	}
	vocab := strings.Fields(normalizeForMatch(vocabulary))
	for _, w := range words {
		if !slices.Contains(vocab, w) {
			return text
		}
	}
	return ""
}

func isSilenceHallucination(text string) bool {
	words := normalizeForMatch(soundDescriptionRe.ReplaceAllString(text, " "))
	return words == "" || silenceHallucinations[words]
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
