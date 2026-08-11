//go:build darwin && sparkle

package updater

/*
#cgo CFLAGS: -x objective-c -F${SRCDIR}/../../build/darwin/Frameworks
#cgo LDFLAGS: -F${SRCDIR}/../../build/darwin/Frameworks -framework Foundation -framework Sparkle
#include "updater_sparkle_darwin.h"
*/
import "C"

// Building this file needs Sparkle.framework under build/darwin/Frameworks,
// and the same framework copied into the bundle's Contents/Frameworks at
// package time. scripts/fetch-sparkle.sh puts it in the first place;
// scripts/package-macos.sh puts it in the second.
//
// Sparkle's install name is @rpath-relative, so the binary also needs an
// LC_RPATH pointing at Contents/Frameworks. That is not set here: cgo rejects
// -Wl,-rpath unless every build exports CGO_LDFLAGS_ALLOW, and a build flag
// that has to be remembered on the command line is one that will be missed.
// The packaging script adds it with install_name_tool instead, before it
// signs, which is the only moment the layout is actually known.

func available() bool { return true }

func start() { C.gedaUpdaterStart() }

func checkNow() { C.gedaUpdaterCheckNow() }
