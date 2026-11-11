package window

import (
	"testing"
	"voxflow/internal/events"
)

func TestOverlayFeedback(t *testing.T) {
	for _, tt := range []struct {
		name   string
		event  string
		data   interface{}
		mode   int
		label  string
		detail string
		ok     bool
	}{
		{"record", events.StateChanged, "Recording", overlayRecording, "Listening…", "", true},
		{"process", events.StateChanged, "Processing", overlayProcessing, "Transcribing…", "", true},
		{"refine", events.StateChanged, "Refining", overlayRefining, "Cleaning up…", "", true},
		{"cancel", events.StateChanged, "Idle", overlayIdle, "", "", true},
		{"paste target", events.ProcessingComplete, map[string]interface{}{"method": "paste", "target_app": "Notes"}, overlaySuccess, "Pasted into Notes", "", true},
		{"type target fallback", events.ProcessingComplete, map[string]interface{}{"method": "type"}, overlaySuccess, "Typed into your app", "", true},
		{"clipboard fallback", events.ProcessingComplete, map[string]interface{}{"method": "clipboard"}, overlaySuccess, "Copied · press ⌘V", "", true},
		{"in app result", events.ProcessingComplete, map[string]interface{}{"method": "none"}, overlaySuccess, "Ready in VoxFlow", "", true},
		{"start failure", events.Error, "Mic blocked", overlayAttention, "Needs attention", "Mic blocked", true},
		{"delivery warning", events.Toast, map[string]interface{}{"type": "warning", "message": "Text copied"}, overlayAttention, "Needs attention", "Text copied", true},
		{"empty toast", events.Toast, map[string]interface{}{}, 0, "", "", false},
		{"irrelevant event", events.PartialTranscript, "words", 0, "", "", false},
		{"malformed result", events.ProcessingComplete, nil, 0, "", "", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mode, label, detail, ok := overlayFeedback(tt.event, tt.data)
			if mode != tt.mode || label != tt.label || detail != tt.detail || ok != tt.ok {
				t.Fatalf("got (%d, %q, %q, %v)", mode, label, detail, ok)
			}
		})
	}
}
