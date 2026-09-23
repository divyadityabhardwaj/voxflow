package orchestrator

import (
	"errors"
	"strings"
	"testing"
	"voxflow/internal/config"
	"voxflow/internal/events"
	"voxflow/internal/llm"
)

type stubRefiner struct {
	text   string
	okToGo bool
	err    error
	calls  int
}

func (r *stubRefiner) RefineText(raw, model string) (string, int, bool, error) {
	r.calls++
	return r.text, 7, r.okToGo, r.err
}
func (r *stubRefiner) CheckModel(string) (int64, float64, error)                   { return 0, 0, nil }
func (r *stubRefiner) RetryWithInstruction(string, string, string) (string, error) { return "", nil }
func (r *stubRefiner) Prewarm(string)                                              {}

type recorder struct {
	events []string
	toasts []map[string]interface{}
	sent   map[string]string // method -> text
}

func newTestPipeline(cfg *config.Config, ref llm.Refiner, rec *recorder, injectErr error) *Pipeline {
	rec.sent = map[string]string{}
	send := func(method string, err error) func(string) error {
		return func(text string) error {
			rec.sent[method] = text
			return err
		}
	}
	return &Pipeline{
		config:  cfg,
		refiner: func() llm.Refiner { return ref },
		emit: func(name string, data ...interface{}) {
			rec.events = append(rec.events, name)
			if name == events.Toast {
				rec.toasts = append(rec.toasts, data[0].(map[string]interface{}))
			}
		},
		inject:   send("paste", injectErr),
		typeText: send("type", injectErr),
		copyText: send("clipboard", nil),
	}
}

func TestDeliver(t *testing.T) {
	for _, env := range []string{"GEMINI_API_KEY", "OPENROUTER_API_KEY", "GROQ_API_KEY", "CEREBRAS_API_KEY"} {
		t.Setenv(env, "")
	}
	const app = "com.apple.Terminal"
	longErr := errors.New(strings.Repeat("<html>proxy page</html>", 50))

	tests := []struct {
		name      string
		cfg       *config.Config
		refiner   stubRefiner
		injectErr error

		wantRefine  bool
		wantText    string
		wantMethod  string
		wantUsedRaw bool
		wantToast   string // substring; "" means no toast
	}{
		{
			name:       "refine with key pastes refined text",
			cfg:        &config.Config{RefinementMode: "refine", GeminiAPIKey: "k"},
			refiner:    stubRefiner{text: "Refined."},
			wantRefine: true, wantText: "Refined.", wantMethod: "paste",
		},
		{
			name:     "raw mode never calls the LLM",
			cfg:      &config.Config{RefinementMode: "raw", GeminiAPIKey: "k"},
			wantText: "raw words", wantMethod: "paste", wantUsedRaw: true,
		},
		{
			name:     "missing key skips refinement silently",
			cfg:      &config.Config{RefinementMode: "refine"},
			wantText: "raw words", wantMethod: "paste", wantUsedRaw: true,
		},
		{
			name:     "key for another provider does not count",
			cfg:      &config.Config{RefinementMode: "refine", LLMProvider: "groq", GeminiAPIKey: "k"},
			wantText: "raw words", wantMethod: "paste", wantUsedRaw: true,
		},
		{
			name:       "local provider needs only a URL",
			cfg:        &config.Config{RefinementMode: "refine", LLMProvider: "local", LocalURL: "http://localhost:11434"},
			refiner:    stubRefiner{text: "Refined."},
			wantRefine: true, wantText: "Refined.", wantMethod: "paste",
		},
		{
			name:       "refine error pastes raw with a short warning",
			cfg:        &config.Config{RefinementMode: "refine", GeminiAPIKey: "k"},
			refiner:    stubRefiner{err: longErr},
			wantRefine: true, wantText: "raw words", wantMethod: "paste",
			wantToast: "gemini error: <html>",
		},
		{
			name:       "ok_to_go keeps raw text",
			cfg:        &config.Config{RefinementMode: "refine", GeminiAPIKey: "k"},
			refiner:    stubRefiner{text: "ignored", okToGo: true},
			wantRefine: true, wantText: "raw words", wantMethod: "paste", wantUsedRaw: true,
		},
		{
			name:       "empty refinement falls back to raw",
			cfg:        &config.Config{RefinementMode: "refine", GeminiAPIKey: "k"},
			wantRefine: true, wantText: "raw words", wantMethod: "paste",
			wantToast: "LLM refining failed",
		},
		{
			name: "app rule overrides the global mode",
			cfg: &config.Config{RefinementMode: "refine", GeminiAPIKey: "k",
				AppRules: map[string]config.AppRule{app: {RefinementMode: "raw"}}},
			wantText: "raw words", wantMethod: "paste", wantUsedRaw: true,
		},
		{
			name: "app rule can type instead of paste",
			cfg: &config.Config{RefinementMode: "raw",
				AppRules: map[string]config.AppRule{app: {InjectMethod: "type"}}},
			wantText: "raw words", wantMethod: "type", wantUsedRaw: true,
		},
		{
			name: "copy-only beats a per-app type rule",
			cfg: &config.Config{RefinementMode: "copy-only",
				AppRules: map[string]config.AppRule{app: {InjectMethod: "type"}}},
			wantText: "raw words", wantMethod: "clipboard", wantUsedRaw: true,
		},
		{
			name:      "injection failure is reported",
			cfg:       &config.Config{RefinementMode: "raw"},
			injectErr: errors.New("not trusted"),
			wantText:  "raw words", wantMethod: "paste", wantUsedRaw: true,
			wantToast: "Accessibility",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref := tt.refiner
			rec := &recorder{}
			p := newTestPipeline(tt.cfg, &ref, rec, tt.injectErr)

			d := p.deliver("raw words", app)

			if got := ref.calls > 0; got != tt.wantRefine {
				t.Errorf("refiner called = %v, want %v", got, tt.wantRefine)
			}
			if d.text != tt.wantText || d.method != tt.wantMethod || d.usedRaw != tt.wantUsedRaw {
				t.Errorf("got text=%q method=%q usedRaw=%v, want %q %q %v",
					d.text, d.method, d.usedRaw, tt.wantText, tt.wantMethod, tt.wantUsedRaw)
			}
			if len(rec.sent) != 1 || rec.sent[tt.wantMethod] != tt.wantText {
				t.Errorf("sent %v, want only %s=%q", rec.sent, tt.wantMethod, tt.wantText)
			}

			switch {
			case tt.wantToast == "" && len(rec.toasts) > 0:
				t.Errorf("unexpected toast %v", rec.toasts)
			case tt.wantToast != "":
				if len(rec.toasts) != 1 {
					t.Fatalf("toasts = %v, want one containing %q", rec.toasts, tt.wantToast)
				}
				msg := rec.toasts[0]["message"].(string)
				if !strings.Contains(msg, tt.wantToast) {
					t.Errorf("toast %q does not contain %q", msg, tt.wantToast)
				}
				if n := len([]rune(msg)); n > maxToastDetail+100 {
					t.Errorf("toast is %d runes long", n)
				}
			}
		})
	}
}
