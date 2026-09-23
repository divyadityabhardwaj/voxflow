package logger

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileRotatesWhileRunning(t *testing.T) {
	path := filepath.Join(t.TempDir(), "voxflow.log")
	oldMax := maxLogSize
	maxLogSize = 1024
	defer func() {
		maxLogSize = oldMax
		Close()
		Setup(os.Stdout, INFO)
	}()

	if err := File(path, INFO); err != nil {
		t.Fatal(err)
	}
	Setup(logFile, INFO) // keep the test output quiet; File's rotation still applies

	for i := 0; i < rotateCheckEvery; i++ {
		Infof("line %d", i)
	}

	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("expected rotated file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() > maxLogSize {
		t.Fatalf("active log is %d bytes after rotation", info.Size())
	}
}
