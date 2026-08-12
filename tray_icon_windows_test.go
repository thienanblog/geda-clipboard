//go:build windows

package main

import (
	"bytes"
	"testing"
)

func TestWindowsTrayIconIsICO(t *testing.T) {
	// ICONDIR begins with reserved=0, type=1. LoadImage(IMAGE_ICON) rejects the
	// PNG signature previously embedded here, but systray only logs that error.
	want := []byte{0, 0, 1, 0}
	if len(trayIcon) < len(want) || !bytes.Equal(trayIcon[:len(want)], want) {
		t.Fatalf("tray icon header = % x, want ICO header % x", trayIcon[:min(len(trayIcon), len(want))], want)
	}
}
