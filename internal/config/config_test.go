package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInjectMethodFor(t *testing.T) {
	c := &Config{AppRules: map[string]AppRule{
		"com.example.paste": {InjectMethod: "paste"},
		"com.example.clip":  {InjectMethod: "clipboard"},
		"com.example.type":  {InjectMethod: "type"},
	}}
	cases := map[string]string{
		"":                  "paste",
		"com.example.none":  "paste",
		"com.example.paste": "paste",
		"com.example.clip":  "clipboard",
		"com.example.type":  "type",
	}
	for id, want := range cases {
		if got := c.InjectMethodFor(id); got != want {
			t.Errorf("InjectMethodFor(%q) = %q, want %q", id, got, want)
		}
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".voxflow", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTruncatedConfigIsKept(t *testing.T) {
	truncated := `{"gemini_api_key": "file-key", "app_rules": {"com.example.term": {"inject_method": "type"`
	path := writeConfig(t, truncated)

	c := &Config{}
	if err := c.Load(); err == nil {
		t.Fatal("Load should report the unreadable file")
	}
	if c.LoadWarning() == "" {
		t.Error("LoadWarning should explain the reset")
	}
	if c.GetHandsFreeHotkey() != "cmd+shift+space" || c.GetLLMProvider() != "gemini" || c.AppRules == nil {
		t.Error("defaults should still be applied")
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}

	backups, _ := filepath.Glob(path + ".corrupt-*")
	if len(backups) != 1 {
		t.Fatalf("want one backup, got %v", backups)
	}
	data, err := os.ReadFile(backups[0])
	if err != nil || string(data) != truncated {
		t.Errorf("backup should hold the original bytes, got %q (%v)", data, err)
	}
	if info, _ := os.Stat(backups[0]); info.Mode().Perm() != 0600 {
		t.Errorf("backup mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestMissingConfigIsNotAnError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	c := &Config{}
	if err := c.Load(); err != nil || c.LoadWarning() != "" {
		t.Errorf("first run: err=%v warning=%q", err, c.LoadWarning())
	}
	if c.GetWhisperModel() != "base" {
		t.Error("defaults should be applied on first run")
	}
}
