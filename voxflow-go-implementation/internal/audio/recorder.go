package audio

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gordonklaus/portaudio"
)

const (
	SampleRate      = 16000 // Whisper expects 16kHz
	Channels        = 1     // Mono
	FramesPerBuffer = 1024
)

// Recorder handles audio capture from the microphone
type Recorder struct {
	stream     *portaudio.Stream
	buffer     []int16
	mu         sync.Mutex
	recording  bool
	sampleRate float64
}

// NewRecorder creates a new audio recorder
func NewRecorder() *Recorder {
	return &Recorder{
		sampleRate: SampleRate,
		buffer:     make([]int16, 0),
	}
}

// Initialize initializes PortAudio
func (r *Recorder) Initialize() error {
	return portaudio.Initialize()
}

// Terminate cleans up PortAudio
func (r *Recorder) Terminate() error {
	return portaudio.Terminate()
}

// Start begins recording audio
func (r *Recorder) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.recording {
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
	r.recording = true

	// Start the stream
	if err := stream.Start(); err != nil {
		r.recording = false
		return fmt.Errorf("failed to start audio stream: %w", err)
	}

	// Start goroutine to read audio data
	go r.readLoop(inputBuffer)

	return nil
}

// readLoop continuously reads audio data from the stream
func (r *Recorder) readLoop(inputBuffer []int16) {
	for {
		r.mu.Lock()
		if !r.recording {
			r.mu.Unlock()
			return
		}
		stream := r.stream
		r.mu.Unlock()

		if stream == nil {
			return
		}

		// Read from the stream
		err := stream.Read()
		if err != nil {
			fmt.Printf("Error reading audio: %v\n", err)
			continue
		}

		// Append to buffer
		r.mu.Lock()
		r.buffer = append(r.buffer, inputBuffer...)
		r.mu.Unlock()
	}
}

// Stop stops recording and returns the path to the WAV file
func (r *Recorder) Stop() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.recording {
		return "", fmt.Errorf("not recording")
	}

	r.recording = false

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

	// Write audio data
	for _, sample := range r.buffer {
		if err := binary.Write(file, binary.LittleEndian, sample); err != nil {
			return "", fmt.Errorf("failed to write audio data: %w", err)
		}
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
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.recording
}
