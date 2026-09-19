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
	stream      *portaudio.Stream
	buffer      []int16
	mu          sync.Mutex
	recording   atomic.Bool
	stopChan    chan struct{}
	stoppedChan chan struct{}
	sampleRate  float64
	initOnce    sync.Once
	initErr     error
	initialized atomic.Bool
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

func (r *Recorder) Initialize() error {
	r.initOnce.Do(func() {
		r.initErr = portaudio.Initialize()
		if r.initErr == nil {
			r.initialized.Store(true)
		}
	})
	return r.initErr
}

func (r *Recorder) Terminate() error {
	if !r.initialized.Load() {
		return nil
	}
	err := portaudio.Terminate()
	if err == nil {
		r.initialized.Store(false)
	}
	return err
}

func (r *Recorder) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize audio: %w", err)
	}

	if r.recording.Load() {
		return fmt.Errorf("already recording")
	}

	r.buffer = make([]int16, 0)

	inputBuffer := make([]int16, FramesPerBuffer)

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

	r.stream = stream
	r.stopChan = make(chan struct{})
	r.stoppedChan = make(chan struct{})

	if err := stream.Start(); err != nil {
		stream.Close()
		return fmt.Errorf("failed to start audio stream: %w", err)
	}

	r.recording.Store(true)

	go r.readLoop(inputBuffer)

	return nil
}

// readLoop: stream pointer stable until this goroutine exits; callback via atomicCallback (no lock on hot path).
func (r *Recorder) readLoop(inputBuffer []int16) {
	defer close(r.stoppedChan)

	r.mu.Lock()
	stream := r.stream
	r.mu.Unlock()
	if stream == nil {
		return
	}

	chunkSize := int(SampleRate) * ChunkDuration
	chunkBuffer := make([]int16, 0, chunkSize)
	chunkStartTime := time.Duration(0)

	for {
		select {
		case <-r.stopChan:
			if cb := r.loadCallback(); cb != nil && len(chunkBuffer) > 0 {
				samples := make([]int16, len(chunkBuffer))
				copy(samples, chunkBuffer)
				cb(samples, chunkStartTime, true)
			}
			return
		default:
		}

		if !r.recording.Load() {
			return
		}

		if err := stream.Read(); err != nil {
			if !r.recording.Load() {
				return
			}
			logger.Errorf("Error reading audio: %v", err)
			time.Sleep(10 * time.Millisecond)
			continue
		}

		r.mu.Lock()
		if r.recording.Load() {
			r.buffer = append(r.buffer, inputBuffer...)
		}
		r.mu.Unlock()

		chunkBuffer = append(chunkBuffer, inputBuffer...)
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
	if !r.recording.Load() {
		return "", fmt.Errorf("not recording")
	}

	r.recording.Store(false)
	close(r.stopChan)

	select {
	case <-r.stoppedChan:
	case <-time.After(2 * time.Second):
		logger.Warnf("Warning: read loop did not stop in time")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.stream != nil {
		r.stream.Stop()
		r.stream.Close()
		r.stream = nil
	}

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

func (r *Recorder) GetDuration() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	samples := len(r.buffer)
	seconds := float64(samples) / r.sampleRate
	return time.Duration(seconds * float64(time.Second))
}

func (r *Recorder) IsRecording() bool {
	return r.recording.Load()
}

func (r *Recorder) HasAudioActivity() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.buffer) == 0 {
		return false
	}

	windowSize := 1600 // 100ms at 16kHz
	if len(r.buffer) < windowSize {
		sum := int64(0)
		for _, s := range r.buffer {
			abs := s
			if abs < 0 {
				abs = -abs
			}
			sum += int64(abs)
		}
		avg := float64(sum) / float64(len(r.buffer))
		logger.Infof("[Audio VAD] Short recording average absolute amplitude: %.2f (threshold: 100)", avg)
		return avg > 100
	}

	maxRMS := float64(0)
	for i := 0; i <= len(r.buffer)-windowSize; i += windowSize {
		sumSq := float64(0)
		for j := 0; j < windowSize; j++ {
			s := float64(r.buffer[i+j])
			sumSq += s * s
		}
		rms := math.Sqrt(sumSq / float64(windowSize))
		if rms > maxRMS {
			maxRMS = rms
		}
	}

	logger.Infof("[Audio VAD] Max sliding window RMS energy: %.2f (threshold: 100)", maxRMS)

	return maxRMS > 100 // conservative vs quiet-room noise (~10–50 RMS)
}

func (r *Recorder) GetBuffer() []int16 {
	r.mu.Lock()
	defer r.mu.Unlock()
	buf := make([]int16, len(r.buffer))
	copy(buf, r.buffer)
	return buf
}
