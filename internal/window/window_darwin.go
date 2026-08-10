//go:build darwin

package window

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#include "window_darwin.h"
*/
import "C"

func moveTo(x, y int) bool {
	return C.gedaMoveWindow(C.int(x), C.int(y)) != 0
}

func setPanelInset(left, radius int) {
	C.gedaSetPanelInset(C.int(left), C.int(radius))
}

func setDockIconVisible(visible bool) {
	var flag C.int
	if visible {
		flag = 1
	}
	C.gedaSetDockIconVisible(flag)
}
