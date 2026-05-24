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

void constrainWindowToScreen() {
    dispatch_async(dispatch_get_main_queue(), ^{
        NSApplication *app = [NSApplication sharedApplication];
        for (NSWindow *window in [app windows]) {
            NSScreen *screen = [window screen];
            if (!screen) {
                screen = [NSScreen mainScreen];
            }
            if (screen) {
                NSRect visibleFrame = [screen visibleFrame];
                NSRect windowFrame = [window frame];
                
                BOOL adjusted = NO;
                
                // Keep X within visible screen frame
                if (windowFrame.origin.x < visibleFrame.origin.x) {
                    windowFrame.origin.x = visibleFrame.origin.x;
                    adjusted = YES;
                } else if (windowFrame.origin.x + windowFrame.size.width > visibleFrame.origin.x + visibleFrame.size.width) {
                    windowFrame.origin.x = visibleFrame.origin.x + visibleFrame.size.width - windowFrame.size.width;
                    adjusted = YES;
                }
                
                // Keep Y within visible screen frame
                if (windowFrame.origin.y < visibleFrame.origin.y) {
                    windowFrame.origin.y = visibleFrame.origin.y;
                    adjusted = YES;
                } else if (windowFrame.origin.y + windowFrame.size.height > visibleFrame.origin.y + visibleFrame.size.height) {
                    windowFrame.origin.y = visibleFrame.origin.y + visibleFrame.size.height - windowFrame.size.height;
                    adjusted = YES;
                }
                
                if (adjusted) {
                    [window setFrame:windowFrame display:YES];
                }
            }
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

// ConstrainWindow bounds the window to the visible area of the screen.
func ConstrainWindow() {
	C.constrainWindowToScreen()
}
