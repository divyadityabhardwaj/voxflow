package whisper

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestWhisperCLICandidatesPreferTheAppBundle(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	got := whisperCLICandidates()
	if want := filepath.Join(filepath.Dir(exe), "whisper-cli"); got[0] != want {
		t.Fatalf("first candidate %q, want %q", got[0], want)
	}
	if want := filepath.Join(filepath.Dir(exe), "..", "Resources", "whisper-cli"); got[1] != want {
		t.Fatalf("second candidate %q, want %q", got[1], want)
	}
	for _, p := range got {
		if filepath.Base(p) != "whisper-cli" {
			t.Errorf("candidate %q is not whisper-cli", p)
		}
	}
}

func TestAllowRestartIsLimitedPerWindow(t *testing.T) {
	now := time.Now()
	ago := func(d time.Duration) time.Time { return now.Add(-d) }
	cases := []struct {
		name     string
		restarts []time.Time
		want     bool
		kept     int
	}{
		{"first crash", nil, true, 1},
		{"third crash in window", []time.Time{ago(4 * time.Minute), ago(time.Minute)}, true, 3},
		{"fourth crash in window", []time.Time{ago(4 * time.Minute), ago(2 * time.Minute), ago(time.Minute)}, false, 3},
		{"old crashes age out", []time.Time{ago(6 * time.Minute), ago(5 * time.Minute), ago(time.Minute)}, true, 2},
	}
	for _, tc := range cases {
		kept, ok := allowRestart(tc.restarts, now)
		if ok != tc.want || len(kept) != tc.kept {
			t.Errorf("%s: allowRestart = (%d kept, %v), want (%d, %v)", tc.name, len(kept), ok, tc.kept, tc.want)
		}
	}
}

func TestTailBufferKeepsTheEnd(t *testing.T) {
	var tb tailBuffer
	tb.Write([]byte(strings.Repeat("x", outputTailSize)))
	tb.Write([]byte("error: failed to load model\n"))
	got := tb.String()
	if len(got) > outputTailSize || !strings.HasSuffix(got, "error: failed to load model") {
		t.Fatalf("tail = %d bytes ending %q", len(got), got[max(0, len(got)-40):])
	}
}

func TestInferenceTimeoutScalesWithAudio(t *testing.T) {
	if got := inferenceTimeout(44); got != 10*time.Second {
		t.Errorf("empty WAV: %s, want 10s", got)
	}
	if got := inferenceTimeout(44 + 8*16000*2); got != 34*time.Second {
		t.Errorf("8 s WAV: %s, want 34s", got)
	}
}

// withTestModel serves payload as catalog model "test" from a local server and
// returns the models dir (under a temp HOME) and the Range headers it received.
func withTestModel(t *testing.T, payload []byte, handler http.HandlerFunc) (string, *[]string) {
	t.Helper()
	isolatedHome(t)
	var ranges []string
	if handler == nil {
		handler = func(w http.ResponseWriter, r *http.Request) {
			ranges = append(ranges, r.Header.Get("Range"))
			http.ServeContent(w, r, "m.bin", time.Time{}, bytes.NewReader(payload))
		}
	}
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	sum := sha256.Sum256(payload)
	oldURL, oldCatalog := modelBaseURL, modelCatalog
	modelBaseURL = srv.URL + "/"
	modelCatalog = append(slices.Clone(modelCatalog), catalogModel{"test", "", hex.EncodeToString(sum[:]), int64(len(payload))})
	t.Cleanup(func() { modelBaseURL, modelCatalog = oldURL, oldCatalog })
	dir, err := GetModelsDir()
	if err != nil {
		t.Fatal(err)
	}
	return dir, &ranges
}

func TestDownloadModelResumesPartialFile(t *testing.T) {
	payload := bytes.Repeat([]byte("voxflow!"), 1<<17)
	dir, ranges := withTestModel(t, payload, nil)
	third := len(payload) / 3
	if err := os.WriteFile(filepath.Join(dir, "ggml-test.bin.tmp"), payload[:third], 0644); err != nil {
		t.Fatal(err)
	}

	var last, total int64
	err := NewService().DownloadModelWithContext(context.Background(), "test", func(d, tot int64) { last, total = d, tot })
	if err != nil {
		t.Fatal(err)
	}
	if want := fmt.Sprintf("bytes=%d-", third); len(*ranges) != 1 || (*ranges)[0] != want {
		t.Fatalf("Range headers %q, want [%q]", *ranges, want)
	}
	got, err := os.ReadFile(filepath.Join(dir, "ggml-test.bin"))
	if err != nil || !bytes.Equal(got, payload) {
		t.Fatalf("model file differs from payload (err %v)", err)
	}
	if last != int64(len(payload)) || total != int64(len(payload)) {
		t.Fatalf("last progress %d/%d, want %d/%d", last, total, len(payload), len(payload))
	}
	if _, err := os.Stat(filepath.Join(dir, "ggml-test.bin.tmp")); !os.IsNotExist(err) {
		t.Fatalf("partial file left behind: %v", err)
	}
}

func TestDownloadModelRejectsCorruptFile(t *testing.T) {
	payload := bytes.Repeat([]byte("x"), 1<<16)
	dir, _ := withTestModel(t, payload, func(w http.ResponseWriter, r *http.Request) {
		w.Write(bytes.Repeat([]byte("y"), len(payload)))
	})
	err := NewService().DownloadModelWithContext(context.Background(), "test", nil)
	if err == nil || !strings.Contains(err.Error(), "integrity") {
		t.Fatalf("err = %v, want integrity failure", err)
	}
	for _, name := range []string{"ggml-test.bin", "ggml-test.bin.tmp"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Errorf("%s exists after a corrupt download", name)
		}
	}
}

func TestDownloadModelGivesUpOnStalledBody(t *testing.T) {
	old := downloadIdleTimeout
	downloadIdleTimeout = 100 * time.Millisecond
	t.Cleanup(func() { downloadIdleTimeout = old })
	payload := bytes.Repeat([]byte("x"), 1<<16)
	dir, _ := withTestModel(t, payload, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
		w.Write(payload[:100])
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	})
	err := NewService().DownloadModelWithContext(context.Background(), "test", nil)
	if err == nil || !strings.Contains(err.Error(), "stalled") {
		t.Fatalf("err = %v, want stalled", err)
	}
	if info, err := os.Stat(filepath.Join(dir, "ggml-test.bin.tmp")); err != nil || info.Size() != 100 {
		t.Fatalf("partial file not kept for resume: %v", err)
	}
}

func TestCheckFreeSpace(t *testing.T) {
	dir := t.TempDir()
	if err := checkFreeSpace(dir, 1<<20); err != nil {
		t.Fatalf("1 MB: %v", err)
	}
	if err := checkFreeSpace(dir, 1<<60); err == nil || !strings.Contains(err.Error(), "not enough disk space") {
		t.Fatalf("1 EB: %v", err)
	}
}

func TestModelCatalogIsWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, m := range modelCatalog {
		if seen[m.name] {
			t.Errorf("%s listed twice", m.name)
		}
		seen[m.name] = true
		if len(m.sha256) != 64 || strings.Trim(m.sha256, "0123456789abcdef") != "" {
			t.Errorf("%s: bad SHA-256 %q", m.name, m.sha256)
		}
		if m.size <= 10<<20 { // IsModelDownloaded treats files this small as corrupt
			t.Errorf("%s: size %d too small", m.name, m.size)
		}
		if m.description == "" {
			t.Errorf("%s: no description", m.name)
		}
	}
	if !seen["base"] {
		t.Error("default model base missing")
	}
}

func TestFindWhisperServerNextToBundledCLI(t *testing.T) {
	dir := t.TempDir()
	cli, srv := filepath.Join(dir, "whisper-cli"), filepath.Join(dir, "whisper-server")
	for _, p := range []string{cli, srv} {
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if got := findWhisperServer(cli); got != srv {
		t.Fatalf("findWhisperServer(%q) = %q, want %q", cli, got, srv)
	}
}
