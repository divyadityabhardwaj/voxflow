//go:build darwin

package window

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>
#include <math.h>
#include <stdlib.h>

extern void voxStatusItemAction(int tag);
extern void setStatusAttention(bool attention);

// A real nonactivating NSPanel is essential: changing the level of the Wails
// NSWindow alone doesn't give it eligibility to join another app's full-screen Space.
@interface VoxOverlayPanel : NSPanel
@end
@implementation VoxOverlayPanel
- (BOOL)canBecomeKeyWindow { return NO; }
- (BOOL)canBecomeMainWindow { return NO; }
@end

@interface VoxOverlayView : NSView
@property int mode;
@property CGFloat level;
@property NSTimeInterval startedAt;
@property(nonatomic, retain) NSTextField *heading;
@property(nonatomic, retain) NSTextField *detail;
@property(nonatomic, retain) NSTextField *clock;
@property(nonatomic, retain) NSButton *primary;
@property(nonatomic, retain) NSButton *dismiss;
- (void)tick;
@end

static VoxOverlayPanel *overlayPanel;
static VoxOverlayView *overlayView;
static NSTimer *overlayTimer;
static NSUInteger overlayGeneration;

static void hideOverlay(void) {
    overlayGeneration++;
    setStatusAttention(false);
    [overlayPanel orderOut:nil];
    [overlayTimer invalidate];
    overlayTimer = nil;
}

@implementation VoxOverlayView
- (BOOL)isFlipped { return YES; }
- (void)drawRect:(NSRect)dirtyRect {
    NSRect capsule = NSInsetRect(self.bounds, 1, 1);
    CGFloat radius = self.mode == 5 ? 16 : 17;
    NSBezierPath *path = [NSBezierPath bezierPathWithRoundedRect:capsule xRadius:radius yRadius:radius];
    [[NSColor colorWithWhite:0.075 alpha:0.97] setFill];
    [path fill];
    [[NSColor colorWithWhite:1 alpha:0.1] setStroke];
    [path setLineWidth:1];
    [path stroke];

    NSColor *ink = self.mode == 1 ? [NSColor systemRedColor] :
                   self.mode == 4 ? [NSColor systemGreenColor] :
                   self.mode == 5 ? [NSColor systemOrangeColor] : [NSColor whiteColor];
    [ink setFill];
    BOOL reduced = NSWorkspace.sharedWorkspace.accessibilityDisplayShouldReduceMotion;
    NSTimeInterval phase = reduced ? 0 : NSDate.timeIntervalSinceReferenceDate * 3;
    for (int i = 0; i < 3; i++) {
        CGFloat height = i == 1 ? 12 : 6;
        if (self.mode == 1) height = 3 + self.level * (i == 1 ? 10 : 7);
        if (self.mode == 2 || self.mode == 3) height = reduced ? height : 4 + (sin(phase - i * 0.8) + 1) * 3;
        [[NSBezierPath bezierPathWithRoundedRect:NSMakeRect(13 + i * 4, 17 - height/2, 2, height)
                                        xRadius:1 yRadius:1] fill];
    }
}
- (void)tick {
    if (self.mode == 1) {
        NSInteger seconds = MAX(0, (NSInteger)(NSDate.timeIntervalSinceReferenceDate - self.startedAt));
        self.clock.stringValue = [NSString stringWithFormat:@"%ld:%02ld", (long)(seconds/60), (long)(seconds%60)];
    }
    [self setNeedsDisplay:YES];
}
- (void)primaryAction:(id)sender {
    if (self.mode == 1) voxStatusItemAction(1);
    else if (self.mode == 2 || self.mode == 3) voxStatusItemAction(5);
    else { hideOverlay(); voxStatusItemAction(2); }
}
- (void)dismissAction:(id)sender {
    if (self.mode >= 1 && self.mode <= 3) voxStatusItemAction(5);
    else hideOverlay();
}
@end

static NSTextField *overlayLabel(CGFloat size, NSColor *color) {
    NSTextField *label = [NSTextField labelWithString:@""];
    label.font = [NSFont systemFontOfSize:size weight:NSFontWeightMedium];
    label.textColor = color;
    label.lineBreakMode = NSLineBreakByTruncatingTail;
    return label;
}

static NSScreen *overlayScreen(void) {
    NSPoint pointer = NSEvent.mouseLocation;
    for (NSScreen *screen in NSScreen.screens) {
        if (NSPointInRect(pointer, screen.frame)) return screen;
    }
    return NSScreen.mainScreen ?: NSScreen.screens.firstObject;
}

// Preserve the top edge as feedback expands downward. visibleFrame handles the
// menu bar, while safeAreaInsets also protects the notch with an auto-hidden bar.
static void positionOverlay(BOOL chooseScreen) {
    NSScreen *screen = chooseScreen ? overlayScreen() : (overlayPanel.screen ?: overlayScreen());
    if (!screen) return;
    NSRect usable = screen.visibleFrame;
    CGFloat top = NSMaxY(usable);
    if (@available(macOS 12.0, *)) top = MIN(top, NSMaxY(screen.frame) - screen.safeAreaInsets.top);
    BOOL attention = overlayView.mode == 5;
    CGFloat width = MIN(attention ? 304 : 232, NSWidth(usable) - 24);
    CGFloat height = attention ? 94 : 34;
    [overlayPanel setFrame:NSMakeRect(NSMidX(usable) - width/2, top - height - 12, width, height) display:YES];
    overlayView.heading.frame = NSMakeRect(32, 8, width - (attention ? 70 : 139), 17);
    overlayView.clock.frame = NSMakeRect(width - 101, 9, 32, 16);
    overlayView.primary.frame = attention ? NSMakeRect(width - 113, 69, 96, 20) : NSMakeRect(width - 65, 6, 24, 22);
    overlayView.dismiss.frame = NSMakeRect(width - 33, 6, 23, 22);
    overlayView.detail.frame = NSMakeRect(15, 33, width - 30, 32);
}

static void installOverlayPanel(void) {
    if (overlayPanel) return;
    overlayPanel = [[VoxOverlayPanel alloc] initWithContentRect:NSMakeRect(0, 0, 232, 34)
        styleMask:NSWindowStyleMaskBorderless | NSWindowStyleMaskNonactivatingPanel
        backing:NSBackingStoreBuffered defer:NO];
    overlayPanel.releasedWhenClosed = NO;
    overlayPanel.floatingPanel = YES;
    overlayPanel.hidesOnDeactivate = NO;
    overlayPanel.opaque = NO;
    overlayPanel.backgroundColor = NSColor.clearColor;
    overlayPanel.hasShadow = YES;
    overlayPanel.level = NSPopUpMenuWindowLevel;
    overlayPanel.animationBehavior = NSWindowAnimationBehaviorNone;
    NSWindowCollectionBehavior behavior = NSWindowCollectionBehaviorCanJoinAllSpaces |
        NSWindowCollectionBehaviorFullScreenAuxiliary | NSWindowCollectionBehaviorStationary |
        NSWindowCollectionBehaviorIgnoresCycle;
    if (@available(macOS 13.0, *)) behavior |= NSWindowCollectionBehaviorCanJoinAllApplications;
    overlayPanel.collectionBehavior = behavior;
    [overlayPanel setAccessibilityLabel:@"VoxFlow dictation"];

    overlayView = [[VoxOverlayView alloc] initWithFrame:NSMakeRect(0, 0, 232, 34)];
    overlayView.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
    overlayView.heading = overlayLabel(11, NSColor.whiteColor);
    overlayView.detail = overlayLabel(10, [NSColor colorWithWhite:0.8 alpha:1]);
    overlayView.detail.maximumNumberOfLines = 2;
    overlayView.detail.cell.wraps = YES;
    overlayView.clock = overlayLabel(10, [NSColor colorWithWhite:0.65 alpha:1]);
    overlayView.clock.font = [NSFont monospacedDigitSystemFontOfSize:10 weight:NSFontWeightRegular];
    overlayView.primary = [NSButton buttonWithTitle:@"■" target:overlayView action:@selector(primaryAction:)];
    overlayView.dismiss = [NSButton buttonWithTitle:@"×" target:overlayView action:@selector(dismissAction:)];
    for (NSButton *button in @[overlayView.primary, overlayView.dismiss]) {
        button.bordered = NO;
        button.font = [NSFont systemFontOfSize:12];
        if (@available(macOS 10.14, *)) button.contentTintColor = NSColor.whiteColor;
    }
    for (NSView *view in @[overlayView.heading, overlayView.detail, overlayView.clock, overlayView.primary, overlayView.dismiss]) {
        [overlayView addSubview:view];
    }
    overlayPanel.contentView = overlayView;

    // Space/display transitions can remove an already-ordered panel from the
    // window-server's active list. Reorder without activating VoxFlow.
    [NSWorkspace.sharedWorkspace.notificationCenter addObserverForName:NSWorkspaceActiveSpaceDidChangeNotification
        object:nil queue:NSOperationQueue.mainQueue usingBlock:^(NSNotification *note) {
            if (overlayPanel.visible) { positionOverlay(YES); [overlayPanel orderFrontRegardless]; }
        }];
    [NSNotificationCenter.defaultCenter addObserverForName:NSApplicationDidChangeScreenParametersNotification
        object:nil queue:NSOperationQueue.mainQueue usingBlock:^(NSNotification *note) {
            if (overlayPanel.visible) { positionOverlay(YES); [overlayPanel orderFrontRegardless]; }
        }];
}

void installNativeOverlay(void) {
    dispatch_async(dispatch_get_main_queue(), ^{ installOverlayPanel(); });
}

void updateNativeOverlay(int mode, const char *label, const char *detail) {
    NSString *heading = [[NSString alloc] initWithUTF8String:label];
    NSString *message = [[NSString alloc] initWithUTF8String:detail];
    dispatch_async(dispatch_get_main_queue(), ^{
        installOverlayPanel();
        if (mode == 0) {
            // Success owns its expiry; errors stay actionable until dismissed.
            if (overlayView.mode != 4 && overlayView.mode != 5) hideOverlay();
        } else if (!(mode == 4 && overlayView.mode == 5)) {
            BOOL newSession = mode == 1 || !overlayPanel.visible;
            overlayGeneration++;
            overlayView.mode = mode;
            setStatusAttention(mode == 5);
            overlayView.heading.stringValue = heading;
            overlayView.heading.toolTip = heading;
            overlayView.detail.stringValue = message;
            overlayView.detail.toolTip = message;
            overlayView.detail.hidden = mode != 5;
            overlayView.clock.hidden = mode != 1;
            overlayView.primary.hidden = mode == 4;
            overlayView.primary.title = mode == 1 ? @"■" : mode == 5 ? @"Open VoxFlow →" : @"×";
            overlayView.primary.font = [NSFont systemFontOfSize:mode == 5 ? 10 : 12];
            overlayView.primary.toolTip = mode == 1 ? @"Stop and deliver" : mode == 5 ? @"Open VoxFlow to resolve" : @"Cancel dictation";
            [overlayView.primary setAccessibilityLabel:overlayView.primary.toolTip];
            overlayView.dismiss.hidden = mode == 2 || mode == 3;
            overlayView.dismiss.toolTip = mode == 1 ? @"Cancel dictation (Esc)" : @"Dismiss";
            [overlayView.dismiss setAccessibilityLabel:overlayView.dismiss.toolTip];
            if (mode == 1) { overlayView.startedAt = NSDate.timeIntervalSinceReferenceDate; overlayView.level = 0; }
            positionOverlay(newSession);
            [overlayView tick];
            BOOL reveal = !overlayPanel.visible && !NSWorkspace.sharedWorkspace.accessibilityDisplayShouldReduceMotion;
            overlayPanel.alphaValue = reveal ? 0 : 1;
            [overlayPanel orderFrontRegardless];
            if (reveal) {
                [NSAnimationContext runAnimationGroup:^(NSAnimationContext *context) {
                    context.duration = 0.12;
                    overlayPanel.animator.alphaValue = 1;
                } completionHandler:nil];
            }
            [overlayTimer invalidate];
            overlayTimer = nil;
            if (mode >= 1 && mode <= 3) {
                overlayTimer = [NSTimer timerWithTimeInterval:0.1 target:overlayView selector:@selector(tick) userInfo:nil repeats:YES];
                [NSRunLoop.mainRunLoop addTimer:overlayTimer forMode:NSRunLoopCommonModes];
            }
            if (mode == 4) {
                NSUInteger generation = overlayGeneration;
                dispatch_after(dispatch_time(DISPATCH_TIME_NOW, 1500 * NSEC_PER_MSEC), dispatch_get_main_queue(), ^{
                    if (generation == overlayGeneration) hideOverlay();
                });
            }
        }
        [heading release];
        [message release];
    });
}

void updateNativeOverlayLevel(double level) {
    dispatch_async(dispatch_get_main_queue(), ^{
        overlayView.level = MAX(0, MIN(1, level));
        if (overlayView.mode == 1) [overlayView setNeedsDisplay:YES];
    });
}
*/
import "C"

import "unsafe"

func InstallOverlay() { C.installNativeOverlay() }

func setOverlayState(mode int, label, detail string) {
	l, d := C.CString(label), C.CString(detail)
	defer C.free(unsafe.Pointer(l))
	defer C.free(unsafe.Pointer(d))
	C.updateNativeOverlay(C.int(mode), l, d)
}

func setOverlayLevel(level float64) { C.updateNativeOverlayLevel(C.double(level)) }
