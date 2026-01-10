//go:build darwin
// +build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>

// NSWindowCollectionBehavior flags
// canJoinAllSpaces = (1 << 0) = 1
// fullScreenAuxiliary = (1 << 8) = 256
// stationary = (1 << 4) = 16

void makeWindowFloatEverywhere() {
    dispatch_async(dispatch_get_main_queue(), ^{
        NSApplication *app = [NSApplication sharedApplication];
        for (NSWindow *window in [app windows]) {
            // Set collection behavior to appear on all spaces and over fullscreen apps
            // canJoinAllSpaces (1) + fullScreenAuxiliary (256) + stationary (16)
            [window setCollectionBehavior:273];

            // Set window level to float above everything (including fullscreen)
            // NSPopUpMenuWindowLevel = 101
            [window setLevel:101];

            // Don't activate when shown (prevents focus stealing)
            [window setAnimationBehavior:NSWindowAnimationBehaviorNone];
        }
    });
}

void resetWindowBehavior() {
    dispatch_async(dispatch_get_main_queue(), ^{
        NSApplication *app = [NSApplication sharedApplication];
        for (NSWindow *window in [app windows]) {
            // Reset to normal window behavior
            [window setCollectionBehavior:0];
            [window setLevel:NSNormalWindowLevel];
        }
    });
}
*/
import "C"

// MakeWindowFloatEverywhere makes the window visible on all desktops/spaces
// and able to appear over fullscreen applications
func MakeWindowFloatEverywhere() {
	C.makeWindowFloatEverywhere()
}

// ResetWindowBehavior resets the window to normal behavior
func ResetWindowBehavior() {
	C.resetWindowBehavior()
}
