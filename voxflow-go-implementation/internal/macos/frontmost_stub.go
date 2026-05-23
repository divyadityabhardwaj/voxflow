//go:build !darwin

package macos

import "errors"

func FrontmostApp() (bundleID, name string, err error) {
	return "", "", errors.New("frontmost app detection is only supported on macOS")
}
