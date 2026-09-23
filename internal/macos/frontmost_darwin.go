//go:build darwin

package macos

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework AppKit

#import <AppKit/AppKit.h>
#include <stdatomic.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

typedef struct {
	char *bundleID;
	char *name;
	int pid;
} appInfo;

// NSRunningApplication is safe to read off the main thread; the main run loop
// keeps NSWorkspace's view of the frontmost app current.
static appInfo infoFor(NSRunningApplication *app) {
	appInfo info = {NULL, NULL, 0};
	if (app) {
		info.bundleID = app.bundleIdentifier ? strdup(app.bundleIdentifier.UTF8String) : NULL;
		info.name = app.localizedName ? strdup(app.localizedName.UTF8String) : NULL;
		info.pid = app.processIdentifier;
	}
	return info;
}

static appInfo frontmostApp(void) {
	@autoreleasepool {
		return infoFor(NSWorkspace.sharedWorkspace.frontmostApplication);
	}
}

static atomic_int lastExternalPID;

static void noteActivated(NSRunningApplication *app) {
	if (app && app.processIdentifier != getpid()) {
		atomic_store(&lastExternalPID, app.processIdentifier);
	}
}

static void startAppTracking(void) {
	static dispatch_once_t once;
	dispatch_once(&once, ^{
		@autoreleasepool {
			NSWorkspace *ws = NSWorkspace.sharedWorkspace;
			noteActivated(ws.frontmostApplication);
			[ws.notificationCenter addObserverForName:NSWorkspaceDidActivateApplicationNotification
			                                   object:nil
			                                    queue:nil
			                               usingBlock:^(NSNotification *note) {
				noteActivated(note.userInfo[NSWorkspaceApplicationKey]);
			}];
		}
	});
}

static appInfo lastExternalApp(void) {
	@autoreleasepool {
		int pid = atomic_load(&lastExternalPID);
		return infoFor(pid > 0 ? [NSRunningApplication runningApplicationWithProcessIdentifier:pid] : nil);
	}
}

static bool activateApp(int pid) {
	@autoreleasepool {
		NSRunningApplication *app = [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
		return app && [app activateWithOptions:0];
	}
}
*/
import "C"

import (
	"errors"
	"time"
	"unsafe"
)

func goAppInfo(c C.appInfo) AppInfo {
	defer C.free(unsafe.Pointer(c.bundleID))
	defer C.free(unsafe.Pointer(c.name))
	return AppInfo{BundleID: C.GoString(c.bundleID), Name: C.GoString(c.name), PID: int(c.pid)}
}

func FrontmostAppInfo() (AppInfo, error) {
	info := goAppInfo(C.frontmostApp())
	if info.PID <= 0 {
		return AppInfo{}, errors.New("no frontmost application")
	}
	return info, nil
}

// StartAppTracking records every app activation other than VoxFlow's own, for
// LastExternalApp, and seeds it with the current frontmost app. Call it once
// from any goroutine (repeat calls are no-ops). Activations are delivered on
// the main run loop, so they only arrive once the Cocoa app is running.
func StartAppTracking() {
	C.startAppTracking()
}

// LastExternalApp is the most recently activated app that is not VoxFlow and
// is still running.
func LastExternalApp() (AppInfo, bool) {
	info := goAppInfo(C.lastExternalApp())
	return info, info.PID > 0
}

// ActivateApp asks pid to come to the front and waits until it is frontmost or
// timeout passes. Only an active app may hand activation to another, so it is
// meant for when VoxFlow is in front. Call it off the main thread: the
// frontmost app is only updated while the main run loop spins.
func ActivateApp(pid int, timeout time.Duration) bool {
	if !C.activateApp(C.int(pid)) {
		return false
	}
	for deadline := time.Now().Add(timeout); ; time.Sleep(10 * time.Millisecond) {
		if front, err := FrontmostAppInfo(); err == nil && front.PID == pid {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
	}
}
