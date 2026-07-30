//go:build windows

package clipboard

import (
	"errors"
	"path/filepath"
	"strings"
	"sync"
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
	procGetForegroundWindow        = user32.NewProc("GetForegroundWindow")
	procSetForegroundWindow        = user32.NewProc("SetForegroundWindow")
	procGetWindowThreadProcessId   = user32.NewProc("GetWindowThreadProcessId")
	procSendInput                  = user32.NewProc("SendInput")
	procQueryFullProcessImageNameW = kernel32.NewProc("QueryFullProcessImageNameW")
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

func read() (Snapshot, error) {
	if err := ensureInit(); err != nil {
		return Snapshot{}, err
	}

	snap := Snapshot{
		Concealed: formatPresent("ExcludeClipboardContentFromMonitorProcessing"),
		Transient: formatPresent("CanIncludeInClipboardHistory") &&
			!formatPresent("CanUploadToCloudClipboard"),
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

// keyboardInput mirrors the KEYBDINPUT half of INPUT. The union in INPUT is
// sized by the largest member (MOUSEINPUT), so the struct is padded to match.
type keyboardInput struct {
	kind    uint32
	vk      uint16
	scan    uint16
	flags   uint32
	time    uint32
	extra   uintptr
	padding [8]byte
}

func sendKey(vk uint16, keyUp bool) {
	in := keyboardInput{kind: inputKeyboard, vk: vk}
	if keyUp {
		in.flags = keyEventFKeyUp
	}
	procSendInput.Call(1, uintptr(unsafe.Pointer(&in)), unsafe.Sizeof(in))
}

func paste() error {
	rememberMu.Lock()
	target := rememberedWindow
	rememberMu.Unlock()

	if target != 0 {
		procSetForegroundWindow.Call(target)
	}

	// Ctrl down, V down, V up, Ctrl up.
	sendKey(vkControl, false)
	sendKey(vkV, false)
	sendKey(vkV, true)
	sendKey(vkControl, true)
	return nil
}

func hasPastePermission(prompt bool) bool { return true }
