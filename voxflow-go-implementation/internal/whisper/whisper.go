package whisper

import (
	"context"
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

// DownloadModelWithContext downloads the specified model with cancellation support
func (s *Service) DownloadModelWithContext(ctx context.Context, modelSize string, progress ProgressCallback) error {
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
		if info.Size() > int64(float64(expectedSize)*0.9) {
			return nil // Already downloaded
		}
	}

	// Create HTTP request with context for cancellation
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() == context.Canceled {
			return fmt.Errorf("download cancelled")
		}
		return fmt.Errorf("failed to download model: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download model: HTTP %d", resp.StatusCode)
	}

	// Create temporary file
	tempPath := modelPath + ".tmp"
	file, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	totalSize := resp.ContentLength
	if totalSize <= 0 {
		totalSize = modelSizes[modelSize]
	}
	var downloaded int64

	// Create a cancellable reader
	reader := &cancellableProgressReader{
		ctx:    ctx,
		reader: resp.Body,
		onProgress: func(n int64) {
			downloaded += n
			if progress != nil {
				progress(downloaded, totalSize)
			}
		},
	}

	// Copy with progress and cancellation support
	bytesWritten, err := io.Copy(file, reader)
	file.Close()

	if err != nil {
		os.Remove(tempPath)
		if ctx.Err() == context.Canceled {
			return fmt.Errorf("download cancelled")
		}
		return fmt.Errorf("failed to save model: %w", err)
	}

	// Check if cancelled during download
	if ctx.Err() == context.Canceled {
		os.Remove(tempPath)
		return fmt.Errorf("download cancelled")
	}

	// Verify downloaded size
	expectedSize := modelSizes[modelSize]
	minSize := int64(float64(expectedSize) * 0.95)
	if bytesWritten < minSize {
		os.Remove(tempPath)
		return fmt.Errorf("download incomplete: got %d bytes, expected at least %d bytes", bytesWritten, minSize)
	}

	// Rename temp file to final name
	if err := os.Rename(tempPath, modelPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to finalize model file: %w", err)
	}

	fmt.Printf("[Whisper] Model %s downloaded successfully (%d bytes)\n", modelSize, bytesWritten)
	return nil
}

// cancellableProgressReader wraps an io.Reader with cancellation and progress
type cancellableProgressReader struct {
	ctx        context.Context
	reader     io.Reader
	onProgress func(n int64)
}

func (r *cancellableProgressReader) Read(p []byte) (int, error) {
	// Check for cancellation before each read
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	default:
	}

	n, err := r.reader.Read(p)
	if n > 0 && r.onProgress != nil {
		r.onProgress(int64(n))
	}
	return n, err
}

// DownloadModel downloads the specified model with progress callback (non-cancellable, for backwards compat)
func (s *Service) DownloadModel(modelSize string, progress ProgressCallback) error {
	return s.DownloadModelWithContext(context.Background(), modelSize, progress)
}

// CleanupPartialDownloads removes any stale .tmp files from failed downloads
func CleanupPartialDownloads() error {
	modelsDir, err := GetModelsDir()
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(modelsDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			tmpPath := filepath.Join(modelsDir, entry.Name())
			fmt.Printf("[Whisper] Cleaning up partial download: %s\n", entry.Name())
			os.Remove(tmpPath)
		}
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
	return s.TranscribeWithPrompt(wavPath, "")
}

// TranscribeWithPrompt transcribes the given WAV file using an optional initial prompt
// to provide context for the model (helps with streaming/chunked transcription).
func (s *Service) TranscribeWithPrompt(wavPath, prompt string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.loaded {
		return "", fmt.Errorf("model not loaded")
	}

	whisperBin := s.findWhisperBinary()
	if whisperBin == "" {
		return "", fmt.Errorf("whisper CLI binary not found. Please install whisper.cpp or provide the binary at ~/.voxflow/bin/whisper-cli")
	}
	return s.transcribeWithCLI(whisperBin, wavPath, prompt)
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
func (s *Service) transcribeWithCLI(whisperBin, wavPath, prompt string) (string, error) {
	// Create a temp file for output
	outputPath := wavPath + ".txt"
	defer os.Remove(outputPath)

	// Run whisper CLI
	args := []string{
		"-m", s.modelPath,
		"-f", wavPath,
		"-otxt",
		"--no-timestamps",
		"-of", strings.TrimSuffix(outputPath, ".txt"),
	}
	if prompt != "" {
		args = append(args, "--prompt", prompt)
	}

	cmd := exec.Command(whisperBin, args...)

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
