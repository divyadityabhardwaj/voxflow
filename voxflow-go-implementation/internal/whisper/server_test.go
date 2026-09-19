package whisper

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// speechClip synthesizes "hello world this is a test" as 16 kHz mono PCM via macOS
func speechClip(t *testing.T) ([]int16, string) {
	t.Helper()
	dir := t.TempDir()
	aiff := filepath.Join(dir, "x.aiff")
	wav := filepath.Join(dir, "x.wav")
	if out, err := exec.Command("say", "-o", aiff, "hello world this is a test").CombinedOutput(); err != nil {
		t.Skipf("say unavailable: %v: %s", err, out)
	}
	if out, err := exec.Command("afconvert", "-f", "WAVE", "-d", "LEI16@16000", "-c", "1", aiff, wav).CombinedOutput(); err != nil {
		t.Skipf("afconvert unavailable: %v: %s", err, out)
	}
	data, err := os.ReadFile(wav)
	if err != nil {
		t.Fatal(err)
	}
	i := bytes.Index(data, []byte("data"))
	if i < 0 {
		t.Fatal("no data chunk in WAV")
	}
	pcm := data[i+8:]
	samples := make([]int16, len(pcm)/2)
	if err := binary.Read(bytes.NewReader(pcm), binary.LittleEndian, samples); err != nil {
		t.Fatal(err)
	}
	return samples, wav
}

func newTestService(t *testing.T, noServer bool) *Service {
	t.Helper()
	home, _ := os.UserHomeDir()
	if _, err := os.Stat(filepath.Join(home, ".voxflow", "models", "ggml-base.bin")); err != nil {
		t.Skip("ggml-base.bin not downloaded")
	}
	svc := NewService()
	svc.noServer = noServer
	if svc.findWhisperBinary() == "" {
		t.Skip("whisper-cli not installed")
	}
	if err := svc.LoadModel("base"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { svc.Close() })
	return svc
}

func currentServer(svc *Service) *whisperServer {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.server
}

func helloChecker(t *testing.T) func(text string, err error) {
	return func(text string, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(strings.ToLower(text), "hello") {
			t.Fatalf("transcript %q does not contain hello", text)
		}
	}
}

func timeCalls(t *testing.T, svc *Service, samples []int16) string {
	t.Helper()
	var each []string
	start := time.Now()
	for i := 0; i < 3; i++ {
		s := time.Now()
		if _, err := svc.TranscribeSamples(samples); err != nil {
			t.Fatal(err)
		}
		each = append(each, time.Since(s).Round(time.Millisecond).String())
	}
	return fmt.Sprintf("3 calls in %s (%s)", time.Since(start).Round(time.Millisecond), strings.Join(each, ", "))
}

func TestServerTranscribes(t *testing.T) {
	if findWhisperServer("") == "" {
		t.Skip("whisper-server not installed")
	}
	samples, wavPath := speechClip(t)
	hello := helloChecker(t)
	svc := newTestService(t, false)
	if err := svc.WarmUp(); err != nil { // waits for the background start
		t.Fatal(err)
	}
	srv := currentServer(svc)
	if srv == nil {
		t.Fatal("whisper-server did not start")
	}

	hello(svc.TranscribeSamples(samples))
	hello(svc.TranscribeWithPrompt(wavPath, ""))
	t.Logf("server: %s", timeCalls(t, svc, samples))

	// A crashed server must not break the next call and gets one restart.
	srv.cmd.Process.Kill()
	<-srv.done
	hello(svc.TranscribeSamples(samples))
	for deadline := time.Now().Add(20 * time.Second); currentServer(svc) == nil && time.Now().Before(deadline); {
		time.Sleep(100 * time.Millisecond)
	}
	srv = currentServer(svc)
	if srv == nil {
		t.Fatal("whisper-server was not restarted after crashing")
	}
	hello(svc.TranscribeSamples(samples))

	svc.Close()
	select {
	case <-srv.done:
	case <-time.After(5 * time.Second):
		t.Fatal("whisper-server still running after Close")
	}
	if currentServer(svc) != nil {
		t.Fatal("server still set after Close")
	}
}

func TestCLITranscribes(t *testing.T) {
	samples, wavPath := speechClip(t)
	hello := helloChecker(t)
	svc := newTestService(t, true)
	if currentServer(svc) != nil {
		t.Fatal("server started with noServer set")
	}
	if err := svc.WarmUp(); err != nil {
		t.Fatal(err)
	}
	hello(svc.TranscribeSamples(samples))
	hello(svc.TranscribeWithPrompt(wavPath, ""))
	t.Logf("cli: %s", timeCalls(t, svc, samples))
}
