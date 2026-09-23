package orchestrator

import (
	"testing"
	"time"
)

func TestMergeStreamingChunks_keepsRepeatedBoundaryWord(t *testing.T) {
	chunks := []streamChunk{
		{Start: 8 * time.Second, Text: "that is right."},
		{Start: 12 * time.Second, Text: ""},
		{Start: 0, Text: "I think that"},
	}
	got := mergeStreamingChunks(chunks)
	want := "I think that that is right."
	if got != want {
		t.Fatalf("mergeStreamingChunks() = %q, want %q", got, want)
	}
}

func TestCleanWhisperText(t *testing.T) {
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
		{"", ""},

		{"Thank you.", ""},
		{" thank you!! ", ""},
		{"Thanks for watching!", ""},
		{"Thank you for watching.", ""},
		{"you", ""},
		{"Bye-bye.", ""},
		{"Subtitles by the Amara.org community", ""},
		{"[BLANK_AUDIO] Thank you.", ""},
		{"(upbeat music) Thank you.", ""},
		{"ご視聴ありがとうございました", ""},
		{"[Music]", ""},
		{"(water running)", ""},
		{"*sighs*", ""},
		{"♪ ♪", ""},
		{"...", ""},

		{"Thank you for the update.", "Thank you for the update."},
		{"You should see this.", "You should see this."},
		{"(laughs) That's great.", "(laughs) That's great."},
		{"Thank you. See you at noon.", "Thank you. See you at noon."},
	}

	for _, tc := range testCases {
		if got := cleanWhisperText(tc.input); got != tc.want {
			t.Errorf("cleanWhisperText(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestCleanWhisperChunk_dropsVocabularyEcho(t *testing.T) {
	const vocab = "Kubernetes, VoxFlow, Terraform, gRPC, Claude Code"
	testCases := []struct {
		input, vocab, want string
	}{
		{"Kubernetes,", vocab, ""},
		{"VoxFlow", vocab, ""},
		{"Kubernetes, VoxFlow.", vocab, ""},
		{"Claude Code", vocab, ""},
		{"[Music] Kubernetes,", vocab, ""},
		{"Kubernetes, VoxFlow, Terraform, gRPC, Claude", vocab, "Kubernetes, VoxFlow, Terraform, gRPC, Claude"},
		{"Deploy it to Kubernetes.", vocab, "Deploy it to Kubernetes."},
		{"Kubernetes,", "", "Kubernetes,"},
		{"Thank you.", vocab, ""},
		{"", vocab, ""},
	}
	for _, tc := range testCases {
		if got := cleanWhisperChunk(tc.input, tc.vocab); got != tc.want {
			t.Errorf("cleanWhisperChunk(%q, %q) = %q, want %q", tc.input, tc.vocab, got, tc.want)
		}
	}
}
