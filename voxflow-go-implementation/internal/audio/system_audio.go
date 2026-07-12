package audio

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"voxflow/internal/logger"
)

// MuteSystemAudio reads the current system output volume, mutes it to 0,
// and returns the original volume level so it can be restored later.
// Returns -1 if the volume could not be read (in which case UnmuteSystemAudio is a no-op).
func MuteSystemAudio() int {
	// Combine reading output volume and muting into a single AppleScript call to save latency.
	script := "set currentVolume to output volume of (get volume settings)\nset volume output volume 0\ncurrentVolume"
	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		logger.Errorf("[Audio] Could not read or mute system volume: %v", err)
		return -1
	}

	vol, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		logger.Errorf("[Audio] Could not parse system volume %q: %v", strings.TrimSpace(string(out)), err)
		return -1
	}

	logger.Infof("[Audio] Muted system audio (was %d%%)", vol)
	return vol
}

// UnmuteSystemAudio restores the system output volume to the level returned by MuteSystemAudio.
// Pass -1 (or any value < 0) to skip restoring (e.g., if muting failed).
func UnmuteSystemAudio(previousVolume int) {
	if previousVolume < 0 {
		return
	}
	script := fmt.Sprintf("set volume output volume %d", previousVolume)
	if err := exec.Command("osascript", "-e", script).Run(); err != nil {
		logger.Errorf("[Audio] Could not restore system volume to %d: %v", previousVolume, err)
		return
	}
	logger.Infof("[Audio] Restored system audio to %d%%", previousVolume)
}
