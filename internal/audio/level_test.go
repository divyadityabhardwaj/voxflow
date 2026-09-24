package audio

import "testing"

func TestMeterLevel(t *testing.T) {
	sq := func(amp float64, n int) float64 { return amp * amp * float64(n) }
	tests := []struct {
		name     string
		sumSq    float64
		n        int
		min, max float64
	}{
		{"no samples", 0, 0, 0, 0},
		{"digital silence", 0, 100, 0, 0},
		{"room noise stays near the bottom", sq(20, 100), 100, 0, 0.1},
		{"speech is mid-scale", sq(3000, 100), 100, 0.5, 0.9},
		{"full scale tops out", sq(32767, 100), 100, 0.99, 1},
		{"clipping is capped", sq(40000, 100), 100, 1, 1},
	}
	for _, tt := range tests {
		if got := meterLevel(tt.sumSq, tt.n); got < tt.min || got > tt.max {
			t.Errorf("%s: meterLevel = %.3f, want %.2f..%.2f", tt.name, got, tt.min, tt.max)
		}
	}
}
