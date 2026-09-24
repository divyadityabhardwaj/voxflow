//go:build !darwin

package hotkey

func startTap() bool { return false }

func isTapRunning() bool { return false }

func setHoldKeycode(int) {}

func setSwallowEscape(bool) {}
