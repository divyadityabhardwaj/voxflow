package hotkey

import (
	"reflect"
	"testing"
	"time"
)

func TestHoldTracker(t *testing.T) {
	type step struct {
		ev string // "down", "up", "other", "esc"
		at time.Duration
	}
	tests := []struct {
		name  string
		steps []step
		want  []holdAction
	}{
		{"hold and release dictates", []step{{"down", 0}, {"up", time.Second}},
			[]holdAction{holdStart, holdStop}},
		{"quick tap is discarded", []step{{"down", 0}, {"up", 150 * time.Millisecond}},
			[]holdAction{holdStart, holdCancel}},
		{"other key while held is a shortcut, not a dictation", []step{{"down", 0}, {"other", 300 * time.Millisecond}, {"up", time.Second}},
			[]holdAction{holdStart, holdCancel, holdNone}},
		{"only the first other key cancels", []step{{"down", 0}, {"other", 10}, {"other", 20}, {"up", time.Second}},
			[]holdAction{holdStart, holdCancel, holdNone, holdNone}},
		{"other keys while not held are ignored", []step{{"other", 0}, {"down", 10}, {"up", time.Second}},
			[]holdAction{holdNone, holdStart, holdStop}},
		{"repeated down is ignored", []step{{"down", 0}, {"down", 100}, {"up", time.Second}},
			[]holdAction{holdStart, holdNone, holdStop}},
		{"stray up is ignored", []step{{"up", 0}},
			[]holdAction{holdNone}},
		{"esc already cancelled, release does nothing", []step{{"down", 0}, {"esc", 500 * time.Millisecond}, {"up", time.Second}},
			[]holdAction{holdStart, holdNone, holdNone}},
		{"a fresh hold after a spoiled one works", []step{{"down", 0}, {"other", 10}, {"up", 20}, {"down", time.Second}, {"up", 2 * time.Second}},
			[]holdAction{holdStart, holdCancel, holdNone, holdStart, holdStop}},
	}
	base := time.Now()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var h holdTracker
			var got []holdAction
			for _, s := range tt.steps {
				now := base.Add(s.at)
				switch s.ev {
				case "down":
					got = append(got, h.down(now))
				case "up":
					got = append(got, h.up(now))
				case "other":
					got = append(got, h.other())
				case "esc":
					h.spoil()
					got = append(got, holdNone)
				}
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHoldKeycode(t *testing.T) {
	for key, want := range map[string]int{HoldRightOption: 61, HoldRightCommand: 54, HoldFn: 63, HoldChord: -1} {
		if got, err := holdKeycode(key); err != nil || got != want {
			t.Errorf("holdKeycode(%q) = %d, %v; want %d", key, got, err, want)
		}
	}
	if _, err := holdKeycode("caps_lock"); err == nil {
		t.Error("unknown key accepted")
	}
}

func TestHoldCancelSparesHandsFree(t *testing.T) {
	m, states := startedManager(t)
	cancels := make(chan bool, 4)
	m.OnCancel = func(silent bool) { cancels <- silent }

	m.keyEvents <- handsFreeDown
	if s := <-states; s != StateRecording {
		t.Fatalf("state %s, want Recording", s)
	}
	m.tapEvents <- tapNow(tapHoldDown) // hold key used as a modifier during hands-free
	m.tapEvents <- tapNow(tapOtherKey)
	m.tapEvents <- tapNow(tapEscape)
	select {
	case silent := <-cancels:
		if silent {
			t.Fatal("Esc should cancel with feedback")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Esc did not cancel")
	}
	select {
	case <-cancels:
		t.Fatal("the hold key cancelled a hands-free recording")
	case <-time.After(50 * time.Millisecond):
	}
}

func tapNow(kind tapKind) tapEvent { return tapEvent{kind: kind, at: time.Now()} }

func TestQuickTapBehindSlowStartIsStillDiscarded(t *testing.T) {
	cancels := make(chan bool, 4)
	m := NewManager(func(s State) {
		if s == StateRecording {
			time.Sleep(300 * time.Millisecond) // a slow microphone open
		}
	})
	m.tapEvents = make(chan tapEvent, 8)
	m.OnCancel = func(silent bool) { cancels <- silent }
	m.running = true
	go m.loop()

	down := time.Now()
	m.tapEvents <- tapEvent{kind: tapHoldDown, at: down}
	m.tapEvents <- tapEvent{kind: tapHoldUp, at: down.Add(100 * time.Millisecond)}
	select {
	case silent := <-cancels:
		if !silent {
			t.Fatal("a quick tap should discard without a toast")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the quick tap was dictated")
	}
}

func TestHoldIsConfirmedOnlyAfterMinHold(t *testing.T) {
	m, _ := startedManager(t)
	started := make(chan struct{}, 4)
	confirmed := make(chan struct{}, 4)
	m.OnHoldStart = func() { started <- struct{}{} }
	m.OnHoldConfirmed = func() { confirmed <- struct{}{} }
	m.OnCancel = func(bool) { m.SetState(StateIdle) }

	// A quick tap starts, then is discarded without confirmation.
	down := time.Now()
	m.tapEvents <- tapEvent{kind: tapHoldDown, at: down}
	m.tapEvents <- tapEvent{kind: tapHoldUp, at: down.Add(50 * time.Millisecond)}
	<-started
	select {
	case <-confirmed:
		t.Fatal("a quick tap was confirmed")
	case <-time.After(minHold + 100*time.Millisecond):
	}

	// A real hold is confirmed while still held.
	m.tapEvents <- tapNow(tapHoldDown)
	<-started
	select {
	case <-confirmed:
	case <-time.After(2 * time.Second):
		t.Fatal("a long hold was never confirmed")
	}
	m.tapEvents <- tapNow(tapHoldUp)
	select {
	case <-confirmed:
		t.Fatal("confirmed twice")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestHoldKeyUsedAsModifierDiscardsSilently(t *testing.T) {
	m, states := startedManager(t)
	cancels := make(chan bool, 4)
	m.OnCancel = func(silent bool) { cancels <- silent }

	m.tapEvents <- tapNow(tapHoldDown)
	if s := <-states; s != StateRecording {
		t.Fatalf("state %s, want Recording", s)
	}
	m.tapEvents <- tapNow(tapOtherKey)
	select {
	case silent := <-cancels:
		if !silent {
			t.Fatal("a hold-key shortcut should discard without a toast")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the recording was not discarded")
	}
}
