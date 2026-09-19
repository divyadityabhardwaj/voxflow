package main

import (
	"voxflow/internal/injection"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

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
