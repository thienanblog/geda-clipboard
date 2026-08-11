package hotkeys

import (
	"errors"
	"testing"
	"time"
)

func TestParseValid(t *testing.T) {
	mods, key, err := Parse("cmd+shift+v")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	wantV, ok := lookupKey("v")
	if !ok {
		t.Fatal("this platform has no code for \"v\"")
	}
	if key != wantV {
		t.Errorf("key = %v, want %v", key, wantV)
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

// fakeRegistration stands in for a real global hotkey so the failure path can
// be exercised without a window server.
type fakeRegistration struct {
	err          error
	keydown      chan struct{}
	unregistered bool
}

func (f *fakeRegistration) Register() error          { return f.err }
func (f *fakeRegistration) Unregister() error        { f.unregistered = true; return nil }
func (f *fakeRegistration) Keydown() <-chan struct{} { return f.keydown }

// swapRegistrations installs a fake factory returning each entry of fakes in
// order, and restores the real one afterwards.
func swapRegistrations(t *testing.T, fakes ...*fakeRegistration) {
	t.Helper()
	original := newRegistration
	i := 0
	newRegistration = func([]Modifier, Key) registration {
		if i >= len(fakes) {
			t.Fatalf("newRegistration called %d times, only %d fakes provided", i+1, len(fakes))
		}
		f := fakes[i]
		i++
		return f
	}
	t.Cleanup(func() { newRegistration = original })
}

func newFake(err error) *fakeRegistration {
	return &fakeRegistration{err: err, keydown: make(chan struct{})}
}

// The point of registering before unregistering: a combination another
// application already owns must leave the working shortcut in place, rather
// than leaving the user with no shortcut at all.
func TestRegisterKeepsTheOldShortcutWhenTheNewOneIsTaken(t *testing.T) {
	working := newFake(nil)
	taken := newFake(errors.New("hotkey is already registered"))
	swapRegistrations(t, working, taken)

	m := New(func() {})
	if err := m.Register("cmd+shift+v"); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	if m.Spec() != "cmd+shift+v" {
		t.Fatalf("Spec = %q, want cmd+shift+v", m.Spec())
	}

	if err := m.Register("cmd+shift+c"); err == nil {
		t.Fatal("Register should report that the combination is taken")
	}

	if m.Spec() != "cmd+shift+v" {
		t.Errorf("Spec = %q, want the previous shortcut to survive a failed swap", m.Spec())
	}
	if working.unregistered {
		t.Error("the working shortcut was released even though the new one failed")
	}
}

func TestRegisterSwapsShortcutsOnSuccess(t *testing.T) {
	first := newFake(nil)
	second := newFake(nil)
	swapRegistrations(t, first, second)

	m := New(func() {})
	if err := m.Register("cmd+shift+v"); err != nil {
		t.Fatal(err)
	}
	if err := m.Register("ctrl+alt+k"); err != nil {
		t.Fatal(err)
	}

	if !first.unregistered {
		t.Error("the previous shortcut should be released once the new one is claimed")
	}
	if m.Spec() != "ctrl+alt+k" {
		t.Errorf("Spec = %q, want ctrl+alt+k", m.Spec())
	}
}

// Re-registering the same spec must not try to claim a combination we already
// hold, which would collide with itself.
func TestRegisterSameSpecIsANoop(t *testing.T) {
	only := newFake(nil)
	swapRegistrations(t, only)

	m := New(func() {})
	if err := m.Register("cmd+shift+v"); err != nil {
		t.Fatal(err)
	}
	if err := m.Register("cmd+shift+v"); err != nil {
		t.Fatalf("re-registering the same spec: %v", err)
	}
	if only.unregistered {
		t.Error("the shortcut should still be held")
	}
}

func TestRegisterEmptySpecUnregisters(t *testing.T) {
	only := newFake(nil)
	swapRegistrations(t, only)

	m := New(func() {})
	if err := m.Register("cmd+shift+v"); err != nil {
		t.Fatal(err)
	}
	if err := m.Register(""); err != nil {
		t.Fatalf("Register(\"\"): %v", err)
	}
	if !only.unregistered {
		t.Error("an empty spec should release the current shortcut")
	}
	if m.Spec() != "" {
		t.Errorf("Spec = %q, want empty", m.Spec())
	}
}

// The settings UI offers every name in keyNames on both platforms, so a table
// missing one would ship a shortcut the user can pick and the app then refuses
// to register -- the failure this package was rewritten to stop hiding.
func TestEveryCanonicalKeyNameResolvesOnThisPlatform(t *testing.T) {
	for _, name := range keyNames {
		if _, ok := lookupKey(name); !ok {
			t.Errorf("lookupKey(%q) failed; this platform's table is missing it", name)
		}
	}
}

// A press has to reach the callback, which is the whole point of the package.
func TestAPressReachesTheCallback(t *testing.T) {
	only := newFake(nil)
	swapRegistrations(t, only)

	fired := make(chan struct{}, 1)
	m := New(func() { fired <- struct{}{} })
	if err := m.Register("cmd+shift+v"); err != nil {
		t.Fatal(err)
	}
	only.keydown <- struct{}{}

	select {
	case <-fired:
	case <-time.After(2 * time.Second):
		t.Fatal("the callback never ran")
	}
}
