package main

import (
	"context"
	"fmt"
	"time"
	"voxflow/internal/config"
	"voxflow/internal/events"
	"voxflow/internal/hotkey"
	"voxflow/internal/logger"
	"voxflow/internal/update"
	"voxflow/internal/window"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) GetUpdateInfo() update.Info {
	a.updateMu.Lock()
	defer a.updateMu.Unlock()
	return a.updateInfo
}

func (a *App) openUpdate() {
	a.showMainWindow()
	runtime.EventsEmit(a.ctx, events.OpenSettings, "about")
}

// watchForUpdates checks shortly after launch and then daily. Cancellation also
// stops the timer so shutdown never leaves an update check running.
func (a *App) watchForUpdates() {
	if version == "" || version == "dev" {
		return
	}
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-timer.C:
			if _, err := a.CheckForUpdates(); err != nil {
				logger.Warnf("[Update] %v", err)
			}
			timer.Reset(24 * time.Hour)
		}
	}
}

func (a *App) emitUpdateInfo() {
	runtime.EventsEmit(a.ctx, events.UpdateChanged, a.GetUpdateInfo())
}

// CheckForUpdates is also bound to the About pane's manual check button.
func (a *App) CheckForUpdates() (update.Info, error) {
	a.updateMu.Lock()
	if a.updateBusy || version == "" || version == "dev" {
		info := a.updateInfo
		a.updateMu.Unlock()
		return info, nil
	}
	a.updateBusy = true
	previous := a.updateInfo.Latest
	a.updateInfo.State, a.updateInfo.Error = "checking", ""
	a.updateMu.Unlock()
	a.emitUpdateInfo()
	info, err := update.Check(a.ctx, version)
	if err == nil && info.CanInstall {
		if reason := update.InstallReason(); reason != "" {
			info.CanInstall, info.Reason = false, reason
		}
	}
	a.updateMu.Lock()
	a.updateBusy = false
	if err != nil {
		a.updateInfo.State, a.updateInfo.Error = "idle", "Couldn't check for updates. "+err.Error()
		info = a.updateInfo
	} else {
		a.updateInfo = info
	}
	a.updateMu.Unlock()
	a.emitUpdateInfo()
	if err == nil && info.Available {
		window.SetUpdateItem("Update available — v" + info.Latest + "…")
		if previous != info.Latest {
			logger.Infof("[Update] v%s is available (running v%s)", info.Latest, info.Current)
			runtime.EventsEmit(a.ctx, events.UpdateAvailable, info)
		}
	}
	return info, err
}

// UpdateAndRestart only quits after a verified app and a ready helper exist.
func (a *App) UpdateAndRestart() (err error) {
	a.updateMu.Lock()
	if a.updateBusy {
		a.updateMu.Unlock()
		return fmt.Errorf("an update operation is already in progress")
	}
	info := a.updateInfo
	if !info.Available || !info.CanInstall {
		a.updateMu.Unlock()
		return fmt.Errorf("check for an installable update first")
	}
	if a.pipeline.State() != hotkey.StateIdle {
		a.updateMu.Unlock()
		return fmt.Errorf("finish your current dictation before updating and restarting")
	}
	a.updateBusy = true
	a.updateInfo.State, a.updateInfo.Error = "downloading", ""
	a.updateInfo.Downloaded = 0
	a.updateMu.Unlock()
	a.emitUpdateInfo()
	defer func() {
		if err != nil {
			a.updateMu.Lock()
			a.updateBusy = false
			a.updateInfo.State, a.updateInfo.Error = "idle", err.Error()
			a.updateMu.Unlock()
			a.emitUpdateInfo()
		}
	}()
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Minute)
	defer cancel()
	prepared, err := update.Prepare(ctx, info, func(state string, downloaded int64) {
		a.updateMu.Lock()
		a.updateInfo.State, a.updateInfo.Downloaded = state, downloaded
		a.updateMu.Unlock()
		a.emitUpdateInfo()
	})
	if err != nil {
		return err
	}
	handedOff := false
	defer func() {
		if !handedOff {
			prepared.Cleanup()
		}
	}()
	// Downloads can take a while. Never interrupt an in-progress dictation.
	if a.pipeline.State() != hotkey.StateIdle {
		return fmt.Errorf("finish your current dictation before updating and restarting")
	}
	if err = a.hotkeyManager.Suspend(true); err != nil {
		return err
	}
	defer func() {
		if !handedOff {
			_ = a.hotkeyManager.Suspend(false)
		}
	}()
	if a.pipeline.State() != hotkey.StateIdle {
		return fmt.Errorf("finish your current dictation before updating and restarting")
	}
	dir, err := config.GetConfigDir()
	if err != nil {
		return err
	}
	if err = update.StartInstaller(ctx, prepared, dir); err != nil {
		return err
	}
	handedOff = true
	a.updateMu.Lock()
	a.updateInfo.State = "restarting"
	a.updateMu.Unlock()
	a.emitUpdateInfo()
	logger.Infof("[Update] Restarting to install v%s", info.Latest)
	a.Quit()
	return nil
}
