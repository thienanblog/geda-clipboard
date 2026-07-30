//go:build darwin

package tray

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>
#include "tray_darwin.h"
*/
import "C"

import (
	"log"
	"unsafe"
)

// Exists reports whether the menu bar item has been created. Creation is
// asynchronous, so this may briefly report false right after Start.
func Exists() bool { return C.gedaTrayExists() != 0 }

//export gedaTrayCreateResult
func gedaTrayCreateResult(ok, hasIcon C.int) {
	if ok == 0 {
		log.Println("tray: failed to create the menu bar item")
		return
	}
	if hasIcon == 0 {
		log.Println("tray: menu bar item created, but the icon could not be decoded; showing a text fallback")
		return
	}
	log.Println("tray: menu bar item created")
}

func start(icon []byte, tooltip string) error {
	cTooltip := C.CString(tooltip)
	defer C.free(unsafe.Pointer(cTooltip))

	var iconPtr unsafe.Pointer
	if len(icon) > 0 {
		iconPtr = C.CBytes(icon)
		defer C.free(iconPtr)
	}

	C.gedaTrayCreate(iconPtr, C.int(len(icon)), cTooltip)
	return nil
}

func stop() {
	C.gedaTrayDestroy()
}

//export gedaTrayClicked
func gedaTrayClicked(isRight, iconX, iconY, iconW, iconH, workW, workH C.int) {
	a := Anchor{
		Icon: Rect{X: int(iconX), Y: int(iconY), W: int(iconW), H: int(iconH)},
		Work: Rect{W: int(workW), H: int(workH)},
	}
	// Hand off to a goroutine: this runs on the AppKit main thread and the
	// handler will want to drive Wails, which must not block the event.
	if isRight != 0 {
		go fireRight(a)
	} else {
		go fireLeft(a)
	}
}
