#import <Foundation/Foundation.h>
#import <UserNotifications/UserNotifications.h>
#include "notify_darwin.h"

// UNUserNotificationCenter throws if the process is not a bundled app, so every
// entry point checks for a bundle identifier first. Running unbundled (as
// `wails dev` does) falls back to osascript on the Go side.
int gedaNotifyBundled(void) {
    @autoreleasepool {
        NSString *bid = [[NSBundle mainBundle] bundleIdentifier];
        return (bid != nil && [bid length] > 0) ? 1 : 0;
    }
}

void gedaNotifyRequestPermission(void) {
    @autoreleasepool {
        if (!gedaNotifyBundled()) {
            return;
        }
        if (@available(macOS 10.14, *)) {
            UNUserNotificationCenter *center = [UNUserNotificationCenter currentNotificationCenter];
            [center requestAuthorizationWithOptions:(UNAuthorizationOptionAlert |
                                                     UNAuthorizationOptionSound)
                                 completionHandler:^(BOOL granted, NSError *error) {
                (void)granted;
                (void)error;
            }];
        }
    }
}

int gedaNotifyAuthStatus(void) {
    @autoreleasepool {
        if (!gedaNotifyBundled()) {
            return -1;
        }
        if (@available(macOS 10.14, *)) {
            // getNotificationSettings is asynchronous and its handler runs on an
            // internal queue, so a bounded semaphore wait is safe here.
            __block int status = -1;
            dispatch_semaphore_t done = dispatch_semaphore_create(0);

            [[UNUserNotificationCenter currentNotificationCenter]
                getNotificationSettingsWithCompletionHandler:^(UNNotificationSettings *settings) {
                    status = (int)settings.authorizationStatus;
                    dispatch_semaphore_signal(done);
                }];

            dispatch_semaphore_wait(done,
                dispatch_time(DISPATCH_TIME_NOW, (int64_t)(2 * NSEC_PER_SEC)));
            return status;
        }
        return -1;
    }
}

int gedaNotifySend(const char *title, const char *subtitle, const char *body) {
    @autoreleasepool {
        if (!gedaNotifyBundled()) {
            return -1;
        }

        if (@available(macOS 10.14, *)) {
            UNMutableNotificationContent *content = [[UNMutableNotificationContent alloc] init];
            if (title != NULL) {
                content.title = [NSString stringWithUTF8String:title];
            }
            if (subtitle != NULL && strlen(subtitle) > 0) {
                content.subtitle = [NSString stringWithUTF8String:subtitle];
            }
            if (body != NULL && strlen(body) > 0) {
                content.body = [NSString stringWithUTF8String:body];
            }
            // Clipboard events are frequent; a sound each time would be hostile.
            content.sound = nil;

            UNNotificationRequest *request =
                [UNNotificationRequest requestWithIdentifier:[[NSUUID UUID] UUIDString]
                                                    content:content
                                                    trigger:nil];

            [[UNUserNotificationCenter currentNotificationCenter]
                addNotificationRequest:request
                 withCompletionHandler:^(NSError *error) { (void)error; }];

            [content release];
            return 0;
        }

        // Pre-10.14: caller falls back to osascript.
        return -1;
    }
}
