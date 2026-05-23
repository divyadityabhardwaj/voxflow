package main

import (
	"voxflow/internal/injection"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// GetOnboardingCompleted reports whether first-run onboarding finished.
func (a *App) GetOnboardingCompleted() bool {
	return a.config.GetOnboardingCompleted()
}

// CompleteOnboarding marks the first-run wizard as done.
func (a *App) CompleteOnboarding() error {
	a.config.SetOnboardingCompleted(true)
	return a.config.Save()
}

// IsAccessibilityGranted exposes macOS Accessibility permission state to the UI.
func (a *App) IsAccessibilityGranted() bool {
	return injection.IsAccessibilityGranted()
}

// RequestAccessibilityPermission shows the system Accessibility grant dialog.
func (a *App) RequestAccessibilityPermission() {
	injection.PromptAccessibility()
}

// PromptAccessibilityExplanation shows why Accessibility is needed (onboarding step).
func (a *App) PromptAccessibilityExplanation() (bool, error) {
	selection, err := runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
		Type:          runtime.QuestionDialog,
		Title:         "Accessibility Permission",
		Message:       "VoxFlow pastes refined text into other apps using a simulated Cmd+V. macOS requires Accessibility permission for that.\n\nClick \"Open System Settings\" to grant access, or skip for now.",
		Buttons:       []string{"Open System Settings", "Skip"},
		DefaultButton: "Open System Settings",
	})
	if err != nil {
		return false, err
	}
	if selection == "Open System Settings" {
		injection.PromptAccessibility()
		return true, nil
	}
	return false, nil
}
