//go:build darwin

package window

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>

void makeWindowFloatEverywhere() {
    dispatch_async(dispatch_get_main_queue(), ^{
        NSApplication *app = [NSApplication sharedApplication];
        for (NSWindow *window in [app windows]) {
            [window setCollectionBehavior:273];
            [window setLevel:101];
            [window setAnimationBehavior:NSWindowAnimationBehaviorNone];
        }
    });
}

void resetWindowBehavior() {
    dispatch_async(dispatch_get_main_queue(), ^{
        NSApplication *app = [NSApplication sharedApplication];
        for (NSWindow *window in [app windows]) {
            [window setCollectionBehavior:0];
            [window setLevel:NSNormalWindowLevel];
        }
    });
}
*/
import "C"

// FloatEverywhere makes the window visible on all desktops/spaces
// and able to appear over fullscreen applications.
func FloatEverywhere() {
	C.makeWindowFloatEverywhere()
}

// ResetBehavior resets the window to normal macOS behavior.
func ResetBehavior() {
	C.resetWindowBehavior()
}
