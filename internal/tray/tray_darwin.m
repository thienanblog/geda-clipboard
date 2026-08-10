//go:build darwin

#import <Cocoa/Cocoa.h>
#include "tray_darwin.h"
#include "_cgo_export.h"

// GedaTrayTarget is the action target for the status bar button. We deliberately
// avoid becoming the NSApplication delegate: Wails owns that, and taking it
// over breaks window and lifecycle handling.
@interface GedaTrayTarget : NSObject
- (void)onClick:(id)sender;
@end

static GedaTrayTarget *gTarget = nil;
static NSStatusItem *gStatusItem = nil;

// AppKit puts the global origin at the bottom-left of the primary display and
// grows y upwards. Everywhere outside AppKit -- CoreGraphics, the Windows
// virtual screen, and this application's own Rect -- puts it at the top-left
// and grows y downwards, so the primary display's height is the pivot for
// converting between the two.
static CGFloat gedaPrimaryHeight(void) {
    NSArray<NSScreen *> *screens = [NSScreen screens];
    if ([screens count] == 0) {
        return 0;
    }
    return [[screens objectAtIndex:0] frame].size.height;
}

// gedaWorkArea reports a screen's usable area -- everything but the menu bar
// and the Dock -- in the global, top-left-origin space.
static void gedaWorkArea(NSScreen *screen, int *x, int *y, int *w, int *h) {
    NSRect work = [screen visibleFrame];
    *x = (int)lround(work.origin.x);
    *y = (int)lround(gedaPrimaryHeight() - (work.origin.y + work.size.height));
    *w = (int)lround(work.size.width);
    *h = (int)lround(work.size.height);
}

@implementation GedaTrayTarget

- (void)onClick:(id)sender {
    NSStatusBarButton *button = (NSStatusBarButton *)sender;
    NSWindow *window = [button window];
    NSScreen *screen = [window screen];
    if (screen == nil) {
        screen = [NSScreen mainScreen];
    }

    // Button bounds -> global screen coordinates (AppKit: bottom-left origin).
    NSRect global = [window convertRectToScreen:[button bounds]];
    NSRect work = [screen visibleFrame];

    // Convert to a top-left origin relative to the screen's visibleFrame.
    int iconX = (int)lround(global.origin.x - work.origin.x);
    int iconY = (int)lround((work.origin.y + work.size.height) -
                            (global.origin.y + global.size.height));

    int workX = 0, workY = 0, workW = 0, workH = 0;
    gedaWorkArea(screen, &workX, &workY, &workW, &workH);

    NSEventType type = [[NSApp currentEvent] type];
    BOOL isRight = (type == NSEventTypeRightMouseDown) ||
                   (type == NSEventTypeRightMouseUp) ||
                   ((type == NSEventTypeLeftMouseDown || type == NSEventTypeLeftMouseUp) &&
                    ([[NSApp currentEvent] modifierFlags] & NSEventModifierFlagControl) != 0);

    gedaTrayClicked(isRight ? 1 : 0,
                    iconX, iconY,
                    (int)lround(global.size.width),
                    (int)lround(global.size.height),
                    workX, workY, workW, workH);
}

@end

static void gedaTrayCreateOnMain(NSData *iconData, NSString *tooltip) {
    if (gStatusItem != nil) {
        gedaTrayCreateResult(1, 1);
        return;
    }

    gStatusItem = [[[NSStatusBar systemStatusBar]
        statusItemWithLength:NSVariableStatusItemLength] retain];
    gTarget = [[GedaTrayTarget alloc] init];

    NSStatusBarButton *button = gStatusItem.button;

    if (iconData != nil && [iconData length] > 0) {
        NSImage *image = [[NSImage alloc] initWithData:iconData];
        if (image != nil) {
            [image setSize:NSMakeSize(18, 18)];
            // Template images are recoloured by the system for light/dark menu
            // bars and for the highlighted state.
            [image setTemplate:YES];
            button.image = image;
            button.imagePosition = NSImageOnly;
            [image release];
        }
    }
    if (button.image == nil) {
        button.title = @"\U0001F4CB";
    }

    if (tooltip != nil) {
        button.toolTip = tooltip;
    }

    button.target = gTarget;
    button.action = @selector(onClick:);
    // Fire on mouse-down for both buttons so the popup feels immediate and we
    // can tell left from right.
    [button sendActionOn:(NSEventMaskLeftMouseDown | NSEventMaskRightMouseDown)];

    gedaTrayCreateResult(gStatusItem != nil ? 1 : 0, button.image != nil ? 1 : 0);
}

void gedaTrayCreate(const void *iconBytes, int iconLen, const char *tooltip) {
    NSData *iconData = nil;
    if (iconBytes != NULL && iconLen > 0) {
        iconData = [NSData dataWithBytes:iconBytes length:iconLen];
    }
    NSString *tip = (tooltip != NULL) ? [NSString stringWithUTF8String:tooltip] : nil;

    if ([NSThread isMainThread]) {
        gedaTrayCreateOnMain(iconData, tip);
        return;
    }

    // Wails calls OnStartup on a goroutine that races [NSApp run], so the main
    // queue may not be serviced yet. Dispatch asynchronously: blocking here
    // would stall the rest of app startup until the run loop comes up.
    [iconData retain];
    [tip retain];
    dispatch_async(dispatch_get_main_queue(), ^{
        gedaTrayCreateOnMain(iconData, tip);
        [iconData release];
        [tip release];
    });
}

int gedaTrayExists(void) {
    return gStatusItem != nil ? 1 : 0;
}

int gedaCursorAnchor(int *x, int *y, int *workX, int *workY, int *workW, int *workH) {
    if (x == NULL || y == NULL || workX == NULL || workY == NULL ||
        workW == NULL || workH == NULL) {
        return 0;
    }

    // +[NSEvent mouseLocation] reads the window server rather than the current
    // event, so it is safe to call from the goroutine that shows the popup and
    // it answers even when no event is being dispatched.
    NSPoint pointer = [NSEvent mouseLocation];

    // The pointer may be on any display, and each has its own origin in the
    // global space, so find the one it is actually over.
    NSScreen *screen = nil;
    for (NSScreen *candidate in [NSScreen screens]) {
        if (NSMouseInRect(pointer, [candidate frame], NO)) {
            screen = candidate;
            break;
        }
    }
    if (screen == nil) {
        screen = [NSScreen mainScreen];
    }
    if (screen == nil) {
        return 0;
    }

    NSRect work = [screen visibleFrame];

    // Same conversion as the tray icon: AppKit's bottom-left origin global
    // space to a top-left origin relative to the screen's visibleFrame. A
    // pointer up in the menu bar lands above the work area, giving a negative
    // y that CursorPopupPosition clamps away.
    *x = (int)lround(pointer.x - work.origin.x);
    *y = (int)lround((work.origin.y + work.size.height) - pointer.y);
    gedaWorkArea(screen, workX, workY, workW, workH);
    return 1;
}

static void gedaTrayDestroyOnMain(void) {
    if (gStatusItem == nil) {
        return;
    }
    [[NSStatusBar systemStatusBar] removeStatusItem:gStatusItem];
    [gStatusItem release];
    gStatusItem = nil;
    [gTarget release];
    gTarget = nil;
}

void gedaTrayDestroy(void) {
    if ([NSThread isMainThread]) {
        gedaTrayDestroyOnMain();
    } else {
        dispatch_sync(dispatch_get_main_queue(), ^{ gedaTrayDestroyOnMain(); });
    }
}
