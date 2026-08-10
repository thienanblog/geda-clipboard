//go:build darwin

#import <Cocoa/Cocoa.h>
#include "window_darwin.h"

// Wails names its NSWindow subclass WailsWindow, which is how the application's
// own window is told apart from the ones AppKit creates on its behalf -- the
// status item lives in an NSStatusBarWindow, and the WebView adds more. Matching
// on the class rather than the title keeps this working if the title changes.
static NSWindow *gedaMainWindow(void) {
    for (NSWindow *candidate in [NSApp windows]) {
        if ([NSStringFromClass([candidate class]) isEqualToString:@"WailsWindow"]) {
            return candidate;
        }
    }
    return nil;
}

static int gedaMoveWindowOnMain(int x, int y) {
    NSWindow *window = gedaMainWindow();
    NSArray<NSScreen *> *screens = [NSScreen screens];
    if (window == nil || [screens count] == 0) {
        return 0;
    }

    // Back from the global top-left origin space into AppKit's bottom-left one,
    // pivoting on the primary display's height. The flip needs the window's
    // height, which is read from the window rather than taken as an argument so
    // that it is the height the framework actually applied.
    CGFloat primaryHeight = [[screens objectAtIndex:0] frame].size.height;
    CGFloat windowHeight = [window frame].size.height;

    [window setFrameOrigin:NSMakePoint(x, primaryHeight - y - windowHeight)];
    return 1;
}

int gedaMoveWindow(int x, int y) {
    if ([NSThread isMainThread]) {
        return gedaMoveWindowOnMain(x, y);
    }

    // The popup is shown from a goroutine -- the hotkey handler or the tray
    // click -- so hop to the main thread, which is the only one allowed to
    // touch a window. dispatch_sync is safe here because the main thread is
    // running the AppKit event loop and never waits on this one.
    __block int moved = 0;
    dispatch_sync(dispatch_get_main_queue(), ^{
        moved = gedaMoveWindowOnMain(x, y);
    });
    return moved;
}

// The frosted material Wails installs is an NSVisualEffectView filling the
// whole content view, which is what makes an otherwise transparent region of
// the page show up as a grey slab. Shrinking that view to the panel is what
// makes the preview gutter genuinely see-through.
static void gedaSetPanelInsetOnMain(int left, int radius) {
    NSWindow *window = gedaMainWindow();
    if (window == nil) {
        return;
    }

    NSView *content = [window contentView];
    for (NSView *view in [content subviews]) {
        if (![view isKindOfClass:[NSVisualEffectView class]]) {
            continue;
        }

        NSRect bounds = [content bounds];
        CGFloat width = bounds.size.width - left;
        if (width < 0) {
            width = 0;
        }
        // Left margin fixed, size following the window: the mask Wails set
        // keeps this frame correct across the resize into the settings view.
        [view setFrame:NSMakeRect(left, 0, width, bounds.size.height)];

        [view setWantsLayer:YES];
        [[view layer] setCornerRadius:radius];
        [[view layer] setMasksToBounds:YES];
    }

    // The window shadow is derived from the opaque parts of the window, so it
    // has to be recomputed or it keeps the old, full-width shape.
    [window invalidateShadow];
}

void gedaSetPanelInset(int left, int radius) {
    if ([NSThread isMainThread]) {
        gedaSetPanelInsetOnMain(left, radius);
        return;
    }
    dispatch_sync(dispatch_get_main_queue(), ^{
        gedaSetPanelInsetOnMain(left, radius);
    });
}
