//go:build windows

package tray

import (
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	testFindWindowExW            = user32.NewProc("FindWindowExW")
	testGetWindowThreadProcessID = user32.NewProc("GetWindowThreadProcessId")
	testPostMessageW             = user32.NewProc("PostMessageW")
)

type clickRecorder struct {
	left  chan struct{}
	right chan struct{}
}

func (r *clickRecorder) OnLeftClick(Anchor)  { r.left <- struct{}{} }
func (r *clickRecorder) OnRightClick(Anchor) { r.right <- struct{}{} }

func TestWindowsTrayDispatchesMouseClicks(t *testing.T) {
	recorder := &clickRecorder{
		left:  make(chan struct{}, 1),
		right: make(chan struct{}, 1),
	}
	if err := Start(recorder, nil, "Geda Clipboard test"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(Stop)

	waitForTrayReady(t)
	hwnd := waitForOwnTrayWindow(t)

	const (
		wmUser      = 0x0400
		wmLButtonUp = 0x0202
		wmRButtonUp = 0x0205
	)

	postTrayMessage(t, hwnd, wmUser+1, wmLButtonUp)
	waitForClick(t, recorder.left, "left")

	postTrayMessage(t, hwnd, wmUser+1, wmRButtonUp)
	waitForClick(t, recorder.right, "right")
}

func waitForTrayReady(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if Exists() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("tray did not become ready")
}

func waitForOwnTrayWindow(t *testing.T) uintptr {
	t.Helper()

	className, err := windows.UTF16PtrFromString("SystrayClass")
	if err != nil {
		t.Fatalf("encode tray window class: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		var after uintptr
		for {
			hwnd, _, _ := testFindWindowExW.Call(
				0,
				after,
				uintptr(unsafe.Pointer(className)),
				0,
			)
			if hwnd == 0 {
				break
			}

			var pid uint32
			testGetWindowThreadProcessID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
			if pid == windows.GetCurrentProcessId() {
				return hwnd
			}
			after = hwnd
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal("tray window was not created")
	return 0
}

func postTrayMessage(t *testing.T, hwnd uintptr, message, mouseEvent uintptr) {
	t.Helper()
	if ok, _, err := testPostMessageW.Call(hwnd, message, 0, mouseEvent); ok == 0 {
		t.Fatalf("PostMessageW(%#x): %v", mouseEvent, err)
	}
}

func waitForClick(t *testing.T, clicked <-chan struct{}, button string) {
	t.Helper()
	select {
	case <-clicked:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("%s-click callback was not dispatched", button)
	}
}
