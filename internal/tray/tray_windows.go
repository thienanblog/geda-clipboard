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
	procMonitorFromPoint      = user32.NewProc("MonitorFromPoint")
	procGetMonitorInfoW       = user32.NewProc("GetMonitorInfoW")
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

type monitorInfo struct {
	CbSize    uint32
	RcMonitor rect
	RcWork    rect
	DwFlags   uint32
}

// workAreaAt returns the usable area of the monitor holding p, in virtual
// screen coordinates. Falling back to SPI_GETWORKAREA gives the primary
// monitor, which is the best guess available when the monitor cannot be
// identified.
func workAreaAt(p point) rect {
	const (
		monitorDefaultToNearest = 0x0002
		smCXScreen              = 0
		smCYScreen              = 1
		spiGetWorkArea          = 0x0030
	)

	// A POINT is two int32s, which the ABI passes by value in a single
	// register. Convert through uint32 so a monitor left of or above the
	// primary one -- where the coordinates are negative -- is not sign-extended
	// into the other half of the word.
	packed := uintptr(uint32(p.X)) | uintptr(uint32(p.Y))<<32
	if monitor, _, _ := procMonitorFromPoint.Call(packed, monitorDefaultToNearest); monitor != 0 {
		info := monitorInfo{CbSize: uint32(unsafe.Sizeof(monitorInfo{}))}
		if ret, _, _ := procGetMonitorInfoW.Call(monitor, uintptr(unsafe.Pointer(&info))); ret != 0 {
			return info.RcWork
		}
	}

	var work rect
	if ret, _, _ := procSystemParametersInfoW.Call(
		spiGetWorkArea, 0, uintptr(unsafe.Pointer(&work)), 0,
	); ret != 0 {
		return work
	}

	// No work area at all: fall back to the full primary screen.
	w, _, _ := procGetSystemMetrics.Call(smCXScreen)
	h, _, _ := procGetSystemMetrics.Call(smCYScreen)
	return rect{0, 0, int32(w), int32(h)}
}

// cursorAnchor reports the pointer position relative to the work area of the
// monitor holding it, matching the macOS convention, plus that work area in
// virtual screen coordinates.
func cursorAnchor() (Anchor, bool) {
	var cursor point
	if ret, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&cursor))); ret == 0 {
		return Anchor{}, false
	}

	work := workAreaAt(cursor)

	return Anchor{
		Icon: Rect{X: int(cursor.X - work.Left), Y: int(cursor.Y - work.Top)},
		Work: Rect{
			X: int(work.Left),
			Y: int(work.Top),
			W: int(work.Right - work.Left),
			H: int(work.Bottom - work.Top),
		},
	}, true
}

// currentAnchor approximates the icon's position using the cursor, which is
// over the icon at click time. Windows offers no supported way to query a
// notification icon's rectangle, and the taskbar can sit on any edge, so the
// work area is used to decide which way the popup should hang.
func currentAnchor() Anchor {
	const assumedIconExtent = 24

	a, ok := cursorAnchor()
	if !ok {
		// A zero work area is the caller's signal that there is no anchor.
		return Anchor{}
	}

	// Widen the pointer into a notional icon so PopupPosition has something to
	// centre on. A taskbar at the bottom puts the cursor below the work area,
	// so the popup should hang upwards; PopupPosition clamps it either way.
	a.Icon.X -= assumedIconExtent / 2
	a.Icon.W = assumedIconExtent
	return a
}
