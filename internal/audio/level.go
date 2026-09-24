package audio

import (
	"math"
	"time"
)

// About 20 meter updates a second.
const levelInterval = 50 * time.Millisecond

// SetLevelCallback receives the input level (0..1) about 20 times a second while recording.
func (r *Recorder) SetLevelCallback(cb func(level float64)) {
	r.levelCallback.Store(cb)
}

func (r *Recorder) loadLevelCallback() func(float64) {
	cb, _ := r.levelCallback.Load().(func(float64))
	return cb
}

// meterLevel maps the RMS of n samples onto 0..1 over -60..0 dBFS, so speech
// fills the meter rather than hugging the bottom as linear RMS would.
func meterLevel(sumSq float64, n int) float64 {
	if n == 0 || sumSq == 0 {
		return 0
	}
	db := 20 * math.Log10(math.Sqrt(sumSq/float64(n))/32768)
	return min(max((db+60)/60, 0), 1)
}
