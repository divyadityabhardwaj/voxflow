package whisper

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
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

// Pinned to a repo commit so the files served always match the SHA-256s below.
var modelBaseURL = "https://huggingface.co/ggerganov/whisper.cpp/resolve/5359861c739e955e79d9a303bcbc70fb988958b1/"

type catalogModel struct {
	name, description, sha256 string
	size                      int64
}

// SHA-256 and byte sizes are the lfs oid/size from the Hugging Face tree API at that commit.
var modelCatalog = []catalogModel{
	{"tiny", "Fastest, least accurate", "be07e048e1e599ad46341c8d2a135645097a538221678b7acdd1b1919c6e1b21", 77691713},
	{"tiny.en", "Fastest, English only", "921e4cf8686fdd993dcd081a5da5b6c365bfde1162e72b08d75ac75289920b1f", 77704715},
	{"base", "Balanced speed and accuracy (recommended)", "60ed5bc3dd14eea856493d334349b405782ddcaf0028d4b5df4088345fba2efe", 147951465},
	{"base.en", "Balanced, more accurate for English only", "a03779c86df3323075f5e796cb2ce5029f00ec8869eee3fdfb897afe36c6d002", 147964211},
	{"small", "More accurate, slower", "1be3a9b2063867b937e64e2ec7483364a79917e157fa98c5d94b5c1fffea987b", 487601967},
	{"small.en", "More accurate for English only, slower", "c6138d6d58ecc8322097e0f987c32f1be8bb0a18532a3f88f734d1bbf9c41e5d", 487614201},
	{"medium", "Very accurate, slow, largest download", "6c14d5adee5f86394037b4e4e8b59f1673b6cee10e3cf0b11bbdbee79c156208", 1533763059},
	{"large-v3-turbo-q5_0", "Most accurate, slow; best on Apple Silicon", "394221709cd5ad1f40c46e6031ca61bce88931e6e088c188294c6d5a55ffa7e2", 574041195},
}

func findCatalogModel(name string) (catalogModel, bool) {
	i := slices.IndexFunc(modelCatalog, func(m catalogModel) bool { return m.name == name })
	if i < 0 {
		return catalogModel{}, false
	}
	return modelCatalog[i], true
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
	serverRestarts []time.Time                 // crash restarts within restartWindow
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
	for _, m := range modelCatalog {
		modelPath := filepath.Join(modelsDir, fmt.Sprintf("ggml-%s.bin", m.name))
		downloaded := false
		if info, err := os.Stat(modelPath); err == nil && info.Size() > 10*1024*1024 {
			downloaded = true
		}

		models = append(models, ModelInfo{
			Name:        m.name,
			Description: m.description,
			Size:        m.size,
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

var (
	downloadClient = func() *http.Client {
		t := http.DefaultTransport.(*http.Transport).Clone()
		t.ResponseHeaderTimeout = 30 * time.Second
		return &http.Client{Transport: t}
	}()
	// A connection that stays open but stops sending never errors on its own.
	downloadIdleTimeout = 30 * time.Second
	errDownloadStalled  = errors.New("download stalled")
)

// DownloadModelWithContext resumes a partial file left by a failed or cancelled
// attempt in this run (CleanupPartialDownloads clears them at launch); the SHA-256
// check covers the stitched file.
func (s *Service) DownloadModelWithContext(ctx context.Context, modelSize string, progress ProgressCallback) error {
	m, ok := findCatalogModel(modelSize)
	if !ok {
		return fmt.Errorf("unknown model size: %s", modelSize)
	}

	modelsDir, err := GetModelsDir()
	if err != nil {
		return err
	}

	modelPath := filepath.Join(modelsDir, fmt.Sprintf("ggml-%s.bin", modelSize))

	if info, err := os.Stat(modelPath); err == nil && info.Size() > m.size*9/10 {
		return nil // Already downloaded
	}

	tempPath := modelPath + ".tmp"
	var offset int64
	if info, err := os.Stat(tempPath); err == nil && info.Size() < m.size {
		offset = info.Size()
	}
	if err := checkFreeSpace(modelsDir, m.size-offset); err != nil {
		return err
	}

	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelBaseURL+"ggml-"+modelSize+".bin", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}

	resp, err := downloadClient.Do(req)
	if err != nil {
		return downloadError(ctx, fmt.Errorf("failed to download model: %w", err))
	}
	defer resp.Body.Close()

	flags := os.O_WRONLY | os.O_CREATE
	switch {
	case resp.StatusCode == http.StatusOK:
		offset = 0
		flags |= os.O_TRUNC
	case resp.StatusCode == http.StatusPartialContent && strings.HasPrefix(resp.Header.Get("Content-Range"), fmt.Sprintf("bytes %d-", offset)):
		flags |= os.O_APPEND
	default:
		os.Remove(tempPath)
		return fmt.Errorf("failed to download model: HTTP %d", resp.StatusCode)
	}

	file, err := os.OpenFile(tempPath, flags, 0644)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	totalSize := m.size
	if resp.ContentLength > 0 {
		totalSize = offset + resp.ContentLength
	}
	downloaded := offset

	idle := time.AfterFunc(downloadIdleTimeout, func() { cancel(errDownloadStalled) })
	defer idle.Stop()
	var lastProgress time.Time
	reader := &cancellableProgressReader{
		ctx:    ctx,
		reader: resp.Body,
		onProgress: func(n int64) {
			idle.Reset(downloadIdleTimeout)
			downloaded += n
			// Each call becomes a webview event; ~10/s is plenty for a progress bar.
			if progress != nil && (downloaded >= totalSize || time.Since(lastProgress) >= 100*time.Millisecond) {
				lastProgress = time.Now()
				progress(downloaded, totalSize)
			}
		},
	}

	_, err = io.Copy(file, reader)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		// Keep the partial file so a retry resumes where this attempt stopped.
		return downloadError(ctx, fmt.Errorf("failed to save model: %w", err))
	}

	if downloaded != m.size {
		os.Remove(tempPath)
		return fmt.Errorf("download incomplete: got %d bytes, expected %d", downloaded, m.size)
	}

	logger.Infof("[Whisper] Verifying SHA-256 integrity of downloaded model %s...", modelSize)
	if err := verifyFileSHA256(tempPath, m.sha256); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("integrity check failed for model %s: %w", modelSize, err)
	}
	logger.Infof("[Whisper] Integrity check passed for model %s", modelSize)

	if err := os.Rename(tempPath, modelPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to finalize model file: %w", err)
	}

	logger.Infof("[Whisper] Model %s downloaded successfully (%d bytes)", modelSize, downloaded)
	return nil
}

func downloadError(ctx context.Context, err error) error {
	switch cause := context.Cause(ctx); {
	case errors.Is(cause, errDownloadStalled):
		return fmt.Errorf("download stalled: no data for %s. Check your connection and try again", downloadIdleTimeout)
	case cause != nil:
		return fmt.Errorf("download cancelled")
	}
	return err
}

func checkFreeSpace(dir string, need int64) error {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return nil // can't tell; let the download try
	}
	free := int64(st.Bavail) * int64(st.Bsize)
	if need += need / 10; free < need {
		return fmt.Errorf("not enough disk space: this model needs %s free, only %s is available", formatSize(need), formatSize(free))
	}
	return nil
}

func formatSize(b int64) string {
	if b >= 1<<30 {
		return fmt.Sprintf("%.1f GB", float64(b)/(1<<30))
	}
	return fmt.Sprintf("%d MB", b>>20)
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
	s.serverRestarts = nil
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
		"-bs", "5",
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
