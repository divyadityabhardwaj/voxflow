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
