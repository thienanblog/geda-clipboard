//go:build darwin

package main

import _ "embed"

// macOS treats this monochrome PNG as a template image, allowing AppKit to
// recolour it for the current menu-bar appearance.
//
//go:embed build/trayicon.png
var trayIcon []byte
