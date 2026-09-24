package main

import (
	"os/exec"
	"path/filepath"
	"voxflow/internal/config"
)

func (a *App) ShowMiniMode() {
	a.windowMgr.ShowMini()
}

func (a *App) HideMiniMode() {
	a.windowMgr.HideMini()
}

func (a *App) SetMiniModeExpanded(expanded bool, height int) {
	a.windowMgr.SetMiniExpanded(expanded, height)
}

func (a *App) ResetWindowPosition() {
	a.windowMgr.ResetPosition()
}

// OpenLogFile shows the log in Console.
func (a *App) OpenLogFile() error {
	dir, err := config.GetConfigDir()
	if err != nil {
		return err
	}
	return exec.Command("open", filepath.Join(dir, "voxflow.log")).Run()
}
