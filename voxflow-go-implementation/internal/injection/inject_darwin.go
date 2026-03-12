package injection

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework CoreGraphics -framework ApplicationServices -framework Foundation

#include <CoreGraphics/CoreGraphics.h>
#include <ApplicationServices/ApplicationServices.h>
#include <Foundation/Foundation.h>

// simulateCmdV synthesizes a Cmd+V keypress using CGEvent.
// This requires Accessibility permission for the calling process (not osascript).
// macOS virtual key code for 'v' is 9.
static int simulateCmdV() {
    CGEventSourceRef src = CGEventSourceCreate(kCGEventSourceStateHIDSystemState);
    if (!src) return -1;

    // Key down: Cmd+V
    CGEventRef keyDown = CGEventCreateKeyboardEvent(src, (CGKeyCode)9, true);
    if (!keyDown) { CFRelease(src); return -2; }
    CGEventSetFlags(keyDown, kCGEventFlagMaskCommand);
    CGEventPost(kCGAnnotatedSessionEventTap, keyDown);
    CFRelease(keyDown);

    // Key up: Cmd+V
    CGEventRef keyUp = CGEventCreateKeyboardEvent(src, (CGKeyCode)9, false);
    if (!keyUp) { CFRelease(src); return -3; }
    CGEventSetFlags(keyUp, kCGEventFlagMaskCommand);
    CGEventPost(kCGAnnotatedSessionEventTap, keyUp);
    CFRelease(keyUp);

    CFRelease(src);
    return 0;
}

// checkAccessibility returns 1 if Accessibility access is granted, 0 otherwise.
static int checkAccessibility() {
    return AXIsProcessTrusted() ? 1 : 0;
}

// promptAccessibility shows the system dialog to request Accessibility access.
static void promptAccessibility() {
    NSDictionary *options = @{ (__bridge NSString*)kAXTrustedCheckOptionPrompt: @YES };
    AXIsProcessTrustedWithOptions((__bridge CFDictionaryRef)options);
}
*/
import "C"

import (
	"fmt"
)

// simulatePaste uses CoreGraphics CGEventPost to send Cmd+V.
// This replaces the osascript/System Events approach — permission is tied
// to the app process itself, not a subprocess, so it survives dev rebuilds.
func simulatePaste() error {
	ret := C.simulateCmdV()
	if ret != 0 {
		return fmt.Errorf("CGEventPost failed (code %d): ensure Accessibility permission is granted to this app in System Preferences → Privacy & Security → Accessibility", int(ret))
	}
	return nil
}

// IsAccessibilityGranted returns true if the process has Accessibility permission.
func IsAccessibilityGranted() bool {
	return C.checkAccessibility() == 1
}

// PromptAccessibility shows the macOS system dialog asking the user to grant
// Accessibility access. Call this once at startup if IsAccessibilityGranted() is false.
func PromptAccessibility() {
	C.promptAccessibility()
}
