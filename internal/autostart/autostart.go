// Package autostart registers or unregisters the app to launch at login.
package autostart

// Set enables or disables launching at login.
func Set(enabled bool) error { return set(enabled) }

// Enabled reports whether the app is currently registered to launch at login.
func Enabled() bool { return isEnabled() }
