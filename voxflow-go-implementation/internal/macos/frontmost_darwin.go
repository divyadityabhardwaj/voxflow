//go:build darwin

package macos

import (
	"errors"
	"os/exec"
	"strings"
	"sync"
)

// frontmostMu serializes osascript calls (System Events is not re-entrant safe under load).
var frontmostMu sync.Mutex

// FrontmostApp returns the bundle identifier and display name of the active app.
// Uses osascript so it is safe to call from hotkey/background goroutines (no AppKit on wrong thread).
func FrontmostApp() (bundleID, name string, err error) {
	frontmostMu.Lock()
	defer frontmostMu.Unlock()

	bundleID, err = runOSA(
		`tell application "System Events" to get bundle identifier of first application process whose frontmost is true`,
	)
	if err != nil {
		return "", "", err
	}
	bundleID = strings.TrimSpace(bundleID)
	if bundleID == "" {
		return "", "", errors.New("no frontmost application")
	}

	name, _ = runOSA(
		`tell application "System Events" to get name of first application process whose frontmost is true`,
	)
	name = strings.TrimSpace(name)

	return bundleID, name, nil
}

func runOSA(script string) (string, error) {
	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
