// Package window places the application's own window in the global coordinate
// space that spans every display.
//
// It exists because Wails' WindowSetPosition takes coordinates relative to the
// screen the window currently occupies, which is exactly the wrong frame of
// reference for a popup that has to open wherever the pointer or the tray icon
// happens to be. Asking Wails to put the window at (0, 0) of another display
// simply moves it to (0, 0) of its own.
package window

// MoveTo places the window's top-left corner at the given global position and
// sizes it to w by h, in logical pixels. It reports false when the platform
// cannot locate the window, which is the caller's cue to fall back to the
// framework's screen-relative positioning.
func MoveTo(x, y, w, h int) bool { return moveTo(x, y, w, h) }
