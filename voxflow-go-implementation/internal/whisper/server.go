package whisper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"voxflow/internal/logger"
)

// whisperServer is a whisper-server process holding one model resident, so each
// transcription skips the ~0.5s process spawn, Metal init and model load of whisper-cli.
type whisperServer struct {
	cmd  *exec.Cmd
	url  string
	done chan struct{} // closed once the process has exited and been reaped
}

var serverClient = &http.Client{Timeout: 5 * time.Minute}

// findWhisperServer locates whisper-server next to whisper-cli, on PATH, or in Homebrew.
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

func startWhisperServer(bin, modelPath string, threads int) (*whisperServer, error) {
	port, err := freePort()
	if err != nil {
		return nil, err
	}
	args := []string{"-m", modelPath, "--host", "127.0.0.1", "--port", strconv.Itoa(port), "-bs", "1", "-bo", "1", "-nf", "-nt"}
	if threads > 0 {
		args = append(args, "-t", strconv.Itoa(threads))
	}
	cmd := exec.Command(bin, args...)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	srv := &whisperServer{cmd: cmd, url: fmt.Sprintf("http://127.0.0.1:%d", port), done: make(chan struct{})}
	go func() {
		cmd.Wait()
		close(srv.done)
	}()

	deadline := time.Now().Add(15 * time.Second)
	for !srv.healthy() {
		if time.Now().After(deadline) {
			srv.stop()
			return nil, fmt.Errorf("not ready after 15s")
		}
		select {
		case <-srv.done:
			return nil, fmt.Errorf("exited during startup: %v", cmd.ProcessState)
		case <-time.After(100 * time.Millisecond):
		}
	}
	logger.Infof("[Whisper] whisper-server started: pid %d, %s, model %s", cmd.Process.Pid, srv.url, filepath.Base(modelPath))
	return srv, nil
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// healthy reports whether the server is bound and its model loaded.
func (w *whisperServer) healthy() bool {
	resp, err := serverClient.Get(w.url + "/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (w *whisperServer) stop() {
	w.cmd.Process.Kill()
	<-w.done
	logger.Infof("[Whisper] whisper-server stopped: pid %d", w.cmd.Process.Pid)
}

// transcribe posts wav to /inference. Every decoding field is sent on every request:
// the server only resets its per-request params after a successful request, so a
// failed one would otherwise leak its prompt/language into the next.
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

	resp, err := serverClient.Post(w.url+"/inference", mw.FormDataContentType(), &body)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
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
// installed). Runs without s.mu held so a slow start never blocks transcription.
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
	srv, err := startWhisperServer(bin, modelPath, threads)
	if err != nil {
		logger.Warnf("[Whisper] whisper-server failed to start, using whisper-cli per call: %v", err)
		return
	}
	s.mu.Lock()
	stale := !s.loaded || s.server != nil || s.modelPath != modelPath || s.threads != threads
	if !stale {
		s.server = srv
	}
	s.mu.Unlock()
	if stale {
		srv.stop()
		return
	}
	go s.watchServer(srv)
}

// stopServerLocked stops the resident server, if any. Call with s.mu held.
func (s *Service) stopServerLocked() {
	if srv := s.server; srv != nil {
		s.server = nil
		srv.stop()
	}
}

// watchServer clears the server when its process dies unexpectedly and restarts it once.
func (s *Service) watchServer(srv *whisperServer) {
	<-srv.done
	s.mu.Lock()
	if s.server != srv { // stopped on purpose
		s.mu.Unlock()
		return
	}
	s.server = nil
	restart := s.loaded && s.serverRestarts == 0
	if restart {
		s.serverRestarts++
	}
	s.mu.Unlock()
	logger.Warnf("[Whisper] whisper-server exited unexpectedly: pid %d, %v", srv.cmd.Process.Pid, srv.cmd.ProcessState)
	if restart {
		s.startServer()
	}
}
