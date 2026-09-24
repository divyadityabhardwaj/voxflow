package main

import (
	"os/exec"
	"voxflow/internal/injection"
	"voxflow/internal/macos"
)

type Permissions struct {
	Microphone    string `json:"microphone"` // "authorized", "denied", "restricted" or "notDetermined"
	Accessibility bool   `json:"accessibility"`
}

func (a *App) GetPermissions() Permissions {
	return Permissions{Microphone: macos.MicrophoneStatus(), Accessibility: injection.IsAccessibilityGranted()}
}

// RequestMicrophoneAccess shows the macOS prompt if it hasn't been answered, and
// blocks until it is. Bound methods run off the main thread, as it requires.
func (a *App) RequestMicrophoneAccess() bool {
	return macos.RequestMicrophoneAccess()
}

// OpenPrivacySettings opens the "microphone" or "accessibility" pane.
func (a *App) OpenPrivacySettings(pane string) error {
	return macos.OpenPrivacySettings(pane)
}

func (a *App) GetOnboardingCompleted() bool {
	return a.config.GetOnboardingCompleted()
}

func (a *App) CompleteOnboarding() error {
	a.config.SetOnboardingCompleted(true)
	return a.config.Save()
}

func (a *App) IsAccessibilityGranted() bool {
	return injection.IsAccessibilityGranted()
}

// PromptAccessibilityExplanation opens System Settings at the Accessibility pane.
// It used to show its own dialog first, which macOS then followed with a second one.
func (a *App) PromptAccessibilityExplanation() (bool, error) {
	// Still ask the system, since that is what adds VoxFlow to the pane's list.
	injection.PromptAccessibility()
	if err := exec.Command("open", "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility").Run(); err != nil {
		return false, err
	}
	return true, nil
}
