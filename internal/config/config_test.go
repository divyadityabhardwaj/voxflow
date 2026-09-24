package config

import (
	"os"
	"path/filepath"
	"strings"
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

func clearKeyEnv(t *testing.T) {
	for _, k := range []string{"GEMINI_API_KEY", "OPENROUTER_API_KEY", "GROQ_API_KEY", "CEREBRAS_API_KEY"} {
		t.Setenv(k, "")
	}
}

func TestEnvAPIKeysAreNotSaved(t *testing.T) {
	path := writeConfig(t, `{"groq_api_key": "file-groq"}`)
	clearKeyEnv(t)
	t.Setenv("GEMINI_API_KEY", "env-gemini")
	t.Setenv("GROQ_API_KEY", "env-groq")

	c := &Config{}
	if err := c.Load(); err != nil {
		t.Fatal(err)
	}
	if c.GetAPIKey("gemini") != "env-gemini" || c.GetAPIKey("groq") != "env-groq" {
		t.Error("env keys should override the file")
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	saved := string(data)
	if strings.Contains(saved, "env-gemini") || strings.Contains(saved, "env-groq") {
		t.Errorf("env keys written to disk:\n%s", saved)
	}
	if !strings.Contains(saved, "file-groq") {
		t.Errorf("the user's own key should survive:\n%s", saved)
	}
}

func TestHasAPIKey(t *testing.T) {
	clearKeyEnv(t)
	t.Setenv("CEREBRAS_API_KEY", "env-c")
	c := &Config{GroqAPIKey: "g", LocalURL: "http://localhost:11434"}
	cases := map[string]bool{
		"groq":       true,
		"cerebras":   true,
		"local":      true,
		"openrouter": false,
		"gemini":     false,
		"":           false,
	}
	for provider, want := range cases {
		if got := c.HasAPIKey(provider); got != want {
			t.Errorf("HasAPIKey(%q) = %v, want %v", provider, got, want)
		}
	}
}

func TestProviderFieldsReadExistingConfig(t *testing.T) {
	writeConfig(t, `{"gemini_api_key":"gk","gemini_model":"gm","openrouter_api_key":"ok","openrouter_model":"om",
		"groq_api_key":"qk","groq_model":"qm","cerebras_api_key":"ck","cerebras_model":"","local_model":"lm"}`)
	clearKeyEnv(t)
	c := &Config{}
	if err := c.Load(); err != nil {
		t.Fatal(err)
	}
	want := map[string][2]string{
		"gemini":     {"gk", "gm"},
		"openrouter": {"ok", "om"},
		"groq":       {"qk", "qm"},
		"cerebras":   {"ck", DefaultCerebrasModel},
		"local":      {"", "lm"},
		"unknown":    {"gk", "gm"},
	}
	for p, w := range want {
		if k, m := c.GetAPIKey(p), c.GetModel(p); k != w[0] || m != w[1] {
			t.Errorf("%s: key %q model %q, want %q %q", p, k, m, w[0], w[1])
		}
	}
	c.SetAPIKey("local", "ignored")
	c.SetModel("groq", "new")
	if c.GroqModel != "new" || c.GetAPIKey("local") != "" {
		t.Error("setters should write the provider's own field")
	}
}
