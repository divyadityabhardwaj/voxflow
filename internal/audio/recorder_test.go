package audio

import "testing"

func TestQuietestCutFindsThePause(t *testing.T) {
	buf := make([]int16, SampleRate*ChunkDuration)
	for i := range buf {
		buf[i] = int16(3000 * (1 - 2*(i%7%2))) // constant loud noise
	}
	gapStart, gapEnd := SampleRate*7, SampleRate*7+SampleRate/5 // 200ms pause at 7.0s
	for i := gapStart; i < gapEnd; i++ {
		buf[i] = 0
	}
	cut := quietestCut(buf)
	if cut < gapStart || cut > gapEnd {
		t.Fatalf("cut at %d, want inside pause [%d,%d)", cut, gapStart, gapEnd)
	}
	if got := quietestCut(buf[:SampleRate]); got != SampleRate {
		t.Fatalf("short buffer should not be cut, got %d", got)
	}
}

func TestAllSilent(t *testing.T) {
	cases := []struct {
		name string
		buf  []int16
		want bool
	}{
		{"empty", nil, false},
		{"digital silence", make([]int16, SampleRate), true},
		{"one non-zero sample", append(make([]int16, SampleRate), 1), false},
		{"quiet noise floor", []int16{0, -1, 2, 0, -3}, false},
	}
	for _, tc := range cases {
		r := NewRecorder()
		r.buffer = tc.buf
		if got := r.AllSilent(); got != tc.want {
			t.Errorf("%s: AllSilent() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestHasActivity(t *testing.T) {
	loud := func(n int) []int16 {
		buf := make([]int16, n)
		for i := range buf {
			buf[i] = int16(3000 * (1 - 2*(i%2)))
		}
		return buf
	}
	noise := make([]int16, SampleRate)
	for i := range noise {
		noise[i] = int16(30 * (1 - 2*(i%2)))
	}
	cases := []struct {
		name string
		buf  []int16
		want bool
	}{
		{"empty", nil, false},
		{"digital silence", make([]int16, SampleRate), false},
		{"quiet room", noise, false},
		{"speech", loud(SampleRate), true},
		{"short speech", loud(800), true},
		{"burst in silence", append(make([]int16, SampleRate), loud(1600)...), true},
		{"min int16", []int16{-32768}, true},
	}
	for _, tc := range cases {
		if got := HasActivity(tc.buf); got != tc.want {
			t.Errorf("%s: HasActivity() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestCanTerminateOnlyWithoutLiveReadLoop(t *testing.T) {
	r := NewRecorder()
	if !r.canTerminateLocked() {
		t.Fatal("fresh recorder should allow re-init")
	}
	r.stoppedChan = make(chan struct{})
	if r.canTerminateLocked() {
		t.Fatal("re-init allowed while a read loop is running")
	}
	close(r.stoppedChan)
	if !r.canTerminateLocked() {
		t.Fatal("re-init refused after the read loop exited")
	}
	r.leakedStream = true
	if r.canTerminateLocked() {
		t.Fatal("re-init allowed with a leaked stream")
	}
}
