package main

import "testing"

// The whole point of comparing components numerically is the cases a string
// comparison gets backwards, so those are what this covers: build 9 against
// build 10, and 0.9.0 against 0.10.0. Getting either wrong would let the
// preflight wave through a build number App Store Connect then rejects.
func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1", "1", 0},
		{"9", "10", -1},
		{"10", "9", 1},
		{"0.9.0", "0.10.0", -1},
		{"0.10.0", "0.9.0", 1},
		{"1.2.3", "1.2.3", 0},
		// Trailing zeros are the same version, however it was spelled.
		{"1.2", "1.2.0", 0},
		{"1.2.0", "1.2", 0},
		{"2.0", "1.9.9", 1},
	}
	for _, c := range cases {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestNextAfter(t *testing.T) {
	cases := map[string]string{
		"1":     "2",
		"9":     "10",
		"1.2.3": "1.2.4",
	}
	for in, want := range cases {
		if got := nextAfter(in); got != want {
			t.Errorf("nextAfter(%q) = %q, want %q", in, got, want)
		}
	}
}

// A released version is the check that has to hold: raising the build number
// does not reopen one, so anything in this set must block the package.
func TestReleasedStatesBlock(t *testing.T) {
	for _, state := range []string{"READY_FOR_SALE", "REPLACED_WITH_NEW_VERSION"} {
		if !releasedStates[state] {
			t.Errorf("%s should count as released", state)
		}
	}
	// Editable states must not, or a rejected version could never be resubmitted.
	for _, state := range []string{"PREPARE_FOR_SUBMISSION", "DEVELOPER_REJECTED", "REJECTED", "METADATA_REJECTED"} {
		if releasedStates[state] {
			t.Errorf("%s is editable and must not count as released", state)
		}
	}
}
