package tray

import "testing"

func TestPopupPositionCentresOnIcon(t *testing.T) {
	a := Anchor{
		// An icon in the menu bar sits above the work area, so its Y is negative.
		Icon: Rect{X: 500, Y: -24, W: 24, H: 24},
		Work: Rect{W: 1440, H: 900},
	}

	x, y := a.PopupPosition(720, 520)

	// Centred: 500 + 12 - 360 = 152.
	if x != 152 {
		t.Errorf("x = %d, want 152 (centred on the icon)", x)
	}
	// Icon bottom lands at 0, which is the top of the work area.
	if y != 0 {
		t.Errorf("y = %d, want 0 (flush under the menu bar)", y)
	}
}

func TestPopupPositionClampsToRightEdge(t *testing.T) {
	a := Anchor{
		Icon: Rect{X: 1400, Y: -24, W: 24, H: 24},
		Work: Rect{W: 1440, H: 900},
	}

	x, _ := a.PopupPosition(720, 520)

	// Centring would put it at 1052, running 332px off-screen.
	if want := 1440 - 720 - 8; x != want {
		t.Errorf("x = %d, want %d (clamped inside the right edge)", x, want)
	}
}

func TestPopupPositionClampsToLeftEdge(t *testing.T) {
	a := Anchor{
		Icon: Rect{X: 4, Y: -24, W: 24, H: 24},
		Work: Rect{W: 1440, H: 900},
	}

	x, _ := a.PopupPosition(720, 520)

	if x != 8 {
		t.Errorf("x = %d, want 8 (clamped inside the left edge)", x)
	}
}

// A taskbar at the bottom of the screen (Windows) puts the icon below the work
// area, so the popup has to be pulled up to stay visible.
func TestPopupPositionClampsBottomEdge(t *testing.T) {
	a := Anchor{
		Icon: Rect{X: 700, Y: 900, W: 24, H: 0},
		Work: Rect{W: 1440, H: 900},
	}

	_, y := a.PopupPosition(720, 520)

	if want := 900 - 520 - 8; y != want {
		t.Errorf("y = %d, want %d (pulled up to fit)", y, want)
	}
}

// A popup taller than the screen should sit at the top rather than be pushed
// off it by the bottom clamp.
func TestPopupPositionOversizedPopupStaysAtTop(t *testing.T) {
	a := Anchor{
		Icon: Rect{X: 700, Y: -24, W: 24, H: 24},
		Work: Rect{W: 1440, H: 400},
	}

	_, y := a.PopupPosition(720, 900)

	if y != 0 {
		t.Errorf("y = %d, want 0 for a popup taller than the work area", y)
	}
}

func TestPopupPositionNarrowWorkArea(t *testing.T) {
	// Work area narrower than the popup: clamping must not produce a position
	// further left than the margin.
	a := Anchor{
		Icon: Rect{X: 100, Y: -24, W: 24, H: 24},
		Work: Rect{W: 500, H: 900},
	}

	x, _ := a.PopupPosition(720, 520)

	if x != 8 {
		t.Errorf("x = %d, want 8 when the popup is wider than the work area", x)
	}
}

func TestCursorPopupPositionHangsFromThePointer(t *testing.T) {
	a := Anchor{
		Icon: Rect{X: 600, Y: 300},
		Work: Rect{W: 1440, H: 900},
	}

	x, y := a.CursorPopupPosition(720, 520)

	// A context menu opens at the pointer, not centred on it.
	if x != 600 || y != 300 {
		t.Errorf("(x, y) = (%d, %d), want (600, 300)", x, y)
	}
}

// The pointer near the bottom-right corner would push most of the popup off the
// screen, so it has to be pulled back inside the work area.
func TestCursorPopupPositionClampsToBottomRight(t *testing.T) {
	a := Anchor{
		Icon: Rect{X: 1430, Y: 890},
		Work: Rect{W: 1440, H: 900},
	}

	x, y := a.CursorPopupPosition(720, 520)

	if wantX, wantY := 1440-720-8, 900-520-8; x != wantX || y != wantY {
		t.Errorf("(x, y) = (%d, %d), want (%d, %d)", x, y, wantX, wantY)
	}
}

// A pointer up in the menu bar sits above the work area and reads as a negative
// y, which must not place the popup off the top of the screen.
func TestCursorPopupPositionClampsAboveTheWorkArea(t *testing.T) {
	a := Anchor{
		Icon: Rect{X: 4, Y: -12},
		Work: Rect{W: 1440, H: 900},
	}

	x, y := a.CursorPopupPosition(720, 520)

	if x != 8 || y != 8 {
		t.Errorf("(x, y) = (%d, %d), want (8, 8)", x, y)
	}
}

// A popup larger than the work area must land at the margin rather than be
// pushed off-screen by the far-edge clamp.
func TestCursorPopupPositionOversizedPopup(t *testing.T) {
	a := Anchor{
		Icon: Rect{X: 200, Y: 200},
		Work: Rect{W: 500, H: 400},
	}

	x, y := a.CursorPopupPosition(720, 900)

	if x != 8 || y != 8 {
		t.Errorf("(x, y) = (%d, %d), want (8, 8)", x, y)
	}
}

// A work area that does not start at the global origin is a second display;
// the popup has to be offset onto it rather than landing at the same spot on
// the primary one.
func TestGlobalOffsetsBySecondDisplayOrigin(t *testing.T) {
	a := Anchor{Work: Rect{X: 1440, Y: 200, W: 1920, H: 1080}}

	x, y := a.Global(600, 300)

	if x != 2040 || y != 500 {
		t.Errorf("(x, y) = (%d, %d), want (2040, 500)", x, y)
	}
}

// On the primary display the two spaces coincide, which is what lets the
// framework's screen-relative positioning stand in when the window cannot be
// moved natively.
func TestGlobalIsIdentityOnThePrimaryDisplay(t *testing.T) {
	a := Anchor{Work: Rect{W: 1440, H: 900}}

	x, y := a.Global(152, 0)

	if x != 152 || y != 0 {
		t.Errorf("(x, y) = (%d, %d), want (152, 0)", x, y)
	}
}
