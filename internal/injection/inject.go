package injection

import (
	"sync"
	"time"
)

// Paste consumers read the pasteboard when they handle ⌘V, which a busy app
// (Electron under load, remote desktops syncing the clipboard) can do well
// after the keystroke. Waiting longer costs nothing: the changeCount check
// never overwrites anything the user copied in the meantime.
const restoreDelay = time.Second

type Service struct {
	preserveClipboard bool
	pasteboard        string // "" is the system pasteboard; tests use a private one

	mu sync.Mutex
	// While restorePending, saved holds the user's clipboard, to be put back
	// if the pasteboard is still at ourChange (holding our dictation).
	restorePending bool
	saved          []pbItem
	ourChange      int
	gen            int
}

func NewService(preserveClipboard bool) (*Service, error) {
	return &Service{preserveClipboard: preserveClipboard}, nil
}

func (s *Service) Inject(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	gen := s.writeTransient(text)

	time.Sleep(30 * time.Millisecond)

	if err := simulatePaste(); err != nil { // Accessibility on this process, not osascript
		return err
	}

	time.Sleep(50 * time.Millisecond)

	if s.preserveClipboard {
		time.AfterFunc(restoreDelay, func() { s.restore(gen) })
	}
	return nil
}

// writeTransient puts text on the pasteboard for one paste, saving what it
// replaces. Callers hold mu.
func (s *Service) writeTransient(text string) int {
	// Back-to-back dictations: the pasteboard still holds the previous one, so
	// keep the clipboard saved before it rather than saving that dictation.
	if s.preserveClipboard && !(s.restorePending && changeCount(s.pasteboard) == s.ourChange) {
		s.saved = readItems(s.pasteboard)
	}
	s.ourChange = writeItems(s.pasteboard, []pbItem{textItem(text, s.preserveClipboard)}, s.preserveClipboard)
	s.restorePending = s.preserveClipboard
	s.gen++
	return s.gen
}

func (s *Service) restore(gen int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if gen != s.gen || !s.restorePending {
		return
	}
	if changeCount(s.pasteboard) == s.ourChange {
		// Don't hand a restored secret to Universal Clipboard.
		writeItems(s.pasteboard, s.saved, concealed(s.saved))
	}
	s.restorePending, s.saved = false, nil
}

// CopyToClipboard leaves text on the clipboard as an ordinary copy, so
// clipboard managers record it.
func (s *Service) CopyToClipboard(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	writeItems(s.pasteboard, []pbItem{textItem(text, false)}, false)
	return nil
}

// Type uses keystrokes instead of paste (vim/tmux Cmd+V remaps); same Accessibility as Inject.
func (s *Service) Type(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return typeText(text)
}
