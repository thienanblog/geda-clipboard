package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// Files whose XML comments codesign or Sparkle will one day have to parse.
var xmlWithComments = []string{
	"build/darwin/Info.plist",
	"build/darwin/Info.dev.plist",
	"build/darwin/entitlements.plist",
	"build/darwin/entitlements.appstore.plist",
	"docs/appcast.xml",
}

var xmlComment = regexp.MustCompile(`(?s)<!--(.*?)-->`)

// XML forbids a double hyphen inside a comment. plutil tolerates it, but the
// parser codesign hands entitlements to does not, and it fails in the worst
// available way: it rejects the file, signs anyway with neither the
// entitlements nor the hardened runtime, and exits zero. The bundle then
// passes codesign --verify and fails notarization pointing somewhere else
// entirely, which is a long afternoon.
//
// package-macos.sh reads the entitlements back out of the finished signature
// for the same reason, but that only catches it once a signing certificate is
// in the keychain. This catches it on any machine, in CI, before the commit
// that introduces it can be merged.
func TestXMLCommentsHaveNoDoubleHyphen(t *testing.T) {
	for _, path := range xmlWithComments {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("read %s: %v", path, err)
			continue
		}
		for _, m := range xmlComment.FindAllStringSubmatch(string(raw), -1) {
			if strings.Contains(m[1], "--") {
				t.Errorf("%s: a comment contains a double hyphen, which XML forbids:\n%s",
					path, strings.TrimSpace(m[1]))
			}
		}
	}
}

// App Store Connect refuses an upload whose bundle does not declare a category,
// and it says so only after Transporter has reported the upload as successful
// and the processing pass has run. Cheaper to fail here.
//
// LSUIElement is checked alongside it because the app has no window of its own:
// losing that key gives every user a Dock tile for an app that never opens
// anything, and nothing else in the build would complain.
func TestInfoPlistDeclaresAppStoreKeys(t *testing.T) {
	raw, err := os.ReadFile("build/darwin/Info.plist")
	if err != nil {
		t.Fatalf("read Info.plist: %v", err)
	}
	plist := string(raw)

	for _, key := range []string{
		"LSApplicationCategoryType",
		"LSUIElement",
		"LSMinimumSystemVersion",
		"ITSAppUsesNonExemptEncryption",
	} {
		if !strings.Contains(plist, "<key>"+key+"</key>") {
			t.Errorf("build/darwin/Info.plist does not declare %s", key)
		}
	}
}

// The App Store build must not ask for anything Sparkle needs. A temporary
// exception whose purpose is self-updating reads as guideline 2.4.5, and the
// network entitlement is a question at review this app has no reason to answer.
func TestAppStoreEntitlementsCarryNoUpdater(t *testing.T) {
	raw, err := os.ReadFile("build/darwin/entitlements.appstore.plist")
	if err != nil {
		t.Fatalf("read entitlements.appstore.plist: %v", err)
	}
	entitlements := string(raw)

	for _, forbidden := range []string{
		"temporary-exception",
		"com.apple.security.network.client",
	} {
		// Comments in this file discuss both by name on purpose, so only the
		// declared keys count.
		if strings.Contains(stripXMLComments(entitlements), forbidden) {
			t.Errorf("entitlements.appstore.plist declares %s, which the App Store build must not carry", forbidden)
		}
	}

	if !strings.Contains(entitlements, "<key>com.apple.security.app-sandbox</key>") {
		t.Error("entitlements.appstore.plist must declare the sandbox")
	}
}

func stripXMLComments(s string) string {
	return xmlComment.ReplaceAllString(s, "")
}
