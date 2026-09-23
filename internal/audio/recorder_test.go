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
