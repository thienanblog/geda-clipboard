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

// SetPanelInset says how much of the window, measured from its left edge, is
// transparent gutter rather than panel, and how far the panel's own corners are
// rounded.
//
// macOS draws the frosted window material behind the entire window, so without
// this the gutter shows up as a grey slab exactly where the popup is meant to
// be see-through. Platforms that have no window-wide material do nothing.
func SetPanelInset(left, radius int) { setPanelInset(left, radius) }

// SetDockIconVisible shows or hides the app's entry in the Dock.
//
// A menu bar app is reached through its tray icon and its shortcut, so the Dock
// tile only costs a slot. Hiding it is a live switch rather than a relaunch,
// and it does not stop the popup taking keyboard focus. Platforms without a
// Dock do nothing.
func SetDockIconVisible(visible bool) { setDockIconVisible(visible) }
