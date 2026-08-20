//go:build darwin && !axpaste

package clipboard

import (
	"errors"
	"testing"
)

// The default macOS build is the one submitted to the App Store, and it must
// have no keystroke path: App Review rejected version 0.7.0 for using
// Accessibility to send Cmd+V. Callers branch on PasteSupported to offer the
// copy-and-refocus path instead, so a change that made this report true again
// without the axpaste tag would put the permission prompt back in front of a
// reviewer.
func TestUntaggedBuildCannotPaste(t *testing.T) {
	if PasteSupported() {
		t.Fatal("a build without the axpaste tag reports that it can paste")
	}
	if err := Paste(); !errors.Is(err, ErrPasteUnsupported) {
		t.Errorf("Paste() = %v, want ErrPasteUnsupported", err)
	}
	if HasPastePermission(true) {
		t.Error("HasPastePermission reports a permission this build never asks for")
	}
	if err := OpenPastePermissionSettings(); !errors.Is(err, ErrPasteUnsupported) {
		t.Errorf("OpenPastePermissionSettings() = %v, want ErrPasteUnsupported", err)
	}
}
