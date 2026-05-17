package injection

import (
	"fmt"
	"sync"
	"time"

	"golang.design/x/clipboard"
)

// Service handles text injection into the active application
type Service struct {
	preserveClipboard bool
	mu                sync.Mutex
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

	s.mu.Lock()
	defer s.mu.Unlock()

	var originalClipboard []byte
	if s.preserveClipboard {
		originalClipboard = clipboard.Read(clipboard.FmtText)
	}

	defer func() {
		if s.preserveClipboard && len(originalClipboard) > 0 {
			time.Sleep(200 * time.Millisecond)
			clipboard.Write(clipboard.FmtText, originalClipboard)
		}
	}()

	clipboard.Write(clipboard.FmtText, []byte(text))

	time.Sleep(50 * time.Millisecond)

	// Simulate Cmd+V using CoreGraphics CGEventPost (requires Accessibility permission
	// to be granted to this app, NOT to osascript/System Events).
	if err := simulatePaste(); err != nil {
		return err
	}

	time.Sleep(100 * time.Millisecond)

	return nil
}

// CopyToClipboard just copies text to clipboard without pasting
func (s *Service) CopyToClipboard(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureClipboardInit(); err != nil {
		return err
	}
	clipboard.Write(clipboard.FmtText, []byte(text))
	return nil
}
