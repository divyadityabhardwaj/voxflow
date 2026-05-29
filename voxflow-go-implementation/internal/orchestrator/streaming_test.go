package orchestrator

import (
	"testing"
	"time"
)

func TestMergeStreamingChunks_dedupesOverlap(t *testing.T) {
	chunks := []streamChunk{
		{Start: 0, Text: "hello world"},
		{Start: time.Second, Text: "world again"},
	}
	got := mergeStreamingChunks(chunks)
	want := "hello world again"
	if got != want {
		t.Fatalf("mergeStreamingChunks() = %q, want %q", got, want)
	}
}

func TestCleanWhisperText_stripsNoiseMarkers(t *testing.T) {
	testCases := []struct {
		input string
		want  string
	}{
		{"hello [MUSIC] world", "hello world"},
		{"hello [BLANK_AUDIO] world", "hello world"},
		{"do so. [BLANK_AUDIO]", "do so."},
		{"hello (blank audio) world", "hello world"},
		{"hello [NO_SPEECH] world", "hello world"},
		{"hello [NO SPEECH] world", "hello world"},
		{"[NOISE] hello", "hello"},
	}

	for _, tc := range testCases {
		got := cleanWhisperText(tc.input)
		if got != tc.want {
			t.Errorf("cleanWhisperText(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
