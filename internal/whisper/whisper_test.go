package whisper

import (
	"os"
	"path/filepath"
	"testing"
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
