package llm

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"syscall"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type reply struct {
	status     int
	retryAfter string
	body       string
	err        error
}

func TestDoWithRetryContext(t *testing.T) {
	offline := &net.OpError{Op: "dial", Net: "tcp", Err: &net.DNSError{Err: "no such host", Name: "api.example.com", IsNotFound: true}}
	reset := &net.OpError{Op: "read", Net: "tcp", Err: syscall.ECONNRESET}

	tests := []struct {
		name       string
		replies    []reply // the last one repeats
		retries    int
		budget     time.Duration
		wantCalls  int
		wantStatus int
		wantErr    bool
	}{
		{name: "offline fails at once", replies: []reply{{err: offline}}, retries: 3, wantCalls: 1, wantErr: true},
		{name: "dropped connection is retried", replies: []reply{{err: reset}, {status: 200}}, retries: 1, wantCalls: 2, wantStatus: 200},
		{name: "503 retried then succeeds", replies: []reply{{status: 503}, {status: 200}}, retries: 1, wantCalls: 2, wantStatus: 200},
		{name: "503 retried at most once on dictation path", replies: []reply{{status: 503}}, retries: 1, wantCalls: 2, wantStatus: 503},
		{name: "no retry that would outlive the budget", replies: []reply{{status: 503}}, retries: 3, budget: 500 * time.Millisecond, wantCalls: 1, wantStatus: 503},
		{name: "429 with long Retry-After is final", replies: []reply{{status: 429, retryAfter: "30"}}, retries: 3, wantCalls: 1, wantStatus: 429},
		{name: "429 for a daily quota is final", replies: []reply{{status: 429, body: `{"error":{"message":"Rate limit exceeded: free-models-per-day"}}`}}, retries: 3, wantCalls: 1, wantStatus: 429},
		{name: "429 for exhausted quota is final", replies: []reply{{status: 429, body: `{"error":{"message":"You exceeded your current quota"}}`}}, retries: 3, wantCalls: 1, wantStatus: 429},
		{name: "short 429 is retried", replies: []reply{{status: 429, retryAfter: "0"}, {status: 200}}, retries: 1, wantCalls: 2, wantStatus: 200},
		{name: "400 is not retried", replies: []reply{{status: 400}}, retries: 3, wantCalls: 1, wantStatus: 400},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			hc := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				r := tt.replies[min(calls, len(tt.replies)-1)]
				calls++
				if r.err != nil {
					return nil, r.err
				}
				h := http.Header{}
				if r.retryAfter != "" {
					h.Set("Retry-After", r.retryAfter)
				}
				return &http.Response{StatusCode: r.status, Header: h, Body: io.NopCloser(strings.NewReader(r.body))}, nil
			})}

			ctx := context.Background()
			if tt.budget > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, tt.budget)
				defer cancel()
			}
			_, status, err := DoWithRetryContext(ctx, hc, tt.retries, func() (*http.Request, error) {
				return http.NewRequest("POST", "https://api.example.com/v1/chat/completions", nil)
			})
			if calls != tt.wantCalls {
				t.Errorf("calls = %d, want %d", calls, tt.wantCalls)
			}
			if status != tt.wantStatus {
				t.Errorf("status = %d, want %d", status, tt.wantStatus)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestStatusError(t *testing.T) {
	long := strings.Repeat("<p>captive portal</p>", 50)
	tests := []struct {
		name    string
		status  int
		body    string
		want    string
		notWant string
	}{
		{name: "provider message preferred", status: 401, body: `{"error":{"message":"Invalid API key"},"echo":"my private dictation"}`, want: "API error (status 401): Invalid API key", notWant: "private"},
		{name: "ollama string error", status: 404, body: `{"error":"model \"llama3\" not found"}`, want: `API error (status 404): model "llama3" not found`},
		{name: "empty body", status: 500, want: "API error (status 500)"},
		{name: "rate limit", status: 429, body: `{"error":{"message":"slow down"}}`, want: "rate limit reached"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StatusError(tt.status, []byte(tt.body)).Error()
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
			if tt.notWant != "" && strings.Contains(got, tt.notWant) {
				t.Errorf("%q leaks %q", got, tt.notWant)
			}
		})
	}

	if !errors.Is(StatusError(429, nil), ErrRateLimited) {
		t.Error("429 should be ErrRateLimited")
	}
	if got := StatusError(502, []byte(long)).Error(); len(got) > maxErrorLen+40 {
		t.Errorf("long body not truncated: %d bytes", len(got))
	}
}

func TestRefineBudget(t *testing.T) {
	tests := []struct {
		words int
		want  time.Duration
	}{
		{0, 4 * time.Second},
		{50, 6 * time.Second},
		{1000, 15 * time.Second},
	}
	for _, tt := range tests {
		if got := RefineBudget(strings.Repeat("word ", tt.words)); got != tt.want {
			t.Errorf("RefineBudget(%d words) = %v, want %v", tt.words, got, tt.want)
		}
	}
}
