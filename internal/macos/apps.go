package macos

import (
	"errors"
	"os"
)

// AppInfo identifies a running app. BundleID is empty for apps that have none.
type AppInfo struct {
	BundleID string
	Name     string
	PID      int
}

func IsSelf(app AppInfo) bool {
	return app.PID == os.Getpid()
}

// FrontmostApp returns the active app's bundle ID (empty if it has none) and name.
func FrontmostApp() (bundleID, name string, err error) {
	info, err := FrontmostAppInfo()
	return info.BundleID, info.Name, err
}

// TargetApp is the app dictated text is meant for: the frontmost app, or, when
// VoxFlow itself is in front (its pill or window was clicked), the app the user
// was in before. The fallback needs StartAppTracking.
func TargetApp() (AppInfo, error) {
	front, err := FrontmostAppInfo()
	if err == nil && !IsSelf(front) {
		return front, nil
	}
	if last, ok := LastExternalApp(); ok {
		return last, nil
	}
	if err == nil {
		err = errors.New("no other app has been active since VoxFlow started")
	}
	return AppInfo{}, err
}
