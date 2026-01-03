package whisper

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// Model sizes and their download URLs (Hugging Face)
var modelURLs = map[string]string{
	"tiny":   "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.bin",
	"base":   "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.bin",
	"small":  "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin",
	"medium": "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-medium.bin",
}

// Model sizes in bytes (approximate)
var modelSizes = map[string]int64{
	"tiny":   75 * 1024 * 1024,   // ~75 MB
	"base":   142 * 1024 * 1024,  // ~142 MB
	"small":  466 * 1024 * 1024,  // ~466 MB
	"medium": 1500 * 1024 * 1024, // ~1.5 GB
}

// Model descriptions for UI
var ModelDescriptions = map[string]string{
	"tiny":   "Fastest, least accurate (~75 MB)",
	"base":   "Good balance of speed and accuracy (~142 MB)",
	"small":  "Better accuracy, slower (~466 MB)",
	"medium": "Best accuracy, slowest (~1.5 GB)",
}

// Whisper CLI binary download URL (pre-compiled for macOS)
// Using ggerganov's official releases
const whisperCLIDownloadURL = "https://github.com/ggerganov/whisper.cpp/releases/download/v1.7.2/whisper-blas-bin-x64.zip"
const whisperCLIMacARM = "https://github.com/ggerganov/whisper.cpp/releases/download/v1.7.2/whisper-bin-arm64-apple-darwin.zip"

// ProgressCallback is called during model download
type ProgressCallback func(downloaded, total int64)

// Service handles Whisper transcription
type Service struct {
	modelSize   string
	modelPath   string
	whisperPath string // Path to whisper.cpp binary
	mu          sync.RWMutex
	loaded      bool
}

// NewService creates a new Whisper service
func NewService() *Service {
	return &Service{}
}

// GetModelsDir returns the directory where models are stored
func GetModelsDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	modelsDir := filepath.Join(homeDir, ".voxflow", "models")
	if err := os.MkdirAll(modelsDir, 0755); err != nil {
		return "", err
	}
	return modelsDir, nil
}

// GetBinDir returns the directory for binaries
func GetBinDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	binDir := filepath.Join(homeDir, ".voxflow", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return "", err
	}
	return binDir, nil
}

// IsWhisperCLIInstalled checks if whisper-cli is available
func (s *Service) IsWhisperCLIInstalled() bool {
	return s.findWhisperBinary() != ""
}

// EnsureWhisperCLI ensures whisper-cli is installed, downloading if needed
func (s *Service) EnsureWhisperCLI(progress ProgressCallback) error {
	// First check if already installed
	if s.findWhisperBinary() != "" {
		return nil
	}

	// Download and install whisper-cli
	return s.downloadWhisperCLI(progress)
}

// downloadWhisperCLI downloads the whisper-cli binary
func (s *Service) downloadWhisperCLI(progress ProgressCallback) error {
	binDir, err := GetBinDir()
	if err != nil {
		return err
	}

	// For now, we'll create a script that tells users to install via Homebrew
	// A production app would download pre-compiled binaries
	whisperPath := filepath.Join(binDir, "whisper-cli")

	// Check if homebrew version exists and symlink it
	homebrewPaths := []string{
		"/opt/homebrew/bin/whisper-cli",
		"/opt/homebrew/Cellar/whisper-cpp/1.8.2/bin/whisper-cli",
		"/usr/local/bin/whisper-cli",
	}

	for _, p := range homebrewPaths {
		if _, err := os.Stat(p); err == nil {
			// Create symlink
			os.Remove(whisperPath) // Remove if exists
			if err := os.Symlink(p, whisperPath); err != nil {
				return fmt.Errorf("failed to create symlink: %w", err)
			}
			return nil
		}
	}

	// If no homebrew version, return helpful error
	return fmt.Errorf("whisper-cli not found. Please install via: brew install whisper-cpp")
}

// ModelInfo contains information about a model for the UI
type ModelInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Size        int64  `json:"size"`
	Downloaded  bool   `json:"downloaded"`
	FilePath    string `json:"file_path"`
}

// GetAllModels returns info about all available models
func (s *Service) GetAllModels() ([]ModelInfo, error) {
	modelsDir, err := GetModelsDir()
	if err != nil {
		return nil, err
	}

	models := []ModelInfo{}
	for _, name := range []string{"tiny", "base", "small", "medium"} {
		modelPath := filepath.Join(modelsDir, fmt.Sprintf("ggml-%s.bin", name))
		downloaded := false
		if info, err := os.Stat(modelPath); err == nil && info.Size() > 10*1024*1024 {
			downloaded = true
		}

		models = append(models, ModelInfo{
			Name:        name,
			Description: ModelDescriptions[name],
			Size:        modelSizes[name],
			Downloaded:  downloaded,
			FilePath:    modelPath,
		})
	}

	return models, nil
}

// DeleteModel deletes a downloaded model
func (s *Service) DeleteModel(modelSize string) error {
	modelsDir, err := GetModelsDir()
	if err != nil {
		return err
	}
	modelPath := filepath.Join(modelsDir, fmt.Sprintf("ggml-%s.bin", modelSize))
	return os.Remove(modelPath)
}

// IsModelDownloaded checks if a model is already downloaded
func (s *Service) IsModelDownloaded(modelSize string) (bool, error) {
	modelsDir, err := GetModelsDir()
	if err != nil {
		return false, err
	}
	modelPath := filepath.Join(modelsDir, fmt.Sprintf("ggml-%s.bin", modelSize))
	info, err := os.Stat(modelPath)
	if err != nil {
		return false, nil
	}
	// Check if file size is reasonable (at least 10MB)
	return info.Size() > 10*1024*1024, nil
}

// DownloadModel downloads the specified model with progress callback
func (s *Service) DownloadModel(modelSize string, progress ProgressCallback) error {
	url, ok := modelURLs[modelSize]
	if !ok {
		return fmt.Errorf("unknown model size: %s", modelSize)
	}

	modelsDir, err := GetModelsDir()
	if err != nil {
		return err
	}

	modelPath := filepath.Join(modelsDir, fmt.Sprintf("ggml-%s.bin", modelSize))

	// Check if already exists and has correct size
	if info, err := os.Stat(modelPath); err == nil {
		expectedSize := modelSizes[modelSize]
		// Allow 10% tolerance
		if info.Size() > int64(float64(expectedSize)*0.9) {
			return nil // Already downloaded
		}
	}

	// Create temporary file
	tempPath := modelPath + ".tmp"
	file, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer file.Close()

	// Download the model
	resp, err := http.Get(url)
	if err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to download model: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		os.Remove(tempPath)
		return fmt.Errorf("failed to download model: HTTP %d", resp.StatusCode)
	}

	totalSize := resp.ContentLength
	if totalSize <= 0 {
		totalSize = modelSizes[modelSize]
	}
	var downloaded int64

	// Create progress reader
	reader := &progressReader{
		reader: resp.Body,
		onProgress: func(n int64) {
			downloaded += n
			if progress != nil {
				progress(downloaded, totalSize)
			}
		},
	}

	// Copy with progress
	_, err = io.Copy(file, reader)
	if err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to save model: %w", err)
	}

	file.Close()

	// Rename temp file to final name
	if err := os.Rename(tempPath, modelPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to finalize model file: %w", err)
	}

	return nil
}

// LoadModel loads the Whisper model
func (s *Service) LoadModel(modelSize string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	modelsDir, err := GetModelsDir()
	if err != nil {
		return err
	}

	modelPath := filepath.Join(modelsDir, fmt.Sprintf("ggml-%s.bin", modelSize))

	// Check if model exists
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return fmt.Errorf("model not found: %s. Please download it first", modelPath)
	}

	s.modelSize = modelSize
	s.modelPath = modelPath
	s.loaded = true

	return nil
}

// Transcribe transcribes the given WAV file using whisper.cpp CLI
func (s *Service) Transcribe(wavPath string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.loaded {
		return "", fmt.Errorf("model not loaded")
	}

	// First, try to use whisper.cpp binary if available
	whisperBin := s.findWhisperBinary()
	if whisperBin != "" {
		return s.transcribeWithCLI(whisperBin, wavPath)
	}

	// Fall back to using go-whisper (if we can build it)
	return s.transcribeWithGoWhisper(wavPath)
}

// findWhisperBinary looks for whisper.cpp binary
func (s *Service) findWhisperBinary() string {
	// Check in our bin directory
	binDir, _ := GetBinDir()
	whisperPath := filepath.Join(binDir, "whisper-cli")
	if _, err := os.Stat(whisperPath); err == nil {
		return whisperPath
	}

	// Check in PATH
	if path, err := exec.LookPath("whisper"); err == nil {
		return path
	}
	if path, err := exec.LookPath("whisper-cli"); err == nil {
		return path
	}

	// Check common locations on macOS
	commonPaths := []string{
		"/opt/homebrew/bin/whisper-cli",
		"/opt/homebrew/Cellar/whisper-cpp/1.8.2/bin/whisper-cli",
		"/usr/local/bin/whisper",
		"/usr/local/bin/whisper-cli",
		"/opt/homebrew/bin/whisper",
		filepath.Join(os.Getenv("HOME"), ".local/bin/whisper"),
		filepath.Join(os.Getenv("HOME"), ".local/bin/whisper-cli"),
	}
	for _, p := range commonPaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return ""
}

// transcribeWithCLI uses the whisper.cpp CLI
func (s *Service) transcribeWithCLI(whisperBin, wavPath string) (string, error) {
	// Create a temp file for output
	outputPath := wavPath + ".txt"
	defer os.Remove(outputPath)

	// Run whisper CLI
	cmd := exec.Command(whisperBin,
		"-m", s.modelPath,
		"-f", wavPath,
		"-otxt",
		"--no-timestamps",
		"-of", strings.TrimSuffix(outputPath, ".txt"),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("whisper CLI failed: %w, output: %s", err, string(output))
	}

	// Read the output file
	content, err := os.ReadFile(outputPath)
	if err != nil {
		// Try to parse from stdout
		return strings.TrimSpace(string(output)), nil
	}

	return strings.TrimSpace(string(content)), nil
}

// transcribeWithGoWhisper uses the Go whisper bindings
// This is a fallback that requires building whisper.cpp
func (s *Service) transcribeWithGoWhisper(wavPath string) (string, error) {
	// Read WAV file and convert to samples
	samples, err := readWavFile(wavPath)
	if err != nil {
		return "", fmt.Errorf("failed to read WAV file: %w", err)
	}

	// For now, return a message indicating CLI is needed
	// In production, we would use the whisper.cpp Go bindings here
	_ = samples
	return "", fmt.Errorf("whisper CLI binary not found. Please install whisper.cpp or provide the binary at ~/.voxflow/bin/whisper-cli")
}

// Close closes the service
func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loaded = false
	return nil
}

// IsLoaded returns whether a model is loaded
func (s *Service) IsLoaded() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.loaded
}

// progressReader wraps an io.Reader to report progress
type progressReader struct {
	reader     io.Reader
	onProgress func(n int64)
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	if n > 0 && pr.onProgress != nil {
		pr.onProgress(int64(n))
	}
	return n, err
}

// readWavFile reads a WAV file and returns the samples as float32
func readWavFile(path string) ([]float32, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Read and parse RIFF header
	reader := bufio.NewReader(file)

	// Skip to data chunk
	header := make([]byte, 44)
	if _, err := io.ReadFull(reader, header); err != nil {
		return nil, fmt.Errorf("failed to read WAV header: %w", err)
	}

	// Verify RIFF header
	if string(header[0:4]) != "RIFF" || string(header[8:12]) != "WAVE" {
		return nil, fmt.Errorf("invalid WAV file format")
	}

	// Get audio format info from header
	// Assuming standard 16-bit PCM WAV

	// Read all remaining data as samples
	var samples []float32
	buf := make([]byte, 4096)

	for {
		n, err := reader.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		// Convert bytes to int16 samples, then to float32
		for i := 0; i < n-1; i += 2 {
			sample := int16(binary.LittleEndian.Uint16(buf[i : i+2]))
			samples = append(samples, float32(sample)/32768.0)
		}
	}

	return samples, nil
}
