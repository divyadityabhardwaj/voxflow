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

// FrontmostApp via osascript — safe from hotkey goroutines (no AppKit off main thread).
func FrontmostApp() (bundleID, name string, err error) {
	frontmostMu.Lock()
	defer frontmostMu.Unlock()

	out, err := runOSA(`tell application "System Events"
	set p to first application process whose frontmost is true
	return "" & (bundle identifier of p) & linefeed & (name of p)
end tell`)
	if err != nil {
		return "", "", err
	}
	bundleID, name, _ = strings.Cut(out, "\n")
	bundleID, name = strings.TrimSpace(bundleID), strings.TrimSpace(name)
	if bundleID == "" {
		return "", "", errors.New("no frontmost application")
	}

	return bundleID, name, nil
}

func runOSA(script string) (string, error) {
	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
