package llm

import (
	"testing"
)

func TestStripCodeFences(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "No code fences",
			input:    "Hello world",
			expected: "Hello world",
		},
		{
			name:     "Markdown json fences",
			input:    "```json\n{\n  \"text\": \"Hello\"\n}\n```",
			expected: "{\n  \"text\": \"Hello\"\n}",
		},
		{
			name:     "Generic markdown fences",
			input:    "```\nsome plain text\n```",
			expected: "some plain text",
		},
		{
			name:     "Fences with spaces",
			input:    "   ```json  \n  spaced text\n  ```   ",
			expected: "spaced text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripCodeFences(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestParseRefineResponse(t *testing.T) {
	rawText := "um hello world"

	tests := []struct {
		name           string
		input          string
		expectedResult string
		expectedOk     bool
		expectedParsed bool
	}{
		{
			name:           "Valid refined text response",
			input:          `{"text": "Hello world.", "refused": false, "ok_to_go": false}`,
			expectedResult: "Hello world.",
			expectedParsed: true,
		},
		{
			name:           "Refused response",
			input:          `{"text": "", "refused": true, "ok_to_go": false}`,
			expectedResult: rawText,
			expectedParsed: true,
		},
		{
			name:           "Ok to go response",
			input:          `{"text": "", "refused": false, "ok_to_go": true}`,
			expectedResult: rawText,
			expectedOk:     true,
			expectedParsed: true,
		},
		{
			name:  "Plain prose instead of JSON",
			input: `Hello world.`,
		},
		{
			name:  "Truncated JSON",
			input: `{"text": "Hello wor`,
		},
		{
			name:  "Empty reply",
			input: "   ",
		},
		{
			name:           "Valid JSON with markdown fences wrapper",
			input:          "```json\n{\"text\": \"Hello world.\", \"refused\": false, \"ok_to_go\": false}\n```",
			expectedResult: "Hello world.",
			expectedParsed: true,
		},
		{
			name:           "Chatty preamble before JSON",
			input:          "Sure! Here is the refined text:\n{\"text\": \"Hello world.\", \"refused\": false, \"ok_to_go\": false}",
			expectedResult: "Hello world.",
			expectedParsed: true,
		},
		{
			name:           "Trailing prose after JSON",
			input:          "{\"text\": \"Hello world.\", \"refused\": false, \"ok_to_go\": false}\nLet me know if you need anything else!",
			expectedResult: "Hello world.",
			expectedParsed: true,
		},
		{
			name:           "Think block before JSON",
			input:          "<think>The user said hello. {maybe} I should clean it.</think>\n{\"text\": \"Hello world.\", \"refused\": false, \"ok_to_go\": false}",
			expectedResult: "Hello world.",
			expectedParsed: true,
		},
		{
			name:  "Unterminated think block",
			input: "<think>The user said hello, so the answer is {\"text\": \"Hello",
		},
		{
			name:           "Answer instead of cleanup falls back to raw",
			input:          `{"text": "Sure! Here is a friendly greeting you could send to everyone today.", "refused": false, "ok_to_go": false}`,
			expectedResult: rawText,
			expectedParsed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok, parsed := ParseRefineResponse(tt.input, rawText)
			if result != tt.expectedResult {
				t.Errorf("expected result %q, got %q", tt.expectedResult, result)
			}
			if ok != tt.expectedOk {
				t.Errorf("expected ok %v, got %v", tt.expectedOk, ok)
			}
			if parsed != tt.expectedParsed {
				t.Errorf("expected parsed %v, got %v", tt.expectedParsed, parsed)
			}
		})
	}
}

func TestPlausibleRefinement(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		refined string
		want    bool
	}{
		{
			name:    "filler removal",
			raw:     "um so like I was thinking you know we should uh ship it on friday",
			refined: "I was thinking we should ship it on Friday.",
			want:    true,
		},
		{
			name:    "list formatting adds newlines and bullets",
			raw:     "um for the trip first we need milk second eggs and third uh bread",
			refined: "For the trip:\n• We need milk\n• Eggs\n• Bread",
			want:    true,
		},
		{
			name:    "voice commands and email formatting",
			raw:     "send it to john at example dot com period thanks",
			refined: "Send it to john@example.com. Thanks",
			want:    true,
		},
		{
			name:    "voice command with no words left",
			raw:     "question mark",
			refined: "?",
			want:    true,
		},
		{
			name:    "one corrected character in unspaced script",
			raw:     "我今天去商店买东西",
			refined: "我今天去商店买东西。",
			want:    true,
		},
		{
			name:    "spoken percentage to digits",
			raw:     "twenty five percent",
			refined: "25%",
			want:    true,
		},
		{
			name:    "spoken port number to digits",
			raw:     "set the port to eight zero eight zero",
			refined: "Set the port to 8080.",
			want:    true,
		},
		{
			name:    "mostly digits after conversion",
			raw:     "twenty twenty four budget ninety nine thousand",
			refined: "2024 budget: 99,000",
			want:    true,
		},
		{
			name:    "digits do not excuse an answer",
			raw:     "what year did the war end",
			refined: "The Second World War ended in 1945.",
			want:    false,
		},
		{
			name:    "short contraction",
			raw:     "do not",
			refined: "Don't.",
			want:    true,
		},
		{
			name:    "chatty preamble",
			raw:     "hello there",
			refined: "Sure! Here is the cleaned up version of your text: Hello there.",
			want:    false,
		},
		{
			name:    "answered question",
			raw:     "what is the capital of france",
			refined: "Paris is the capital and most populous city of France, with an estimated population of over two million residents in the city proper.",
			want:    false,
		},
		{
			name:    "obeyed instruction",
			raw:     "write me a poem about the sea",
			refined: "Waves roll in beneath a silver moon, / Whispering secrets, a gentle tune.",
			want:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := plausibleRefinement(tt.raw, tt.refined); got != tt.want {
				t.Errorf("plausibleRefinement(%q, %q) = %v, want %v", tt.raw, tt.refined, got, tt.want)
			}
		})
	}
}

func TestRefineMaxTokens(t *testing.T) {
	if got := RefineMaxTokens("short"); got != 768 {
		t.Errorf("floor should be 768, got %d", got)
	}
	long := make([]byte, 6000)
	if got := RefineMaxTokens(string(long)); got != 3256 {
		t.Errorf("6000 chars should give 3256, got %d", got)
	}
}
