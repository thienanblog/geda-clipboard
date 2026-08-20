//go:build darwin && !axpaste

package clipboard

// The macOS build with no keystroke path: the Mac App Store build, and any
// plain `wails build` or `go test` run.
//
// Sending Cmd+V to another application requires Accessibility permission, and
// App Review rejected exactly that under guideline 2.4.5 -- Accessibility may
// not be used to automate other apps. So the feature is additive, like the
// sparkle tag: `wails build` with no tags produces a binary that cannot ask
// for Accessibility and does not contain a single Accessibility symbol, which
// is what App Review inspects. scripts/package-macos.sh passes the tag for the
// Developer ID build; scripts/package-appstore.sh verifies the absence.
//
// Selecting an entry still copies it and still hands focus back to the app the
// user came from, so pasting is one keystroke they press themselves.

func pasteSupported() bool { return false }

func paste() error { return ErrPasteUnsupported }

func hasPastePermission(bool) bool { return false }

func openPastePermissionSettings() error { return ErrPasteUnsupported }
