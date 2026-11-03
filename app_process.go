package main

import (
	"voxflow/internal/hotkey"
	"voxflow/internal/orchestrator"
)

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

func (a *App) GetFailedRecording() orchestrator.FailedRecordingInfo {
	return a.pipeline.GetFailedRecording()
}
func (a *App) RetryFailedRecording() error { return a.pipeline.RetryFailedRecording() }
func (a *App) DiscardFailedRecording()     { a.pipeline.DiscardFailedRecording() }
