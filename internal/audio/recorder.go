package audio

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"voxflow/internal/logger"

	"github.com/gordonklaus/portaudio"
)

const (
	SampleRate      = 16000
	Channels        = 1
	FramesPerBuffer = 1024
	ChunkDuration   = 8
)

var chunkPool = sync.Pool{
	New: func() interface{} {
		return make([]int16, SampleRate*ChunkDuration)
	},
}

func RecycleChunk(samples []int16) {
	if cap(samples) == SampleRate*ChunkDuration {
		chunkPool.Put(samples)
	}
}

type ChunkCallback func(samples []int16, startTime time.Duration, isFinal bool)

type Recorder struct {
	stream         *portaudio.Stream
	buffer         []int16
	mu             sync.Mutex
	recording      atomic.Bool
	stopChan       chan struct{}
	stoppedChan    chan struct{}
	sampleRate     float64
	initialized    bool         // guarded by mu
	leakedStream   bool         // Stop gave up on a readLoop; guarded by mu
	atomicCallback atomic.Value // readLoop hot path without mu
}

func NewRecorder() *Recorder {
	return &Recorder{
		sampleRate: SampleRate,
		buffer:     make([]int16, 0),
	}
}

func (r *Recorder) SetChunkCallback(callback ChunkCallback) {
	r.atomicCallback.Store(callback)
}

func (r *Recorder) ClearChunkCallback() {
	r.atomicCallback.Store(ChunkCallback(nil))
}

func (r *Recorder) loadCallback() ChunkCallback {
	if v := r.atomicCallback.Load(); v != nil {
		if cb, ok := v.(ChunkCallback); ok {
			return cb
		}
	}
	return nil
}

// Initialize (re)starts PortAudio. PortAudio snapshots the device list and default
// input at init, so Start re-runs this before every recording to pick up AirPods,
// USB mics and a changed default input (about 2 ms once CoreAudio is warm).
// ponytail: re-enumerates on every recording; if Bluetooth devices make that slow,
// re-init only from a kAudioHardwarePropertyDefaultInputDevice listener.
func (r *Recorder) Initialize() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.initializeLocked()
}

func (r *Recorder) initializeLocked() error {
	if r.initialized {
		if !r.canTerminateLocked() {
			return nil
		}
		if err := portaudio.Terminate(); err != nil {
			return err
		}
		r.initialized = false
	}
	if err := portaudio.Initialize(); err != nil {
		return err
	}
	r.initialized = true
	return nil
}

// Pa_Terminate closes every open stream, so it must not run while a readLoop may
// still be using one.
func (r *Recorder) canTerminateLocked() bool {
	if r.leakedStream {
		return false
	}
	if r.stoppedChan == nil {
		return true
	}
	select {
	case <-r.stoppedChan:
		return true
	default:
		return false
	}
}

func (r *Recorder) Terminate() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.initialized || !r.canTerminateLocked() {
		return nil
	}
	if err := portaudio.Terminate(); err != nil {
		return err
	}
	r.initialized = false
	return nil
}

func (r *Recorder) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.recording.Load() {
		return fmt.Errorf("already recording")
	}

	if err := r.initializeLocked(); err != nil {
		return fmt.Errorf("failed to initialize audio: %w", err)
	}

	r.buffer = make([]int16, 0)

	// A pointer, so readLoop can shrink each Read to the frames already buffered.
	inputBuffer := new([]int16)
	*inputBuffer = make([]int16, FramesPerBuffer)

	stream, err := portaudio.OpenDefaultStream(
		Channels,
		0,
		r.sampleRate,
		FramesPerBuffer,
		inputBuffer,
	)
	if err != nil {
		return fmt.Errorf("failed to open audio stream: %w", err)
	}

	if err := stream.Start(); err != nil {
		stream.Close()
		return fmt.Errorf("failed to start audio stream: %w", err)
	}

	stop, stopped := make(chan struct{}), make(chan struct{})
	r.stream, r.stopChan, r.stoppedChan = stream, stop, stopped
	r.recording.Store(true)

	go r.readLoop(stream, inputBuffer, stop, stopped)

	return nil
}

// readLoop owns one recording's stream; callback via atomicCallback (no lock on hot path).
// stopped is closed under r.mu, so Stop's timeout path sees either a finished
// loop or one that will still find r.stream changed and close its own stream.
func (r *Recorder) readLoop(stream *portaudio.Stream, inputBuffer *[]int16, stop <-chan struct{}, stopped chan<- struct{}) {
	full := *inputBuffer
	chunkSize := int(SampleRate) * ChunkDuration
	chunkBuffer := make([]int16, 0, chunkSize)
	chunkStartTime := time.Duration(0)

	for {
		select {
		case <-stop:
			if cb := r.loadCallback(); cb != nil && len(chunkBuffer) > 0 {
				samples := make([]int16, len(chunkBuffer))
				copy(samples, chunkBuffer)
				cb(samples, chunkStartTime, true)
			}
			r.mu.Lock()
			if r.stream != stream {
				stream.Abort()
				stream.Close()
			}
			close(stopped)
			r.mu.Unlock()
			return
		default:
		}

		// Pa_ReadStream busy-waits until it has every requested frame and never checks
		// whether the stream stopped, so a vanished device would hang it forever and a
		// Stop/Close under it frees its ring buffer. Only ask for frames already there.
		n, err := stream.AvailableToRead()
		if err != nil || n == 0 {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		*inputBuffer = full[:min(n, len(full))]
		// InputOverflowed still fills the buffer; it only reports frames dropped earlier.
		if err := stream.Read(); err != nil && err != portaudio.InputOverflowed {
			logger.Errorf("Error reading audio: %v", err)
			time.Sleep(10 * time.Millisecond)
			continue
		}
		in := *inputBuffer

		r.mu.Lock()
		if r.stream != stream {
			// Stop gave up waiting for this loop; only this goroutine can close the stream safely.
			stream.Abort()
			stream.Close()
			close(stopped)
			r.mu.Unlock()
			return
		}
		r.buffer = append(r.buffer, in...)
		r.mu.Unlock()

		chunkBuffer = append(chunkBuffer, in...)
		if len(chunkBuffer) >= chunkSize {
			cb := r.loadCallback()
			if cb == nil {
				chunkBuffer = chunkBuffer[:0]
				continue
			}
			cut := quietestCut(chunkBuffer[:chunkSize]) // split on pause, not at 8s boundary
			chunkSamples := chunkPool.Get().([]int16)[:cut]
			copy(chunkSamples, chunkBuffer[:cut])
			remaining := make([]int16, len(chunkBuffer)-cut, chunkSize)
			copy(remaining, chunkBuffer[cut:])
			chunkBuffer = remaining
			start := chunkStartTime
			chunkStartTime += time.Duration(cut) * time.Second / SampleRate
			cb(chunkSamples, start, false)
		}
	}
}

// quietestCut picks a low-energy point in the last 1.5s so chunks split in pauses.
func quietestCut(buf []int16) int {
	const window, step, search = SampleRate / 10, SampleRate / 40, SampleRate * 3 / 2
	n := len(buf)
	if n < search+window {
		return n
	}
	best, bestEnergy := n, math.MaxFloat64
	for start := n - search; start+window <= n; start += step {
		var e float64
		for _, v := range buf[start : start+window] {
			e += float64(v) * float64(v)
		}
		if e < bestEnergy {
			bestEnergy, best = e, start+window/2
		}
	}
	return best
}

func (r *Recorder) Stop() (string, error) {
	if !r.recording.CompareAndSwap(true, false) {
		return "", fmt.Errorf("not recording")
	}

	r.mu.Lock()
	stream, stopped := r.stream, r.stoppedChan
	close(r.stopChan)
	r.mu.Unlock()

	select {
	case <-stopped:
		stream.Stop()
		stream.Close()
	case <-time.After(2 * time.Second):
		// Never Stop/Close under a live readLoop: leave the stream to it and keep
		// PortAudio up for good rather than risk a use-after-free.
		logger.Warnf("[Audio] Read loop did not stop in time; leaking its stream")
		r.mu.Lock()
		r.stream, r.leakedStream = nil, true
		select {
		case <-stopped: // exited just after the timeout, leaving the stream to us
			stream.Abort()
			stream.Close()
			r.leakedStream = false
		default:
		}
		r.mu.Unlock()
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.stream = nil
	return r.saveToWav()
}

func (r *Recorder) saveToWav() (string, error) {
	if len(r.buffer) == 0 {
		return "", fmt.Errorf("no audio data recorded")
	}

	tempDir := os.TempDir()
	filename := fmt.Sprintf("voxflow_recording_%d.wav", time.Now().UnixNano())
	filepath := filepath.Join(tempDir, filename)

	file, err := os.Create(filepath)
	if err != nil {
		return "", fmt.Errorf("failed to create WAV file: %w", err)
	}
	defer file.Close()

	if err := r.writeWavHeader(file, len(r.buffer)); err != nil {
		return "", fmt.Errorf("failed to write WAV header: %w", err)
	}

	if err := binary.Write(file, binary.LittleEndian, r.buffer); err != nil { // one write, not per-sample
		return "", fmt.Errorf("failed to write audio data: %w", err)
	}

	return filepath, nil
}

func CleanupTempFiles() error {
	tempDir := os.TempDir()
	pattern := filepath.Join(tempDir, "voxflow_*.wav")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	for _, f := range matches {
		logger.Infof("[Audio] Cleaning up stale temp WAV file: %s", filepath.Base(f))
		_ = os.Remove(f)
	}
	return nil
}

func (r *Recorder) writeWavHeader(file *os.File, numSamples int) error {
	bitsPerSample := 16
	byteRate := int(r.sampleRate) * Channels * bitsPerSample / 8
	blockAlign := Channels * bitsPerSample / 8
	dataSize := numSamples * 2
	fileSize := 36 + dataSize

	header := bytes.NewBuffer(nil)

	header.WriteString("RIFF")
	binary.Write(header, binary.LittleEndian, int32(fileSize))
	header.WriteString("WAVE")

	header.WriteString("fmt ")
	binary.Write(header, binary.LittleEndian, int32(16))
	binary.Write(header, binary.LittleEndian, int16(1))
	binary.Write(header, binary.LittleEndian, int16(Channels))
	binary.Write(header, binary.LittleEndian, int32(r.sampleRate))
	binary.Write(header, binary.LittleEndian, int32(byteRate))
	binary.Write(header, binary.LittleEndian, int16(blockAlign))
	binary.Write(header, binary.LittleEndian, int16(bitsPerSample))

	header.WriteString("data")
	binary.Write(header, binary.LittleEndian, int32(dataSize))

	_, err := file.Write(header.Bytes())
	return err
}

func (r *Recorder) IsRecording() bool {
	return r.recording.Load()
}

func (r *Recorder) GetDuration() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	samples := len(r.buffer)
	seconds := float64(samples) / r.sampleRate
	return time.Duration(seconds * float64(time.Second))
}

// AllSilent reports whether the last recording holds samples and every one is
// exactly zero: what macOS delivers when microphone access is denied or the input
// is muted. A real microphone in a quiet room still has a non-zero noise floor.
func (r *Recorder) AllSilent() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.buffer {
		if s != 0 {
			return false
		}
	}
	return len(r.buffer) > 0
}

func (r *Recorder) HasAudioActivity() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	level := activityLevel(r.buffer)
	logger.Infof("[Audio VAD] Max window energy: %.2f (threshold: %d)", level, activityThreshold)
	return level > activityThreshold
}

const activityThreshold = 100 // conservative vs quiet-room noise (~10–50 RMS)

// HasActivity reports whether samples hold anything louder than a quiet room.
func HasActivity(samples []int16) bool {
	return activityLevel(samples) > activityThreshold
}

// activityLevel is the loudest 100 ms window's RMS, or the mean absolute
// amplitude when samples are shorter than one window.
func activityLevel(samples []int16) float64 {
	if len(samples) == 0 {
		return 0
	}

	windowSize := 1600 // 100ms at 16kHz
	if len(samples) < windowSize {
		sum := int64(0)
		for _, s := range samples {
			abs := int64(s)
			if abs < 0 {
				abs = -abs
			}
			sum += abs
		}
		return float64(sum) / float64(len(samples))
	}

	maxRMS := float64(0)
	for i := 0; i <= len(samples)-windowSize; i += windowSize {
		sumSq := float64(0)
		for j := 0; j < windowSize; j++ {
			s := float64(samples[i+j])
			sumSq += s * s
		}
		rms := math.Sqrt(sumSq / float64(windowSize))
		if rms > maxRMS {
			maxRMS = rms
		}
	}
	return maxRMS
}

func (r *Recorder) GetBuffer() []int16 {
	r.mu.Lock()
	defer r.mu.Unlock()
	buf := make([]int16, len(r.buffer))
	copy(buf, r.buffer)
	return buf
}
