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

// AppKit already transfers focus to the WebView when Wails shows the window.
func focus() {}

// Browser blur is the established source of truth on macOS. Returning false
// preserves that path while Windows filters its WebView2 focus hand-off.
func isForeground() bool { return false }

func setPanelInset(left, top, right, bottom, radius int) {
	C.gedaSetPanelInset(C.int(left), C.int(top), C.int(right), C.int(bottom), C.int(radius))
}

func setDockIconVisible(visible bool) {
	var flag C.int
	if visible {
		flag = 1
	}
	C.gedaSetDockIconVisible(flag)
}
