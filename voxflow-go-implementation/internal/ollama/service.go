package ollama

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"voxflow/internal/logger"
)

const (
	defaultHost      = "127.0.0.1"
	defaultPort      = 11434
	serverReadyDelay = 30 * time.Second
)

type Service struct {
	mu   sync.Mutex
	cmd  *exec.Cmd
	port int
}

type ProgressCallback func(downloaded, total int64)

func NewService() *Service {
	return &Service{port: defaultPort}
}

func (s *Service) BaseURL() string {
	return fmt.Sprintf("http://%s:%d", defaultHost, s.port)
}

func (s *Service) EnsureServer(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cmd != nil {
		return nil
	}

	return s.startLocked(ctx)
}

func (s *Service) startLocked(ctx context.Context) error {
	// First, check if Ollama is already running (e.g. user started it, or system service)
	checkCtx, checkCancel := context.WithTimeout(ctx, 3*time.Second)
	defer checkCancel()
	if err := s.waitReady(checkCtx); err == nil {
		logger.Infof("[Ollama] Server already running on %s:%d", defaultHost, s.port)
		return nil
	}

	binPath, err := exec.LookPath("ollama")
	if err != nil {
		return fmt.Errorf("ollama not found in PATH. Install via: brew install ollama")
	}

	logger.Infof("[Ollama] Starting ollama serve on %s:%d...", defaultHost, s.port)

	args := []string{"serve"}
	// Use Background context for the serve process — it must stay alive beyond this call.
	// The caller's ctx is only used for the readiness timeout check below.
	cmd := exec.CommandContext(context.Background(), binPath, args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("OLLAMA_HOST=%s:%d", defaultHost, s.port))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ollama serve: %w", err)
	}

	s.cmd = cmd

	readyCtx, cancel := context.WithTimeout(ctx, serverReadyDelay)
	defer cancel()

	if err := s.waitReady(readyCtx); err != nil {
		logger.Errorf("[Ollama] Server failed to become ready within %v", serverReadyDelay)
		_ = s.stopLocked()
		return err
	}

	logger.Infof("[Ollama] Server is ready")
	return nil
}

func (s *Service) waitReady(ctx context.Context) error {
	client := &http.Client{Timeout: 2 * time.Second}
	url := fmt.Sprintf("%s/v1/models", s.BaseURL())

	for {
		if ctx.Err() != nil {
			return errors.New("ollama serve failed to start")
		}

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return err
		}

		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}

		time.Sleep(300 * time.Millisecond)
	}
}

func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopLocked()
}

func (s *Service) stopLocked() error {
	if s.cmd == nil || s.cmd.Process == nil {
		s.cmd = nil
		return nil
	}

	_ = s.cmd.Process.Signal(os.Interrupt)

	done := make(chan error, 1)
	go func() {
		done <- s.cmd.Wait()
	}()

	select {
	case <-time.After(5 * time.Second):
		_ = s.cmd.Process.Kill()
		<-done
	case <-done:
	}

	s.cmd = nil
	return nil
}

func (s *Service) PullModel(ctx context.Context, model string, progress ProgressCallback) error {
	if model == "" {
		return fmt.Errorf("model name is required")
	}

	if ctx == nil {
		ctx = context.Background()
	}

	if progress != nil {
		progress(0, 1)
	}

	logger.Infof("[Ollama] Pulling model %s via API", model)

	// Use Ollama HTTP API instead of CLI for real-time progress
	url := fmt.Sprintf("%s/api/pull", s.BaseURL())
	reqBody := fmt.Sprintf(`{"name":"%s","stream":true}`, model)

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create pull request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("failed to pull model %s: %w", model, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("pull failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	// Parse streaming JSON responses for progress
	decoder := json.NewDecoder(resp.Body)
	for decoder.More() {
		var msg struct {
			Status    string `json:"status"`
			Total     int64  `json:"total"`
			Completed int64  `json:"completed"`
			Error     string `json:"error"`
		}
		if err := decoder.Decode(&msg); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			// EOF or partial read at end is normal
			break
		}

		if msg.Error != "" {
			return fmt.Errorf("pull failed: %s", msg.Error)
		}

		if progress != nil && msg.Total > 0 {
			progress(msg.Completed, msg.Total)
		}

		logger.Debugf("[Ollama] Pull status: %s (%d/%d)", msg.Status, msg.Completed, msg.Total)
	}

	if progress != nil {
		progress(1, 1)
	}

	logger.Infof("[Ollama] Model %s pulled successfully", model)
	return nil
}

func (s *Service) RemoveModel(ctx context.Context, model string) error {
	if model == "" {
		return fmt.Errorf("model name is required")
	}

	if ctx == nil {
		ctx = context.Background()
	}

	binPath, err := exec.LookPath("ollama")
	if err != nil {
		return fmt.Errorf("ollama not found in PATH. Install via: brew install ollama")
	}

	cmd := exec.CommandContext(ctx, binPath, "rm", model)
	cmd.Env = append(os.Environ(), fmt.Sprintf("OLLAMA_HOST=%s:%d", defaultHost, s.port))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	logger.Infof("[Ollama] Removing model %s", model)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove model %s: %w", model, err)
	}

	return nil
}

func (s *Service) ListInstalledModels(ctx context.Context) (map[string]int64, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	binPath, err := exec.LookPath("ollama")
	if err != nil {
		return nil, fmt.Errorf("ollama not found in PATH")
	}

	installed := make(map[string]int64)

	cmd := exec.CommandContext(ctx, binPath, "list", "--json")
	cmd.Env = append(os.Environ(), fmt.Sprintf("OLLAMA_HOST=%s:%d", defaultHost, s.port))
	output, err := cmd.CombinedOutput()
	if err != nil {
		outStr := string(output)
		if strings.Contains(outStr, "could not connect") {
			return installed, nil
		}
		if strings.Contains(outStr, "unknown flag") {
			cmd = exec.CommandContext(ctx, binPath, "list")
			cmd.Env = append(os.Environ(), fmt.Sprintf("OLLAMA_HOST=%s:%d", defaultHost, s.port))
			output, err = cmd.CombinedOutput()
			if err != nil {
				if strings.Contains(string(output), "could not connect") {
					return installed, nil
				}
				return nil, fmt.Errorf("failed to list ollama models: %s", strings.TrimSpace(string(output)))
			}
			lines := strings.Split(string(output), "\n")
			for i, line := range lines {
				if i == 0 || strings.TrimSpace(line) == "" {
					continue
				}
				parts := strings.Fields(line)
				if len(parts) > 0 && parts[0] != "NAME" {
					installed[parts[0]] = 0
				}
			}
			return installed, nil
		}
		return nil, fmt.Errorf("failed to list ollama models: %s", strings.TrimSpace(outStr))
	}

	if len(output) == 0 {
		return installed, nil
	}

	var wrapper struct {
		Models []map[string]any `json:"models"`
	}
	if err := json.Unmarshal(output, &wrapper); err == nil && len(wrapper.Models) > 0 {
		for _, entry := range wrapper.Models {
			s.addModelEntry(installed, entry)
		}
		return installed, nil
	}

	var entries []map[string]any
	if err := json.Unmarshal(output, &entries); err != nil {
		return nil, fmt.Errorf("failed to parse ollama list output: %w", err)
	}

	for _, entry := range entries {
		s.addModelEntry(installed, entry)
	}

	return installed, nil
}

func (s *Service) addModelEntry(installed map[string]int64, entry map[string]any) {
	if entry == nil {
		return
	}

	var size int64
	if s, ok := entry["size"].(float64); ok {
		size = int64(s)
	}

	if name, ok := entry["name"].(string); ok && name != "" {
		installed[name] = size
	}
	if model, ok := entry["model"].(string); ok && model != "" {
		installed[model] = size
	}
}

func (s *Service) IsModelInstalled(ctx context.Context, model string) (bool, error) {
	if model == "" {
		return false, fmt.Errorf("model name is required")
	}

	installed, err := s.ListInstalledModels(ctx)
	if err != nil {
		return false, err
	}

	_, ok := installed[model]
	return ok, nil
}
