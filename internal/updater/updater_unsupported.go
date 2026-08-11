//go:build !darwin || !sparkle

package updater

// The build the App Store package is made from, and every non-macOS build.
// Nothing here references Sparkle, so no framework is linked and none is
// embedded: the App Store submission cannot carry an update mechanism even by
// accident, because the code that would use one is not in the binary.

func available() bool { return false }

func start() {}

func checkNow() {}
