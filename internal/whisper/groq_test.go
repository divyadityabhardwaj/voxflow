package whisper

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func withGroqServer(t *testing.T, h http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	old := groqBaseURL
	groqBaseURL = srv.URL
	t.Cleanup(func() { groqBaseURL = old })
}

func TestTranscribeGroq(t *testing.T) {
	withGroqServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer k" || r.FormValue("model") != GroqModel || r.FormValue("language") != "en" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if _, _, err := r.FormFile("file"); err != nil {
			http.Error(w, "no file", http.StatusBadRequest)
			return
		}
		w.Write([]byte(`{"text":" Hello there. "}`))
	})
	text, err := TranscribeGroq("k", make([]int16, 1600), "en", "VoxFlow")
	if err != nil || text != "Hello there." {
		t.Fatalf("got %q, %v", text, err)
	}
}

func TestTranscribeGroqErrors(t *testing.T) {
	if _, err := TranscribeGroq("", nil, "en", ""); err != ErrNoGroqKey {
		t.Fatalf("no key: %v", err)
	}
	cases := []struct {
		status     int
		retryAfter string
		body       string
		want       string
	}{
		{429, "", `{"error":{"message":"Rate limit reached ... Please try again in 7m12.5s. Need more tokens?"}}`, "used up — it resets in 8 minutes"},
		{429, "42", `{}`, "resets in 42 seconds"},
		{429, "", `{}`, "used up for now"},
		{401, "", `{"error":{"message":"Invalid API Key"}}`, "rejected your API key"},
		{503, "", ``, "having trouble (error 503)"},
	}
	for _, c := range cases {
		withGroqServer(t, func(w http.ResponseWriter, r *http.Request) {
			if c.retryAfter != "" {
				w.Header().Set("Retry-After", c.retryAfter)
			}
			w.WriteHeader(c.status)
			w.Write([]byte(c.body))
		})
		_, err := TranscribeGroq("k", make([]int16, 1600), "en", "")
		var cloud *CloudError
		if !errors.As(err, &cloud) || !strings.Contains(cloud.Message, c.want) {
			t.Errorf("status %d: got %v, want message containing %q", c.status, err, c.want)
		}
	}
}

func TestFormatWait(t *testing.T) {
	for d, want := range map[string]string{"30s": "30 seconds", "61s": "2 minutes", "1h": "1 h", "2h30m": "2 h 30 min"} {
		if got := formatWait(mustDuration(t, d)); got != want {
			t.Errorf("formatWait(%s) = %q, want %q", d, got, want)
		}
	}
}

func mustDuration(t *testing.T, s string) (d time.Duration) {
	t.Helper()
	d, err := time.ParseDuration(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestGroqCancellationStopsRequest(t *testing.T) {
	started := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		close(started)
		<-r.Context().Done()
	}))
	defer srv.Close()
	old := groqBaseURL
	groqBaseURL = srv.URL
	defer func() { groqBaseURL = old }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := TranscribeGroqContext(ctx, "key", make([]int16, 1600), "en", ""); done <- err }()
	<-started
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("request did not cancel")
	}
}
