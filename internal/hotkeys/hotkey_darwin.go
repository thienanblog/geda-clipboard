//go:build darwin

package hotkeys

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Carbon -framework Cocoa
#include "hotkey_darwin.h"
*/
import "C"

import (
	"fmt"
	"sync"
)

// Modifier is a Carbon modifier mask, and Key a macOS virtual keycode. Both
// are the values RegisterEventHotKey expects, which is why they are declared
// per platform: a Windows virtual-key code is a different number for the same
// key.
type (
	Modifier uint32
	Key      uint32
)

// Carbon modifier masks; see HIToolbox's Events.h.
const (
	modCmd     Modifier = 0x0100
	modShift   Modifier = 0x0200
	modOption  Modifier = 0x0800
	modControl Modifier = 0x1000
)

func lookupModifier(name string) (Modifier, bool) {
	switch name {
	case "cmd", "command", "meta", "super", "win":
		return modCmd, true
	case "ctrl", "control":
		return modControl, true
	case "alt", "option", "opt":
		return modOption, true
	case "shift":
		return modShift, true
	}
	return 0, false
}

func lookupKey(name string) (Key, bool) {
	key, ok := keyByName[name]
	return key, ok
}

// macOS virtual keycodes; see HIToolbox's Events.h. They describe a physical
// position rather than a character, so they do not change with the layout.
var keyByName = map[string]Key{
	"a": 0x00, "b": 0x0B, "c": 0x08, "d": 0x02, "e": 0x0E, "f": 0x03,
	"g": 0x05, "h": 0x04, "i": 0x22, "j": 0x26, "k": 0x28, "l": 0x25,
	"m": 0x2E, "n": 0x2D, "o": 0x1F, "p": 0x23, "q": 0x0C, "r": 0x0F,
	"s": 0x01, "t": 0x11, "u": 0x20, "v": 0x09, "w": 0x0D, "x": 0x07,
	"y": 0x10, "z": 0x06,

	"0": 0x1D, "1": 0x12, "2": 0x13, "3": 0x14, "4": 0x15,
	"5": 0x17, "6": 0x16, "7": 0x1A, "8": 0x1C, "9": 0x19,

	"f1": 0x7A, "f2": 0x78, "f3": 0x63, "f4": 0x76, "f5": 0x60, "f6": 0x61,
	"f7": 0x62, "f8": 0x64, "f9": 0x65, "f10": 0x6D, "f11": 0x67, "f12": 0x6F,

	"space":  0x31,
	"return": 0x24,
	"enter":  0x24,
	"tab":    0x30,
	"escape": 0x35,
	"esc":    0x35,
	"delete": 0x33,
	"up":     0x7E,
	"down":   0x7D,
	"left":   0x7B,
	"right":  0x7C,
}

// Carbon identifies a registration by a 32-bit id rather than by a pointer, so
// live registrations are kept here and the callback looks its own up. The
// alternative, handing Carbon a Go pointer, is not allowed.
var (
	liveMu sync.Mutex
	live   = map[uint32]*carbonHotkey{}
	nextID uint32
)

type carbonHotkey struct {
	mods    Modifier
	key     Key
	id      uint32
	ref     C.GedaHotkeyRef
	keydown chan struct{}
}

func newPlatformRegistration(mods []Modifier, key Key) registration {
	var mask Modifier
	for _, mod := range mods {
		mask |= mod
	}
	// Buffered, and sent to without blocking: the send happens on the main
	// thread inside the Carbon handler, so a full channel must drop the press
	// rather than stall the event loop. One slot is enough for a shortcut that
	// toggles a window -- a second press arriving before the first is handled
	// would only toggle it back.
	return &carbonHotkey{mods: mask, key: key, keydown: make(chan struct{}, 1)}
}

func (h *carbonHotkey) Register() error {
	liveMu.Lock()
	nextID++
	h.id = nextID
	live[h.id] = h
	liveMu.Unlock()

	var ref C.GedaHotkeyRef
	status := C.gedaHotkeyRegister(C.uint32_t(h.id), C.uint32_t(h.key), C.uint32_t(h.mods), &ref)
	if status != 0 {
		liveMu.Lock()
		delete(live, h.id)
		liveMu.Unlock()
		if status == -9878 { // eventHotKeyExistsErr
			return fmt.Errorf("another application already uses this combination")
		}
		return fmt.Errorf("macOS refused the shortcut (OSStatus %d)", int32(status))
	}
	h.ref = ref
	return nil
}

func (h *carbonHotkey) Unregister() error {
	liveMu.Lock()
	delete(live, h.id)
	liveMu.Unlock()

	C.gedaHotkeyUnregister(h.ref)
	h.ref = nil
	return nil
}

func (h *carbonHotkey) Keydown() <-chan struct{} { return h.keydown }

//export gedaHotkeyPressed
func gedaHotkeyPressed(id C.uint32_t) {
	liveMu.Lock()
	h := live[uint32(id)]
	liveMu.Unlock()
	if h == nil {
		return // released between the press and this callback
	}
	select {
	case h.keydown <- struct{}{}:
	default:
	}
}
