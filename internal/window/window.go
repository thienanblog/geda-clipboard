// Package window places the application's own window in the global coordinate
// space that spans every display.
//
// It exists because Wails' WindowSetPosition takes coordinates relative to the
// screen the window currently occupies, which is exactly the wrong frame of
// reference for a popup that has to open wherever the pointer or the tray icon
// happens to be. Asking Wails to put the window at (0, 0) of another display
// simply moves it to (0, 0) of its own.
package window

// MoveTo places the window's top-left corner at the given global position. It
// reports false when the platform cannot locate the window, which is the
// caller's cue to fall back to the framework's screen-relative positioning.
//
// The window is moved, never resized. Its size is the framework's business:
// Wails scales a logical size by the display's DPI on Windows, so a size
// passed through here in logical pixels would be applied as physical ones and
// shrink the window on any scaled display.
func MoveTo(x, y int) bool { return moveTo(x, y) }

// Focus asks the native window to hand keyboard focus to its WebView.
//
// Wails normally does this as part of WindowShow. On Windows, however, the
// first show of a StartHidden WebView2 window can focus only the top-level
// frame. Posting another focus message after the show has been queued makes
// Wails call WebView2's programmatic MoveFocus path once the window is visible.
func Focus() { focus() }

// IsForeground reports whether the application window is still the OS
// foreground window. It distinguishes a transient WebView blur during focus
// hand-off from an actual click into another application.
func IsForeground() bool { return isForeground() }

// SetPanelInset says how much of the window, on each side, is transparent
// surround rather than panel, and how far the panel's own corners are rounded.
// The surround is the preview gutter on the left plus the margin every side
// keeps for the panel's drop shadow.
//
// macOS draws the frosted window material behind the entire window, so without
// this the surround shows up as a grey slab exactly where the popup is meant to
// be see-through. Platforms that have no window-wide material do nothing.
func SetPanelInset(left, top, right, bottom, radius int) {
	setPanelInset(left, top, right, bottom, radius)
}

// SetDockIconVisible shows or hides the app's entry in the Dock.
//
// A menu bar app is reached through its tray icon and its shortcut, so the Dock
// tile only costs a slot. Hiding it is a live switch rather than a relaunch,
// and it does not stop the popup taking keyboard focus. Platforms without a
// Dock do nothing.
func SetDockIconVisible(visible bool) { setDockIconVisible(visible) }
