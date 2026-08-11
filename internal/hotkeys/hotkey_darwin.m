#import <Carbon/Carbon.h>
#import <Cocoa/Cocoa.h>

#include "hotkey_darwin.h"

// Implemented in Go; see hotkey_darwin.go.
extern void gedaHotkeyPressed(uint32_t id);

// The application-wide Carbon handler, installed once and never removed.
// Removing it would only matter if the last shortcut were released, and the
// app registers one again as soon as the user picks a different combination.
static EventHandlerRef gHandler = NULL;

// A four-character signature identifying our registrations to Carbon. Any
// value works as long as it is ours; it pairs with the id to form the
// EventHotKeyID that comes back with each press.
static const OSType kSignature = 'geda';

static OSStatus hotkeyPressed(EventHandlerCallRef next, EventRef event,
                              void *userData) {
	EventHotKeyID hotkeyID;
	if (GetEventParameter(event, kEventParamDirectObject, typeEventHotKeyID,
	                      NULL, sizeof(hotkeyID), NULL, &hotkeyID) != noErr) {
		return eventNotHandledErr;
	}
	if (hotkeyID.signature != kSignature) {
		return eventNotHandledErr;
	}
	gedaHotkeyPressed(hotkeyID.id);
	return noErr;
}

// runOnMain serialises Carbon calls onto the main thread, which owns the
// application event target. The thread check is not a nicety: dispatch_sync to
// the main queue from the main queue deadlocks, and Wails calls into this
// package from both its startup callback and its own main thread.
static void runOnMain(void (^block)(void)) {
	if ([NSThread isMainThread]) {
		block();
		return;
	}
	dispatch_sync(dispatch_get_main_queue(), block);
}

int32_t gedaHotkeyRegister(uint32_t id, uint32_t keyCode, uint32_t modifiers,
                           GedaHotkeyRef *out) {
	__block OSStatus status = noErr;
	runOnMain(^{
		if (gHandler == NULL) {
			EventTypeSpec spec = {kEventClassKeyboard, kEventHotKeyPressed};
			status = InstallApplicationEventHandler(&hotkeyPressed, 1, &spec,
			                                       NULL, &gHandler);
			if (status != noErr) {
				gHandler = NULL;
				return;
			}
		}

		EventHotKeyID hotkeyID;
		hotkeyID.signature = kSignature;
		hotkeyID.id = id;

		EventHotKeyRef ref = NULL;
		status = RegisterEventHotKey(keyCode, modifiers, hotkeyID,
		                             GetApplicationEventTarget(), 0, &ref);
		if (status == noErr) {
			*out = (GedaHotkeyRef)ref;
		}
	});
	return (int32_t)status;
}

void gedaHotkeyUnregister(GedaHotkeyRef ref) {
	if (ref == NULL) {
		return;
	}
	runOnMain(^{
		UnregisterEventHotKey((EventHotKeyRef)ref);
	});
}
