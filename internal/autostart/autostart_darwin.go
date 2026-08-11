//go:build darwin

package autostart

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework ServiceManagement
#include <stdlib.h>
#include "autostart_darwin.h"
*/
import "C"

import (
	"errors"
	"unsafe"
)

func set(enabled bool) error {
	flag := C.int(0)
	if enabled {
		flag = 1
	}

	var cErr *C.char
	if C.gedaLoginItemSet(flag, &cErr) == 0 {
		return nil
	}

	msg := "unknown error"
	if cErr != nil {
		msg = C.GoString(cErr)
		C.free(unsafe.Pointer(cErr))
	}

	// Unregistering a service that was never registered reports an error.
	// Asking for off and already being off is what the caller wanted, so do
	// not hand them a failure for it.
	if !enabled && !isEnabled() {
		return nil
	}
	return errors.New(msg)
}

func isEnabled() bool {
	return C.gedaLoginItemEnabled() == 1
}
