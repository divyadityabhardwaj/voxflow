//go:build darwin

package macos

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework AVFoundation

#import <AVFoundation/AVFoundation.h>

static long micStatus(void) {
	@autoreleasepool {
		return [AVCaptureDevice authorizationStatusForMediaType:AVMediaTypeAudio];
	}
}

static bool requestMic(void) {
	@autoreleasepool {
		dispatch_semaphore_t answered = dispatch_semaphore_create(0);
		__block BOOL granted = NO;
		[AVCaptureDevice requestAccessForMediaType:AVMediaTypeAudio completionHandler:^(BOOL ok) {
			granted = ok;
			dispatch_semaphore_signal(answered);
		}];
		dispatch_semaphore_wait(answered, DISPATCH_TIME_FOREVER);
		return granted;
	}
}
*/
import "C"

import (
	"fmt"
	"os/exec"
)

// MicrophoneStatus is "authorized", "denied", "restricted" or "notDetermined".
func MicrophoneStatus() string {
	switch C.micStatus() {
	case C.AVAuthorizationStatusAuthorized:
		return "authorized"
	case C.AVAuthorizationStatusDenied:
		return "denied"
	case C.AVAuthorizationStatusRestricted:
		return "restricted"
	default:
		return "notDetermined"
	}
}

// RequestMicrophoneAccess shows the macOS microphone prompt if the user has not
// answered it yet and blocks until they do; otherwise it returns at once. Call
// it off the main thread. The process must have NSMicrophoneUsageDescription in
// its Info.plist or macOS kills it.
func RequestMicrophoneAccess() bool {
	switch MicrophoneStatus() {
	case "authorized":
		return true
	case "notDetermined":
		return bool(C.requestMic())
	default:
		return false
	}
}

func privacySettingsURL(pane string) (string, error) {
	switch pane {
	case "accessibility":
		return "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility", nil
	case "microphone":
		return "x-apple.systempreferences:com.apple.preference.security?Privacy_Microphone", nil
	default:
		return "", fmt.Errorf("unknown privacy pane %q", pane)
	}
}

// OpenPrivacySettings opens System Settings › Privacy & Security at pane:
// "accessibility" or "microphone".
func OpenPrivacySettings(pane string) error {
	url, err := privacySettingsURL(pane)
	if err != nil {
		return err
	}
	return exec.Command("open", url).Run()
}
