package orchestrator

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"voxflow/internal/config"
	"voxflow/internal/events"
	"voxflow/internal/hotkey"
	"voxflow/internal/injection"
	"voxflow/internal/llm"
	"voxflow/internal/macos"
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
		focusWarn string

		wantRefine  bool
		wantText    string
		wantMethod  string
		wantUsedRaw bool
		wantToast   string            // substring; "" means no toast
		wantSent    map[string]string // defaults to wantMethod=wantText
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
			name:      "missing Accessibility leaves the text copied",
			cfg:       &config.Config{RefinementMode: "raw"},
			injectErr: injection.ErrNoAccessibility,
			wantText:  "raw words", wantMethod: "clipboard", wantUsedRaw: true,
			wantToast: "Accessibility",
			wantSent:  map[string]string{"paste": "raw words"},
		},
		{
			name:      "other paste failures copy the text",
			cfg:       &config.Config{RefinementMode: "raw"},
			injectErr: errors.New("CGEventPost failed"),
			wantText:  "raw words", wantMethod: "clipboard", wantUsedRaw: true,
			wantToast: "text copied",
			wantSent:  map[string]string{"paste": "raw words", "clipboard": "raw words"},
		},
		{
			name:      "focus moved to another app copies instead of pasting",
			cfg:       &config.Config{RefinementMode: "raw"},
			focusWarn: "Focus moved to Slack — text copied, press ⌘V",
			wantText:  "raw words", wantMethod: "clipboard", wantUsedRaw: true,
			wantToast: "Focus moved to Slack",
		},
		{
			name:      "focus check does not block copy-only",
			cfg:       &config.Config{RefinementMode: "copy-only"},
			focusWarn: "Focus moved to Slack",
			wantText:  "raw words", wantMethod: "clipboard", wantUsedRaw: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref := tt.refiner
			rec := &recorder{}
			p := newTestPipeline(tt.cfg, &ref, rec, tt.injectErr)
			if tt.focusWarn != "" {
				p.focusTarget = func(macos.AppInfo) string { return tt.focusWarn }
			}

			d := p.deliver("raw words", app)

			if got := ref.calls > 0; got != tt.wantRefine {
				t.Errorf("refiner called = %v, want %v", got, tt.wantRefine)
			}
			if d.text != tt.wantText || d.method != tt.wantMethod || d.usedRaw != tt.wantUsedRaw {
				t.Errorf("got text=%q method=%q usedRaw=%v, want %q %q %v",
					d.text, d.method, d.usedRaw, tt.wantText, tt.wantMethod, tt.wantUsedRaw)
			}
			wantSent := tt.wantSent
			if wantSent == nil {
				wantSent = map[string]string{tt.wantMethod: tt.wantText}
			}
			if !reflect.DeepEqual(rec.sent, wantSent) {
				t.Errorf("sent %v, want %v", rec.sent, wantSent)
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

func TestStartRecordingWithoutModelReturnsToIdle(t *testing.T) {
	rec := &recorder{}
	p := newTestPipeline(&config.Config{}, &stubRefiner{}, rec, nil)
	p.modelReady = func() bool { return false }

	if err := p.StartRecording(); err == nil {
		t.Fatal("expected an error")
	}
	if p.State() != hotkey.StateIdle {
		t.Fatalf("state = %s, want Idle", p.State())
	}
	if !strings.Contains(strings.Join(rec.events, ","), events.StateChanged) {
		t.Fatal("the frontend was never told the state is Idle")
	}
}

func TestAssembleTranscript(t *testing.T) {
	const sec = 16000
	ok := func(from, to int, text string) span {
		return span{start: from * sec, end: to * sec, text: text, ok: true}
	}
	failed := func(from, to int) span { return span{start: from * sec, end: to * sec} }

	tests := []struct {
		name      string
		spans     []span
		total     int
		failAt    int // gap start (in seconds) whose transcription fails; -1 for none
		wantText  string
		wantCalls []string
		wantErr   bool
	}{
		{
			name:  "no streamed chunks transcribes everything once",
			total: 5 * sec, failAt: -1,
			wantText: "T0-5", wantCalls: []string{"0-5"},
		},
		{
			name:  "full coverage needs no extra call",
			spans: []span{ok(0, 8, "A"), ok(8, 16, "B")}, total: 16 * sec, failAt: -1,
			wantText: "A B",
		},
		{
			name:  "failed middle chunk is filled in place",
			spans: []span{ok(0, 8, "A"), failed(8, 16), ok(16, 20, "C")}, total: 20 * sec, failAt: -1,
			wantText: "A T8-16 C", wantCalls: []string{"8-16"},
		},
		{
			name:  "missing tail is transcribed",
			spans: []span{ok(0, 8, "A")}, total: 20 * sec, failAt: -1,
			wantText: "A T8-20", wantCalls: []string{"8-20"},
		},
		{
			name:  "chunks arriving out of order are merged by time",
			spans: []span{ok(8, 16, "B"), ok(0, 8, "A")}, total: 16 * sec, failAt: -1,
			wantText: "A B",
		},
		{
			name:  "silent chunk still counts as covered",
			spans: []span{ok(0, 8, ""), ok(8, 10, "B")}, total: 10 * sec, failAt: -1,
			wantText: "B",
		},
		{
			name:  "failed tail keeps the streamed text",
			spans: []span{ok(0, 8, "A")}, total: 20 * sec, failAt: 8,
			wantText: "A", wantCalls: []string{"8-20"}, wantErr: true,
		},
		{
			name:  "gap too short to transcribe is skipped",
			spans: []span{ok(0, 8, "A")}, total: 8*sec + minTranscribeSamples - 1, failAt: -1,
			wantText: "A",
		},
		{
			name:  "final chunk running past the buffer is fine",
			spans: []span{ok(0, 8, "A"), ok(8, 9, "B")}, total: 8*sec + 100, failAt: -1,
			wantText: "A B",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls []string
			got, err := assembleTranscript(tt.spans, tt.total, func(from, to int) (string, error) {
				calls = append(calls, fmt.Sprintf("%d-%d", from/sec, to/sec))
				if from == tt.failAt*sec {
					return "", errors.New("whisper down")
				}
				return fmt.Sprintf(" T%d-%d ", from/sec, to/sec), nil
			})
			if got != tt.wantText {
				t.Errorf("text = %q, want %q", got, tt.wantText)
			}
			if fmt.Sprint(calls) != fmt.Sprint(tt.wantCalls) {
				t.Errorf("transcribed %v, want %v", calls, tt.wantCalls)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestStreamSessionIgnoresLateChunks(t *testing.T) {
	s := &streamSession{jobs: make(chan streamJob, 1)}
	s.send(streamJob{Samples: []int16{1}, IsFinal: true})
	s.send(streamJob{Samples: []int16{2}, IsFinal: true}) // full: replaces the oldest
	s.close(false)
	s.send(streamJob{Samples: []int16{3}, IsFinal: true}) // after close: must not panic

	var got []int16
	for job := range s.jobs {
		got = append(got, job.Samples[0])
	}
	if fmt.Sprint(got) != "[2]" {
		t.Fatalf("queued %v, want [2]", got)
	}
}

func TestStartRecordingWithMicrophoneDeniedReturnsToIdle(t *testing.T) {
	rec := &recorder{}
	p := newTestPipeline(&config.Config{}, &stubRefiner{}, rec, nil)
	p.micStatus = func() string { return "denied" }

	if err := p.StartRecording(); err == nil {
		t.Fatal("expected an error")
	}
	if p.State() != hotkey.StateIdle {
		t.Fatalf("state = %s, want Idle", p.State())
	}
	if len(rec.toasts) != 1 || rec.toasts[0]["settings"] != "microphone" {
		t.Fatalf("toasts = %v, want one pointing at the microphone settings", rec.toasts)
	}
}
