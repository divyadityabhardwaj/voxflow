package main

import "voxflow/internal/hotkey"

func (a *App) onHotkeyPressed(state hotkey.State) {
	if state == hotkey.StateRecording {
		go a.pipeline.CaptureRecordingTarget()
	}
	a.pipeline.HandleHotkeyState(state)
}

func (a *App) ToggleRecording() string {
	return a.pipeline.ToggleRecording()
}
