package window

import "voxflow/internal/events"

// Idle settles the current feedback without erasing success or an actionable error.
const (
	overlayIdle = iota
	overlayRecording
	overlayProcessing
	overlayRefining
	overlaySuccess
	overlayAttention
)

func HandleOverlayEvent(name string, data ...interface{}) {
	if len(data) == 0 {
		return
	}
	if name == events.AudioLevel {
		if level, ok := data[0].(float64); ok {
			setOverlayLevel(level)
		}
		return
	}
	if mode, label, detail, ok := overlayFeedback(name, data[0]); ok {
		setOverlayState(mode, label, detail)
	}
}

func overlayFeedback(name string, data interface{}) (int, string, string, bool) {
	switch name {
	case events.StateChanged:
		switch data {
		case "Recording":
			return overlayRecording, "Listening…", "", true
		case "Processing":
			return overlayProcessing, "Transcribing…", "", true
		case "Refining":
			return overlayRefining, "Cleaning up…", "", true
		case "Idle":
			return overlayIdle, "", "", true
		}
	case events.Error:
		if message, ok := data.(string); ok {
			return overlayAttention, "Needs attention", message, true
		}
	case events.Toast:
		if toast, ok := data.(map[string]interface{}); ok {
			message, _ := toast["message"].(string)
			if message == "" {
				break
			}
			if toast["type"] == "success" {
				return overlaySuccess, message, "", true
			}
			return overlayAttention, "Needs attention", message, true
		}
	case events.ProcessingComplete:
		if result, ok := data.(map[string]interface{}); ok {
			app, _ := result["target_app"].(string)
			if app == "" {
				app = "your app"
			}
			label := "Ready in VoxFlow"
			switch result["method"] {
			case "paste":
				label = "Pasted into " + app
			case "type":
				label = "Typed into " + app
			case "clipboard":
				label = "Copied · press ⌘V"
			}
			return overlaySuccess, label, "", true
		}
	}
	return 0, "", "", false
}
