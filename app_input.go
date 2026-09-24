package main

import "voxflow/internal/audio"

func (a *App) GetInputDevices() ([]audio.InputDevice, error) {
	return a.audioRecorder.ListInputDevices()
}

// SetInputDevice picks the microphone by name; "" follows the system default.
func (a *App) SetInputDevice(name string) error {
	a.audioRecorder.SetInputDevice(name)
	a.config.SetInputDevice(name)
	return a.config.Save()
}

type PushToTalkKeyInfo struct {
	Key    string `json:"key"`    // "right_option", "right_command", "fn" or "chord"
	Active bool   `json:"active"` // false: no Accessibility yet, so the push-to-talk combination is used
}

func (a *App) GetPushToTalkKey() PushToTalkKeyInfo {
	return PushToTalkKeyInfo{Key: a.config.GetPushToTalkKey(), Active: a.hotkeyManager.HoldKeyActive()}
}

// SetPushToTalkKey also retries the keyboard event tap, so calling it again
// after Accessibility is granted activates the hold key.
func (a *App) SetPushToTalkKey(key string) (PushToTalkKeyInfo, error) {
	if err := a.hotkeyManager.SetHoldKey(key); err != nil {
		return a.GetPushToTalkKey(), err
	}
	a.config.SetPushToTalkKey(key)
	return a.GetPushToTalkKey(), a.config.Save()
}
