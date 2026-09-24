//go:build darwin

package hotkey

/*
#cgo LDFLAGS: -framework ApplicationServices

#include <ApplicationServices/ApplicationServices.h>
#include <stdatomic.h>
#include <stdbool.h>

// Implemented in Go (tap_export_darwin.go).
extern void voxTapEvent(int kind);

enum { evHoldDown, evHoldUp, evOther, evEscape };
enum { kcEscape = 53 };

static atomic_int holdKeycode = -1;
static atomic_bool swallowEscape;
static CFMachPortRef tap;

// Device-dependent flag of each supported modifier, so the left-hand key doesn't count.
static CGEventFlags flagFor(int keycode) {
	switch (keycode) {
	case 61: return 0x40; // NX_DEVICERALTKEYMASK
	case 54: return 0x10; // NX_DEVICERCMDKEYMASK
	case 63: return kCGEventFlagMaskSecondaryFn;
	}
	return 0;
}

// Runs on the tap's own thread, so a busy main thread never delays typing.
static CGEventRef onEvent(CGEventTapProxy proxy, CGEventType type, CGEventRef event, void *info) {
	if (type == kCGEventTapDisabledByTimeout || type == kCGEventTapDisabledByUserInput) {
		CGEventTapEnable(tap, true);
		// The release may have happened while the tap was off; don't leave the recording open.
		int hold = atomic_load(&holdKeycode);
		if (hold >= 0 && !(CGEventSourceFlagsState(kCGEventSourceStateHIDSystemState) & flagFor(hold))) {
			voxTapEvent(evHoldUp);
		}
		return event;
	}
	if (type == kCGEventLeftMouseDown || type == kCGEventRightMouseDown || type == kCGEventOtherMouseDown) {
		voxTapEvent(evOther); // an ⌥-click or ⌘-click, not a dictation
		return event;
	}
	int code = (int)CGEventGetIntegerValueField(event, kCGKeyboardEventKeycode);
	if (type == kCGEventKeyDown) {
		if (code != kcEscape) {
			voxTapEvent(evOther);
			return event;
		}
		voxTapEvent(evEscape);
		return atomic_load(&swallowEscape) ? NULL : event;
	}
	int hold = atomic_load(&holdKeycode);
	if (code == hold) {
		voxTapEvent((CGEventGetFlags(event) & flagFor(hold)) ? evHoldDown : evHoldUp);
	} else {
		voxTapEvent(evOther);
	}
	return event;
}

// Needs Accessibility; checking first avoids a system prompt before onboarding asks.
static bool createTap(void) {
	if (!AXIsProcessTrusted()) {
		return false;
	}
	CGEventMask mask = CGEventMaskBit(kCGEventKeyDown) | CGEventMaskBit(kCGEventFlagsChanged) |
		CGEventMaskBit(kCGEventLeftMouseDown) | CGEventMaskBit(kCGEventRightMouseDown) | CGEventMaskBit(kCGEventOtherMouseDown);
	tap = CGEventTapCreate(kCGSessionEventTap, kCGHeadInsertEventTap, kCGEventTapOptionDefault, mask, onEvent, NULL);
	return tap != NULL;
}

static void runTap(void) {
	CFRunLoopSourceRef source = CFMachPortCreateRunLoopSource(NULL, tap, 0);
	CFRunLoopAddSource(CFRunLoopGetCurrent(), source, kCFRunLoopCommonModes);
	CFRelease(source);
	CGEventTapEnable(tap, true);
	CFRunLoopRun();
}

static void setHoldKeycode(int code) { atomic_store(&holdKeycode, code); }
static void setSwallowEscape(bool on) { atomic_store(&swallowEscape, on); }
*/
import "C"

import (
	"runtime"
	"sync"
)

var (
	tapMu      sync.Mutex
	tapRunning bool
)

// startTap installs the keyboard event tap once; it lives for the process.
func startTap() bool {
	tapMu.Lock()
	defer tapMu.Unlock()
	if tapRunning {
		return true
	}
	created := make(chan bool)
	go func() {
		runtime.LockOSThread() // the tap's run loop owns this thread for good
		if !C.createTap() {
			runtime.UnlockOSThread()
			created <- false
			return
		}
		created <- true
		C.runTap()
	}()
	tapRunning = <-created
	return tapRunning
}

func isTapRunning() bool {
	tapMu.Lock()
	defer tapMu.Unlock()
	return tapRunning
}

func setHoldKeycode(code int) { C.setHoldKeycode(C.int(code)) }

func setSwallowEscape(on bool) { C.setSwallowEscape(C.bool(on)) }
