#ifndef GEDA_HOTKEY_DARWIN_H
#define GEDA_HOTKEY_DARWIN_H

#include <stdint.h>

// Opaque handle to a claimed shortcut, so the Go side never has to name a
// Carbon type.
typedef void *GedaHotkeyRef;

// gedaHotkeyRegister claims a system-wide shortcut through Carbon's
// RegisterEventHotKey. keyCode is a macOS virtual keycode and modifiers is a
// Carbon modifier mask (cmdKey, shiftKey, optionKey, controlKey).
//
// Presses arrive back as gedaHotkeyPressed(id), so id must identify the
// registration on the Go side; Carbon only carries a 32-bit value through the
// event, which is why this is an id rather than a pointer.
//
// Returns 0 on success, or the non-zero Carbon OSStatus. Notably
// eventHotKeyExistsErr (-9878) means another application already owns the
// combination.
int32_t gedaHotkeyRegister(uint32_t id, uint32_t keyCode, uint32_t modifiers,
                           GedaHotkeyRef *out);

// gedaHotkeyUnregister releases a shortcut claimed by gedaHotkeyRegister.
// Passing NULL is a no-op.
void gedaHotkeyUnregister(GedaHotkeyRef ref);

#endif
