package update

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDownloadVerification(t *testing.T) {
	for _, test := range []struct {
		name   string
		size   int64
		digest string
		status int
		ok     bool
	}{
		{"valid", 7, fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("payload"))), 200, true},
		{"corrupt", 7, "sha256:" + strings.Repeat("0", 64), 200, false},
		{"truncated", 8, "", 200, false},
		{"oversized response", 6, "", 200, false},
		{"invalid size", maxDownload + 1, "", 200, false},
		{"http failure", 7, "", 404, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			mockHTTP(t, func(*http.Request) (*http.Response, error) { return response(test.status, "payload"), nil })
			file := filepath.Join(t.TempDir(), "update.zip")
			var downloaded int64
			err := download(context.Background(), releaseAsset{URL: "https://github.com/update", Size: test.size, Digest: test.digest}, file, func(n int64) { downloaded = n })
			if (err == nil) != test.ok {
				t.Fatalf("err = %v", err)
			}
			if test.ok && downloaded != 7 {
				t.Fatalf("progress = %d", downloaded)
			}
		})
	}
}

func writeArchive(t *testing.T, entries []struct {
	name string
	mode os.FileMode
	body string
}) string {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for _, entry := range entries {
		h := &zip.FileHeader{Name: entry.name, Method: zip.Deflate}
		h.SetMode(entry.mode)
		f, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(entry.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "update.zip")
	if err := os.WriteFile(file, b.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestExtractRejectsUnsafeArchives(t *testing.T) {
	for _, name := range []string{"../escaped", "/absolute", "voxflow.app/../../escaped", "voxflow.app/../evil", "voxflow.app\\evil", "another.app/file"} {
		t.Run(name, func(t *testing.T) {
			file := writeArchive(t, []struct {
				name string
				mode os.FileMode
				body string
			}{{name, 0644, "bad"}})
			if err := extract(context.Background(), file, t.TempDir()); err == nil {
				t.Fatal("accepted unsafe path")
			}
		})
	}
	for _, mode := range []os.FileMode{os.ModeSymlink | 0777, os.ModeNamedPipe | 0600} {
		file := writeArchive(t, []struct {
			name string
			mode os.FileMode
			body string
		}{{"voxflow.app/link", mode, "../../escape"}})
		if err := extract(context.Background(), file, t.TempDir()); err == nil {
			t.Fatal("accepted link or special file")
		}
	}
	file := writeArchive(t, []struct {
		name string
		mode os.FileMode
		body string
	}{{"voxflow.app/file", 0644, "one"}, {"voxflow.app/file", 0644, "two"}})
	if err := extract(context.Background(), file, t.TempDir()); err == nil {
		t.Fatal("accepted duplicate path")
	}
}

func TestExtractPreservesExecutableAndCancels(t *testing.T) {
	file := writeArchive(t, []struct {
		name string
		mode os.FileMode
		body string
	}{{"voxflow.app/Contents/MacOS/voxflow", 0755, "binary"}})
	dir := t.TempDir()
	if err := extract(context.Background(), file, dir); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(filepath.Join(dir, "voxflow.app/Contents/MacOS/voxflow"))
	if err != nil || fi.Mode().Perm() != 0755 {
		t.Fatalf("executable = %v, %v", fi, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := extract(ctx, file, t.TempDir()); err != context.Canceled {
		t.Fatalf("err = %v", err)
	}
}

func TestInstalledBundle(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Folder with spaces", "VoxFlow.app")
	file := filepath.Join(root, "Contents/MacOS/voxflow")
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, nil, 0755); err != nil {
		t.Fatal(err)
	}
	got, err := installedBundle(file)
	want, _ := filepath.EvalSymlinks(root)
	if err != nil || got != want {
		t.Fatalf("got %s, %v; want %s", got, err, want)
	}
	for _, relative := range []string{"AppTranslocation/token/d/VoxFlow.app/Contents/MacOS/voxflow", "bin/voxflow"} {
		file := filepath.Join(t.TempDir(), relative)
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, nil, 0755); err != nil {
			t.Fatal(err)
		}
		if _, err := installedBundle(file); err == nil {
			t.Fatalf("accepted %s", file)
		}
	}
}
