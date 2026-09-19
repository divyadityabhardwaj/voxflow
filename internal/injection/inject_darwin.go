package injection

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework CoreGraphics -framework ApplicationServices -framework Foundation

#include <unistd.h>
#include <CoreGraphics/CoreGraphics.h>
#include <ApplicationServices/ApplicationServices.h>
#include <Foundation/Foundation.h>

// pressKey synthesizes a key down/up pair using CGEvent.
// This requires Accessibility permission for the calling process (not osascript).
static int pressKey(CGKeyCode key, CGEventFlags flags) {
    CGEventSourceRef src = CGEventSourceCreate(kCGEventSourceStateHIDSystemState);
    if (!src) return -1;

    CGEventRef keyDown = CGEventCreateKeyboardEvent(src, key, true);
    if (!keyDown) { CFRelease(src); return -2; }
    CGEventSetFlags(keyDown, flags);
    CGEventPost(kCGAnnotatedSessionEventTap, keyDown);
    CFRelease(keyDown);

    CGEventRef keyUp = CGEventCreateKeyboardEvent(src, key, false);
    if (!keyUp) { CFRelease(src); return -3; }
    CGEventSetFlags(keyUp, flags);
    CGEventPost(kCGAnnotatedSessionEventTap, keyUp);
    CFRelease(keyUp);

    CFRelease(src);
    return 0;
}

// typeUnicode posts text as keyboard events carrying a unicode payload, so the
// result is layout-independent. Apple caps each event at 20 UTF-16 units, and
// fast apps drop characters without a short gap between events.
static int typeUnicode(const UniChar *chars, int len) {
    CGEventSourceRef src = CGEventSourceCreate(kCGEventSourceStateHIDSystemState);
    if (!src) return -1;

    for (int i = 0; i < len; ) {
        int n = len - i < 20 ? len - i : 20;
        // Never split a surrogate pair across events or the emoji arrives as garbage.
        if (n == 20 && (chars[i + n - 1] & 0xFC00) == 0xD800) n--;

        CGEventRef keyDown = CGEventCreateKeyboardEvent(src, (CGKeyCode)0, true);
        if (!keyDown) { CFRelease(src); return -2; }
        CGEventSetFlags(keyDown, 0);
        CGEventKeyboardSetUnicodeString(keyDown, n, chars + i);
        CGEventPost(kCGAnnotatedSessionEventTap, keyDown);
        CFRelease(keyDown);
        usleep(3000);

        CGEventRef keyUp = CGEventCreateKeyboardEvent(src, (CGKeyCode)0, false);
        if (!keyUp) { CFRelease(src); return -3; }
        CGEventSetFlags(keyUp, 0);
        CGEventKeyboardSetUnicodeString(keyUp, n, chars + i);
        CGEventPost(kCGAnnotatedSessionEventTap, keyUp);
        CFRelease(keyUp);
        usleep(3000);
        i += n;
    }

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
	"strings"
	"time"
	"unicode/utf16"
	"unsafe"
)

const (
	keyV      = 9
	keyReturn = 36
)

func cgEventErr(ret C.int) error {
	return fmt.Errorf("CGEventPost failed (code %d): ensure Accessibility permission is granted to this app in System Preferences → Privacy & Security → Accessibility", int(ret))
}

func simulatePaste() error {
	if ret := C.pressKey(keyV, C.kCGEventFlagMaskCommand); ret != 0 {
		return cgEventErr(ret)
	}
	return nil
}

// typeText: newlines as Return keys (many apps ignore unicode LF).
func typeText(text string) error {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	for i, line := range strings.Split(text, "\n") {
		if i > 0 {
			if ret := C.pressKey(keyReturn, 0); ret != 0 {
				return cgEventErr(ret)
			}
			time.Sleep(3 * time.Millisecond)
		}
		if line == "" {
			continue
		}
		u := utf16.Encode([]rune(line))
		if ret := C.typeUnicode((*C.UniChar)(unsafe.Pointer(&u[0])), C.int(len(u))); ret != 0 {
			return cgEventErr(ret)
		}
	}
	return nil
}

func IsAccessibilityGranted() bool {
	return C.checkAccessibility() == 1
}

func PromptAccessibility() {
	C.promptAccessibility()
}
