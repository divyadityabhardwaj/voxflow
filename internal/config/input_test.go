package config

import "testing"

func TestPushToTalkKeyMigration(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
		want string
	}{
		{"new install", &Config{}, "right_option"},
		{"old default combination", &Config{PushToTalkHotkey: "cmd+shift+p"}, "right_option"},
		{"custom combination is kept", &Config{PushToTalkHotkey: "ctrl+alt+space"}, "chord"},
		{"explicit choice wins", &Config{PushToTalkHotkey: "cmd+shift+p", PushToTalkKey: "fn"}, "fn"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.cfg.applyDefaults()
			if got := tt.cfg.GetPushToTalkKey(); got != tt.want {
				t.Fatalf("PushToTalkKey = %q, want %q", got, tt.want)
			}
		})
	}
}
