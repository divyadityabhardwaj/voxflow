package main

import "voxflow/internal/hotkey"

func (a *App) onHotkeyPressed(state hotkey.State) {
	a.pipeline.HandleHotkeyState(state)
}

func (a *App) onHotkeyCancel(silent bool) {
	if silent {
		a.pipeline.DiscardRecording()
	} else {
		a.pipeline.CancelRecording()
	}
}

func (a *App) ToggleRecording() string {
	return a.pipeline.ToggleRecording()
}

// CancelRecording discards the recording in progress without transcribing or pasting it.
func (a *App) CancelRecording() {
	a.pipeline.CancelRecording()
}
