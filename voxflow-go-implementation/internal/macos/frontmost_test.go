//go:build darwin

package macos

import (
	"testing"
)

func TestFrontmostApp(t *testing.T) {
	bundleID, name, err := FrontmostApp()
	if err != nil {
		t.Skipf("FrontmostApp unavailable in this environment: %v", err)
	}
	if bundleID == "" {
		t.Fatal("expected non-empty bundle ID")
	}
	t.Logf("frontmost: %s (%s)", name, bundleID)
}
