package hotkey

import (
	"fmt"
	"time"
)

// Hold-to-talk keys. "chord" means the push-to-talk hotkey combination.
const (
	HoldRightOption  = "right_option"
	HoldRightCommand = "right_command"
	HoldFn           = "fn"
	HoldChord        = "chord"
)

// holdKeycode is the virtual keycode of a hold-to-talk modifier, or -1 for the chord.
func holdKeycode(key string) (int, error) {
	switch key {
	case HoldRightOption:
		return 61, nil
	case HoldRightCommand:
		return 54, nil
	case HoldFn:
		return 63, nil
	case HoldChord:
		return -1, nil
	default:
		return -1, fmt.Errorf("unknown push-to-talk key %q", key)
	}
}

// Events reported by the keyboard event tap; the values are shared with the C side.
type tapEvent int

const (
	tapHoldDown tapEvent = iota
	tapHoldUp
	tapOtherKey
	tapEscape
)

// tapEvents is fed by the event tap without blocking; there is one tap per process.
var tapEvents = make(chan tapEvent, 64)

type holdAction int

const (
	holdNone holdAction = iota
	holdStart
	holdStop
	holdCancel // discard silently: the modifier was used for something else
)

// Shorter holds are taps of the modifier, not dictations.
const minHold = 200 * time.Millisecond

// holdTracker turns one modifier's presses into recording actions. Recording
// starts on press for latency; a release within minHold, or any other key while
// it is held, means the modifier was used normally, so the recording is dropped.
type holdTracker struct {
	held, spoiled bool
	since         time.Time
}

func (h *holdTracker) down(now time.Time) holdAction {
	if h.held {
		return holdNone
	}
	*h = holdTracker{held: true, since: now}
	return holdStart
}

func (h *holdTracker) other() holdAction {
	if !h.held || h.spoiled {
		return holdNone
	}
	h.spoiled = true
	return holdCancel
}

// spoil ends the hold without an action, e.g. when Esc already cancelled.
func (h *holdTracker) spoil() {
	h.spoiled = h.held
}

func (h *holdTracker) up(now time.Time) holdAction {
	if !h.held {
		return holdNone
	}
	since, spoiled := h.since, h.spoiled
	*h = holdTracker{}
	switch {
	case spoiled:
		return holdNone
	case now.Sub(since) < minHold:
		return holdCancel
	default:
		return holdStop
	}
}
