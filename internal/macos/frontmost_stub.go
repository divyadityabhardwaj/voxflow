//go:build !darwin

package macos

import (
	"errors"
	"time"
)

func FrontmostAppInfo() (AppInfo, error) {
	return AppInfo{}, errors.New("frontmost app detection is only supported on macOS")
}

func StartAppTracking() {}

func LastExternalApp() (AppInfo, bool) { return AppInfo{}, false }

func ActivateApp(pid int, timeout time.Duration) bool { return false }
