//go:build darwin

package hotkeys

import "golang.design/x/hotkey"

func lookupModifier(name string) (hotkey.Modifier, bool) {
	switch name {
	case "cmd", "command", "meta", "super", "win":
		return hotkey.ModCmd, true
	case "ctrl", "control":
		return hotkey.ModCtrl, true
	case "alt", "option", "opt":
		return hotkey.ModOption, true
	case "shift":
		return hotkey.ModShift, true
	}
	return 0, false
}
