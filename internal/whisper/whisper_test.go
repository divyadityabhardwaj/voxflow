package whisper

import (
	"os"
	"path/filepath"
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
