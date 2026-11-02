package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func mockHTTP(t *testing.T, response func(*http.Request) (*http.Response, error)) {
	t.Helper()
	old := client
	client = &http.Client{Transport: roundTripFunc(response)}
	t.Cleanup(func() { client = old })
}

func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Status: fmt.Sprintf("%d", status), Body: io.NopCloser(strings.NewReader(body))}
}

func testAsset() releaseAsset {
	name := "voxflow-" + runtime.GOOS + "-" + runtime.GOARCH + ".zip"
	return releaseAsset{Name: name, URL: "https://github.com/" + repository + "/releases/download/v1.2.3/" + name,
		Size: 100, Digest: "sha256:" + strings.Repeat("a", 64)}
}

func TestCheckRelease(t *testing.T) {
	for _, test := range []struct {
		name                   string
		patch                  func(map[string]any)
		available, installable bool
	}{
		{"verified asset", func(map[string]any) {}, true, runtime.GOOS == "darwin"},
		{"missing archive", func(r map[string]any) { r["assets"] = []releaseAsset{} }, true, false},
		{"missing digest", func(r map[string]any) { a := testAsset(); a.Digest = ""; r["assets"] = []releaseAsset{a} }, true, false},
		{"wrong download origin", func(r map[string]any) {
			a := testAsset()
			a.URL = "https://example.com/update.zip"
			r["assets"] = []releaseAsset{a}
		}, true, false},
		{"prerelease", func(r map[string]any) { r["prerelease"] = true }, false, false},
		{"draft", func(r map[string]any) { r["draft"] = true }, false, false},
		{"old release", func(r map[string]any) { r["tag_name"] = "v0.9.0" }, false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			release := map[string]any{"tag_name": "v1.2.3", "html_url": "https://example.com/untrusted", "body": "## What's new\n- Faster startup\n- Fix [recording](https://example.com)", "assets": []releaseAsset{testAsset()}}
			test.patch(release)
			data, _ := json.Marshal(release)
			mockHTTP(t, func(req *http.Request) (*http.Response, error) {
				if req.URL.String() != latestURL || req.Header.Get("User-Agent") == "" {
					t.Fatalf("unexpected request: %s", req.URL)
				}
				return response(200, string(data)), nil
			})
			info, err := Check(context.Background(), "1.0.0")
			if err != nil || info.Available != test.available || info.CanInstall != test.installable {
				t.Fatalf("info = %+v, err = %v", info, err)
			}
			if len(info.Summary) != 2 || info.Summary[1] != "Fix recording" || strings.Contains(info.URL, "example.com") {
				t.Fatalf("unexpected release display: %+v", info)
			}
		})
	}
}

func TestCheckErrors(t *testing.T) {
	for _, test := range []struct {
		status int
		body   string
	}{{403, "rate limited"}, {500, "server error"}, {200, "not json"}} {
		t.Run(fmt.Sprint(test.status, test.body), func(t *testing.T) {
			mockHTTP(t, func(*http.Request) (*http.Response, error) { return response(test.status, test.body), nil })
			if _, err := Check(context.Background(), "1.0.0"); err == nil {
				t.Fatal("expected a check error")
			}
		})
	}
}

func TestSummaryBounds(t *testing.T) {
	summary := summarize("# heading\nFull Changelog: https://example.com\n- **" + strings.Repeat("界", 200) + "**\n" + strings.Repeat("* `Fix` [bug](https://example.com)\n", 9))
	if len(summary) != 5 || len([]rune(summary[0])) > 180 || summary[1] != "Fix bug" {
		t.Fatalf("unexpected summary: %#v", summary)
	}
}

func TestNewer(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"1.2.3", "1.2.2", true},
		{"v1.2.3", "1.2.3", false},
		{"1.2.3", "v1.2.3", false},
		{"1.10.0", "1.9.9", true},
		{"1.9.9", "1.10.0", false},
		{"2.0", "1.99.99", true},
		{"1.2", "1.2.0", false},
		{"1.2.1", "1.2", true},
		{"1.3.0-beta.1", "1.2.9", true},
		{"1.2.3", "1.2.3-rc1", false},
		{"dev", "1.0.0", false},
		{"1.0.0", "dev", false},
		{"", "1.0.0", false},
		{"1.0.x", "0.9", false},
	}
	for _, tt := range tests {
		if got := Newer(tt.a, tt.b); got != tt.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}
