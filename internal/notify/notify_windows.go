//go:build windows

package notify

import (
	"strings"

	"github.com/gen2brain/beeep"
)

const appID = "Geda Clipboard"

func requestPermission() {
	// Windows toasts need no per-app permission grant.
}

func permission() Status { return StatusUnknown }

func openSettings() error {
	// Notifications are configured per-app under Settings > System >
	// Notifications; there is no reliable deep link, so this is a no-op and the
	// UI never offers the button (permission() never reports denied).
	return nil
}

func send(n Notification) error {
	beeep.AppName = appID

	// Windows toasts have a single body, so fold the subtitle in.
	body := n.Body
	if n.Subtitle != "" {
		if body == "" {
			body = n.Subtitle
		} else {
			body = n.Subtitle + " — " + body
		}
	}

	title := n.Title
	if title == "" {
		title = appID
	}

	// beeep renders a toast via the notification API, falling back to a
	// message box style alert on older Windows builds.
	return beeep.Notify(title, strings.TrimSpace(body), "")
}
