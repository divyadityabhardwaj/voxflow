package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type installPlan struct {
	Stage     string `json:"stage"`
	Target    string `json:"target"`
	ParentPID int    `json:"parent_pid"`
	ErrorFile string `json:"error_file"`
}

// StartInstaller hands off to a standalone helper, detached from the app. The
// readiness handshake ensures a missing/broken helper never makes the app quit.
func StartInstaller(ctx context.Context, p Prepared, dataDir string) error {
	helper := filepath.Join(p.Stage, "voxflow-updater")
	if err := copyHelper(filepath.Join(p.Target, "Contents", "MacOS", "voxflow-updater"), helper); err != nil {
		return fmt.Errorf("prepare update helper: %w", err)
	}
	plan := installPlan{Stage: p.Stage, Target: p.Target, ParentPID: os.Getpid(), ErrorFile: filepath.Join(dataDir, "update-error")}
	data, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	planFile := filepath.Join(p.Stage, "plan.json")
	if err := os.WriteFile(planFile, data, 0600); err != nil {
		return err
	}
	log, err := os.OpenFile(filepath.Join(dataDir, "update.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer log.Close()
	cmd := exec.Command(helper, planFile)
	cmd.Dir = "/"
	cmd.Stdout, cmd.Stderr = log, log
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start update helper: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case err := <-done:
			return fmt.Errorf("update helper exited before installation: %v; see update.log", err)
		case <-ctx.Done():
			_ = cmd.Process.Kill()
			return ctx.Err()
		case <-timer.C:
			_ = cmd.Process.Kill()
			return fmt.Errorf("update helper didn't become ready; see update.log")
		case <-ticker.C:
			if _, err := os.Stat(filepath.Join(p.Stage, "ready")); err == nil {
				return nil
			}
		}
	}
}

func copyHelper(source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

// RunInstaller is the helper's entry point. It has no Wails/native dependencies
// and never requests administrator privileges.
func RunInstaller(planFile string) error {
	return runInstaller(planFile, launchBundle)
}

func runInstaller(planFile string, launch func(string) error) (err error) {
	f, err := os.Open(planFile)
	if err != nil {
		return err
	}
	var plan installPlan
	err = json.NewDecoder(io.LimitReader(f, 16<<10)).Decode(&plan)
	f.Close()
	if err != nil {
		return err
	}
	if err := validatePlan(plan, planFile); err != nil {
		return err
	}
	parentExited := false
	defer func() {
		if err != nil {
			// The previous app reads this after a failed installation/relaunch.
			_ = os.WriteFile(plan.ErrorFile, []byte("VoxFlow couldn't finish updating: "+err.Error()), 0600)
			if parentExited {
				_ = launch(plan.Target)
			}
		}
	}()
	if err := os.WriteFile(filepath.Join(plan.Stage, "ready"), nil, 0600); err != nil {
		return err
	}
	if err := waitForExit(plan.ParentPID, 2*time.Minute); err != nil {
		// The original app is still running, so leave it untouched.
		_ = os.RemoveAll(plan.Stage)
		return err
	}
	parentExited = true
	backup := filepath.Join(plan.Stage, "previous.app")
	err = replaceBundle(plan.Target, filepath.Join(plan.Stage, "voxflow.app"), backup, launch)
	if err != nil {
		// Preserve the backup on recovery errors for manual recovery.
		return err
	}
	_ = os.RemoveAll(plan.Stage)
	return nil
}

func validatePlan(p installPlan, filename string) error {
	if !filepath.IsAbs(p.Stage) || filepath.Clean(p.Stage) != p.Stage ||
		!strings.HasPrefix(filepath.Base(p.Stage), ".voxflow-update-") ||
		!filepath.IsAbs(p.Target) || filepath.Clean(p.Target) != p.Target || filepath.Ext(p.Target) != ".app" ||
		filepath.Dir(p.Stage) != filepath.Dir(p.Target) || filename != filepath.Join(p.Stage, "plan.json") || p.ParentPID <= 1 ||
		!filepath.IsAbs(p.ErrorFile) {
		return fmt.Errorf("invalid update installation plan")
	}
	for _, dir := range []string{p.Stage, p.Target, filepath.Join(p.Stage, "voxflow.app")} {
		fi, err := os.Lstat(dir)
		if err != nil || !fi.IsDir() || fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("invalid update directory %s", dir)
		}
	}
	return nil
}

func waitForExit(pid int, timeout time.Duration) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	defer process.Release()
	deadline := time.Now().Add(timeout)
	for {
		err := process.Signal(syscall.Signal(0))
		if errors.Is(err, os.ErrProcessDone) || errors.Is(err, syscall.ESRCH) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("wait for VoxFlow to exit: %w", err)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("VoxFlow didn't exit in time; the installed app was left unchanged")
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// Both renames are on the same volume. Keep the old bundle until the new one
// has been accepted by Launch Services; restore it if replacement/relaunch fails.
func replaceBundle(target, staged, backup string, launch func(string) error) error {
	if _, err := os.Lstat(backup); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("update backup path already exists or is inaccessible")
	}
	if err := os.Rename(target, backup); err != nil {
		return fmt.Errorf("back up installed app: %w", err)
	}
	if err := os.Rename(staged, target); err != nil {
		if restoreErr := os.Rename(backup, target); restoreErr != nil {
			return fmt.Errorf("replacement failed (%v); restore the app from %s: %w", err, backup, restoreErr)
		}
		return fmt.Errorf("replace app: %w", err)
	}
	if err := launch(target); err != nil {
		// Move the failed update aside before putting the previous app back.
		if moveErr := os.Rename(target, staged); moveErr != nil {
			return fmt.Errorf("relaunch failed (%v); previous app is at %s: %w", err, backup, moveErr)
		}
		if restoreErr := os.Rename(backup, target); restoreErr != nil {
			return fmt.Errorf("relaunch failed (%v); restore the app from %s: %w", err, backup, restoreErr)
		}
		return fmt.Errorf("relaunch failed; previous version restored: %w", err)
	}
	return nil
}

func launchBundle(bundle string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "/usr/bin/open", "-n", bundle).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func ConsumeInstallError(dataDir string) string {
	file := filepath.Join(dataDir, "update-error")
	data, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	_ = os.Remove(file)
	return string(data)
}
