//go:build darwin && axpaste

package clipboard

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework ApplicationServices -framework CoreGraphics
#include "clipboard_paste_darwin.h"
*/
import "C"

import "errors"

// The paste-back keystroke, compiled in only with the axpaste tag. See
// clipboard_nopaste_darwin.go for why the tag adds the feature rather than
// removing it.

func pasteSupported() bool { return true }

func paste() error {
	if rc := C.gedaPaste(); rc == -1 {
		return ErrNoPastePermission
	}
	return nil
}

func hasPastePermission(prompt bool) bool {
	p := C.int(0)
	if prompt {
		p = 1
	}
	return C.gedaHasAccessibility(p) != 0
}

func openPastePermissionSettings() error {
	if C.gedaOpenAccessibilitySettings() == 0 {
		return errors.New("could not open System Settings")
	}
	return nil
}
