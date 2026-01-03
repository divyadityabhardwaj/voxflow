package injection

import (
	"fmt"
	"os/exec"
	"time"

	"golang.design/x/clipboard"
)

// Service handles text injection into the active application
type Service struct {
	originalClipboard []byte
	preserveClipboard bool
}

// NewService creates a new injection service
func NewService(preserveClipboard bool) (*Service, error) {
	// Initialize clipboard
	if err := clipboard.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize clipboard: %w", err)
	}

	return &Service{
		preserveClipboard: preserveClipboard,
	}, nil
}

// Inject injects text into the currently focused application
func (s *Service) Inject(text string) error {
	// Optionally save current clipboard content
	if s.preserveClipboard {
		s.originalClipboard = clipboard.Read(clipboard.FmtText)
	}

	// Copy text to clipboard
	clipboard.Write(clipboard.FmtText, []byte(text))

	// Small delay to ensure clipboard is updated
	time.Sleep(50 * time.Millisecond)

	// Simulate Cmd+V using AppleScript (macOS only, but avoids CGO)
	err := simulatePasteAppleScript()
	if err != nil {
		return err
	}

	// Small delay before restoring clipboard
	time.Sleep(100 * time.Millisecond)

	// Optionally restore original clipboard content
	if s.preserveClipboard && len(s.originalClipboard) > 0 {
		// Delay a bit more to ensure paste completed
		time.Sleep(200 * time.Millisecond)
		clipboard.Write(clipboard.FmtText, s.originalClipboard)
	}

	return nil
}

// simulatePasteAppleScript uses AppleScript to simulate Cmd+V
func simulatePasteAppleScript() error {
	script := `
		tell application "System Events"
			keystroke "v" using command down
		end tell
	`
	cmd := exec.Command("osascript", "-e", script)
	return cmd.Run()
}

// CopyToClipboard just copies text to clipboard without pasting
func (s *Service) CopyToClipboard(text string) error {
	clipboard.Write(clipboard.FmtText, []byte(text))
	return nil
}
