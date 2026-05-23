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
	rawText := "Hello world"

	tests := []struct {
		name           string
		input          string
		expectedResult string
		expectedOk     bool
		expectedParsed bool
	}{
		{
			name:           "Valid refined text response",
			input:          `{"text": "Polished text", "refused": false, "ok_to_go": false}`,
			expectedResult: "Polished text",
			expectedOk:     false,
			expectedParsed: true,
		},
		{
			name:           "Refused response",
			input:          `{"text": "", "refused": true, "ok_to_go": false}`,
			expectedResult: rawText,
			expectedOk:     false,
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
			name:           "Invalid JSON response",
			input:          `Not a JSON block`,
			expectedResult: "",
			expectedOk:     false,
			expectedParsed: false,
		},
		{
			name:           "Valid JSON with markdown fences wrapper",
			input:          "```json\n{\"text\": \"Polished\", \"refused\": false, \"ok_to_go\": false}\n```",
			expectedResult: "Polished",
			expectedOk:     false,
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
