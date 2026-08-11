//go:build darwin && sparkle

// The constraint above is not decoration. The _darwin suffix keeps this file
// off Windows, but only the tag keeps it out of the App Store build, and
// without it the toolchain offers this .m to a package that is not using cgo
// and refuses to build at all.

#import <Foundation/Foundation.h>
#import <Sparkle/Sparkle.h>

#include "updater_sparkle_darwin.h"

// SPUStandardUpdaterController is Sparkle's batteries-included entry point: it
// owns the updater, the scheduled checks and the user-facing windows. A menu
// bar app has no menu to wire it into with a nib, so it is held here and
// driven from Go.
//
// The feed URL, the public EdDSA key and SUEnableInstallerLauncherService live
// in the bundle's Info.plist rather than in this file. Sparkle reads them from
// there, and keeping them out of the binary means the packaging step is the
// only thing that decides where updates come from.

static SPUStandardUpdaterController *gUpdaterController = nil;

static void ensureStarted(void) {
    if (gUpdaterController != nil) {
        return;
    }
    // startingUpdater:YES begins the scheduled check cycle immediately.
    // Sparkle asks the user for permission before its first automatic check,
    // so this does not reach the network behind their back.
    gUpdaterController =
        [[SPUStandardUpdaterController alloc] initWithStartingUpdater:YES
                                                      updaterDelegate:nil
                                                   userDriverDelegate:nil];
}

void gedaUpdaterStart(void) {
    @autoreleasepool {
        // Sparkle's UI is AppKit, so it has to be built on the main thread.
        if ([NSThread isMainThread]) {
            ensureStarted();
        } else {
            dispatch_sync(dispatch_get_main_queue(), ^{
                ensureStarted();
            });
        }
    }
}

void gedaUpdaterCheckNow(void) {
    @autoreleasepool {
        dispatch_async(dispatch_get_main_queue(), ^{
            ensureStarted();
            [gUpdaterController checkForUpdates:nil];
        });
    }
}
