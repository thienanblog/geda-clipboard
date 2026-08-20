//go:build darwin && axpaste

#import <Cocoa/Cocoa.h>
#import <ApplicationServices/ApplicationServices.h>
#include <unistd.h>
#include "clipboard_darwin.h"
#include "clipboard_paste_darwin.h"

int gedaHasAccessibility(int prompt) {
    @autoreleasepool {
        NSDictionary *options = @{
            (__bridge id)kAXTrustedCheckOptionPrompt : (prompt ? @YES : @NO)
        };
        return AXIsProcessTrustedWithOptions((__bridge CFDictionaryRef)options) ? 1 : 0;
    }
}

int gedaOpenAccessibilitySettings(void) {
    @autoreleasepool {
        // Handing the URL to Launch Services rather than spawning /usr/bin/open
        // keeps this working inside the App Sandbox. The query string is the
        // anchor System Settings uses to select the Accessibility list; without
        // it the user lands on Privacy & Security and has to find it.
        NSURL *url = [NSURL URLWithString:@"x-apple.systempreferences:"
                                          @"com.apple.preference.security?"
                                          @"Privacy_Accessibility"];
        return [[NSWorkspace sharedWorkspace] openURL:url] ? 1 : 0;
    }
}

// sendPasteKeystroke synthesises Cmd+V at the HID level so the frontmost app
// receives it as an ordinary paste.
static void sendPasteKeystroke(void) {
    const CGKeyCode kVK_ANSI_V = 0x09;

    CGEventSourceRef source = CGEventSourceCreate(kCGEventSourceStateCombinedSessionState);

    CGEventRef keyDown = CGEventCreateKeyboardEvent(source, kVK_ANSI_V, true);
    CGEventRef keyUp = CGEventCreateKeyboardEvent(source, kVK_ANSI_V, false);

    CGEventSetFlags(keyDown, kCGEventFlagMaskCommand);
    CGEventSetFlags(keyUp, kCGEventFlagMaskCommand);

    CGEventPost(kCGHIDEventTap, keyDown);
    CGEventPost(kCGHIDEventTap, keyUp);

    if (keyDown != NULL) CFRelease(keyDown);
    if (keyUp != NULL) CFRelease(keyUp);
    if (source != NULL) CFRelease(source);
}

int gedaPaste(void) {
    @autoreleasepool {
        if (!AXIsProcessTrusted()) {
            return -1; // caller surfaces the permission requirement
        }

        if (gedaActivateRemembered()) {
            // The target reports itself active a moment before it is ready to
            // take key events; sending the keystroke too early loses it.
            usleep(40 * 1000);
        }

        sendPasteKeystroke();
        return 0;
    }
}
