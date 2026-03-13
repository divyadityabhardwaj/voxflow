package injection

import (
	"fmt"
	"sync"
	"time"

	"golang.design/x/clipboard"
)

// Service handles text injection into the active application
type Service struct {
	originalClipboard []byte
	preserveClipboard bool
	initOnce          sync.Once
	initErr           error
}

// NewService creates a new injection service
func NewService(preserveClipboard bool) (*Service, error) {
	return &Service{
		preserveClipboard: preserveClipboard,
	}, nil
}

func (s *Service) ensureClipboardInit() error {
	s.initOnce.Do(func() {
		s.initErr = clipboard.Init()
	})
	if s.initErr != nil {
		return fmt.Errorf("failed to initialize clipboard: %w", s.initErr)
	}
	return nil
}

// Inject injects text into the target application (identified by bundle ID).
func (s *Service) Inject(text string) error {
	if err := s.ensureClipboardInit(); err != nil {
		return err
	}

	// Optionally save current clipboard content
	if s.preserveClipboard {
		s.originalClipboard = clipboard.Read(clipboard.FmtText)
	}

	// Copy text to clipboard
	clipboard.Write(clipboard.FmtText, []byte(text))

	// Small delay to ensure clipboard is updated
	time.Sleep(50 * time.Millisecond)

	// Simulate Cmd+V using CoreGraphics CGEventPost (requires Accessibility permission
	// to be granted to this app, NOT to osascript/System Events).
	if err := simulatePaste(); err != nil {
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

// CopyToClipboard just copies text to clipboard without pasting
func (s *Service) CopyToClipboard(text string) error {
	if err := s.ensureClipboardInit(); err != nil {
		return err
	}
	clipboard.Write(clipboard.FmtText, []byte(text))
	return nil
}
