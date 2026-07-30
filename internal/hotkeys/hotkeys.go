// Package hotkeys registers the global shortcut that toggles the popup.
package hotkeys

import (
	"fmt"
	"strings"
	"sync"

	"golang.design/x/hotkey"
)

// registration is the part of hotkey.Hotkey the Manager uses. It exists as an
// interface so the interesting failure -- a combination another application
// already owns -- can be exercised in a test without a window server.
type registration interface {
	Register() error
	Unregister() error
	Keydown() <-chan hotkey.Event
}

// newRegistration is a variable so tests can substitute a fake.
var newRegistration = func(mods []hotkey.Modifier, key hotkey.Key) registration {
	return hotkey.New(mods, key)
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
func Parse(spec string) ([]hotkey.Modifier, hotkey.Key, error) {
	if strings.TrimSpace(spec) == "" {
		return nil, 0, nil
	}

	parts := strings.Split(strings.ToLower(strings.TrimSpace(spec)), "+")
	var mods []hotkey.Modifier
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

	key, ok := keyByName[keyName]
	if !ok {
		return nil, 0, fmt.Errorf("unknown key %q in shortcut %q", keyName, spec)
	}
	return mods, key, nil
}

var keyByName = map[string]hotkey.Key{
	"a": hotkey.KeyA, "b": hotkey.KeyB, "c": hotkey.KeyC, "d": hotkey.KeyD,
	"e": hotkey.KeyE, "f": hotkey.KeyF, "g": hotkey.KeyG, "h": hotkey.KeyH,
	"i": hotkey.KeyI, "j": hotkey.KeyJ, "k": hotkey.KeyK, "l": hotkey.KeyL,
	"m": hotkey.KeyM, "n": hotkey.KeyN, "o": hotkey.KeyO, "p": hotkey.KeyP,
	"q": hotkey.KeyQ, "r": hotkey.KeyR, "s": hotkey.KeyS, "t": hotkey.KeyT,
	"u": hotkey.KeyU, "v": hotkey.KeyV, "w": hotkey.KeyW, "x": hotkey.KeyX,
	"y": hotkey.KeyY, "z": hotkey.KeyZ,

	"0": hotkey.Key0, "1": hotkey.Key1, "2": hotkey.Key2, "3": hotkey.Key3,
	"4": hotkey.Key4, "5": hotkey.Key5, "6": hotkey.Key6, "7": hotkey.Key7,
	"8": hotkey.Key8, "9": hotkey.Key9,

	"f1": hotkey.KeyF1, "f2": hotkey.KeyF2, "f3": hotkey.KeyF3,
	"f4": hotkey.KeyF4, "f5": hotkey.KeyF5, "f6": hotkey.KeyF6,
	"f7": hotkey.KeyF7, "f8": hotkey.KeyF8, "f9": hotkey.KeyF9,
	"f10": hotkey.KeyF10, "f11": hotkey.KeyF11, "f12": hotkey.KeyF12,

	"space":  hotkey.KeySpace,
	"return": hotkey.KeyReturn,
	"enter":  hotkey.KeyReturn,
	"tab":    hotkey.KeyTab,
	"escape": hotkey.KeyEscape,
	"esc":    hotkey.KeyEscape,
	"delete": hotkey.KeyDelete,
	"up":     hotkey.KeyUp,
	"down":   hotkey.KeyDown,
	"left":   hotkey.KeyLeft,
	"right":  hotkey.KeyRight,
}
