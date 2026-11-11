//go:build darwin

package window

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>

void resetWindowBehavior() {
    dispatch_async(dispatch_get_main_queue(), ^{
        Class wailsWindow = NSClassFromString(@"WailsWindow");
        for (NSWindow *window in [NSApp windows]) {
            if (![window isKindOfClass:wailsWindow]) {
                continue;
            }
            // The green button zooms rather than entering a full-screen Space the pill can't leave.
            [window setCollectionBehavior:NSWindowCollectionBehaviorFullScreenNone];
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

extern void voxWindowFrameChanged(void);
extern void voxWindowCloseClicked(void);

@interface VoxCloseTarget : NSObject
@end

@implementation VoxCloseTarget
- (void)collapse:(id)sender {
    voxWindowCloseClicked();
}
@end

static VoxCloseTarget *closeTarget;

// Only the workspace is a WailsWindow. The native overlay remains independent
// of workspace chrome, activation policy and visibility.
void setWindowChrome(bool full) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (!closeTarget) {
            closeTarget = [VoxCloseTarget new];
        }
        Class wailsWindow = NSClassFromString(@"WailsWindow");
        for (NSWindow *window in [NSApp windows]) {
            if (![window isKindOfClass:wailsWindow]) {
                continue;
            }
            NSWindowStyleMask mask = [window styleMask];
            [window setStyleMask:full ? (mask | NSWindowStyleMaskResizable) : (mask & ~NSWindowStyleMaskResizable)];
            // After setStyleMask, which can rebuild the title bar buttons.
            for (NSNumber *kind in @[@(NSWindowCloseButton), @(NSWindowZoomButton)]) {
                [[window standardWindowButton:(NSWindowButton)[kind integerValue]] setHidden:!full];
            }
            // Background mode replaces minimizing to the Dock.
            [[window standardWindowButton:NSWindowMiniaturizeButton] setHidden:YES];
            // Red hides the workspace; quitting would kill the hotkeys.
            NSButton *close = [window standardWindowButton:NSWindowCloseButton];
            [close setTarget:closeTarget];
            [close setAction:@selector(collapse:)];
        }
        [NSApp setActivationPolicy:full ? NSApplicationActivationPolicyRegular : NSApplicationActivationPolicyAccessory];
        if (full) {
            [NSApp activateIgnoringOtherApps:YES];
        }
    });
}

void observeWindowFrame() {
    dispatch_async(dispatch_get_main_queue(), ^{
        NSNotificationCenter *center = [NSNotificationCenter defaultCenter];
        Class wailsWindow = NSClassFromString(@"WailsWindow");
        for (NSWindow *window in [NSApp windows]) {
            // Skip the status item's window.
            if (![window isKindOfClass:wailsWindow]) {
                continue;
            }
            for (NSNotificationName name in @[NSWindowDidMoveNotification, NSWindowDidResizeNotification]) {
                [center addObserverForName:name object:window queue:nil usingBlock:^(NSNotification *note) {
                    voxWindowFrameChanged();
                }];
            }
        }
    });
}
*/
import "C"

import "sync/atomic"

func setChrome(full bool) {
	C.setWindowChrome(C.bool(full))
}

var observedManager atomic.Pointer[Manager]

func observeWindowFrame(m *Manager) {
	observedManager.Store(m)
	C.observeWindowFrame()
}

func ResetBehavior() {
	C.resetWindowBehavior()
}

func ConstrainWindow() {
	C.constrainWindowToScreen()
}
