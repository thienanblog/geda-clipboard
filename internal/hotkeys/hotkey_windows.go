//go:build windows

package hotkeys

import "golang.design/x/hotkey"

// Modifier and Key are the values RegisterHotKey expects. golang.design/x/hotkey
// wraps that call directly on Windows, which claims the combination without
// observing other keystrokes and so needs no permission; only the macOS side of
// that library uses an event tap, which is why macOS has its own implementation
// here instead.
type (
	Modifier hotkey.Modifier
	Key      hotkey.Key
)

func lookupModifier(name string) (Modifier, bool) {
	switch name {
	// Windows has no Command key; treat it as the Windows key so a shortcut
	// string written on macOS still registers to something sensible.
	case "cmd", "command", "meta", "super", "win":
		return Modifier(hotkey.ModWin), true
	case "ctrl", "control":
		return Modifier(hotkey.ModCtrl), true
	case "alt", "option", "opt":
		return Modifier(hotkey.ModAlt), true
	case "shift":
		return Modifier(hotkey.ModShift), true
	}
	return 0, false
}

func lookupKey(name string) (Key, bool) {
	key, ok := keyByName[name]
	return key, ok
}

var keyByName = map[string]Key{
	"a": Key(hotkey.KeyA), "b": Key(hotkey.KeyB), "c": Key(hotkey.KeyC),
	"d": Key(hotkey.KeyD), "e": Key(hotkey.KeyE), "f": Key(hotkey.KeyF),
	"g": Key(hotkey.KeyG), "h": Key(hotkey.KeyH), "i": Key(hotkey.KeyI),
	"j": Key(hotkey.KeyJ), "k": Key(hotkey.KeyK), "l": Key(hotkey.KeyL),
	"m": Key(hotkey.KeyM), "n": Key(hotkey.KeyN), "o": Key(hotkey.KeyO),
	"p": Key(hotkey.KeyP), "q": Key(hotkey.KeyQ), "r": Key(hotkey.KeyR),
	"s": Key(hotkey.KeyS), "t": Key(hotkey.KeyT), "u": Key(hotkey.KeyU),
	"v": Key(hotkey.KeyV), "w": Key(hotkey.KeyW), "x": Key(hotkey.KeyX),
	"y": Key(hotkey.KeyY), "z": Key(hotkey.KeyZ),

	"0": Key(hotkey.Key0), "1": Key(hotkey.Key1), "2": Key(hotkey.Key2),
	"3": Key(hotkey.Key3), "4": Key(hotkey.Key4), "5": Key(hotkey.Key5),
	"6": Key(hotkey.Key6), "7": Key(hotkey.Key7), "8": Key(hotkey.Key8),
	"9": Key(hotkey.Key9),

	"f1": Key(hotkey.KeyF1), "f2": Key(hotkey.KeyF2), "f3": Key(hotkey.KeyF3),
	"f4": Key(hotkey.KeyF4), "f5": Key(hotkey.KeyF5), "f6": Key(hotkey.KeyF6),
	"f7": Key(hotkey.KeyF7), "f8": Key(hotkey.KeyF8), "f9": Key(hotkey.KeyF9),
	"f10": Key(hotkey.KeyF10), "f11": Key(hotkey.KeyF11), "f12": Key(hotkey.KeyF12),

	"space":  Key(hotkey.KeySpace),
	"return": Key(hotkey.KeyReturn),
	"enter":  Key(hotkey.KeyReturn),
	"tab":    Key(hotkey.KeyTab),
	"escape": Key(hotkey.KeyEscape),
	"esc":    Key(hotkey.KeyEscape),
	"delete": Key(hotkey.KeyDelete),
	"up":     Key(hotkey.KeyUp),
	"down":   Key(hotkey.KeyDown),
	"left":   Key(hotkey.KeyLeft),
	"right":  Key(hotkey.KeyRight),
}

// windowsHotkey adapts the library's typed event channel to the plain signal
// the Manager consumes, and drops presses rather than blocking the library's
// message loop when the previous one is still being handled.
type windowsHotkey struct {
	hk      *hotkey.Hotkey
	keydown chan struct{}
	stop    chan struct{}
}

func newPlatformRegistration(mods []Modifier, key Key) registration {
	converted := make([]hotkey.Modifier, len(mods))
	for i, mod := range mods {
		converted[i] = hotkey.Modifier(mod)
	}
	return &windowsHotkey{
		hk:      hotkey.New(converted, hotkey.Key(key)),
		keydown: make(chan struct{}, 1),
	}
}

func (w *windowsHotkey) Register() error {
	if err := w.hk.Register(); err != nil {
		return err
	}
	stop := make(chan struct{})
	w.stop = stop
	events := w.hk.Keydown()
	go func() {
		for {
			select {
			case <-stop:
				return
			case _, ok := <-events:
				if !ok {
					return
				}
				select {
				case w.keydown <- struct{}{}:
				default:
				}
			}
		}
	}()
	return nil
}

func (w *windowsHotkey) Unregister() error {
	if w.stop != nil {
		close(w.stop)
		w.stop = nil
	}
	return w.hk.Unregister()
}

func (w *windowsHotkey) Keydown() <-chan struct{} { return w.keydown }
