//go:build darwin

package window

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#include "window_darwin.h"
*/
import "C"

func moveTo(x, y, w, h int) bool {
	return C.gedaMoveWindow(C.int(x), C.int(y), C.int(w), C.int(h)) != 0
}
