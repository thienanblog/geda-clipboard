//go:build darwin

package clipboard

/*
#cgo CFLAGS: -x objective-c -Wno-deprecated-declarations
#cgo LDFLAGS: -framework Cocoa -framework CoreGraphics
#include <stdlib.h>
#include "clipboard_darwin.h"
*/
import "C"

import (
	"encoding/json"
	"errors"
	"fmt"
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
		filesJSON *C.char
		concealed C.int
		transient C.int
		remote    C.int
		pending   C.int
	)

	C.gedaRead(&kind, &text, &img, &imgLen, &filesJSON, &concealed, &transient, &remote, &pending)

	snap := Snapshot{
		Concealed: concealed != 0,
		Transient: transient != 0,
		Remote:    remote != 0,
		Pending:   pending != 0,
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
	case 3:
		if filesJSON != nil {
			raw := C.GoString(filesJSON)
			C.free(unsafe.Pointer(filesJSON))
			if err := json.Unmarshal([]byte(raw), &snap.Files); err != nil {
				return Snapshot{}, fmt.Errorf("decode file clipboard: %w", err)
			}
			if len(snap.Files) > 0 {
				snap.Kind = KindFile
			}
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

func writeFiles(paths []string) (int64, error) {
	if len(paths) == 0 {
		return 0, errors.New("empty file list")
	}
	raw, err := json.Marshal(paths)
	if err != nil {
		return 0, fmt.Errorf("encode file clipboard: %w", err)
	}
	cs := C.CString(string(raw))
	defer C.free(unsafe.Pointer(cs))
	change := int64(C.gedaWriteFiles(cs))
	if change == 0 {
		return 0, errors.New("write file clipboard")
	}
	return change, nil
}

func startFileAccess(path, bookmark string) (string, func(), error) {
	if bookmark == "" {
		return path, func() {}, nil
	}
	encoded := C.CString(bookmark)
	defer C.free(unsafe.Pointer(encoded))

	var resolved, message *C.char
	token := C.gedaStartFileAccess(encoded, &resolved, &message)
	if message != nil {
		defer C.free(unsafe.Pointer(message))
	}
	if token == nil {
		if message != nil {
			return "", func() {}, errors.New(C.GoString(message))
		}
		return "", func() {}, errors.New("restore security-scoped file access")
	}
	if resolved == nil {
		C.gedaStopFileAccess(token)
		return "", func() {}, errors.New("security-scoped bookmark has no path")
	}
	resolvedPath := C.GoString(resolved)
	C.free(unsafe.Pointer(resolved))
	return resolvedPath, func() { C.gedaStopFileAccess(token) }, nil
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

func canBeginFocusReturn() bool {
	return C.gedaCanBeginFocusReturn() != 0
}

func mouseDownCount() uint64 {
	return uint64(C.gedaMouseDownCount())
}

func canRestoreFocus() bool {
	return C.gedaCanRestoreFocus() != 0
}

func restoreFocus() bool {
	return C.gedaActivateRemembered() != 0
}
