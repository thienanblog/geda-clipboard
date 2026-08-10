// Package tray owns the menu bar / system tray icon and reports clicks back to
// the application, together with the on-screen geometry needed to anchor the
// popup window underneath the icon.
package tray

// Rect is a rectangle in logical pixels, top-left origin, expressed relative to
// the work area of the screen holding the tray icon (see Anchor.Work).
type Rect struct {
	X, Y, W, H int
}

// Anchor describes where the tray icon is on screen. Icon is the rect of the
// icon itself, expressed relative to Work, so placing a window at Icon.X, 0
// puts it flush under the menu bar. Work is the usable area of the same screen
// -- on macOS that excludes the menu bar, on Windows the taskbar -- and its own
// X and Y locate that screen in the global space (see Global).
type Anchor struct {
	Icon Rect
	Work Rect
}

// Global converts a position relative to the anchor's work area into the
// global coordinate space: top-left origin at the top-left of the primary
// display, spanning every display. That is the space a window has to be placed
// in for the popup to be able to open on a screen other than the one the
// window is currently sitting on.
func (a Anchor) Global(x, y int) (int, int) {
	return a.Work.X + x, a.Work.Y + y
}

// Handler receives tray interactions.
type Handler interface {
	OnLeftClick(a Anchor)
	OnRightClick(a Anchor)
}

var handler Handler

// Start creates the tray icon. It must be called after the GUI framework has
// initialised its application object. icon should be a small PNG (16x16 or
// 32x32); on macOS it is treated as a template image so it adapts automatically
// to light and dark menu bars.
func Start(h Handler, icon []byte, tooltip string) error {
	handler = h
	return start(icon, tooltip)
}

// Stop removes the tray icon.
func Stop() { stop() }

func fireLeft(a Anchor) {
	if handler != nil {
		handler.OnLeftClick(a)
	}
}

func fireRight(a Anchor) {
	if handler != nil {
		handler.OnRightClick(a)
	}
}

// CursorAnchor reports where the mouse pointer is, in the same coordinate space
// as Anchor: top-left origin, relative to the work area of the screen holding
// the pointer. Icon is a zero-sized rect at the pointer itself, so the result
// can be fed straight to CursorPopupPosition. ok is false when the platform
// cannot answer, which is the caller's cue to fall back to the tray anchor.
func CursorAnchor() (Anchor, bool) { return cursorAnchor() }

// CursorPopupPosition returns the top-left position for a popup of the given
// size opened at the pointer, the way a context menu behaves: the popup hangs
// down and to the right of the cursor, and is pulled back inside the work area
// when the cursor is close to an edge. Call it on an Anchor from CursorAnchor.
func (a Anchor) CursorPopupPosition(w, h int) (int, int) {
	const margin = 8

	x, y := a.Icon.X, a.Icon.Y

	// Order matters: when the popup is larger than the work area the first
	// clamp produces a value left of the margin, and the second pulls it back.
	if maxX := a.Work.W - w - margin; x > maxX {
		x = maxX
	}
	if x < margin {
		x = margin
	}
	if maxY := a.Work.H - h - margin; y > maxY {
		y = maxY
	}
	if y < margin {
		y = margin
	}
	return x, y
}

// PopupPosition returns the top-left position, in the coordinate space Wails'
// WindowSetPosition expects, at which a popup of the given size should be
// placed so that it hangs below the tray icon and stays on screen.
func (a Anchor) PopupPosition(w, h int) (int, int) {
	const margin = 8

	// Centre horizontally on the icon.
	x := a.Icon.X + a.Icon.W/2 - w/2

	// Keep it inside the work area.
	if maxX := a.Work.W - w - margin; x > maxX {
		x = maxX
	}
	if x < margin {
		x = margin
	}

	// The icon sits above the work area (in the menu bar), so its bottom edge
	// lands at or above y=0. Clamp to the top of the work area.
	y := a.Icon.Y + a.Icon.H
	if y < 0 {
		y = 0
	}
	if maxY := a.Work.H - h - margin; h < a.Work.H && y > maxY {
		y = maxY
	}
	return x, y
}
