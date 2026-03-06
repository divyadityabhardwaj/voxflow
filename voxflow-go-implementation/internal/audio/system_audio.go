package audio

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// MuteSystemAudio reads the current system output volume, mutes it to 0,
// and returns the original volume level so it can be restored later.
// Returns -1 if the volume could not be read (in which case UnmuteSystemAudio is a no-op).
func MuteSystemAudio() int {
	// Get current output volume (0–100)
	getScript := `output volume of (get volume settings)`
	out, err := exec.Command("osascript", "-e", getScript).Output()
	if err != nil {
		fmt.Printf("[Audio] Could not read system volume: %v\n", err)
		return -1
	}

	vol, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		fmt.Printf("[Audio] Could not parse system volume %q: %v\n", strings.TrimSpace(string(out)), err)
		return -1
	}

	// Mute output
	muteScript := `set volume output volume 0`
	if err := exec.Command("osascript", "-e", muteScript).Run(); err != nil {
		fmt.Printf("[Audio] Could not mute system volume: %v\n", err)
		return -1
	}

	fmt.Printf("[Audio] Muted system audio (was %d%%)\n", vol)
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
		fmt.Printf("[Audio] Could not restore system volume to %d: %v\n", previousVolume, err)
		return
	}
	fmt.Printf("[Audio] Restored system audio to %d%%\n", previousVolume)
}
