//go:build windows

package hotkeys

import "golang.design/x/hotkey"

func lookupModifier(name string) (hotkey.Modifier, bool) {
	switch name {
	// Windows has no Command key; treat it as the Windows key so a shortcut
	// string written on macOS still registers to something sensible.
	case "cmd", "command", "meta", "super", "win":
		return hotkey.ModWin, true
	case "ctrl", "control":
		return hotkey.ModCtrl, true
	case "alt", "option", "opt":
		return hotkey.ModAlt, true
	case "shift":
		return hotkey.ModShift, true
	}
	return 0, false
}
