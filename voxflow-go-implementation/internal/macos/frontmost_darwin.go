//go:build darwin

package macos

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>
#import <stdlib.h>

// Returns bundle ID and localized name of the frontmost application.
// Caller must free both returned C strings with free().
static void frontmostApp(char** outBundle, char** outName) {
    *outBundle = NULL;
    *outName = NULL;
    NSRunningApplication *app = [[NSWorkspace sharedWorkspace] frontmostApplication];
    if (!app) return;
    NSString *bundle = [app bundleIdentifier];
    NSString *name = [app localizedName];
    if (bundle) {
        *outBundle = strdup([[bundle UTF8String] UTF8String]);
    }
    if (name) {
        *outName = strdup([[name UTF8String] UTF8String]);
    }
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

// FrontmostApp returns the bundle identifier and display name of the active app.
func FrontmostApp() (bundleID, name string, err error) {
	var cBundle, cName *C.char
	C.frontmostApp(&cBundle, &cName)
	defer func() {
		if cBundle != nil {
			C.free(unsafe.Pointer(cBundle))
		}
		if cName != nil {
			C.free(unsafe.Pointer(cName))
		}
	}()

	if cBundle == nil {
		return "", "", errors.New("no frontmost application")
	}
	bundleID = C.GoString(cBundle)
	if cName != nil {
		name = C.GoString(cName)
	}
	return bundleID, name, nil
}
