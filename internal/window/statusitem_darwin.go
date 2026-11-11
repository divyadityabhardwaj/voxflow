//go:build darwin

package window

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>
#include <stdlib.h>
#include <math.h>

// Implemented in Go (statusitem_export_darwin.go). The //export must live in a
// file whose preamble has no definitions, so the class stays here.
extern void voxStatusItemAction(int tag);

static void refreshStatusMark(void);

@interface VoxStatusTarget : NSObject
@end

@implementation VoxStatusTarget
- (void)tick:(NSTimer *)timer { refreshStatusMark(); }
- (void)menuAction:(NSMenuItem *)sender {
    voxStatusItemAction((int)[sender tag]);
}
@end

// Process-lifetime objects; never released (cgo compiles without ARC).
static NSStatusItem *statusItem;
static VoxStatusTarget *statusTarget;
static NSMenuItem *recordItem;
static NSMenuItem *cancelItem;
static NSMenuItem *updateItem;
static NSTimer *statusTimer;
static int statusState;
static BOOL statusAttention;

// Match the three rounded waveform bars in the workspace brand. Template
// rendering lets macOS handle contrast and menu selection in every appearance.
static void refreshStatusMark(void) {
    BOOL reduced = NSWorkspace.sharedWorkspace.accessibilityDisplayShouldReduceMotion;
    CGFloat phase = reduced ? 0 : NSDate.timeIntervalSinceReferenceDate * (statusState == 1 ? 4 : 2.5);
    NSImage *image = [NSImage imageWithSize:NSMakeSize(20, 18) flipped:NO drawingHandler:^BOOL(NSRect rect) {
        [NSColor.blackColor setFill];
        for (int i = 0; i < 3; i++) {
            CGFloat height = i == 1 ? 16 : 8;
            if (statusState != 0 && !reduced) height = 5 + (sin(phase - i * 0.9) + 1) * 5;
            [[NSBezierPath bezierPathWithRoundedRect:NSMakeRect(3 + i * 5, 9 - height/2, 3, height)
                                            xRadius:1.5 yRadius:1.5] fill];
        }
        if (statusAttention) [[NSBezierPath bezierPathWithOvalInRect:NSMakeRect(17, 1, 3, 3)] fill];
        return YES;
    }];
    image.template = YES;
    statusItem.button.image = image;
    NSString *description = statusState == 1 ? @"VoxFlow · Listening" : statusState == 2 ? @"VoxFlow · Processing" : statusAttention ? @"VoxFlow · Needs attention" : @"VoxFlow · Ready";
    statusItem.button.toolTip = description;
    [statusItem.button setAccessibilityLabel:description];
    if (@available(macOS 10.14, *)) statusItem.button.contentTintColor = statusState == 1 ? NSColor.systemRedColor : nil;
}

void setStatusAttention(bool attention) {
    // All callers are already on the Cocoa main queue.
    statusAttention = attention;
    refreshStatusMark();
}

static NSMenuItem *addStatusMenuItem(NSMenu *menu, NSString *title, int tag) {
    NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:title action:@selector(menuAction:) keyEquivalent:@""];
    [item setTarget:statusTarget];
    [item setTag:tag];
    [menu addItem:item];
    return item;
}

static void applyStatusItemState(int state) {
    statusState = state;
    if (state == 1) statusAttention = NO;
    NSString *title = state == 1 ? @"Stop Recording" : state == 2 ? @"Processing…" : @"Start Recording";
    [recordItem setTitle:title];
    [recordItem setEnabled:state != 2];
    [cancelItem setHidden:state == 0];
    [statusTimer invalidate];
    statusTimer = nil;
    refreshStatusMark();
    if (state != 0 && !NSWorkspace.sharedWorkspace.accessibilityDisplayShouldReduceMotion) {
        statusTimer = [NSTimer timerWithTimeInterval:0.2 target:statusTarget selector:@selector(tick:) userInfo:nil repeats:YES];
        [NSRunLoop.mainRunLoop addTimer:statusTimer forMode:NSRunLoopCommonModes];
    }
}

void installStatusItem(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (statusItem) {
            return;
        }
        statusItem = [[[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength] retain];
        statusTarget = [[VoxStatusTarget alloc] init];

        NSMenu *menu = [[NSMenu alloc] initWithTitle:@""];
        // Otherwise NSMenu re-enables every item with a valid target on open.
        [menu setAutoenablesItems:NO];
        recordItem = addStatusMenuItem(menu, @"Start Recording", 1);
        cancelItem = addStatusMenuItem(menu, @"Cancel Dictation", 5);
        addStatusMenuItem(menu, @"Open VoxFlow", 2);
        addStatusMenuItem(menu, @"Settings…", 3);
        [menu addItem:[NSMenuItem separatorItem]];
        addStatusMenuItem(menu, @"Quit VoxFlow", 4);
        [statusItem setMenu:menu];

        applyStatusItemState(0);
    });
}

// Copies title before returning.
void setUpdateItem(const char *title) {
    NSString *t = [[NSString alloc] initWithUTF8String:title];
    dispatch_async(dispatch_get_main_queue(), ^{
        if (statusItem) {
            NSMenu *menu = [statusItem menu];
            if (!updateItem) {
                updateItem = [[NSMenuItem alloc] initWithTitle:t action:@selector(menuAction:) keyEquivalent:@""];
                [updateItem setTarget:statusTarget];
                [updateItem setTag:6];
                [menu insertItem:updateItem atIndex:0];
                [menu insertItem:[NSMenuItem separatorItem] atIndex:1];
            }
            [updateItem setTitle:t];
        }
        [t release];
    });
}

void setStatusItemState(int state) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (statusItem) {
            applyStatusItemState(state);
        }
    });
}
*/
import "C"

import "unsafe"

var statusCallbacks StatusItemCallbacks

func InstallStatusItem(cb StatusItemCallbacks) {
	statusCallbacks = cb
	C.installStatusItem()
}

// SetUpdateItem shows a menu item at the top of the status menu that calls
// StatusItemCallbacks.OpenUpdate.
func SetUpdateItem(title string) {
	cs := C.CString(title)
	defer C.free(unsafe.Pointer(cs))
	C.setUpdateItem(cs)
}

func SetStatusItemState(state string) {
	var s C.int
	switch state {
	case "Recording":
		s = 1
	case "Processing":
		s = 2
	}
	C.setStatusItemState(s)
}
