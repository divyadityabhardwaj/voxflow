package update

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestInstallerHandoff(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "VoxFlow with spaces.app")
	helper := filepath.Join(target, "Contents/MacOS/voxflow-updater")
	if err := os.MkdirAll(filepath.Dir(helper), 0755); err != nil {
		t.Fatal(err)
	}
	// A harmless stand-in checks that the copied helper starts, receives the plan,
	// and acknowledges readiness. Removing the stage also ends the stand-in.
	script := "#!/bin/sh\nstage=${1%/*}\ntouch \"$stage/ready\"\nwhile [ -d \"$stage\" ]; do /bin/sleep 0.05; done\n"
	if err := os.WriteFile(helper, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	stage, err := os.MkdirTemp(root, ".voxflow-update-")
	if err != nil {
		t.Fatal(err)
	}
	p := Prepared{Stage: stage, Target: target}
	t.Cleanup(p.Cleanup)
	if err := StartInstaller(context.Background(), p, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatal("handoff changed the running app")
	}
}

func TestInstallerLifecycle(t *testing.T) {
	for _, failLaunch := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "rollback"}[failLaunch], func(t *testing.T) {
			dir := t.TempDir()
			stage, err := os.MkdirTemp(dir, ".voxflow-update-")
			if err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(dir, "VoxFlow.app")
			for path, version := range map[string]string{target: "old", filepath.Join(stage, "voxflow.app"): "new"} {
				if err := os.Mkdir(path, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(path, "version"), []byte(version), 0600); err != nil {
					t.Fatal(err)
				}
			}
			parent := exec.Command("/usr/bin/true")
			if err := parent.Run(); err != nil {
				t.Fatal(err)
			}
			plan := installPlan{Stage: stage, Target: target, ParentPID: parent.Process.Pid, ErrorFile: filepath.Join(dir, "update-error")}
			data, _ := json.Marshal(plan)
			file := filepath.Join(stage, "plan.json")
			if err := os.WriteFile(file, data, 0600); err != nil {
				t.Fatal(err)
			}
			calls := 0
			err = runInstaller(file, func(path string) error {
				calls++
				if failLaunch && calls == 1 {
					return errors.New("cannot launch update")
				}
				if failLaunch {
					if _, err := os.Stat(plan.ErrorFile); err != nil {
						t.Fatal("old app restarted before error was persisted")
					}
				}
				return nil
			})
			if failLaunch {
				if err == nil || calls != 2 {
					t.Fatalf("err=%v, launches=%d", err, calls)
				}
				data, _ := os.ReadFile(filepath.Join(target, "version"))
				if string(data) != "old" {
					t.Fatalf("rollback left %s installed", data)
				}
			} else {
				if err != nil || calls != 1 {
					t.Fatalf("err=%v, launches=%d", err, calls)
				}
				if _, err := os.Stat(stage); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("stage not cleaned: %v", err)
				}
			}
		})
	}
}

func TestReplaceBundleAndRollback(t *testing.T) {
	for _, scenario := range []string{"success", "missing payload", "launch failure", "backup exists"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			target, staged, backup := filepath.Join(dir, "VoxFlow.app"), filepath.Join(dir, "new.app"), filepath.Join(dir, "previous.app")
			makeBundle := func(path, version string) {
				t.Helper()
				if err := os.Mkdir(path, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(path, "version"), []byte(version), 0600); err != nil {
					t.Fatal(err)
				}
			}
			makeBundle(target, "old")
			if scenario != "missing payload" {
				makeBundle(staged, "new")
			}
			if scenario == "backup exists" {
				makeBundle(backup, "existing")
			}
			launched := false
			err := replaceBundle(target, staged, backup, func(path string) error {
				launched = true
				if path != target {
					t.Fatalf("launched wrong path: %s", path)
				}
				if scenario == "launch failure" {
					return errors.New("launch denied")
				}
				return nil
			})
			want := "old"
			if scenario == "success" {
				want = "new"
				if err != nil || !launched {
					t.Fatalf("err=%v, launched=%v", err, launched)
				}
			} else if err == nil {
				t.Fatal("expected installation error")
			}
			data, readErr := os.ReadFile(filepath.Join(target, "version"))
			if readErr != nil || string(data) != want {
				t.Fatalf("installed=%s, err=%v; want %s", data, readErr, want)
			}
			if scenario == "success" {
				data, err := os.ReadFile(filepath.Join(backup, "version"))
				if err != nil || string(data) != "old" {
					t.Fatalf("backup=%s, err=%v", data, err)
				}
			}
		})
	}
}

func TestValidatePlan(t *testing.T) {
	dir := t.TempDir()
	stage := filepath.Join(dir, ".voxflow-update-test")
	target := filepath.Join(dir, "VoxFlow.app")
	for _, path := range []string{stage, target, filepath.Join(stage, "voxflow.app")} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	plan := installPlan{Stage: stage, Target: target, ParentPID: os.Getpid(), ErrorFile: filepath.Join(dir, "update-error")}
	file := filepath.Join(stage, "plan.json")
	if err := validatePlan(plan, file); err != nil {
		t.Fatal(err)
	}
	bad := plan
	bad.Target = "/Applications/Other.app"
	if err := validatePlan(bad, file); err == nil {
		t.Fatal("accepted a different volume/directory")
	}
	if err := os.RemoveAll(filepath.Join(stage, "voxflow.app")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(stage, "voxflow.app")); err != nil {
		t.Fatal(err)
	}
	if err := validatePlan(plan, file); err == nil {
		t.Fatal("accepted a symlink payload")
	}
}

func TestWaitForExitDoesNotModifyLiveApp(t *testing.T) {
	if err := waitForExit(os.Getpid(), time.Millisecond); err == nil {
		t.Fatal("live parent was treated as exited")
	}
}

func TestConsumeInstallError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "update-error"), []byte("Update failed"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := ConsumeInstallError(dir); got != "Update failed" {
		t.Fatalf("message = %q", got)
	}
	if got := ConsumeInstallError(dir); got != "" {
		t.Fatalf("error was not consumed: %q", got)
	}
}
