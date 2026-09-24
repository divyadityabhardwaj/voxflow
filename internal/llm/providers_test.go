package llm

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

func TestProviders(t *testing.T) {
	models := map[string]struct{ listed, want []string }{
		"gemini": {
			listed: []string{"models/gemini-3.5-flash", "models/gemini-3.8-flash-tts", "models/gemini-embedding-2", "models/gemma-4-31b-it", "models/gemini-3.8-live-extended-thinking"},
			want:   []string{"gemini-3.5-flash"},
		},
		"openrouter": {
			listed: []string{"b/model:free", "a/model", "c/thinking-model:free"},
			want:   []string{"b/model:free"},
		},
		"groq": {
			listed: []string{"whisper-large-v3", "openai/gpt-oss-20b", "llama-guard-4", "llama-3.1-8b-instant"},
			want:   []string{"llama-3.1-8b-instant", "openai/gpt-oss-20b"},
		},
		"cerebras": {
			listed: []string{"llama3.1-8b", "text-embedding"},
			want:   []string{"llama3.1-8b"},
		},
		"local": {
			listed: []string{"qwen3:4b"},
			want:   []string{"qwen3:4b"},
		},
	}

	for _, p := range Providers {
		t.Run(p.ID, func(t *testing.T) {
			var chat map[string]any
			var headers http.Header
			var modelsURL string
			fail := false
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				headers = r.Header.Clone()
				if fail {
					w.WriteHeader(http.StatusUnauthorized)
					w.Write([]byte(`[{"error":{"code":401,"message":"API key not valid"}}]`))
					return
				}
				switch {
				case strings.HasSuffix(r.URL.Path, "/chat/completions"):
					json.NewDecoder(r.Body).Decode(&chat)
					w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"{\"text\":\"Hello there.\",\"refused\":false,\"ok_to_go\":false}"}}],"usage":{"completion_tokens":5}}`))
				case strings.HasSuffix(r.URL.Path, "/models"):
					modelsURL = r.URL.String()
					var data []map[string]string
					for _, id := range models[p.ID].listed {
						data = append(data, map[string]string{"id": id})
					}
					json.NewEncoder(w).Encode(map[string]any{"data": data})
				default:
					http.NotFound(w, r)
				}
			}))
			defer srv.Close()

			c := NewClient(p, "")
			if p.NeedsKey {
				if _, _, _, err := c.RefineText("hello there", "m"); !errors.Is(err, ErrNoAPIKey) {
					t.Fatalf("no key: err = %v, want ErrNoAPIKey", err)
				}
				c.APIKey = "secret"
			}
			if p.Local {
				c.SetServerURL(srv.URL + "/")
			} else {
				c.BaseURL = srv.URL + "/v1"
			}

			text, tokens, _, err := c.RefineText("hello there", "some-model")
			if err != nil || text != "Hello there." || tokens != 5 {
				t.Fatalf("RefineText = %q, %d, %v", text, tokens, err)
			}
			if chat["model"] != "some-model" || len(chat["messages"].([]any)) != 2 {
				t.Errorf("request body = %v", chat)
			}
			if got, want := headers.Get("Authorization"), map[bool]string{true: "Bearer secret"}[p.NeedsKey]; got != want {
				t.Errorf("Authorization = %q, want %q", got, want)
			}
			for k, v := range p.Headers {
				if headers.Get(k) != v {
					t.Errorf("header %s = %q, want %q", k, headers.Get(k), v)
				}
			}
			if _, ok := chat["reasoning"]; ok != p.DisableReasoning {
				t.Errorf("reasoning sent = %v, want %v", ok, p.DisableReasoning)
			}

			got, err := c.GetModels()
			if err != nil || !slices.Equal(got, models[p.ID].want) {
				t.Errorf("GetModels = %v, %v; want %v", got, err, models[p.ID].want)
			}
			if !strings.HasSuffix(modelsURL, "/v1/models"+p.ModelsQuery) {
				t.Errorf("models URL = %s", modelsURL)
			}

			fail = true
			if _, _, _, err := c.RefineText("hello there", "some-model"); err == nil || err.Error() != "API error (status 401): API key not valid" {
				t.Errorf("error = %v", err)
			}
			if got, err := c.GetModels(); err == nil || !slices.Equal(got, p.FallbackModels) {
				t.Errorf("failed GetModels = %v, %v; want fallback", got, err)
			}
		})
	}
}
