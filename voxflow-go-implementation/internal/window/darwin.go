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
            [window setHasShadow:NO];
        }
    });
}

void resetWindowBehavior() {
    dispatch_async(dispatch_get_main_queue(), ^{
        NSApplication *app = [NSApplication sharedApplication];
        for (NSWindow *window in [app windows]) {
            [window setCollectionBehavior:0];
            [window setLevel:NSNormalWindowLevel];
            [window setHasShadow:YES];
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

                if (windowFrame.origin.x < visibleFrame.origin.x) {
                    windowFrame.origin.x = visibleFrame.origin.x;
                    adjusted = YES;
                } else if (windowFrame.origin.x + windowFrame.size.width > visibleFrame.origin.x + visibleFrame.size.width) {
                    windowFrame.origin.x = visibleFrame.origin.x + visibleFrame.size.width - windowFrame.size.width;
                    adjusted = YES;
                }

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

// All spaces + above fullscreen apps.
func FloatEverywhere() {
	C.makeWindowFloatEverywhere()
}

func ResetBehavior() {
	C.resetWindowBehavior()
}

func ConstrainWindow() {
	C.constrainWindowToScreen()
}
