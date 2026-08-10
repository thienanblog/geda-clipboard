//go:build windows

package window

import (
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32 = windows.NewLazySystemDLL("user32.dll")

	procEnumWindows              = user32.NewProc("EnumWindows")
	procGetClassNameW            = user32.NewProc("GetClassNameW")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procIsWindow                 = user32.NewProc("IsWindow")
	procSetWindowPos             = user32.NewProc("SetWindowPos")
)

// formClass is the window class that winc -- the toolkit Wails uses on Windows
// -- registers for its top-level form. Matching on it tells the application's
// own window apart from the WebView2 and message-only windows in the same
// process, and unlike a title it is not user-visible and so will not drift.
const formClass = "winc_Form"

var (
	// mu guards both the cached handle and the search state the enumeration
	// callback writes to. EnumWindows runs the callback synchronously, so
	// holding mu across the call is enough to keep the state to one search.
	mu      sync.Mutex
	cached  uintptr
	found   uintptr
	wantPID uint32
)

// enumCallback is created once and reused: every syscall.NewCallback consumes a
// process-wide slot that is never released, so building one per search would
// eventually exhaust them.
var enumCallback = syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
	const keepGoing, stop = 1, 0

	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid != wantPID {
		return keepGoing
	}

	// The class name is bounded; a longer one cannot be ours.
	buf := make([]uint16, 64)
	n, _, _ := procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 || windows.UTF16ToString(buf[:n]) != formClass {
		return keepGoing
	}

	found = hwnd
	return stop
})

// mainWindow finds the application's top-level window, remembering it so the
// enumeration only happens once per window.
func mainWindow() uintptr {
	mu.Lock()
	defer mu.Unlock()

	if cached != 0 {
		if alive, _, _ := procIsWindow.Call(cached); alive != 0 {
			return cached
		}
		cached = 0
	}

	found = 0
	wantPID = windows.GetCurrentProcessId()
	procEnumWindows.Call(enumCallback, 0)

	cached = found
	return cached
}

func moveTo(x, y int) bool {
	hwnd := mainWindow()
	if hwnd == 0 {
		return false
	}

	const (
		hwndTop       = 0
		swpNoSize     = 0x0001
		swpNoZOrder   = 0x0004
		swpNoActivate = 0x0010
	)

	// A monitor left of or above the primary one has negative virtual screen
	// coordinates; the conversion keeps them intact in the low 32 bits, which
	// is where SetWindowPos reads its int arguments from. The size arguments
	// are ignored under SWP_NOSIZE.
	ret, _, _ := procSetWindowPos.Call(
		hwnd,
		hwndTop,
		uintptr(uint32(int32(x))),
		uintptr(uint32(int32(y))),
		0,
		0,
		swpNoSize|swpNoZOrder|swpNoActivate,
	)
	return ret != 0
}
