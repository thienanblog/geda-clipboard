// Package notify posts native desktop notifications.
package notify

import "strings"

// Notification is a single desktop notification.
type Notification struct {
	Title    string
	Subtitle string
	Body     string
}

// Send posts the notification. Errors are returned but are rarely actionable:
// a failed notification should never interrupt clipboard handling.
func Send(n Notification) error {
	return send(n)
}

// RequestPermission asks the OS for notification permission, if the platform
// requires it. Safe to call more than once.
func RequestPermission() { requestPermission() }

// Status describes whether the OS will deliver our notifications.
type Status string

const (
	// StatusUnknown means the platform does not report a status; notifications
	// are attempted regardless.
	StatusUnknown Status = "unknown"
	// StatusNotDetermined means the user has not yet answered the prompt.
	StatusNotDetermined Status = "notDetermined"
	// StatusDenied means notifications are switched off for this app, so the
	// copy/paste alerts will not appear until the user re-enables them.
	StatusDenied Status = "denied"
	// StatusAuthorized means notifications will be delivered.
	StatusAuthorized Status = "authorized"
)

// Permission reports whether notifications will actually be delivered.
func Permission() Status { return permission() }

// OpenSettings opens the OS notification settings for this app so the user can
// enable notifications after denying them.
func OpenSettings() error { return openSettings() }

// Summarise shortens s to at most max runes for use as a notification body,
// collapsing whitespace so multi-line payloads stay readable.
func Summarise(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if max <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return strings.TrimRight(string(runes[:max]), " ") + "…"
}
