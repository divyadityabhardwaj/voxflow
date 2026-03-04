package llamaserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"
)

const (
	defaultHost = "127.0.0.1"
	defaultPort = 8081
)

type Server struct {
	mu        sync.Mutex
	cmd       *exec.Cmd
	port      int
	modelPath string
}

func NewServer() *Server {
	return &Server{port: defaultPort}
}

func (s *Server) BaseURL() string {
	return fmt.Sprintf("http://%s:%d", defaultHost, s.port)
}

func (s *Server) Ensure(modelPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cmd != nil && s.modelPath == modelPath {
		return nil
	}

	if s.cmd != nil {
		_ = s.stopLocked()
	}

	return s.startLocked(modelPath)
}

func (s *Server) startLocked(modelPath string) error {
	binPath, err := exec.LookPath("llama-server")
	if err != nil {
		return fmt.Errorf("llama-server not found in PATH. Install via: brew install llama.cpp")
	}

	args := []string{
		"--host", defaultHost,
		"--port", fmt.Sprintf("%d", s.port),
		"--model", modelPath,
		"--n-predict", "2048",
	}

	cmd := exec.Command(binPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start llama-server: %w", err)
	}

	s.cmd = cmd
	s.modelPath = modelPath

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.waitReady(ctx); err != nil {
		_ = s.stopLocked()
		return err
	}

	return nil
}

func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopLocked()
}

func (s *Server) stopLocked() error {
	if s.cmd == nil || s.cmd.Process == nil {
		s.cmd = nil
		s.modelPath = ""
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
	s.modelPath = ""
	return nil
}

func (s *Server) waitReady(ctx context.Context) error {
	client := &http.Client{Timeout: 2 * time.Second}
	url := fmt.Sprintf("%s/v1/models", s.BaseURL())

	for {
		if ctx.Err() != nil {
			return errors.New("llama-server failed to start")
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
