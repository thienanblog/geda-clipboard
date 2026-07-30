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

    // Convert to the space Wails' SetPosition uses: top-left origin, relative to
    // the screen's visibleFrame.
    int iconX = (int)lround(global.origin.x - work.origin.x);
    int iconY = (int)lround((work.origin.y + work.size.height) -
                            (global.origin.y + global.size.height));

    NSEventType type = [[NSApp currentEvent] type];
    BOOL isRight = (type == NSEventTypeRightMouseDown) ||
                   (type == NSEventTypeRightMouseUp) ||
                   ((type == NSEventTypeLeftMouseDown || type == NSEventTypeLeftMouseUp) &&
                    ([[NSApp currentEvent] modifierFlags] & NSEventModifierFlagControl) != 0);

    gedaTrayClicked(isRight ? 1 : 0,
                    iconX, iconY,
                    (int)lround(global.size.width),
                    (int)lround(global.size.height),
                    (int)lround(work.size.width),
                    (int)lround(work.size.height));
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
