package main

import (
	"context"
	"sync"
	"time"
	"voxflow/internal/events"
	"voxflow/internal/logger"
	"voxflow/internal/update"
	"voxflow/internal/window"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var (
	updateMu   sync.Mutex
	updateInfo = update.Info{Current: version}
)

func (a *App) GetUpdateInfo() update.Info {
	updateMu.Lock()
	defer updateMu.Unlock()
	return updateInfo
}

func (a *App) openUpdate() {
	if info := a.GetUpdateInfo(); info.URL != "" {
		runtime.BrowserOpenURL(a.ctx, info.URL)
	}
}

// watchForUpdates checks shortly after launch and then daily. Dev builds have
// no version to compare.
func (a *App) watchForUpdates() {
	if version == "" || version == "dev" {
		return
	}
	time.Sleep(10 * time.Second)
	for {
		a.checkForUpdate()
		time.Sleep(24 * time.Hour)
	}
}

func (a *App) checkForUpdate() {
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancel()
	info, err := update.Check(ctx, version)
	if err != nil {
		logger.Warnf("[Update] %v", err)
		return
	}
	updateMu.Lock()
	updateInfo = info
	updateMu.Unlock()
	if !info.Available {
		return
	}
	logger.Infof("[Update] v%s is available (running v%s)", info.Latest, info.Current)
	window.SetUpdateItem("Update available — v" + info.Latest + "…")
	runtime.EventsEmit(a.ctx, events.UpdateAvailable, info)
}
