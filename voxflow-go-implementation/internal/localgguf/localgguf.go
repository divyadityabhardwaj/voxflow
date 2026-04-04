package localgguf

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"voxflow/internal/logger"
)

var modelURLs = map[string]string{
	"qwen3-0.5b":    "https://huggingface.co/Qwen/Qwen3-0.5B-GGUF/resolve/main/qwen3-0.5b-q4_k_m.gguf",
	"qwen3-1.8b":    "https://huggingface.co/Qwen/Qwen3-1.8B-GGUF/resolve/main/qwen3-1.8b-q4_k_m.gguf",
	"qwen3-4.7b":    "https://huggingface.co/Qwen/Qwen3-4.7B-GGUF/resolve/main/qwen3-4.7b-q4_k_m.gguf",
	"qwen3-8b":      "https://huggingface.co/Qwen/Qwen3-8B-GGUF/resolve/main/qwen3-8b-q4_k_m.gguf",
	"qwen3-14b":     "https://huggingface.co/Qwen/Qwen3-14B-GGUF/resolve/main/qwen3-14b-q4_k_m.gguf",
	"qwen3-32b":     "https://huggingface.co/Qwen/Qwen3-32B-GGUF/resolve/main/qwen3-32b-q4_k_m.gguf",
	"qwen3-0.5b-q8": "https://huggingface.co/Qwen/Qwen3-0.5B-GGUF/resolve/main/qwen3-0.5b-q8_0.gguf",
	"qwen3-1.8b-q8": "https://huggingface.co/Qwen/Qwen3-1.8B-GGUF/resolve/main/qwen3-1.8b-q8_0.gguf",
	"qwen3-4.7b-q8": "https://huggingface.co/Qwen/Qwen3-4.7B-GGUF/resolve/main/qwen3-4.7b-q8_0.gguf",
	"llama3-8b":     "https://huggingface.co/Mozilla/Meta-Llama-3-8B-Instruct-GGUF/resolve/main/llama3-8b-instruct-q4_k_m.gguf",
	"phi4":          "https://huggingface.co/microsoft/Phi-4-mini-instruct-GGUF/resolve/main/phi-4-mini-instruct-q4_k_m.gguf",
}

// ollamaModelAliases maps the friendly display names to the aliases used by Ollama.
// These must match the actual tags on https://ollama.com/library/qwen3
var ollamaModelAliases = map[string]string{
	"qwen3-0.5b":    "qwen3:0.6b",
	"qwen3-1.8b":    "qwen3:1.7b",
	"qwen3-4.7b":    "qwen3:4b",
	"qwen3-8b":      "qwen3:8b",
	"qwen3-14b":     "qwen3:14b",
	"qwen3-32b":     "qwen3:32b",
	"qwen3-0.5b-q8": "qwen3:0.6b-q8_0",
	"qwen3-1.8b-q8": "qwen3:1.7b-q8_0",
	"qwen3-4.7b-q8": "qwen3:4b-q8_0",
	"llama3-8b":     "llama3:8b",
	"phi4":          "phi4",
}

// Use alternative public mirrors for models
func getDownloadURL(modelName string) string {
	// Try direct URL first
	if url, ok := modelURLs[modelName]; ok {
		return url
	}

	// For models that require auth, try the HF co-signer or mirror
	// This is a workaround - in production, user should set HF_TOKEN
	switch modelName {
	case "qwen3-0.5b", "qwen3-1.8b", "qwen3-4.7b", "qwen3-8b", "qwen3-14b", "qwen3-32b",
		"qwen3-0.5b-q8", "qwen3-1.8b-q8", "qwen3-4.7b-q8":
		// Try using huggingface.co/download which may work
		return fmt.Sprintf("https://huggingface.co/%s/resolve/main/%s", getModelRepo(modelName), getModelFile(modelName))
	}

	return modelURLs[modelName]
}

func getModelRepo(modelName string) string {
	repos := map[string]string{
		"qwen3-0.5b":    "Qwen/Qwen3-0.5B-GGUF",
		"qwen3-1.8b":    "Qwen/Qwen3-1.8B-GGUF",
		"qwen3-4.7b":    "Qwen/Qwen3-4.7B-GGUF",
		"qwen3-8b":      "Qwen/Qwen3-8B-GGUF",
		"qwen3-14b":     "Qwen/Qwen3-14B-GGUF",
		"qwen3-32b":     "Qwen/Qwen3-32B-GGUF",
		"qwen3-0.5b-q8": "Qwen/Qwen3-0.5B-GGUF",
		"qwen3-1.8b-q8": "Qwen/Qwen3-1.8B-GGUF",
		"qwen3-4.7b-q8": "Qwen/Qwen3-4.7B-GGUF",
		"llama3-8b":     "Mozilla/Meta-Llama-3-8B-Instruct-GGUF",
		"phi4":          "microsoft/Phi-4-mini-instruct-GGUF",
	}
	if repo, ok := repos[modelName]; ok {
		return repo
	}
	return "unknown"
}

func getModelFile(modelName string) string {
	files := map[string]string{
		"qwen3-0.5b":    "qwen3-0.5b-q4_k_m.gguf",
		"qwen3-1.8b":    "qwen3-1.8b-q4_k_m.gguf",
		"qwen3-4.7b":    "qwen3-4.7b-q4_k_m.gguf",
		"qwen3-8b":      "qwen3-8b-q4_k_m.gguf",
		"qwen3-14b":     "qwen3-14b-q4_k_m.gguf",
		"qwen3-32b":     "qwen3-32b-q4_k_m.gguf",
		"qwen3-0.5b-q8": "qwen3-0.5b-q8_0.gguf",
		"qwen3-1.8b-q8": "qwen3-1.8b-q8_0.gguf",
		"qwen3-4.7b-q8": "qwen3-4.7b-q8_0.gguf",
		"llama3-8b":     "llama3-8b-instruct-q4_k_m.gguf",
		"phi4":          "phi-4-mini-instruct-q4_k_m.gguf",
	}
	if file, ok := files[modelName]; ok {
		return file
	}
	return "model.gguf"
}

var modelDescriptions = map[string]string{
	"qwen3-0.5b":    "Qwen 3 0.5B - Fastest, lowest memory",
	"qwen3-1.8b":    "Qwen 3 1.8B - Fast, good quality",
	"qwen3-4.7b":    "Qwen 3 4.7B - Balanced speed/quality",
	"qwen3-8b":      "Qwen 3 8B - High quality",
	"qwen3-14b":     "Qwen 3 14B - Very high quality",
	"qwen3-32b":     "Qwen 3 32B - Best quality, slower",
	"qwen3-0.5b-q8": "Qwen 3 0.5B Q8 - Higher precision",
	"qwen3-1.8b-q8": "Qwen 3 1.8B Q8 - Higher precision",
	"qwen3-4.7b-q8": "Qwen 3 4.7B Q8 - Higher precision",
	"llama3-8b":     "Llama 3 8B - Meta's model",
	"phi4":          "Microsoft Phi-4 Mini",
}

func GetModelDescriptions() map[string]string {
	return modelDescriptions
}

func GetOllamaModelAlias(modelName string) string {
	if alias, ok := ollamaModelAliases[modelName]; ok {
		return alias
	}

	if idx := strings.Index(modelName, "-"); idx != -1 {
		return fmt.Sprintf("%s:%s", modelName[:idx], modelName[idx+1:])
	}

	return modelName
}

type ProgressCallback func(downloaded, total int64)

type Service struct {
	mu          sync.RWMutex
	loadedModel string
	modelPath   string
	loaded      bool
}

func NewService() *Service {
	return &Service{}
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

type ModelInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Size        int64  `json:"size"`
	Downloaded  bool   `json:"downloaded"`
	FilePath    string `json:"file_path"`
}

func (s *Service) GetModelSizeFromServer(modelName string) (int64, error) {
	logger.Debugf("[LocalGGUF] Getting model size from server for: %s", modelName)

	_, ok := modelURLs[modelName]
	if !ok {
		logger.Errorf("[LocalGGUF] Unknown model: %s", modelName)
		return 0, fmt.Errorf("unknown model: %s", modelName)
	}

	url := getDownloadURL(modelName)

	logger.Debugf("[LocalGGUF] Sending HEAD request to: %s", url)
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		logger.Errorf("[LocalGGUF] Failed to create HEAD request: %v", err)
		return 0, err
	}

	// Add User-Agent to avoid 401 from HuggingFace
	req.Header.Set("User-Agent", "voxflow/1.0")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.Errorf("[LocalGGUF] HEAD request failed: %v", err)
		return 0, err
	}
	defer resp.Body.Close()

	logger.Debugf("[LocalGGUF] HEAD response status: %d", resp.StatusCode)

	// If 401/403, return estimated size and proceed without checking
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		logger.Warnf("[LocalGGUF] Auth required for HEAD, using estimated size")
		estimatedSize := int64(5 * 1024 * 1024 * 1024) // Default 5GB estimate
		logger.Infof("[LocalGGUF] Model %s estimated size: %d bytes (auth required)", modelName, estimatedSize)
		return estimatedSize, nil
	}

	if resp.StatusCode == http.StatusNotFound {
		logger.Warnf("[LocalGGUF] HEAD returned 404. The model might not be accessible.")
		estimatedSize := int64(5 * 1024 * 1024 * 1024)
		logger.Infof("[LocalGGUF] Model %s estimated size: %d bytes (HEAD 404)", modelName, estimatedSize)
		return estimatedSize, nil
	}

	if resp.StatusCode != http.StatusOK {
		logger.Errorf("[LocalGGUF] HEAD request failed with status: %d", resp.StatusCode)
		return 0, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	logger.Infof("[LocalGGUF] Model %s size: %d bytes", modelName, resp.ContentLength)
	return resp.ContentLength, nil
}

func (s *Service) GetAllModels() ([]ModelInfo, error) {
	modelsDir, err := GetModelsDir()
	if err != nil {
		return nil, err
	}

	models := []ModelInfo{}
	for name := range modelURLs {
		modelPath := filepath.Join(modelsDir, fmt.Sprintf("%s.gguf", name))
		downloaded := false
		var fileSize int64
		if info, err := os.Stat(modelPath); err == nil && info.Size() > 1024*1024 {
			downloaded = true
			fileSize = info.Size()
		}

		models = append(models, ModelInfo{
			Name:        name,
			Description: modelDescriptions[name],
			Size:        fileSize,
			Downloaded:  downloaded,
			FilePath:    modelPath,
		})
	}

	return models, nil
}

func (s *Service) IsModelDownloaded(modelName string) (bool, error) {
	modelsDir, err := GetModelsDir()
	if err != nil {
		return false, err
	}
	modelPath := filepath.Join(modelsDir, fmt.Sprintf("%s.gguf", modelName))
	info, err := os.Stat(modelPath)
	if err != nil {
		return false, nil
	}
	return info.Size() > 1024*1024, nil
}

func (s *Service) DeleteModel(modelName string) error {
	modelsDir, err := GetModelsDir()
	if err != nil {
		return err
	}
	modelPath := filepath.Join(modelsDir, fmt.Sprintf("%s.gguf", modelName))
	return os.Remove(modelPath)
}

func (s *Service) DownloadModelWithContext(ctx context.Context, modelName string, progress ProgressCallback) error {
	logger.Debugf("[LocalGGUF] Starting download for model: %s", modelName)

	_, ok := modelURLs[modelName]
	if !ok {
		logger.Errorf("[LocalGGUF] Unknown model: %s", modelName)
		return fmt.Errorf("unknown model: %s", modelName)
	}

	url := getDownloadURL(modelName)

	logger.Debugf("[LocalGGUF] URL for %s: %s", modelName, url)

	modelsDir, err := GetModelsDir()
	if err != nil {
		logger.Errorf("[LocalGGUF] Failed to get models dir: %v", err)
		return err
	}

	logger.Debugf("[LocalGGUF] Models directory: %s", modelsDir)

	modelPath := filepath.Join(modelsDir, fmt.Sprintf("%s.gguf", modelName))
	logger.Debugf("[LocalGGUF] Target model path: %s", modelPath)

	// Check if already downloaded
	if info, err := os.Stat(modelPath); err == nil && info.Size() > 1024*1024 {
		logger.Infof("[LocalGGUF] Model already downloaded: %s (%d bytes)", modelName, info.Size())
		return nil
	}

	logger.Debugf("[LocalGGUF] Model not found, fetching size from server...")

	// Get size from server
	expectedSize, err := s.GetModelSizeFromServer(modelName)
	if err != nil {
		logger.Errorf("[LocalGGUF] Failed to get model size from server: %v", err)
		return fmt.Errorf("failed to get model size: %w", err)
	}
	logger.Infof("[LocalGGUF] Model %s size: %d bytes", modelName, expectedSize)

	logger.Debugf("[LocalGGUF] Creating HTTP request...")
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		logger.Errorf("[LocalGGUF] Failed to create request: %v", err)
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add User-Agent to avoid 401 from HuggingFace
	req.Header.Set("User-Agent", "voxflow/1.0")

	logger.Debugf("[LocalGGUF] Sending HTTP request...")
	client := &http.Client{Timeout: 30 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() == context.Canceled {
			logger.Warnf("[LocalGGUF] Download cancelled by user")
			return fmt.Errorf("download cancelled")
		}
		logger.Errorf("[LocalGGUF] HTTP request failed: %v", err)
		return fmt.Errorf("failed to download model: %w", err)
	}
	defer resp.Body.Close()

	logger.Debugf("[LocalGGUF] Response status: %d", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			logger.Errorf("[LocalGGUF] Model %s not found (HTTP 404)", modelName)
			return fmt.Errorf("failed to download model: HTTP %d", resp.StatusCode)
		}
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			logger.Errorf("[LocalGGUF] Authentication required for model download. Status: %d", resp.StatusCode)
			return fmt.Errorf("authentication required (HTTP %d)", resp.StatusCode)
		}
		logger.Errorf("[LocalGGUF] HTTP error: status %d", resp.StatusCode)
		return fmt.Errorf("failed to download model: HTTP %d", resp.StatusCode)
	}

	logger.Debugf("[LocalGGUF] Creating temp file...")
	tempPath := modelPath + ".tmp"
	file, err := os.Create(tempPath)
	if err != nil {
		logger.Errorf("[LocalGGUF] Failed to create temp file: %v", err)
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	totalSize := resp.ContentLength
	if totalSize <= 0 {
		totalSize = expectedSize
	}
	logger.Infof("[LocalGGUF] Starting download: %d bytes expected", totalSize)

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
			logger.Warnf("[LocalGGUF] Download cancelled during copy")
			return fmt.Errorf("download cancelled")
		}
		logger.Errorf("[LocalGGUF] Failed to copy data: %v", err)
		return fmt.Errorf("failed to save model: %w", err)
	}

	if ctx.Err() == context.Canceled {
		os.Remove(tempPath)
		logger.Warnf("[LocalGGUF] Download cancelled after copy")
		return fmt.Errorf("download cancelled")
	}

	logger.Debugf("[LocalGGUF] Download complete: %d bytes written", bytesWritten)

	minSize := int64(float64(expectedSize) * 0.95)
	if bytesWritten < minSize {
		os.Remove(tempPath)
		logger.Errorf("[LocalGGUF] Download incomplete: got %d bytes, expected at least %d", bytesWritten, minSize)
		return fmt.Errorf("download incomplete: got %d bytes, expected at least %d bytes", bytesWritten, minSize)
	}

	logger.Debugf("[LocalGGUF] Renaming temp file to final location...")
	if err := os.Rename(tempPath, modelPath); err != nil {
		os.Remove(tempPath)
		logger.Errorf("[LocalGGUF] Failed to rename temp file: %v", err)
		return fmt.Errorf("failed to finalize model file: %w", err)
	}

	logger.Infof("[LocalGGUF] Model %s downloaded successfully (%d bytes)", modelName, bytesWritten)
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

func (s *Service) DownloadModel(modelName string, progress ProgressCallback) error {
	return s.DownloadModelWithContext(context.Background(), modelName, progress)
}

func (s *Service) LoadModel(modelName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	modelPath, err := s.GetModelPath(modelName)
	if err != nil {
		return err
	}

	s.loadedModel = modelName
	s.modelPath = modelPath
	s.loaded = true

	return nil
}

func (s *Service) GetLoadedModelPath() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.modelPath
}

func (s *Service) GetModelPath(modelName string) (string, error) {
	modelsDir, err := GetModelsDir()
	if err != nil {
		return "", err
	}
	modelPath := filepath.Join(modelsDir, fmt.Sprintf("%s.gguf", modelName))
	info, err := os.Stat(modelPath)
	if err != nil {
		return "", fmt.Errorf("model not found: %s. Please download it first", modelName)
	}
	if info.Size() <= 1024*1024 {
		return "", fmt.Errorf("model file too small: %s", modelName)
	}
	return modelPath, nil
}

func (s *Service) IsLoaded() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.loaded
}

func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loaded = false
	return nil
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
			logger.Infof("[LocalGGUF] Cleaning up partial download: %s", entry.Name())
			os.Remove(tmpPath)
		}
	}
	return nil
}
