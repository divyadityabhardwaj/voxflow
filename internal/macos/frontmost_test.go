//go:build darwin

package macos

import (
	"os"
	"testing"
	"time"
)

func TestFrontmostApp(t *testing.T) {
	info, err := FrontmostAppInfo()
	if err != nil {
		t.Skipf("FrontmostApp unavailable in this environment: %v", err)
	}
	if info.PID <= 0 || info.Name == "" {
		t.Fatalf("expected a pid and a name, got %+v", info)
	}
	bundleID, name, err := FrontmostApp()
	if err != nil || name == "" {
		t.Fatalf("FrontmostApp() = %q, %q, %v", bundleID, name, err)
	}
	t.Logf("frontmost: %s (%s, pid %d)", info.Name, info.BundleID, info.PID)
}

func TestIsSelf(t *testing.T) {
	tests := []struct {
		app  AppInfo
		want bool
	}{
		{AppInfo{PID: os.Getpid()}, true},
		{AppInfo{BundleID: "com.voxflow.app", PID: os.Getpid() + 1}, false},
		{AppInfo{}, false},
	}
	for _, tt := range tests {
		if got := IsSelf(tt.app); got != tt.want {
			t.Errorf("IsSelf(%+v) = %v, want %v", tt.app, got, tt.want)
		}
	}
}

func TestAppTrackingSeedsFromFrontmost(t *testing.T) {
	front, err := FrontmostAppInfo()
	if err != nil {
		t.Skipf("FrontmostApp unavailable in this environment: %v", err)
	}
	StartAppTracking()
	StartAppTracking()

	last, ok := LastExternalApp()
	if !ok || IsSelf(last) {
		t.Fatalf("LastExternalApp() = %+v, %v; want an external app", last, ok)
	}
	target, err := TargetApp()
	if err != nil || IsSelf(target) {
		t.Fatalf("TargetApp() = %+v, %v; want an external app", target, err)
	}
	t.Logf("frontmost pid %d, last external pid %d", front.PID, last.PID)
}

func TestActivateAppUnknownPID(t *testing.T) {
	start := time.Now()
	if ActivateApp(-1, time.Second) {
		t.Fatal("activating a nonexistent pid should fail")
	}
	if time.Since(start) > 100*time.Millisecond {
		t.Fatal("a nonexistent pid should fail without waiting for the timeout")
	}
}

func TestMicrophoneStatus(t *testing.T) {
	switch s := MicrophoneStatus(); s {
	case "authorized", "denied", "restricted", "notDetermined":
	default:
		t.Fatalf("unexpected status %q", s)
	}
}

func TestPrivacySettingsURL(t *testing.T) {
	tests := []struct {
		pane, want string
		wantErr    bool
	}{
		{"accessibility", "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility", false},
		{"microphone", "x-apple.systempreferences:com.apple.preference.security?Privacy_Microphone", false},
		{"camera", "", true},
		{"", "", true},
	}
	for _, tt := range tests {
		got, err := privacySettingsURL(tt.pane)
		if got != tt.want || (err != nil) != tt.wantErr {
			t.Errorf("privacySettingsURL(%q) = %q, %v", tt.pane, got, err)
		}
	}
}
