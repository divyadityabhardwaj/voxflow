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

		{"Thank you.", "Thank you."},
		{"Bye-bye.", "Bye-bye."},
		{"[BLANK_AUDIO] Thank you.", "Thank you."},
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

func TestCleanWhisperSpan_dropsStockPhrasesOnlyWithoutSpeech(t *testing.T) {
	silent := make([]int16, 16000)
	speech := make([]int16, 16000)
	for i := range speech {
		speech[i] = int16(3000 * (1 - 2*(i%2)))
	}
	testCases := []struct {
		input   string
		samples []int16
		want    string
	}{
		{"Thank you.", silent, ""},
		{" thank you!! ", silent, ""},
		{"Thanks for watching!", silent, ""},
		{"you", silent, ""},
		{"Bye-bye.", silent, ""},
		{"Subtitles by the Amara.org community", silent, ""},
		{"(upbeat music) Thank you.", silent, ""},
		{"ご視聴ありがとうございました", silent, ""},
		{"[Music]", speech, ""},
		{"Thank you for the update.", silent, "Thank you for the update."},
		{"Thank you.", speech, "Thank you."},
		{"Thanks.", speech, "Thanks."},
		{"Bye.", speech, "Bye."},
		{"[BLANK_AUDIO] Thank you.", speech, "Thank you."},
	}
	for _, tc := range testCases {
		if got := cleanWhisperSpan(tc.input, tc.samples); got != tc.want {
			t.Errorf("cleanWhisperSpan(%q) = %q, want %q", tc.input, got, tc.want)
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
		{"Thank you.", vocab, "Thank you."},
		{"", vocab, ""},
	}
	for _, tc := range testCases {
		if got := cleanWhisperChunk(tc.input, tc.vocab); got != tc.want {
			t.Errorf("cleanWhisperChunk(%q, %q) = %q, want %q", tc.input, tc.vocab, got, tc.want)
		}
	}
}
