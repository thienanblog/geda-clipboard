//go:build windows

package main

import _ "embed"

// The Windows systray implementation loads IMAGE_ICON through Win32, which
// only accepts ICO data. Passing the macOS template PNG leaves the notification
// icon without an image while the rest of the tray process keeps running.
//
//go:embed build/windows/icon.ico
var trayIcon []byte
