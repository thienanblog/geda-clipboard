// Package hotkeys registers the global shortcut that toggles the popup.
//
// The shortcut must work without any permission the user has to grant first.
// A clipboard manager whose only way in is a shortcut is unreachable until
// that shortcut registers, and on macOS the App Store build cannot ask for
// Accessibility before it has shown the user anything. Each platform therefore
// uses the API that claims a system-wide shortcut outright -- Carbon's
// RegisterEventHotKey on macOS, RegisterHotKey on Windows -- rather than a
// keyboard event tap, which observes every keystroke in the session and is
// gated behind Accessibility for that reason.
package hotkeys

import (
	"fmt"
	"strings"
	"sync"
)

// registration is the part of a platform hotkey the Manager uses. It exists as
// an interface so the interesting failure -- a combination another application
// already owns -- can be exercised in a test without a window server.
type registration interface {
	Register() error
	Unregister() error
	// Keydown fires once per press. Implementations must not block on it: the
	// send happens on the thread the OS delivers the event to, which on macOS
	// is the main thread, and blocking there freezes the whole application.
	Keydown() <-chan struct{}
}

// newRegistration is a variable so tests can substitute a fake.
var newRegistration = func(mods []Modifier, key Key) registration {
	return newPlatformRegistration(mods, key)
}

// Manager owns at most one registered hotkey and can swap it at runtime when
// the user changes the preference.
type Manager struct {
	mu      sync.Mutex
	current registration
	stop    chan struct{}
	spec    string

	onTrigger func()
}

// New builds a Manager that calls onTrigger each time the hotkey fires.
func New(onTrigger func()) *Manager {
	return &Manager{onTrigger: onTrigger}
}

// Spec returns the currently registered shortcut, or "" when none is active.
func (m *Manager) Spec() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.current == nil {
		return ""
	}
	return m.spec
}

// Register parses spec (e.g. "cmd+shift+v") and registers it, replacing any
// previously registered shortcut. An empty spec just unregisters.
//
// The new shortcut is claimed before the old one is released, so a combination
// that another application already owns leaves the working shortcut in place
// instead of dropping the user to no shortcut at all.
func (m *Manager) Register(spec string) error {
	mods, key, err := Parse(spec)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if spec == "" {
		m.unregisterLocked()
		return nil
	}
	if m.current != nil && m.spec == spec {
		return nil // already ours; re-claiming it would collide with itself
	}

	hk := newRegistration(mods, key)
	if err := hk.Register(); err != nil {
		return fmt.Errorf("register %q: %w", spec, err)
	}

	m.unregisterLocked()

	m.current = hk
	m.spec = spec
	stop := make(chan struct{})
	m.stop = stop

	go func() {
		for {
			select {
			case <-stop:
				return
			case _, ok := <-hk.Keydown():
				if !ok {
					return
				}
				if m.onTrigger != nil {
					m.onTrigger()
				}
			}
		}
	}()

	return nil
}

// Unregister releases the current shortcut.
func (m *Manager) Unregister() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.unregisterLocked()
}

func (m *Manager) unregisterLocked() {
	if m.stop != nil {
		close(m.stop)
		m.stop = nil
	}
	if m.current != nil {
		m.current.Unregister()
		m.current = nil
	}
	m.spec = ""
}

// Parse converts a shortcut string such as "cmd+shift+v" into the modifier and
// key values the platform expects. Modifier names accepted:
// cmd/command/meta/super/win, ctrl/control, alt/option/opt, shift.
func Parse(spec string) ([]Modifier, Key, error) {
	if strings.TrimSpace(spec) == "" {
		return nil, 0, nil
	}

	parts := strings.Split(strings.ToLower(strings.TrimSpace(spec)), "+")
	var mods []Modifier
	var keyName string

	for _, raw := range parts {
		p := strings.TrimSpace(raw)
		if p == "" {
			continue
		}
		if mod, ok := lookupModifier(p); ok {
			mods = append(mods, mod)
			continue
		}
		if keyName != "" {
			return nil, 0, fmt.Errorf("shortcut %q has more than one non-modifier key", spec)
		}
		keyName = p
	}

	if keyName == "" {
		return nil, 0, fmt.Errorf("shortcut %q has no key", spec)
	}
	if len(mods) == 0 {
		return nil, 0, fmt.Errorf("shortcut %q needs at least one modifier", spec)
	}

	key, ok := lookupKey(keyName)
	if !ok {
		return nil, 0, fmt.Errorf("unknown key %q in shortcut %q", keyName, spec)
	}
	return mods, key, nil
}

// keyNames lists every non-modifier key a shortcut may name. The codes behind
// them are per-platform -- macOS virtual keycodes are unrelated to Windows
// virtual-key codes -- so each platform keeps its own table and a test asserts
// that both tables cover this list. Drift would otherwise show up only as a
// shortcut the settings UI offers and the other platform refuses to register.
var keyNames = []string{
	"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m",
	"n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z",

	"0", "1", "2", "3", "4", "5", "6", "7", "8", "9",

	"f1", "f2", "f3", "f4", "f5", "f6",
	"f7", "f8", "f9", "f10", "f11", "f12",

	"space", "return", "enter", "tab", "escape", "esc", "delete",
	"up", "down", "left", "right",
}
