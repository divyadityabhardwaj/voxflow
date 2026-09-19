//go:build darwin

package window

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>

// Implemented in Go (statusitem_export_darwin.go). The //export must live in a
// file whose preamble has no definitions, so the class stays here.
extern void voxStatusItemAction(int tag);

@interface VoxStatusTarget : NSObject
@end

@implementation VoxStatusTarget
- (void)menuAction:(NSMenuItem *)sender {
    voxStatusItemAction((int)[sender tag]);
}
@end

// Process-lifetime objects; never released (cgo compiles without ARC).
static NSStatusItem *statusItem;
static VoxStatusTarget *statusTarget;
static NSMenuItem *recordItem;

static NSMenuItem *addStatusMenuItem(NSMenu *menu, NSString *title, int tag) {
    NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:title action:@selector(menuAction:) keyEquivalent:@""];
    [item setTarget:statusTarget];
    [item setTag:tag];
    [menu addItem:item];
    return item;
}

static void applyStatusItemState(int state) {
    NSString *symbol = @"mic";
    NSString *title = @"Start Recording";
    NSColor *tint = nil;
    if (state == 1) {
        symbol = @"mic.fill";
        title = @"Stop Recording";
        tint = [NSColor systemRedColor];
    } else if (state == 2) {
        symbol = @"hourglass";
        title = @"Processing…";
    }
    [recordItem setTitle:title];
    [recordItem setEnabled:state != 2];

    NSStatusBarButton *button = [statusItem button];
    if (@available(macOS 11.0, *)) {
        NSImage *image = [NSImage imageWithSystemSymbolName:symbol accessibilityDescription:@"VoxFlow"];
        [image setTemplate:YES];
        [button setImage:image];
    } else {
        [button setTitle:@"Vox"];
    }
    if (@available(macOS 10.14, *)) {
        [button setContentTintColor:tint];
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
        addStatusMenuItem(menu, @"Open VoxFlow", 2);
        addStatusMenuItem(menu, @"Settings…", 3);
        [menu addItem:[NSMenuItem separatorItem]];
        addStatusMenuItem(menu, @"Quit VoxFlow", 4);
        [statusItem setMenu:menu];

        applyStatusItemState(0);
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

var statusCallbacks StatusItemCallbacks

func InstallStatusItem(cb StatusItemCallbacks) {
	statusCallbacks = cb
	C.installStatusItem()
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
