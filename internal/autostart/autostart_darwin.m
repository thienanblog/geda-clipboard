#import <Foundation/Foundation.h>
#import <ServiceManagement/ServiceManagement.h>
#include <stdlib.h>
#include <string.h>

#include "autostart_darwin.h"

// SMAppService replaces the LaunchAgent plist this package used to write.
// Two reasons, and the second is the one that forced it: the sandbox denies
// writes to ~/Library/LaunchAgents outright, and the App Store forbids
// installing anything outside the bundle regardless. SMAppService registers
// the bundle by its own identity, so it also survives the app being renamed
// or moved, which the plist never did.
//
// It is macOS 13 and later. Below that the app still runs; launch at login is
// simply unavailable, and says so rather than failing silently.

static char *copyErrorMessage(NSError *error) {
    NSString *message = error != nil ? [error localizedDescription] : nil;
    if (message == nil) {
        message = @"unknown error";
    }
    const char *utf8 = [message UTF8String];
    return utf8 != NULL ? strdup(utf8) : NULL;
}

int gedaLoginItemSet(int enabled, char **errOut) {
    @autoreleasepool {
        if (errOut != NULL) {
            *errOut = NULL;
        }

        if (@available(macOS 13.0, *)) {
            SMAppService *service = [SMAppService mainAppService];
            NSError *error = nil;
            BOOL ok = enabled ? [service registerAndReturnError:&error]
                              : [service unregisterAndReturnError:&error];
            if (ok) {
                return 0;
            }
            if (errOut != NULL) {
                *errOut = copyErrorMessage(error);
            }
            return -1;
        }

        if (errOut != NULL) {
            *errOut = strdup("launch at login requires macOS 13 or later");
        }
        return -1;
    }
}

int gedaLoginItemEnabled(void) {
    @autoreleasepool {
        if (@available(macOS 13.0, *)) {
            return [[SMAppService mainAppService] status] == SMAppServiceStatusEnabled ? 1 : 0;
        }
        return 0;
    }
}
