package injection

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"golang.design/x/clipboard"
)

// Service handles text injection into the active application
type Service struct {
	originalClipboard []byte
	preserveClipboard bool
	targetBundleID    string // Bundle ID of the app to paste into, captured before recording
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

// SetTargetApp sets the bundle ID of the application to inject text into.
// This should be called before recording starts, while the target app still has focus.
func (s *Service) SetTargetApp(bundleID string) {
	s.targetBundleID = bundleID
}

// Inject injects text into the target application (identified by bundle ID).
func (s *Service) Inject(text string) error {
	// Optionally save current clipboard content
	if s.preserveClipboard {
		s.originalClipboard = clipboard.Read(clipboard.FmtText)
	}

	// Copy text to clipboard
	clipboard.Write(clipboard.FmtText, []byte(text))

	// Small delay to ensure clipboard is updated
	time.Sleep(50 * time.Millisecond)

	// Simulate Cmd+V, targeting the specific app if we have its bundle ID
	err := simulatePasteAppleScript(s.targetBundleID)
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

// simulatePasteAppleScript uses AppleScript to simulate Cmd+V.
// If bundleID is provided, it activates that specific app first to work around
// the focus-steal race condition where Voxflow grabs focus during processing.
func simulatePasteAppleScript(bundleID string) error {
	var script string
	bundleID = strings.TrimSpace(bundleID)

	if bundleID != "" {
		// Activate the target app, bring it to front, then paste
		script = fmt.Sprintf(`
			tell application id "%s"
				activate
			end tell
			delay 0.1
			tell application "System Events"
				keystroke "v" using command down
			end tell
		`, bundleID)
	} else {
		// Fallback: paste into whatever is frontmost
		script = `
			tell application "System Events"
				keystroke "v" using command down
			end tell
		`
	}

	cmd := exec.Command("osascript", "-e", script)
	return cmd.Run()
}

// CopyToClipboard just copies text to clipboard without pasting
func (s *Service) CopyToClipboard(text string) error {
	clipboard.Write(clipboard.FmtText, []byte(text))
	return nil
}
