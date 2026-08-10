//go:build windows

package clipboard

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	"unsafe"

	xclipboard "golang.design/x/clipboard"
	"golang.org/x/sys/windows"
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procGetClipboardSequenceNumber = user32.NewProc("GetClipboardSequenceNumber")
	procIsClipboardFormatAvailable = user32.NewProc("IsClipboardFormatAvailable")
	procRegisterClipboardFormatW   = user32.NewProc("RegisterClipboardFormatW")
	procOpenClipboard              = user32.NewProc("OpenClipboard")
	procCloseClipboard             = user32.NewProc("CloseClipboard")
	procGetClipboardData           = user32.NewProc("GetClipboardData")
	procGetForegroundWindow        = user32.NewProc("GetForegroundWindow")
	procSetForegroundWindow        = user32.NewProc("SetForegroundWindow")
	procGetWindowThreadProcessId   = user32.NewProc("GetWindowThreadProcessId")
	procSendInput                  = user32.NewProc("SendInput")
	procQueryFullProcessImageNameW = kernel32.NewProc("QueryFullProcessImageNameW")
	procGlobalLock                 = kernel32.NewProc("GlobalLock")
	procGlobalUnlock               = kernel32.NewProc("GlobalUnlock")
	procGlobalSize                 = kernel32.NewProc("GlobalSize")
	procRtlMoveMemory              = kernel32.NewProc("RtlMoveMemory")
)

// initOnce guards x/clipboard, which must be initialised before use.
var (
	initOnce sync.Once
	initErr  error
)

func ensureInit() error {
	initOnce.Do(func() { initErr = xclipboard.Init() })
	return initErr
}

func changeCount() int64 {
	// GetClipboardSequenceNumber increments on every change, including a repeat
	// copy of identical content, which is what the copy counter needs.
	n, _, _ := procGetClipboardSequenceNumber.Call()
	return int64(n)
}

// registerFormat resolves a named clipboard format to its ID, caching the
// result. Windows applications use these names to opt out of clipboard
// managers and history, the same role as the org.nspasteboard.* types on macOS.
var (
	formatMu    sync.Mutex
	formatCache = map[string]uint32{}
)

func registerFormat(name string) uint32 {
	formatMu.Lock()
	defer formatMu.Unlock()

	if id, ok := formatCache[name]; ok {
		return id
	}
	ptr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		formatCache[name] = 0
		return 0
	}
	id, _, _ := procRegisterClipboardFormatW.Call(uintptr(unsafe.Pointer(ptr)))
	formatCache[name] = uint32(id)
	return uint32(id)
}

func formatPresent(name string) bool {
	id := registerFormat(name)
	if id == 0 {
		return false
	}
	// IsClipboardFormatAvailable needs no OpenClipboard, so it cannot contend
	// with the read below.
	ret, _, _ := procIsClipboardFormatAvailable.Call(uintptr(id))
	return ret != 0
}

// optedOut reports whether the source app published name with a DWORD payload
// of 0, which is how Windows apps decline clipboard history and cloud sync.
//
// The value carries the meaning, not the presence: the same format set to 1 is
// an explicit opt *in*. An absent format means the app expressed no preference,
// which is not an opt-out either.
func optedOut(name string) bool {
	id := registerFormat(name)
	if id == 0 || !formatPresent(name) {
		return false
	}

	// GetClipboardData is only valid on the thread that opened the clipboard, so
	// pin the goroutine for the duration.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := openClipboardRetry(); err != nil {
		// Another process holds the clipboard. Treating that as "no preference"
		// would risk recording a payload the app asked us to skip, so err
		// towards excluding it.
		return true
	}
	defer procCloseClipboard.Call()

	hMem, _, _ := procGetClipboardData.Call(uintptr(id))
	if hMem == 0 {
		return false
	}
	if size, _, _ := procGlobalSize.Call(hMem); size < 4 {
		return false
	}
	ptr, _, _ := procGlobalLock.Call(hMem)
	if ptr == 0 {
		return false
	}
	defer procGlobalUnlock.Call(hMem)

	// Copy the DWORD out rather than casting the locked address to a Go
	// pointer: passing it straight back to a syscall keeps this free of the
	// uintptr-to-unsafe.Pointer conversion that go vet rightly objects to.
	var value uint32
	procRtlMoveMemory.Call(uintptr(unsafe.Pointer(&value)), ptr, unsafe.Sizeof(value))

	return value == 0
}

// openClipboardRetry opens the clipboard, retrying briefly: another process may
// hold it for a moment after a copy.
func openClipboardRetry() error {
	var lastErr error
	for i := 0; i < 5; i++ {
		ret, _, err := procOpenClipboard.Call(0)
		if ret != 0 {
			return nil
		}
		lastErr = err
		time.Sleep(10 * time.Millisecond)
	}
	return lastErr
}

func read() (Snapshot, error) {
	if err := ensureInit(); err != nil {
		return Snapshot{}, err
	}

	// ExcludeClipboardContentFromMonitorProcessing is a bare marker: publishing
	// it at all is the request. The other two carry a DWORD, where 0 means
	// "don't" and 1 means "you may", so their value has to be read.
	//
	// Snapshot.Remote stays false here: content arriving from the Windows cloud
	// clipboard reaches this process as an ordinary local copy, with no format
	// marking it as coming from another device.
	snap := Snapshot{
		Concealed: formatPresent("ExcludeClipboardContentFromMonitorProcessing"),
		Transient: optedOut("CanIncludeInClipboardHistory") ||
			optedOut("CanUploadToCloudClipboard"),
	}

	// Prefer text: on Windows a copied image usually offers no text at all,
	// while copied rich content offers both and the text is the useful part.
	if text := xclipboard.Read(xclipboard.FmtText); len(text) > 0 {
		snap.Kind = KindText
		snap.Text = string(text)
		return snap, nil
	}

	if img := xclipboard.Read(xclipboard.FmtImage); len(img) > 0 {
		snap.Kind = KindImage
		snap.Image = img
		return snap, nil
	}

	return snap, nil
}

func writeText(s string) (int64, error) {
	if err := ensureInit(); err != nil {
		return 0, err
	}
	xclipboard.Write(xclipboard.FmtText, []byte(s))
	return changeCount(), nil
}

func writeImage(png []byte) (int64, error) {
	if len(png) == 0 {
		return 0, errors.New("empty image")
	}
	if err := ensureInit(); err != nil {
		return 0, err
	}
	xclipboard.Write(xclipboard.FmtImage, png)
	return changeCount(), nil
}

// rememberedWindow is the foreground window captured before the popup opened.
var (
	rememberMu       sync.Mutex
	rememberedWindow uintptr
)

func frontmost() App {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return App{}
	}
	return appForWindow(hwnd)
}

func appForWindow(hwnd uintptr) App {
	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return App{}
	}

	const processQueryLimitedInformation = 0x1000
	handle, err := windows.OpenProcess(processQueryLimitedInformation, false, pid)
	if err != nil {
		return App{}
	}
	defer windows.CloseHandle(handle)

	buf := make([]uint16, windows.MAX_LONG_PATH)
	size := uint32(len(buf))
	ret, _, _ := procQueryFullProcessImageNameW.Call(
		uintptr(handle), 0,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if ret == 0 {
		return App{}
	}

	path := windows.UTF16ToString(buf[:size])
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	return App{Name: name, BundleID: path}
}

// appIconPNG is not implemented on Windows: extracting an executable's icon
// requires unpacking an HICON into a bitmap, which is a lot of surface area for
// a purely decorative detail. The UI omits the icon when this returns nil.
func appIconPNG(bundleID string, px int) []byte { return nil }

func rememberFrontmost() {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return
	}

	// Ignore our own window, so the target survives repeated popup toggles.
	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == uint32(windows.Getpid()) {
		return
	}

	rememberMu.Lock()
	rememberedWindow = hwnd
	rememberMu.Unlock()
}

// ErrNoPastePermission exists for API parity with macOS; Windows needs no
// permission to synthesise input, so it is never returned.
var ErrNoPastePermission = errors.New("permission required to paste")

// Virtual key codes and SendInput structures.
const (
	vkControl = 0x11
	vkV       = 0x56

	inputKeyboard     = 1
	keyEventFKeyUp    = 0x0002
	keyEventFScanCode = 0x0008
)

// keyboardInput mirrors INPUT holding its KEYBDINPUT member. The layout has to
// match the C struct exactly, because SendInput rejects the call outright when
// cbSize is not sizeof(INPUT).
//
// The union is 8-aligned on the 64-bit targets Wails builds for (MOUSEINPUT
// ends in a ULONG_PTR), so it starts at offset 8 rather than 4 and the whole
// struct is 40 bytes: type@0, wVk@8, wScan@10, dwFlags@12, time@16,
// dwExtraInfo@24, union tail padding@32. The explicit padding fields below
// reproduce that; without them Go packs the struct into 32 bytes with wVk at
// offset 4 and every SendInput call fails with ERROR_INVALID_PARAMETER.
type keyboardInput struct {
	kind uint32
	_    uint32 // union alignment
	vk   uint16
	scan uint16

	flags uint32
	time  uint32
	_     uint32 // dwExtraInfo alignment
	extra uintptr

	// Tail padding: the union is sized by MOUSEINPUT, which is larger than
	// KEYBDINPUT.
	_ [8]byte
}

// Compile-time checks that the struct still matches INPUT. Getting this wrong
// is silent at runtime -- SendInput simply refuses the call -- so fail the
// build instead. Any deviation makes one of these constants overflow.
//
// The size alone is not enough: dropping the first padding field happens to
// leave the total at 40 while shifting wVk to offset 4, so pin the field
// offsets too.
const (
	_ = uint(unsafe.Sizeof(keyboardInput{}) - 40)
	_ = uint(unsafe.Offsetof(keyboardInput{}.vk) - 8)
	_ = uint(unsafe.Offsetof(keyboardInput{}.scan) - 10)
	_ = uint(unsafe.Offsetof(keyboardInput{}.flags) - 12)
	_ = uint(unsafe.Offsetof(keyboardInput{}.time) - 16)
	_ = uint(unsafe.Offsetof(keyboardInput{}.extra) - 24)
)

func sendKey(vk uint16, keyUp bool) bool {
	in := keyboardInput{kind: inputKeyboard, vk: vk}
	if keyUp {
		in.flags = keyEventFKeyUp
	}
	sent, _, _ := procSendInput.Call(1, uintptr(unsafe.Pointer(&in)), unsafe.Sizeof(in))
	return sent == 1
}

func paste() error {
	rememberMu.Lock()
	target := rememberedWindow
	rememberMu.Unlock()

	if target != 0 {
		procSetForegroundWindow.Call(target)

		// Activation is asynchronous, and Windows refuses SetForegroundWindow
		// outright in several cases (foreground lock timeout, minimised target).
		// Sending the keystroke before the target is actually foreground would
		// paste into whatever window still has focus, so wait for it.
		if !waitForForeground(target, 400*time.Millisecond) {
			return fmt.Errorf("could not focus the target window")
		}
	}

	// Ctrl down, V down, V up, Ctrl up.
	ok := sendKey(vkControl, false)
	ok = sendKey(vkV, false) && ok
	ok = sendKey(vkV, true) && ok
	ok = sendKey(vkControl, true) && ok
	if !ok {
		return fmt.Errorf("the paste keystroke was blocked")
	}
	return nil
}

// waitForForeground polls until hwnd is the foreground window or timeout
// elapses, reporting whether it got there.
func waitForForeground(hwnd uintptr, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if current, _, _ := procGetForegroundWindow.Call(); current == hwnd {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func hasPastePermission(prompt bool) bool { return true }
