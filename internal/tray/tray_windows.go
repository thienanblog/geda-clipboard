//go:build windows

package tray

import (
	"sync"
	"unsafe"

	"github.com/energye/systray"
	"golang.org/x/sys/windows"
)

var (
	user32 = windows.NewLazySystemDLL("user32.dll")

	procGetCursorPos          = user32.NewProc("GetCursorPos")
	procGetSystemMetrics      = user32.NewProc("GetSystemMetrics")
	procSystemParametersInfoW = user32.NewProc("SystemParametersInfoW")
)

var (
	startOnce sync.Once
	stopFn    func()
)

// start creates the notification-area icon. systray owns its own Win32 message
// loop on a dedicated thread, so unlike macOS there is no conflict with the
// Wails event loop and no need for a hand-written shell icon.
func start(icon []byte, tooltip string) error {
	var err error
	startOnce.Do(func() {
		ready := func() {
			if len(icon) > 0 {
				systray.SetIcon(icon)
			}
			if tooltip != "" {
				systray.SetTooltip(tooltip)
			}
			systray.SetOnClick(func(menu systray.IMenu) { fireLeft(currentAnchor()) })
			systray.SetOnRClick(func(menu systray.IMenu) { fireRight(currentAnchor()) })
		}

		var startFn func()
		startFn, stopFn = systray.RunWithExternalLoop(ready, func() {})
		startFn()
	})
	return err
}

func stop() {
	if stopFn != nil {
		stopFn()
	}
}

// Exists reports whether the tray icon has been created.
func Exists() bool { return stopFn != nil }

type point struct {
	X, Y int32
}

type rect struct {
	Left, Top, Right, Bottom int32
}

// currentAnchor approximates the icon's position using the cursor, which is
// over the icon at click time. Windows offers no supported way to query a
// notification icon's rectangle, and the taskbar can sit on any edge, so the
// work area is used to decide which way the popup should hang.
func currentAnchor() Anchor {
	const (
		smCXScreen        = 0
		smCYScreen        = 1
		spiGetWorkArea    = 0x0030
		assumedIconExtent = 24
	)

	var cursor point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&cursor)))

	var work rect
	if ret, _, _ := procSystemParametersInfoW.Call(
		spiGetWorkArea, 0, uintptr(unsafe.Pointer(&work)), 0,
	); ret == 0 {
		// Fall back to the full screen if the work area is unavailable.
		w, _, _ := procGetSystemMetrics.Call(smCXScreen)
		h, _, _ := procGetSystemMetrics.Call(smCYScreen)
		work = rect{0, 0, int32(w), int32(h)}
	}

	workW := int(work.Right - work.Left)
	workH := int(work.Bottom - work.Top)

	// Position relative to the work area, matching the macOS convention.
	iconX := int(cursor.X-work.Left) - assumedIconExtent/2
	iconY := int(cursor.Y - work.Top)

	// A taskbar at the bottom puts the cursor below the work area, so the popup
	// should hang upwards; PopupPosition clamps it into range either way.
	return Anchor{
		Icon: Rect{X: iconX, Y: iconY, W: assumedIconExtent, H: 0},
		Work: Rect{W: workW, H: workH},
	}
}
