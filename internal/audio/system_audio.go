package audio

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"voxflow/internal/logger"
)

// MuteSystemAudio returns prior output volume, or -1 if mute failed (UnmuteSystemAudio no-ops).
func MuteSystemAudio() int {
	script := "set currentVolume to output volume of (get volume settings)\nset volume output volume 0\ncurrentVolume" // one osascript round-trip
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
