package main

func (a *App) ShowMiniMode() {
	a.windowMgr.ShowMini()
}

func (a *App) HideMiniMode() {
	a.windowMgr.HideMini()
}

func (a *App) SetMiniModeExpanded(expanded bool, height int) {
	a.windowMgr.SetMiniExpanded(expanded, height)
}
