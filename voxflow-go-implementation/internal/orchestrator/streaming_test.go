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
	got := cleanWhisperText("hello [MUSIC] world")
	if got != "hello world" {
		t.Fatalf("cleanWhisperText() = %q, want %q", got, "hello world")
	}
}
