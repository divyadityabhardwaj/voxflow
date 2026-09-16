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

	clipboard.Write(clipboard.FmtText, []byte(text))

	time.Sleep(30 * time.Millisecond)

	// Simulate Cmd+V using CoreGraphics CGEventPost (requires Accessibility permission
	// to be granted to this app, NOT to osascript/System Events).
	if err := simulatePaste(); err != nil {
		return err
	}

	time.Sleep(50 * time.Millisecond)

	// Restore clipboard asynchronously so Inject returns immediately.
	// Electron apps and terminals can service the paste well after the key event,
	// so wait long enough that they read our text, not the restored original.
	// A nil original means it was not text (image, file): leave ours in place
	// rather than clearing it.
	if s.preserveClipboard && len(originalClipboard) > 0 {
		go func() {
			time.Sleep(500 * time.Millisecond)
			s.mu.Lock()
			defer s.mu.Unlock()
			// If the user has copied something else in the meantime, do not overwrite it.
			current := clipboard.Read(clipboard.FmtText)
			if string(current) == text {
				clipboard.Write(clipboard.FmtText, originalClipboard)
			}
		}()
	}

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

// Type types text as keystrokes without touching the clipboard, for apps that
// remap Cmd+V (vim-mode editors, tmux). Requires Accessibility permission like Inject.
func (s *Service) Type(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return typeText(text)
}
