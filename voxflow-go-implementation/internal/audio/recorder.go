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
	SampleRate      = 16000 // Whisper expects 16kHz
	Channels        = 1     // Mono
	FramesPerBuffer = 1024
	ChunkDuration   = 8 // seconds per chunk for streaming transcription
)

var chunkPool = sync.Pool{
	New: func() interface{} {
		return make([]int16, SampleRate*ChunkDuration)
	},
}

// RecycleChunk returns a chunk buffer to the pool for reuse.
func RecycleChunk(samples []int16) {
	if cap(samples) == SampleRate*ChunkDuration {
		chunkPool.Put(samples)
	}
}

// ChunkCallback is called with audio chunks during recording
type ChunkCallback func(samples []int16, startTime time.Duration, isFinal bool)

// Recorder handles audio capture from the microphone
type Recorder struct {
	stream      *portaudio.Stream
	buffer      []int16
	mu          sync.Mutex // guards stream, buffer, and saveToWav
	recording   atomic.Bool
	stopChan    chan struct{}
	stoppedChan chan struct{}
	sampleRate  float64
	initOnce    sync.Once
	initErr     error
	initialized atomic.Bool
	// atomicCallback allows the readLoop to read the callback without taking mu.
	// Stores a ChunkCallback (function type); nil means no callback set.
	atomicCallback atomic.Value
}

// NewRecorder creates a new audio recorder
func NewRecorder() *Recorder {
	return &Recorder{
		sampleRate: SampleRate,
		buffer:     make([]int16, 0),
	}
}

// SetChunkCallback sets a callback to receive audio chunks during recording.
// Chunks are sent every ChunkDuration seconds while recording.
// The isFinal flag is true when the chunk is the final one (recording stopped).
func (r *Recorder) SetChunkCallback(callback ChunkCallback) {
	r.atomicCallback.Store(callback)
}

// ClearChunkCallback removes the chunk callback.
func (r *Recorder) ClearChunkCallback() {
	r.atomicCallback.Store(ChunkCallback(nil))
}

// loadCallback returns the currently set ChunkCallback, or nil.
func (r *Recorder) loadCallback() ChunkCallback {
	if v := r.atomicCallback.Load(); v != nil {
		if cb, ok := v.(ChunkCallback); ok {
			return cb
		}
	}
	return nil
}

// Initialize initializes PortAudio
func (r *Recorder) Initialize() error {
	r.initOnce.Do(func() {
		r.initErr = portaudio.Initialize()
		if r.initErr == nil {
			r.initialized.Store(true)
		}
	})
	return r.initErr
}

// Terminate cleans up PortAudio
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

// Start begins recording audio
func (r *Recorder) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize audio: %w", err)
	}

	if r.recording.Load() {
		return fmt.Errorf("already recording")
	}

	// Clear the buffer
	r.buffer = make([]int16, 0)

	// Create input buffer
	inputBuffer := make([]int16, FramesPerBuffer)

	// Open default input stream
	stream, err := portaudio.OpenDefaultStream(
		Channels,        // input channels
		0,               // output channels
		r.sampleRate,    // sample rate
		FramesPerBuffer, // frames per buffer
		inputBuffer,     // buffer
	)
	if err != nil {
		return fmt.Errorf("failed to open audio stream: %w", err)
	}

	r.stream = stream
	r.stopChan = make(chan struct{})
	r.stoppedChan = make(chan struct{})

	// Start the stream
	if err := stream.Start(); err != nil {
		stream.Close()
		return fmt.Errorf("failed to start audio stream: %w", err)
	}

	r.recording.Store(true)

	// Start goroutine to read audio data
	go r.readLoop(inputBuffer)

	return nil
}

// readLoop continuously reads audio data from the stream.
//
// Design: r.stream is written only in Start() (before readLoop starts) and Stop()
// (only after stoppedChan is closed, i.e. after readLoop exits). So it is safe
// to cache it here without holding the mutex for every frame.
// The chunk callback is read via atomicCallback, avoiding any lock on the hot path.
func (r *Recorder) readLoop(inputBuffer []int16) {
	defer close(r.stoppedChan)

	// Cache stream — valid for the lifetime of this goroutine (see comment above).
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
		// Check stop signal first.
		select {
		case <-r.stopChan:
			// Send the final partial chunk if any remains.
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

		// Blocking read — stream is stable for the lifetime of this loop.
		if err := stream.Read(); err != nil {
			if !r.recording.Load() {
				return
			}
			logger.Errorf("Error reading audio: %v", err)
			time.Sleep(10 * time.Millisecond)
			continue
		}

		// Append inputBuffer directly to the persistent buffer (mu required for Stop() reader).
		r.mu.Lock()
		if r.recording.Load() {
			r.buffer = append(r.buffer, inputBuffer...)
		}
		r.mu.Unlock()

		// Accumulate chunk buffer and fire callback when full — no lock needed
		// because chunkBuffer is local to this goroutine.
		chunkBuffer = append(chunkBuffer, inputBuffer...)
		if len(chunkBuffer) >= chunkSize {
			if cb := r.loadCallback(); cb != nil {
				// Rent a buffer from the pool to avoid allocations
				chunkSamples := chunkPool.Get().([]int16)
				copy(chunkSamples, chunkBuffer[:chunkSize])
				var remaining []int16
				if len(chunkBuffer) > chunkSize {
					remaining = make([]int16, len(chunkBuffer)-chunkSize)
					copy(remaining, chunkBuffer[chunkSize:])
				}
				chunkBuffer = remaining
				start := chunkStartTime
				chunkStartTime += time.Duration(ChunkDuration) * time.Second
				// Invoke callback synchronously (it will queue to a channel in O(1))
				cb(chunkSamples, start, false)
			}
		}
	}
}

// Stop stops recording and returns the path to the WAV file
func (r *Recorder) Stop() (string, error) {
	if !r.recording.Load() {
		return "", fmt.Errorf("not recording")
	}

	// Signal the read loop to stop
	r.recording.Store(false)
	close(r.stopChan)

	// Wait for read loop to finish (with timeout)
	select {
	case <-r.stoppedChan:
		// Read loop finished
	case <-time.After(2 * time.Second):
		logger.Warnf("Warning: read loop did not stop in time")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Stop and close the stream
	if r.stream != nil {
		r.stream.Stop()
		r.stream.Close()
		r.stream = nil
	}

	// Save buffer to WAV file
	return r.saveToWav()
}

// saveToWav saves the recorded buffer to a WAV file
func (r *Recorder) saveToWav() (string, error) {
	if len(r.buffer) == 0 {
		return "", fmt.Errorf("no audio data recorded")
	}

	// Create temp file
	tempDir := os.TempDir()
	filename := fmt.Sprintf("voxflow_recording_%d.wav", time.Now().UnixNano())
	filepath := filepath.Join(tempDir, filename)

	file, err := os.Create(filepath)
	if err != nil {
		return "", fmt.Errorf("failed to create WAV file: %w", err)
	}
	defer file.Close()

	// Write WAV header
	if err := r.writeWavHeader(file, len(r.buffer)); err != nil {
		return "", fmt.Errorf("failed to write WAV header: %w", err)
	}

	// Write all audio data in a single call (avoids per-sample syscall overhead)
	if err := binary.Write(file, binary.LittleEndian, r.buffer); err != nil {
		return "", fmt.Errorf("failed to write audio data: %w", err)
	}

	return filepath, nil
}

// writeSamplesToWav writes int16 PCM mono samples to a temp WAV file.
func (r *Recorder) writeSamplesToWav(samples []int16) (string, error) {
	if len(samples) == 0 {
		return "", fmt.Errorf("no samples")
	}
	tempDir := os.TempDir()
	filename := fmt.Sprintf("voxflow_segment_%d.wav", time.Now().UnixNano())
	filepath := filepath.Join(tempDir, filename)

	file, err := os.Create(filepath)
	if err != nil {
		return "", fmt.Errorf("failed to create WAV file: %w", err)
	}
	defer file.Close()

	if err := r.writeWavHeader(file, len(samples)); err != nil {
		return "", fmt.Errorf("failed to write WAV header: %w", err)
	}
	// Write all samples in a single call
	if err := binary.Write(file, binary.LittleEndian, samples); err != nil {
		return "", fmt.Errorf("failed to write audio data: %w", err)
	}
	return filepath, nil
}

// writeWavHeader writes a WAV file header
func (r *Recorder) writeWavHeader(file *os.File, numSamples int) error {
	// WAV file format constants
	bitsPerSample := 16
	byteRate := int(r.sampleRate) * Channels * bitsPerSample / 8
	blockAlign := Channels * bitsPerSample / 8
	dataSize := numSamples * 2 // 2 bytes per sample (int16)
	fileSize := 36 + dataSize

	header := bytes.NewBuffer(nil)

	// RIFF header
	header.WriteString("RIFF")
	binary.Write(header, binary.LittleEndian, int32(fileSize))
	header.WriteString("WAVE")

	// fmt subchunk
	header.WriteString("fmt ")
	binary.Write(header, binary.LittleEndian, int32(16))            // Subchunk size
	binary.Write(header, binary.LittleEndian, int16(1))             // Audio format (PCM)
	binary.Write(header, binary.LittleEndian, int16(Channels))      // Num channels
	binary.Write(header, binary.LittleEndian, int32(r.sampleRate))  // Sample rate
	binary.Write(header, binary.LittleEndian, int32(byteRate))      // Byte rate
	binary.Write(header, binary.LittleEndian, int16(blockAlign))    // Block align
	binary.Write(header, binary.LittleEndian, int16(bitsPerSample)) // Bits per sample

	// data subchunk
	header.WriteString("data")
	binary.Write(header, binary.LittleEndian, int32(dataSize))

	_, err := file.Write(header.Bytes())
	return err
}

// GetDuration returns the duration of the recorded audio
func (r *Recorder) GetDuration() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	samples := len(r.buffer)
	seconds := float64(samples) / r.sampleRate
	return time.Duration(seconds * float64(time.Second))
}

// IsRecording returns whether the recorder is currently recording
func (r *Recorder) IsRecording() bool {
	return r.recording.Load()
}

// HasAudioActivity calculates if there is sufficient audio energy in the recorded buffer
func (r *Recorder) HasAudioActivity() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.buffer) == 0 {
		return false
	}

	// Calculate RMS energy in sliding windows of 100ms
	// 100ms window at 16kHz = 1600 samples
	windowSize := 1600
	if len(r.buffer) < windowSize {
		// For extremely short recordings, check if average absolute amplitude > threshold
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

	// Threshold: background quiet room noise is typically 10-50 RMS.
	// Low-level whispers or speech yield >100.
	// Normal speech usually yields 1000-8000 RMS.
	// Setting threshold to 100 is highly safe, fast, and conservative.
	return maxRMS > 100
}
