package hotkey

import (
	"reflect"
	"testing"
	"time"
)

// Events are fed straight into the loop; registering real Carbon hotkeys needs a Cocoa run loop.
func startedManager(t *testing.T) (*Manager, chan State) {
	t.Helper()
	states := make(chan State, 8)
	m := NewManager(func(s State) { states <- s })
	m.running = true
	go m.loop()
	return m, states
}

func TestKeyEventsAreHandledInArrivalOrder(t *testing.T) {
	m, states := startedManager(t)

	// Queued together, as when the loop is busy in a slow callback.
	m.keyEvents <- pushToTalkDown
	m.keyEvents <- pushToTalkUp

	var got []State
	for len(got) < 2 {
		select {
		case s := <-states:
			got = append(got, s)
		case <-time.After(2 * time.Second):
			t.Fatalf("got %v, want two transitions", got)
		}
	}
	if want := []State{StateRecording, StateProcessing}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestUpdateWhileSuspendedOnlyValidates(t *testing.T) {
	m, _ := startedManager(t)

	if err := m.Suspend(true); err != nil {
		t.Fatal(err)
	}
	if err := m.Update("cmd+shift+space", "cmd+bogus+p"); err == nil {
		t.Fatal("invalid hotkey accepted while suspended")
	}
	if err := m.Update("cmd+shift+space", "ctrl+alt+space"); err != nil {
		t.Fatal(err)
	}

	var registered bool
	_ = m.do(func() error {
		registered = m.handsFreeHK != nil || m.pushToTalkHK != nil
		return nil
	})
	if registered {
		t.Fatal("hotkeys registered while suspended")
	}
	if m.pttStr != "ctrl+alt+space" {
		t.Fatalf("pending ptt = %q, want the last update", m.pttStr)
	}
}
