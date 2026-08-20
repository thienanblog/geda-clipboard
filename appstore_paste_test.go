package main

import (
	"os"
	"strings"
	"testing"
)

// The keystroke that pastes a chosen entry back needs Accessibility
// permission, and App Review rejected version 0.7.0 for it under guideline
// 2.4.5: Accessibility may not be used to automate other applications. The
// feature now lives behind the axpaste build tag, which only the Developer ID
// build passes.
//
// The tag being additive is what makes that safe -- a build made without it
// simply lacks the feature -- but only as long as the two packaging scripts
// disagree about it. This pins that disagreement. Losing it is invisible in a
// diff and expensive to learn about: a submission carrying Accessibility code
// costs a review cycle, and a marketing version a rejection has burned cannot
// be resubmitted.
func TestPackagingScriptsDisagreeOnPasteTag(t *testing.T) {
	devID, err := os.ReadFile("scripts/package-macos.sh")
	if err != nil {
		t.Fatalf("read package-macos.sh: %v", err)
	}
	if !strings.Contains(string(devID), "wails build -clean -platform darwin/universal -tags sparkle,axpaste") {
		t.Error("package-macos.sh no longer builds with -tags sparkle,axpaste, so the Developer ID build has lost its updater or its paste-back")
	}

	appStore, err := os.ReadFile("scripts/package-appstore.sh")
	if err != nil {
		t.Fatalf("read package-appstore.sh: %v", err)
	}
	if strings.Contains(string(appStore), "axpaste") && !strings.Contains(string(appStore), "no sparkle or axpaste tag") {
		t.Error("package-appstore.sh mentions the axpaste tag outside its comment; the App Store build must not carry it")
	}
	if strings.Contains(string(appStore), "-tags") {
		t.Error("package-appstore.sh passes a build tag; the App Store build is the untagged one")
	}
}

// The scripts are one half of the protection. The other half is checking the
// binary, because a call reaching an untagged file would pass the test above
// and still ship. These are the symbols such a call leaves behind.
func TestAppStorePackagingChecksForAccessibilitySymbols(t *testing.T) {
	raw, err := os.ReadFile("scripts/package-appstore.sh")
	if err != nil {
		t.Fatalf("read package-appstore.sh: %v", err)
	}
	script := string(raw)

	for _, symbol := range []string{
		"_AXIsProcessTrusted",
		"_AXIsProcessTrustedWithOptions",
		"_CGEventCreateKeyboardEvent",
		"_CGEventPost",
	} {
		if !strings.Contains(script, symbol) {
			t.Errorf("package-appstore.sh no longer rejects a bundle referencing %s", symbol)
		}
	}
	if !strings.Contains(script, "nm -u") {
		t.Error("package-appstore.sh no longer reads the binary's undefined symbols, which is where a stray Accessibility call shows up")
	}
}
