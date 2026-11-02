package update

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const maxDownload = 512 << 20
const maxExtracted = 1 << 30

// Prepared lives beside the installed app so replacement uses same-volume renames.
// User configuration, history and models are never copied or removed.
type Prepared struct {
	Stage  string
	Target string
}

func (p Prepared) Cleanup() { _ = os.RemoveAll(p.Stage) }

func installedBundle(executable string) (string, error) {
	executable, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return "", err
	}
	macos := filepath.Dir(executable)
	contents := filepath.Dir(macos)
	bundle := filepath.Dir(contents)
	if filepath.Base(macos) != "MacOS" || filepath.Base(contents) != "Contents" ||
		filepath.Ext(bundle) != ".app" || strings.Contains(bundle, "/AppTranslocation/") || strings.HasPrefix(bundle, "/Volumes/") {
		return "", fmt.Errorf("move VoxFlow to Applications and open it there before updating")
	}
	return bundle, nil
}

// InstallReason explains installations that cannot be updated without privileges.
func InstallReason() string {
	if runtime.GOOS != "darwin" {
		return "In-app updates are currently available on macOS only."
	}
	executable, err := os.Executable()
	if err != nil {
		return "Couldn't locate the installed app."
	}
	target, err := installedBundle(executable)
	if err != nil {
		return err.Error()
	}
	if _, err := os.Stat(filepath.Join(target, "Contents", "MacOS", "voxflow-updater")); err != nil {
		return "Install the latest DMG once to enable in-app updates."
	}
	return ""
}

// Prepare downloads and verifies the archive before extracting into a private
// staging directory. Nothing in the running app is changed here.
func Prepare(ctx context.Context, info Info, progress func(state string, downloaded int64)) (prepared Prepared, err error) {
	if !info.Available || !info.CanInstall || !validAsset(info.asset, info.tag, runtime.GOOS, runtime.GOARCH) {
		return prepared, fmt.Errorf("no verified update is available for this Mac")
	}
	executable, err := os.Executable()
	if err != nil {
		return prepared, err
	}
	target, err := installedBundle(executable)
	if err != nil {
		return prepared, err
	}
	stage, err := os.MkdirTemp(filepath.Dir(target), ".voxflow-update-")
	if err != nil {
		return prepared, fmt.Errorf("VoxFlow's folder isn't writable; move it to ~/Applications or a writable Applications folder: %w", err)
	}
	prepared = Prepared{Stage: stage, Target: target}
	defer func() {
		if err != nil {
			prepared.Cleanup()
		}
	}()
	archive := filepath.Join(stage, "update.zip")
	if err = download(ctx, info.asset, archive, func(n int64) { progress("downloading", n) }); err != nil {
		return prepared, err
	}
	progress("verifying", info.asset.Size)
	if err = extract(ctx, archive, stage); err != nil {
		return prepared, fmt.Errorf("unpack update: %w", err)
	}
	app := filepath.Join(stage, "voxflow.app")
	if err = verifyBundle(ctx, app, info.Latest); err != nil {
		return prepared, err
	}
	if err = os.Remove(archive); err != nil {
		return prepared, err
	}
	return prepared, nil
}

type progressWriter struct {
	n        int64
	last     time.Time
	progress func(int64)
}

func (w *progressWriter) Write(p []byte) (int, error) {
	w.n += int64(len(p))
	if time.Since(w.last) >= 150*time.Millisecond {
		w.progress(w.n)
		w.last = time.Now()
	}
	return len(p), nil
}

func download(ctx context.Context, asset releaseAsset, destination string, progress func(int64)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "VoxFlow-Updater")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download update: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download update: GitHub returned %s", resp.Status)
	}
	if asset.Size <= 0 || asset.Size > maxDownload {
		return fmt.Errorf("invalid update size")
	}
	f, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	hash := sha256.New()
	w := &progressWriter{progress: progress}
	n, err := io.Copy(io.MultiWriter(f, hash, w), io.LimitReader(resp.Body, asset.Size+1))
	if err != nil {
		return fmt.Errorf("download update: %w", err)
	}
	if n != asset.Size || "sha256:"+hex.EncodeToString(hash.Sum(nil)) != asset.Digest {
		return fmt.Errorf("update verification failed: size or SHA-256 doesn't match GitHub; please try again")
	}
	if err := f.Close(); err != nil {
		return err
	}
	progress(n)
	return nil
}

func extract(ctx context.Context, archive, destination string) error {
	z, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer z.Close()
	if len(z.File) > 10000 {
		return fmt.Errorf("update archive contains too many files")
	}
	var total uint64
	seen := map[string]bool{}
	for _, entry := range z.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := strings.TrimSuffix(entry.Name, "/")
		if path.Clean(name) != name || !filepath.IsLocal(name) || strings.Contains(name, "\\") ||
			(name != "voxflow.app" && !strings.HasPrefix(name, "voxflow.app/")) || seen[name] {
			return fmt.Errorf("unsafe update archive path %q", entry.Name)
		}
		seen[name] = true
		if entry.Mode()&os.ModeType != 0 && !entry.FileInfo().IsDir() {
			return fmt.Errorf("update archive contains a link or special file")
		}
		if entry.UncompressedSize64 > maxExtracted || total > maxExtracted-entry.UncompressedSize64 {
			return fmt.Errorf("update archive is too large")
		}
		total += entry.UncompressedSize64
		target := filepath.Join(destination, filepath.FromSlash(name))
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if err := extractFile(entry, target); err != nil {
			return err
		}
	}
	return nil
}

func extractFile(entry *zip.File, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	in, err := entry.Open()
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, io.LimitReader(in, maxExtracted+1)); err != nil {
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chmod(target, entry.Mode().Perm()&0755|0400)
}

func verifyBundle(ctx context.Context, app, version string) error {
	for _, name := range []string{"voxflow", "voxflow-updater", "whisper-cli", "whisper-server"} {
		f, err := os.Stat(filepath.Join(app, "Contents", "MacOS", name))
		if err != nil || !f.Mode().IsRegular() || f.Mode().Perm()&0111 == 0 {
			return fmt.Errorf("update is missing executable %s", name)
		}
	}
	plist := filepath.Join(app, "Contents", "Info.plist")
	for key, expected := range map[string]string{
		"CFBundleIdentifier":         "com.wails.voxflow",
		"CFBundleExecutable":         "voxflow",
		"CFBundleShortVersionString": version,
	} {
		out, err := exec.CommandContext(ctx, "/usr/libexec/PlistBuddy", "-c", "Print :"+key, plist).Output()
		if err != nil || strings.TrimSpace(string(out)) != expected {
			return fmt.Errorf("update has an unexpected app identity or version")
		}
	}
	if out, err := exec.CommandContext(ctx, "/usr/bin/codesign", "--verify", "--deep", "--strict", app).CombinedOutput(); err != nil {
		return fmt.Errorf("update code signature is invalid: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
