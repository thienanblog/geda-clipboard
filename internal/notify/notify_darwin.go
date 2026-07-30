//go:build darwin

package notify

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework UserNotifications
#include <stdlib.h>
#include "notify_darwin.h"
*/
import "C"

import (
	"os/exec"
	"strings"
	"unsafe"
)

func bundled() bool {
	return C.gedaNotifyBundled() != 0
}

func requestPermission() {
	C.gedaNotifyRequestPermission()
}

func permission() Status {
	switch C.gedaNotifyAuthStatus() {
	case 0:
		return StatusNotDetermined
	case 1:
		return StatusDenied
	case 2, 3, 4:
		return StatusAuthorized
	default:
		// Unbundled or pre-10.14: we fall back to osascript, which needs no
		// per-app authorisation of its own.
		return StatusUnknown
	}
}

func openSettings() error {
	return exec.Command("open",
		"x-apple.systempreferences:com.apple.preference.notifications").Run()
}

func send(n Notification) error {
	if bundled() {
		cTitle := C.CString(n.Title)
		cSubtitle := C.CString(n.Subtitle)
		cBody := C.CString(n.Body)
		defer C.free(unsafe.Pointer(cTitle))
		defer C.free(unsafe.Pointer(cSubtitle))
		defer C.free(unsafe.Pointer(cBody))

		if rc := C.gedaNotifySend(cTitle, cSubtitle, cBody); rc == 0 {
			return nil
		}
	}
	return sendViaOSAScript(n)
}

// sendViaOSAScript is the unbundled fallback (e.g. under `wails dev`). The
// notification is attributed to the script runner rather than to this app, but
// it does at least appear.
func sendViaOSAScript(n Notification) error {
	script := `display notification ` + quoteAS(n.Body) +
		` with title ` + quoteAS(n.Title)
	if n.Subtitle != "" {
		script += ` subtitle ` + quoteAS(n.Subtitle)
	}
	return exec.Command("osascript", "-e", script).Run()
}

// quoteAS renders s as an AppleScript string literal.
func quoteAS(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	// AppleScript string literals cannot contain raw newlines.
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return `"` + s + `"`
}
