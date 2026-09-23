package whisper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
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

// whisperServer is a whisper-server process holding one model resident, so each
type whisperServer struct {
	cmd    *exec.Cmd
	url    string
	done   chan struct{} // closed once the process has exited and been reaped
	output *tailBuffer   // last few KB of its stdout/stderr
}

// Inference deadlines come from inferenceTimeout; /health must answer quickly.
var (
	serverClient = &http.Client{}
	healthClient = &http.Client{Timeout: 2 * time.Second}
)

// inferenceTimeout is generous for any model on the CPU; a request past it means a
// wedged server that would stall every later chunk too.
func inferenceTimeout(wavLen int) time.Duration {
	audio := time.Duration(max(wavLen-44, 0)) * time.Second / (16000 * 2)
	return 10*time.Second + 3*audio
}

const outputTailSize = 4 << 10

// tailBuffer keeps the last outputTailSize bytes written to it.
type tailBuffer struct {
	mu  sync.Mutex
	buf []byte
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if extra := len(t.buf) - outputTailSize; extra > 0 {
		t.buf = append(t.buf[:0], t.buf[extra:]...)
	}
	return len(p), nil
}

func (t *tailBuffer) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return strings.TrimSpace(string(t.buf))
}

func findWhisperServer(cliPath string) string {
	var candidates []string
	if cliPath != "" {
		candidates = append(candidates, filepath.Join(filepath.Dir(cliPath), "whisper-server"))
		if resolved, err := filepath.EvalSymlinks(cliPath); err == nil {
			candidates = append(candidates, filepath.Join(filepath.Dir(resolved), "whisper-server"))
		}
	}
	if p, err := exec.LookPath("whisper-server"); err == nil {
		candidates = append(candidates, p)
	}
	candidates = append(candidates, "/opt/homebrew/bin/whisper-server", "/usr/local/bin/whisper-server")
	for _, p := range candidates {
		if isSecureBinary(p) {
			return p
		}
	}
	return ""
}

func spawnWhisperServer(bin, modelPath string, threads int) (*whisperServer, error) {
	port, err := freePort()
	if err != nil {
		return nil, err
	}
	args := []string{"-m", modelPath, "--host", "127.0.0.1", "--port", strconv.Itoa(port), "-bs", "1", "-bo", "1", "-nf", "-nt"}
	if threads > 0 {
		args = append(args, "-t", strconv.Itoa(threads))
	}
	cmd := exec.Command(bin, args...)
	output := &tailBuffer{}
	cmd.Stdout, cmd.Stderr = output, output
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	srv := &whisperServer{cmd: cmd, url: fmt.Sprintf("http://127.0.0.1:%d", port), done: make(chan struct{}), output: output}
	go func() {
		cmd.Wait()
		close(srv.done)
	}()
	writePidFile(cmd.Process.Pid)
	return srv, nil
}

func (w *whisperServer) waitReady() error {
	deadline := time.Now().Add(15 * time.Second)
	for !w.healthy() {
		if time.Now().After(deadline) {
			w.stop()
			return fmt.Errorf("not ready after 15s")
		}
		select {
		case <-w.done:
			return fmt.Errorf("exited during startup: %v\n%s", w.cmd.ProcessState, w.output)
		case <-time.After(100 * time.Millisecond):
		}
	}
	logger.Infof("[Whisper] whisper-server started: pid %d, %s", w.cmd.Process.Pid, w.url)
	return nil
}

// macOS has no parent-death signal, so a crash or force-quit would leave the
// server (and its resident model) running forever. Record the pid and reap it
func pidFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".voxflow", "whisper-server.pid")
}

func writePidFile(pid int) {
	if p := pidFilePath(); p != "" {
		_ = os.WriteFile(p, []byte(strconv.Itoa(pid)), 0600)
	}
}

func reapStaleServer() {
	p := pidFilePath()
	if p == "" {
		return
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return
	}
	os.Remove(p)
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 1 {
		return
	}
	out, err := exec.Command("ps", "-o", "comm=", "-p", strconv.Itoa(pid)).Output()
	if err != nil || filepath.Base(strings.TrimSpace(string(out))) != "whisper-server" {
		return
	}
	logger.Warnf("[Whisper] Killing orphaned whisper-server from a previous run: pid %d", pid)
	_ = syscall.Kill(pid, syscall.SIGKILL)
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func (w *whisperServer) healthy() bool {
	resp, err := healthClient.Get(w.url + "/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (w *whisperServer) stop() {
	w.cmd.Process.Kill()
	<-w.done
	if p := pidFilePath(); p != "" {
		os.Remove(p)
	}
	logger.Infof("[Whisper] whisper-server stopped: pid %d", w.cmd.Process.Pid)
}

// Send every decoding field each request: server only resets params after success, else they leak.
func (w *whisperServer) transcribe(wav []byte, language, prompt string) (string, error) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("file", "audio.wav")
	if err != nil {
		return "", err
	}
	part.Write(wav)
	mw.WriteField("response_format", "json")
	mw.WriteField("temperature", "0")
	mw.WriteField("temperature_inc", "0") // whisper-server 1.8 ignores -nf; this is what disables fallback passes
	mw.WriteField("prompt", prompt)
	if language != "" {
		mw.WriteField("language", language)
	}
	mw.Close()

	timeout := inferenceTimeout(len(wav))
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.url+"/inference", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	var data []byte
	resp, err := serverClient.Do(req)
	if err == nil {
		defer resp.Body.Close()
		data, err = io.ReadAll(resp.Body)
	}
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			logger.Warnf("[Whisper] whisper-server gave no answer within %s; killing it so it restarts", timeout)
			w.cmd.Process.Kill()
		}
		return "", err
	}
	var out struct {
		Text  string `json:"text"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(data, &out); err != nil || resp.StatusCode != http.StatusOK || out.Error != "" {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return strings.TrimSpace(out.Text), nil
}

// startServer spawns whisper-server for the loaded model and installs it unless the
// service changed underneath (Close, another model or thread count, a server already
// installed). Runs without s.mu held so a slow start never blocks transcription;
func (s *Service) startServer() {
	s.mu.RLock()
	loaded, noServer, modelPath, threads := s.loaded, s.noServer, s.modelPath, s.threads
	s.mu.RUnlock()
	if !loaded || noServer {
		return
	}
	bin := findWhisperServer(s.findWhisperBinary())
	if bin == "" {
		logger.Infof("[Whisper] whisper-server not found, using whisper-cli per call")
		return
	}
	reapStaleServer()
	srv, err := spawnWhisperServer(bin, modelPath, threads)
	if err != nil {
		logger.Warnf("[Whisper] whisper-server failed to start, using whisper-cli per call: %v", err)
		return
	}
	s.mu.Lock()
	if s.starting == nil {
		s.starting = map[*whisperServer]struct{}{}
	}
	s.starting[srv] = struct{}{}
	s.mu.Unlock()

	err = srv.waitReady()

	s.mu.Lock()
	delete(s.starting, srv)
	stale := err != nil || !s.loaded || s.server != nil || s.modelPath != modelPath || s.threads != threads
	if !stale {
		s.server = srv
	}
	s.mu.Unlock()
	if err != nil {
		logger.Warnf("[Whisper] whisper-server failed to start, using whisper-cli per call: %v", err)
		return
	}
	if stale {
		srv.stop()
		return
	}
	go s.watchServer(srv)
}

// Call with s.mu held.
func (s *Service) stopServerLocked() {
	for srv := range s.starting {
		delete(s.starting, srv)
		srv.stop()
	}
	if srv := s.server; srv != nil {
		s.server = nil
		srv.stop()
	}
}

// watchServer clears the server when its process dies unexpectedly and restarts it,
// unless it already crashed maxRestarts times within restartWindow: then whisper-cli
// serves every call until the next LoadModel.
func (s *Service) watchServer(srv *whisperServer) {
	<-srv.done
	s.mu.Lock()
	if s.server != srv { // stopped on purpose
		s.mu.Unlock()
		return
	}
	s.server = nil
	loaded, restart := s.loaded, false
	if loaded {
		s.serverRestarts, restart = allowRestart(s.serverRestarts, time.Now())
	}
	s.mu.Unlock()
	logger.Warnf("[Whisper] whisper-server exited unexpectedly: pid %d, %v\n%s", srv.cmd.Process.Pid, srv.cmd.ProcessState, srv.output)
	if restart {
		s.startServer()
	} else if loaded {
		logger.Warnf("[Whisper] whisper-server crashed %d times in %s; using whisper-cli per call", maxRestarts, restartWindow)
	}
}

const (
	maxRestarts   = 3
	restartWindow = 5 * time.Minute
)

// allowRestart drops restarts older than restartWindow and, if another fits, records one at now.
func allowRestart(restarts []time.Time, now time.Time) ([]time.Time, bool) {
	recent := slices.DeleteFunc(restarts, func(t time.Time) bool { return now.Sub(t) >= restartWindow })
	if len(recent) >= maxRestarts {
		return recent, false
	}
	return append(recent, now), true
}
