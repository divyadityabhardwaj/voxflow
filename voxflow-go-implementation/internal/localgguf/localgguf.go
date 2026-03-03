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
	url, ok := modelURLs[modelName]
	if !ok {
		return 0, fmt.Errorf("unknown model: %s", modelName)
	}

	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return 0, err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

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
	url, ok := modelURLs[modelName]
	if !ok {
		return fmt.Errorf("unknown model: %s", modelName)
	}

	modelsDir, err := GetModelsDir()
	if err != nil {
		return err
	}

	modelPath := filepath.Join(modelsDir, fmt.Sprintf("%s.gguf", modelName))

	// Check if already downloaded
	if info, err := os.Stat(modelPath); err == nil && info.Size() > 1024*1024 {
		return nil
	}

	// Get size from server
	expectedSize, err := s.GetModelSizeFromServer(modelName)
	if err != nil {
		return fmt.Errorf("failed to get model size: %w", err)
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
		totalSize = expectedSize
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

	minSize := int64(float64(expectedSize) * 0.95)
	if bytesWritten < minSize {
		os.Remove(tempPath)
		return fmt.Errorf("download incomplete: got %d bytes, expected at least %d bytes", bytesWritten, minSize)
	}

	if err := os.Rename(tempPath, modelPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to finalize model file: %w", err)
	}

	fmt.Printf("[LocalGGUF] Model %s downloaded successfully (%d bytes)\n", modelName, bytesWritten)
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

	modelsDir, err := GetModelsDir()
	if err != nil {
		return err
	}

	modelPath := filepath.Join(modelsDir, fmt.Sprintf("%s.gguf", modelName))

	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return fmt.Errorf("model not found: %s. Please download it first", modelName)
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
			fmt.Printf("[LocalGGUF] Cleaning up partial download: %s\n", entry.Name())
			os.Remove(tmpPath)
		}
	}
	return nil
}
