package main

import (
	"encoding/json"
	"os"
	"regexp"
	"testing"
)

// The version lives in two places: appVersion, which the About panel reports,
// and wails.json's productVersion, which is templated into the macOS
// Info.plist and the Windows resource block at build time. Nothing keeps them
// in step, so a release could easily ship a binary that reports one version
// while its bundle metadata claims another. Fail the build instead.
func TestVersionMatchesWailsConfig(t *testing.T) {
	raw, err := os.ReadFile("wails.json")
	if err != nil {
		t.Fatalf("read wails.json: %v", err)
	}

	var cfg struct {
		Info struct {
			ProductVersion string `json:"productVersion"`
		} `json:"info"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("parse wails.json: %v", err)
	}

	if cfg.Info.ProductVersion != appVersion {
		t.Errorf("wails.json productVersion = %q but appVersion = %q; bump both together",
			cfg.Info.ProductVersion, appVersion)
	}
}

// A release is tagged from this value, so it has to be a bare semver string --
// no leading "v", no stray whitespace.
func TestVersionIsSemver(t *testing.T) {
	if !regexp.MustCompile(`^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$`).MatchString(appVersion) {
		t.Errorf("appVersion = %q, want a bare semver such as 1.2.3", appVersion)
	}
}
