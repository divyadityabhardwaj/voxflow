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
            // Mini Mode: Borderless, Floating, All Spaces
            
            // Set style to Borderless (0)
            [window setStyleMask: NSWindowStyleMaskBorderless];
            
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
            // Normal Mode: Titled, Resizable, Standard Controls
            
            // Set style to Titled | Closable | Miniaturizable | Resizable
            // 1 | 2 | 4 | 8 = 15
            NSWindowStyleMask normalMask = NSWindowStyleMaskTitled | NSWindowStyleMaskClosable | NSWindowStyleMaskMiniaturizable | NSWindowStyleMaskResizable;
            [window setStyleMask: normalMask];

            // Set Titlebar to appear transparent if desired, or standard. 
            // Wails config sets TitlebarAppearsTransparent: true. 
            // We want it to look "native" so we keep the properties consistent with a standard window.
            [window setTitlebarAppearsTransparent:YES];
            [window setTitleVisibility:NSWindowTitleHidden]; // Hide text title, keep traffic lights
            
            // Reset to normal window behavior
            [window setCollectionBehavior:NSWindowCollectionBehaviorDefault];
            [window setLevel:NSNormalWindowLevel];
            
            // Ensure functionality
            [window setMovable:YES];
            [window setMovableByWindowBackground:YES]; // Allow dragging by background since title is hidden
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
