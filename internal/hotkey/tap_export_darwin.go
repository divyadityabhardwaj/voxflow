//go:build darwin

package hotkey

import "C"

// Called for every key press on the tap thread, so it must never block.
//
//export voxTapEvent
func voxTapEvent(kind C.int) {
	select {
	case tapEvents <- tapEvent(kind):
	default:
	}
}
