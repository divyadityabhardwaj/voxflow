package injection

import (
	"fmt"
	"sync"
	"time"

	"golang.design/x/clipboard"
)

type Service struct {
	preserveClipboard bool
	mu                sync.Mutex
	initOnce          sync.Once
	initErr           error
}

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

	if err := simulatePaste(); err != nil { // Accessibility on this process, not osascript
		return err
	}

	time.Sleep(50 * time.Millisecond)

	// Restore clipboard async: slow targets read paste after the key event; skip if user copied meanwhile.
	if s.preserveClipboard && len(originalClipboard) > 0 {
		go func() {
			time.Sleep(500 * time.Millisecond)
			s.mu.Lock()
			defer s.mu.Unlock()
			current := clipboard.Read(clipboard.FmtText)
			if string(current) == text {
				clipboard.Write(clipboard.FmtText, originalClipboard)
			}
		}()
	}

	return nil
}

func (s *Service) CopyToClipboard(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureClipboardInit(); err != nil {
		return err
	}
	clipboard.Write(clipboard.FmtText, []byte(text))
	return nil
}

// Type uses keystrokes instead of paste (vim/tmux Cmd+V remaps); same Accessibility as Inject.
func (s *Service) Type(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return typeText(text)
}
