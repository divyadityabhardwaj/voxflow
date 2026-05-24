package main

import "voxflow/internal/hotkey"

func (a *App) onHotkeyPressed(state hotkey.State) {
	if state == hotkey.StateRecording {
		go a.pipeline.CaptureRecordingTarget()
	}
	a.pipeline.HandleHotkeyState(state)
}

func (a *App) StartRecording() error {
	go a.pipeline.CaptureRecordingTarget()
	return a.pipeline.StartRecording()
}

func (a *App) StopRecording() {
	a.pipeline.StopRecording()
}

func (a *App) ToggleRecording() string {
	return a.pipeline.ToggleRecording()
}
