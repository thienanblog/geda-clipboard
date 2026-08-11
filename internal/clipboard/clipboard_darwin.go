//go:build darwin

package clipboard

/*
#cgo CFLAGS: -x objective-c -Wno-deprecated-declarations
#cgo LDFLAGS: -framework Cocoa -framework ApplicationServices -framework CoreGraphics
#include <stdlib.h>
#include "clipboard_darwin.h"
*/
import "C"

import (
	"errors"
	"unsafe"
)

func changeCount() int64 {
	return int64(C.gedaChangeCount())
}

func read() (Snapshot, error) {
	var (
		kind      C.int
		text      *C.char
		img       unsafe.Pointer
		imgLen    C.int
		concealed C.int
		transient C.int
		remote    C.int
	)

	C.gedaRead(&kind, &text, &img, &imgLen, &concealed, &transient, &remote)

	snap := Snapshot{
		Concealed: concealed != 0,
		Transient: transient != 0,
		Remote:    remote != 0,
	}

	switch kind {
	case 1:
		if text != nil {
			snap.Kind = KindText
			snap.Text = C.GoString(text)
			C.free(unsafe.Pointer(text))
		}
	case 2:
		if img != nil {
			snap.Kind = KindImage
			snap.Image = C.GoBytes(img, imgLen)
			C.free(img)
		}
	}

	return snap, nil
}

func writeText(s string) (int64, error) {
	cs := C.CString(s)
	defer C.free(unsafe.Pointer(cs))
	return int64(C.gedaWriteText(cs)), nil
}

func writeImage(png []byte) (int64, error) {
	if len(png) == 0 {
		return 0, errors.New("empty image")
	}
	buf := C.CBytes(png)
	defer C.free(buf)
	return int64(C.gedaWriteImage(buf, C.int(len(png)))), nil
}

func frontmost() App {
	var name, bundleID *C.char
	C.gedaFrontmost(&name, &bundleID)

	var app App
	if name != nil {
		app.Name = C.GoString(name)
		C.free(unsafe.Pointer(name))
	}
	if bundleID != nil {
		app.BundleID = C.GoString(bundleID)
		C.free(unsafe.Pointer(bundleID))
	}
	return app
}

func appIconPNG(bundleID string, px int) []byte {
	if bundleID == "" {
		return nil
	}
	cs := C.CString(bundleID)
	defer C.free(unsafe.Pointer(cs))

	var outLen C.int
	buf := C.gedaAppIconPNG(cs, C.int(px), &outLen)
	if buf == nil || outLen <= 0 {
		return nil
	}
	defer C.free(buf)
	return C.GoBytes(buf, outLen)
}

func rememberFrontmost() {
	C.gedaRememberFrontmost()
}

// ErrNoPastePermission indicates macOS Accessibility permission is missing, so
// the app cannot synthesise the paste keystroke.
var ErrNoPastePermission = errors.New("accessibility permission required to paste")

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
