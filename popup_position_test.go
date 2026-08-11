package main

import (
	"testing"

	"geda-clipboard/internal/settings"
	"geda-clipboard/internal/tray"
)

// stubCursor replaces the pointer lookup for the duration of a test.
func stubCursor(t *testing.T, a tray.Anchor, ok bool) {
	t.Helper()
	previous := cursorAnchor
	cursorAnchor = func() (tray.Anchor, bool) { return a, ok }
	t.Cleanup(func() { cursorAnchor = previous })
}

var (
	// The primary display, whose work area starts at the global origin.
	pointer = tray.Anchor{
		Icon: tray.Rect{X: 600, Y: 300},
		Work: tray.Rect{W: 1440, H: 900},
	}
	menuBarIcon = tray.Anchor{
		Icon: tray.Rect{X: 500, Y: -24, W: 24, H: 24},
		Work: tray.Rect{W: 1440, H: 900},
	}
	// A second display to the right of the primary one. Its work area starts
	// 1440px along, which is what has to reach the window for the popup to open
	// over there rather than at the same spot on the primary display.
	pointerOnSecondDisplay = tray.Anchor{
		Icon: tray.Rect{X: 600, Y: 300},
		Work: tray.Rect{X: 1440, Y: 0, W: 1920, H: 1080},
	}
)

func TestPopupPositionAtCursor(t *testing.T) {
	stubCursor(t, pointer, true)

	// Both sources are available: cursor placement must ignore the tray icon.
	p, ok := popupPosition(settings.PlacementCursor, menuBarIcon, 420, 520, 0, 0)

	if !ok || p.RelX != 600 || p.RelY != 300 {
		t.Errorf("relative = (%d, %d), ok = %v, want (600, 300), true", p.RelX, p.RelY, ok)
	}
}

func TestPopupPositionAtMenuBar(t *testing.T) {
	stubCursor(t, pointer, true)

	p, ok := popupPosition(settings.PlacementMenuBar, menuBarIcon, 420, 520, 0, 0)

	// Centred on the icon: 500 + 12 - 210 = 302, flush under the menu bar.
	if !ok || p.RelX != 302 || p.RelY != 0 {
		t.Errorf("relative = (%d, %d), ok = %v, want (302, 0), true", p.RelX, p.RelY, ok)
	}
}

// On the primary display the two coordinate spaces coincide, which is what
// makes the fallback to Wails' screen-relative positioning correct there.
func TestPopupPositionGlobalMatchesRelativeOnPrimary(t *testing.T) {
	stubCursor(t, pointer, true)

	p, ok := popupPosition(settings.PlacementCursor, tray.Anchor{}, 420, 520, 0, 0)

	if !ok || p.GlobalX != p.RelX || p.GlobalY != p.RelY {
		t.Errorf("global = (%d, %d), relative = (%d, %d), want them equal",
			p.GlobalX, p.GlobalY, p.RelX, p.RelY)
	}
}

// The whole point of the global coordinates: a pointer on a second display has
// to produce a position offset by that display's origin.
func TestPopupPositionGlobalOnSecondDisplay(t *testing.T) {
	stubCursor(t, pointerOnSecondDisplay, true)

	p, ok := popupPosition(settings.PlacementCursor, menuBarIcon, 420, 520, 0, 0)

	if !ok {
		t.Fatal("no position resolved")
	}
	if p.RelX != 600 || p.RelY != 300 {
		t.Errorf("relative = (%d, %d), want (600, 300)", p.RelX, p.RelY)
	}
	if p.GlobalX != 1440+600 || p.GlobalY != 300 {
		t.Errorf("global = (%d, %d), want (2040, 300)", p.GlobalX, p.GlobalY)
	}
}

// The tray anchor is unknown until the icon has been clicked once, which is the
// common case for a user who only ever uses the shortcut.
func TestPopupPositionMenuBarFallsBackToCursor(t *testing.T) {
	stubCursor(t, pointer, true)

	p, ok := popupPosition(settings.PlacementMenuBar, tray.Anchor{}, 420, 520, 0, 0)

	if !ok || p.RelX != 600 || p.RelY != 300 {
		t.Errorf("relative = (%d, %d), ok = %v, want (600, 300), true", p.RelX, p.RelY, ok)
	}
}

func TestPopupPositionCursorFallsBackToMenuBar(t *testing.T) {
	stubCursor(t, tray.Anchor{}, false)

	p, ok := popupPosition(settings.PlacementCursor, menuBarIcon, 420, 520, 0, 0)

	if !ok || p.RelX != 302 || p.RelY != 0 {
		t.Errorf("relative = (%d, %d), ok = %v, want (302, 0), true", p.RelX, p.RelY, ok)
	}
}

// The preview gutter and the shadow margins are transparent window, not chrome:
// the window starts that much further left and up so the list panel itself
// still opens at the pointer.
func TestPopupPositionShiftsBackByTheInsets(t *testing.T) {
	stubCursor(t, pointer, true)

	p, ok := popupPosition(settings.PlacementCursor, tray.Anchor{}, 420, 520, 316, 8)

	if !ok || p.RelX != 284 || p.RelY != 292 {
		t.Errorf("relative = (%d, %d), ok = %v, want (284, 292), true", p.RelX, p.RelY, ok)
	}
}

// Close to an edge there is no room for the insets; the window stops at the
// edge rather than hanging off it, which leaves the panel inset-deep in from
// where it was aimed. That is why the top inset is kept small: a popup
// anchored to the tray icon lands here every time, and the inset is the whole
// gap between it and the menu bar.
func TestPopupPositionClampsInsetsAtTheEdges(t *testing.T) {
	stubCursor(t, tray.Anchor{
		Icon: tray.Rect{X: 40, Y: 4},
		Work: tray.Rect{W: 1440, H: 900},
	}, true)

	p, ok := popupPosition(settings.PlacementCursor, tray.Anchor{}, 420, 520, 316, 8)

	if !ok || p.RelX != 0 || p.RelY != 0 {
		t.Errorf("relative = (%d, %d), ok = %v, want (0, 0), true", p.RelX, p.RelY, ok)
	}
}

// With no pointer and no tray anchor the caller has to centre the window.
func TestPopupPositionWithoutAnySource(t *testing.T) {
	stubCursor(t, tray.Anchor{}, false)

	for _, mode := range []string{settings.PlacementCursor, settings.PlacementMenuBar} {
		if _, ok := popupPosition(mode, tray.Anchor{}, 420, 520, 0, 0); ok {
			t.Errorf("mode %q reported a position with no source available", mode)
		}
	}
}
