package injection

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"

	"golang.design/x/hotkey/mainthread"
)

// The keyboard-layout lookup runs on the main thread, so tests need its run loop.
func TestMain(m *testing.M) {
	mainthread.Init(func() { os.Exit(m.Run()) })
}

// testService works on a private pasteboard so tests never touch the user's clipboard.
func testService(t *testing.T, preserve bool) *Service {
	s := &Service{preserveClipboard: preserve, pasteboard: fmt.Sprintf("voxflow.test.%d.%s", os.Getpid(), t.Name())}
	t.Cleanup(func() { writeItems(s.pasteboard, nil, false) })
	return s
}

var userClipboard = []pbItem{
	{{utiPlainText, []byte("hunter2")}, {utiConcealed, []byte{}}},
	{{"public.png", []byte{0x89, 'P', 'N', 'G', 0}}, {"com.example.private", []byte("x")}},
}

func assertPasteboard(t *testing.T, s *Service, want []pbItem) {
	t.Helper()
	if got := readItems(s.pasteboard); !reflect.DeepEqual(got, want) {
		t.Fatalf("pasteboard = %q\nwant %q", got, want)
	}
}

func TestInjectWriteRestoresEveryItem(t *testing.T) {
	tests := []struct {
		name     string
		original []pbItem
	}{
		{"concealed text and an image", userClipboard},
		{"empty clipboard", []pbItem{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testService(t, true)
			writeItems(s.pasteboard, tt.original, false)

			gen := s.writeTransient("dictated")
			assertPasteboard(t, s, []pbItem{textItem("dictated", true)})

			s.restore(gen)
			assertPasteboard(t, s, tt.original)
		})
	}
}

func TestRestoreKeepsWhatTheUserCopiedMeanwhile(t *testing.T) {
	s := testService(t, true)
	writeItems(s.pasteboard, userClipboard, false)

	gen := s.writeTransient("dictated")
	copied := []pbItem{textItem("copied by the user", false)}
	writeItems(s.pasteboard, copied, false)

	s.restore(gen)
	assertPasteboard(t, s, copied)
}

func TestBackToBackDictationsRestoreTheOriginalClipboard(t *testing.T) {
	s := testService(t, true)
	writeItems(s.pasteboard, userClipboard, false)

	first := s.writeTransient("one")
	second := s.writeTransient("two")

	s.restore(first)
	assertPasteboard(t, s, []pbItem{textItem("two", true)})

	s.restore(second)
	assertPasteboard(t, s, userClipboard)
}

func TestWithoutPreserveTheDictationStays(t *testing.T) {
	s := testService(t, false)
	writeItems(s.pasteboard, userClipboard, false)

	s.restore(s.writeTransient("dictated"))
	assertPasteboard(t, s, []pbItem{textItem("dictated", false)})
}

func TestCopyToClipboardIsAnOrdinaryCopy(t *testing.T) {
	s := testService(t, true)
	_ = s.CopyToClipboard("keep me")
	assertPasteboard(t, s, []pbItem{{{utiPlainText, []byte("keep me")}}})
}

func TestInjectWithoutAccessibilityCopies(t *testing.T) {
	if IsAccessibilityGranted() {
		t.Skip("this process may post events: Inject would paste into the frontmost app")
	}
	s := testService(t, true)
	writeItems(s.pasteboard, userClipboard, false)

	for name, deliver := range map[string]func(string) error{"Inject": s.Inject, "Type": s.Type} {
		if err := deliver(name); !errors.Is(err, ErrNoAccessibility) {
			t.Fatalf("%s error = %v, want ErrNoAccessibility", name, err)
		}
		assertPasteboard(t, s, []pbItem{textItem(name, false)})
	}
}

func TestPasteKeyCode(t *testing.T) {
	tests := []struct {
		layout string
		want   int
	}{
		{"com.apple.keylayout.US", 9},
		{"com.apple.keylayout.Dvorak", 47},
		{"com.apple.keylayout.DVORAK-QWERTYCMD", 9},
		{"com.apple.keylayout.Colemak", 9},
		{"com.apple.keylayout.Russian", 9},
		{"com.example.no-such-layout", keyV},
	}
	for _, tt := range tests {
		if got := pasteKeyCode(tt.layout); got != tt.want {
			t.Errorf("pasteKeyCode(%q) = %d, want %d", tt.layout, got, tt.want)
		}
	}
	if got := pasteKeyCode(""); got < 0 || got > 127 {
		t.Errorf("pasteKeyCode(current layout) = %d", got)
	}
}
