package whisper

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"voxflow/internal/logger"
)

var modelURLs = map[string]string{
	"tiny":   "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.bin",
	"base":   "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.bin",
	"small":  "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin",
	"medium": "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-medium.bin",
}

var modelSizes = map[string]int64{
	"tiny":   75 * 1024 * 1024,
	"base":   142 * 1024 * 1024,
	"small":  466 * 1024 * 1024,
	"medium": 1500 * 1024 * 1024,
}

// Pinned SHA-256 for download integrity
var modelSHA256s = map[string]string{
	"tiny":   "be07e048e1e599ad46341c8d2a135645097a538221678b7acdd1b1919c6e1b21",
	"base":   "60ed5bc3dd14eea856493d334349b405782ddcaf0028d4b5df4088345fba2efe",
	"small":  "1be3a9b2063867b937e64e2ec7483364a79917e157fa98c5d94b5c1fffea987b",
	"medium": "6c14d5adee5f86394037b4e4e8b59f1673b6cee10e3cf0b11bbdbee79c156208",
}

var ModelDescriptions = map[string]string{
	"tiny":   "Fastest, least accurate (~75 MB)",
	"base":   "Good balance of speed and accuracy (~142 MB)",
	"small":  "Better accuracy, slower (~466 MB)",
	"medium": "Best accuracy, slowest (~1.5 GB)",
}

type ProgressCallback func(downloaded, total int64)

type Service struct {
	modelSize   string
	modelPath   string
	whisperPath string // cached whisper-cli path, guarded by binMu
	binMu       sync.Mutex
	language    string
	threads     int
	prompt      string // initial prompt: custom vocabulary
	mu          sync.RWMutex
	loaded      bool

	server         *whisperServer              // resident whisper-server; nil means whisper-cli per call
	starting       map[*whisperServer]struct{} // spawned but not yet healthy; Close kills these too
	startsPending  int                         // LoadModel starts in flight, so WarmUp can wait for them
	serverRestarts int                         // crash restarts since LoadModel, capped at one
	noServer       bool                        // force the whisper-cli path (tests)
}

func NewService() *Service {
	return &Service{language: "en"}
}

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

func (s *Service) IsWhisperCLIInstalled() bool {
	return s.findWhisperBinary() != ""
}

type ModelInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Size        int64  `json:"size"`
	Downloaded  bool   `json:"downloaded"`
	FilePath    string `json:"file_path"`
}

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

func (s *Service) DeleteModel(modelSize string) error {
	modelsDir, err := GetModelsDir()
	if err != nil {
		return err
	}
	modelPath := filepath.Join(modelsDir, fmt.Sprintf("ggml-%s.bin", modelSize))
	return os.Remove(modelPath)
}

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
	// Reject tiny/corrupt files (<10MB).
	return info.Size() > 10*1024*1024, nil
}

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

	if info, err := os.Stat(modelPath); err == nil {
		expectedSize := modelSizes[modelSize]
		if info.Size() > int64(float64(expectedSize)*0.9) {
			return nil // Already downloaded
		}
	}

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

	bytesWritten, err := io.Copy(file, reader)
	file.Close()

	if err != nil {
		os.Remove(tempPath)
		if ctx.Err() == context.Canceled {
			return fmt.Errorf("download cancelled")
		}
		return fmt.Errorf("failed to save model: %w", err)
	}

	if ctx.Err() == context.Canceled {
		os.Remove(tempPath)
		return fmt.Errorf("download cancelled")
	}

	expectedSize := modelSizes[modelSize]
	minSize := int64(float64(expectedSize) * 0.95)
	if bytesWritten < minSize {
		os.Remove(tempPath)
		return fmt.Errorf("download incomplete: got %d bytes, expected at least %d bytes", bytesWritten, minSize)
	}

	if expectedHash, exists := modelSHA256s[modelSize]; exists {
		logger.Infof("[Whisper] Verifying SHA-256 integrity of downloaded model %s...", modelSize)
		if err := verifyFileSHA256(tempPath, expectedHash); err != nil {
			os.Remove(tempPath)
			return fmt.Errorf("integrity check failed for model %s: %w", modelSize, err)
		}
		logger.Infof("[Whisper] Integrity check passed for model %s", modelSize)
	}

	if err := os.Rename(tempPath, modelPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to finalize model file: %w", err)
	}

	logger.Infof("[Whisper] Model %s downloaded successfully (%d bytes)", modelSize, bytesWritten)
	return nil
}

func verifyFileSHA256(filePath string, expectedHash string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}

	actualHash := fmt.Sprintf("%x", h.Sum(nil))
	if actualHash != expectedHash {
		return fmt.Errorf("SHA-256 mismatch: got %s, expected %s", actualHash, expectedHash)
	}
	return nil
}

type cancellableProgressReader struct {
	ctx        context.Context
	reader     io.Reader
	onProgress func(n int64)
}

func (r *cancellableProgressReader) Read(p []byte) (int, error) {
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

func (s *Service) DownloadModel(modelSize string, progress ProgressCallback) error {
	return s.DownloadModelWithContext(context.Background(), modelSize, progress)
}

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
			logger.Infof("[Whisper] Cleaning up partial download: %s", entry.Name())
			os.Remove(tmpPath)
		}
	}
	return nil
}

func (s *Service) LoadModel(modelSize string) error {
	modelsDir, err := GetModelsDir()
	if err != nil {
		return err
	}

	modelPath := filepath.Join(modelsDir, fmt.Sprintf("ggml-%s.bin", modelSize))

	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return fmt.Errorf("model not found: %s. Please download it first", modelPath)
	}

	s.mu.Lock()
	if s.loaded && s.modelPath == modelPath && s.server != nil {
		s.mu.Unlock()
		return nil
	}
	s.modelSize = modelSize
	s.modelPath = modelPath
	s.loaded = true
	s.serverRestarts = 0
	s.stopServerLocked()
	s.startsPending++
	s.mu.Unlock()

	// Start in the background: recording must not be refused while the model loads;
	// transcribeWAV uses whisper-cli until the server is up.
	go func() {
		s.startServer()
		s.mu.Lock()
		s.startsPending--
		s.mu.Unlock()
	}()
	return nil
}

func (s *Service) awaitServer(max time.Duration) {
	deadline := time.Now().Add(max)
	for time.Now().Before(deadline) {
		s.mu.RLock()
		pending := s.startsPending > 0
		s.mu.RUnlock()
		if !pending {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (s *Service) Transcribe(wavPath string) (string, error) {
	s.mu.RLock()
	prompt := s.prompt
	s.mu.RUnlock()
	return s.TranscribeWithPrompt(wavPath, prompt)
}

func (s *Service) TranscribeWithPrompt(wavPath, prompt string) (string, error) {
	wav, err := os.ReadFile(wavPath)
	if err != nil {
		return "", err
	}
	return s.transcribeWAV(wav, wavPath, prompt)
}

// Server when up; else whisper-cli (needs wavPath or temp file).
func (s *Service) transcribeWAV(wav []byte, wavPath, prompt string) (string, error) {
	s.mu.RLock()
	loaded, modelPath, language, threads, srv := s.loaded, s.modelPath, s.language, s.threads, s.server
	s.mu.RUnlock()

	if !loaded {
		return "", fmt.Errorf("model not loaded")
	}

	if srv != nil {
		text, err := srv.transcribe(wav, language, prompt)
		if err == nil {
			return text, nil
		}
		logger.Warnf("[Whisper] whisper-server request failed, falling back to whisper-cli: %v", err)
	}

	whisperBin := s.findWhisperBinary()
	if whisperBin == "" {
		return "", fmt.Errorf("speech engine (whisper-cli) not found. Reinstall VoxFlow, or run: brew install whisper-cpp")
	}
	if wavPath == "" {
		var err error
		if wavPath, err = writeTempWav(wav); err != nil {
			return "", err
		}
		defer os.Remove(wavPath)
	}
	return s.transcribeWithCLI(whisperBin, modelPath, wavPath, prompt, language, threads)
}

func (s *Service) SetLanguage(language string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(language) == "" {
		language = "en"
	}
	s.language = strings.TrimSpace(language)
}

func (s *Service) SetPrompt(prompt string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prompt = strings.TrimSpace(prompt)
}

// Threads fixed at whisper-server start; restart if changed.
func (s *Service) SetThreads(threads int) {
	if threads < 0 {
		threads = 0
	}
	s.mu.Lock()
	changed := s.threads != threads
	s.threads = threads
	if changed {
		s.stopServerLocked()
	}
	s.mu.Unlock()
	if changed {
		s.startServer()
	}
}

func (s *Service) findWhisperBinary() string {
	s.binMu.Lock()
	defer s.binMu.Unlock()
	if s.whisperPath != "" && isSecureBinary(s.whisperPath) {
		return s.whisperPath
	}
	s.whisperPath = locateWhisperBinary()
	return s.whisperPath
}

func locateWhisperBinary() string {
	for _, p := range whisperCLICandidates() {
		if isSecureBinary(p) {
			return p
		}
	}
	return ""
}

// whisperCLICandidates lists where whisper-cli may be, best first: bundled in the app,
// then user-provided, then PATH, then Homebrew. Only whisper.cpp's own name counts: a
// bare "whisper" is usually OpenAI's Python CLI, which rejects whisper.cpp's flags.
func whisperCLICandidates() []string {
	var paths []string
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		paths = append(paths, filepath.Join(dir, "whisper-cli"), filepath.Join(dir, "..", "Resources", "whisper-cli"))
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		paths = append(paths, filepath.Join(home, ".voxflow", "bin", "whisper-cli"))
	}
	if p, err := exec.LookPath("whisper-cli"); err == nil {
		paths = append(paths, p)
	}
	for _, prefix := range []string{"/opt/homebrew", "/usr/local"} {
		paths = append(paths, filepath.Join(prefix, "bin", "whisper-cli"))
		versions, _ := filepath.Glob(filepath.Join(prefix, "Cellar", "whisper-cpp", "*", "bin", "whisper-cli"))
		slices.Reverse(versions)
		paths = append(paths, versions...)
	}
	if home != "" {
		paths = append(paths, filepath.Join(home, ".local", "bin", "whisper-cli"))
	}
	return paths
}

func (s *Service) transcribeWithCLI(whisperBin, modelPath, wavPath, prompt, language string, threads int) (string, error) {
	outputPath := wavPath + ".txt"
	defer os.Remove(outputPath)

	args := []string{
		"-m", modelPath,
		"-f", wavPath,
		"-otxt",
		"--no-timestamps",
		"-of", strings.TrimSuffix(outputPath, ".txt"),
		"-bs", "1",
		"-bo", "1",
		"--no-fallback",
	}
	if strings.TrimSpace(language) != "" {
		args = append(args, "-l", language)
	}
	if threads > 0 {
		args = append(args, "-t", strconv.Itoa(threads))
	}
	if prompt != "" {
		args = append(args, "--prompt", prompt)
	}

	cmd := exec.Command(whisperBin, args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("whisper CLI failed: %w, output: %s", err, string(output))
	}

	content, err := os.ReadFile(outputPath)
	if err != nil {
		return strings.TrimSpace(string(output)), nil
	}

	return strings.TrimSpace(string(content)), nil
}

func (s *Service) TranscribeSamples(samples []int16) (string, error) {
	if len(samples) == 0 {
		return "", fmt.Errorf("no samples")
	}
	s.mu.RLock()
	prompt := s.prompt
	s.mu.RUnlock()
	return s.transcribeWAV(wavBytes(samples, 16000), "", prompt)
}

func wavBytes(samples []int16, sampleRate int) []byte {
	dataSize := len(samples) * 2
	buf := bytes.NewBuffer(make([]byte, 0, 44+dataSize))
	le := binary.LittleEndian
	buf.WriteString("RIFF")
	binary.Write(buf, le, int32(36+dataSize))
	buf.WriteString("WAVEfmt ")
	binary.Write(buf, le, int32(16)) // fmt chunk size
	binary.Write(buf, le, int16(1))  // PCM
	binary.Write(buf, le, int16(1))  // mono
	binary.Write(buf, le, int32(sampleRate))
	binary.Write(buf, le, int32(sampleRate*2)) // byte rate
	binary.Write(buf, le, int16(2))            // block align
	binary.Write(buf, le, int16(16))           // bits per sample
	buf.WriteString("data")
	binary.Write(buf, le, int32(dataSize))
	binary.Write(buf, le, samples)
	return buf.Bytes()
}

func writeTempWav(wav []byte) (string, error) {
	f, err := os.CreateTemp("", "voxflow_*.wav")
	if err != nil {
		return "", err
	}
	if _, err := f.Write(wav); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), f.Close()
}

// Warm-up path; first whisper-server request also compiles Metal shaders.
func (s *Service) WarmUp() error {
	s.awaitServer(20 * time.Second)
	_, err := s.transcribeWAV(wavBytes(syntheticSamples(900*time.Millisecond), 16000), "", "")
	return err
}

// Synthetic tone so warm-up exercises the decoder without real speech.
func syntheticSamples(d time.Duration) []int16 {
	const sampleRate = 16000
	n := int(float64(sampleRate) * d.Seconds())
	if n < sampleRate/2 {
		n = sampleRate / 2
	}
	samples := make([]int16, n)
	for i := range samples {
		t := float64(i) / sampleRate
		envelope := 0.5 + 0.5*math.Sin(2*math.Pi*1.8*t)
		v := envelope * (0.55*math.Sin(2*math.Pi*180*t) + 0.35*math.Sin(2*math.Pi*320*t))
		samples[i] = int16(v * 12000)
	}
	return samples
}

func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loaded = false
	s.stopServerLocked()
	return nil
}

func isSecureBinary(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	mode := info.Mode()
	// Reject world-writable binaries.
	if (mode & 0002) != 0 {
		logger.Warnf("[Security] Binary at %s is world-writable! Rejecting for security.", path)
		return false
	}

	// Reject root-owned group-writable binaries.
	if (mode & 0020) != 0 {
		if sys, ok := info.Sys().(*syscall.Stat_t); ok {
			if sys.Uid == 0 {
				logger.Warnf("[Security] Binary at %s is owned by root and group-writable! Rejecting for security.", path)
				return false
			}
		}
	}

	return true
}
