package hotkeys

import (
	"testing"

	"golang.design/x/hotkey"
)

func TestParseValid(t *testing.T) {
	mods, key, err := Parse("cmd+shift+v")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if key != hotkey.KeyV {
		t.Errorf("key = %v, want KeyV", key)
	}
	if len(mods) != 2 {
		t.Fatalf("got %d modifiers, want 2", len(mods))
	}
}

func TestParseIsCaseAndSpaceInsensitive(t *testing.T) {
	a, keyA, err := Parse("  CMD + Shift + V ")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	b, keyB, err := Parse("cmd+shift+v")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if keyA != keyB || len(a) != len(b) {
		t.Errorf("parsing differed: %v/%v vs %v/%v", a, keyA, b, keyB)
	}
}

// An empty spec means "no shortcut" rather than an error, so the caller can
// unregister by saving a blank value.
func TestParseEmptyMeansNoShortcut(t *testing.T) {
	mods, key, err := Parse("")
	if err != nil {
		t.Errorf("Parse(\"\") returned an error: %v", err)
	}
	if mods != nil || key != 0 {
		t.Errorf("Parse(\"\") = %v, %v; want nil, 0", mods, key)
	}
	if _, _, err := Parse("   "); err != nil {
		t.Errorf("Parse(whitespace) returned an error: %v", err)
	}
}

func TestParseRejectsInvalid(t *testing.T) {
	cases := []struct {
		name string
		spec string
	}{
		{"no modifier", "v"},
		{"modifier only", "cmd"},
		{"two modifiers no key", "cmd+shift"},
		{"two non-modifier keys", "cmd+a+b"},
		{"unknown key", "cmd+notakey"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := Parse(tc.spec); err == nil {
				t.Errorf("Parse(%q) succeeded, want an error", tc.spec)
			}
		})
	}
}

func TestParseNamedKeys(t *testing.T) {
	for _, name := range []string{"space", "return", "enter", "escape", "esc", "f5", "f12", "up", "1"} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := Parse("ctrl+alt+" + name); err != nil {
				t.Errorf("Parse(ctrl+alt+%s) failed: %v", name, err)
			}
		})
	}
}

// Parse must accept the aliases the settings UI can produce for each platform.
func TestParseModifierAliases(t *testing.T) {
	for _, alias := range []string{"cmd", "command", "meta", "super", "win", "ctrl", "control", "alt", "option", "opt", "shift"} {
		t.Run(alias, func(t *testing.T) {
			mods, _, err := Parse(alias + "+k")
			if err != nil {
				t.Fatalf("Parse(%s+k) failed: %v", alias, err)
			}
			if len(mods) != 1 {
				t.Errorf("got %d modifiers, want 1", len(mods))
			}
		})
	}
}
