//go:build darwin

package window

import "C"

// Tags match the menu items built in statusitem_darwin.go.
//
//export voxStatusItemAction
func voxStatusItemAction(tag C.int) {
	var cb func()
	switch tag {
	case 1:
		cb = statusCallbacks.ToggleRecording
	case 2:
		cb = statusCallbacks.OpenApp
	case 3:
		cb = statusCallbacks.OpenSettings
	case 4:
		cb = statusCallbacks.Quit
	}
	if cb != nil {
		// Off the Cocoa main thread, like a bound method called from the webview.
		go cb()
	}
}

// Called on the main thread by the observer installed in darwin.go.
//
//export voxWindowFrameChanged
func voxWindowFrameChanged() {
	if m := observedManager.Load(); m != nil {
		go m.frameChanged()
	}
}
