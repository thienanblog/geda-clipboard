// Package clipboard reads and writes the system clipboard, identifies the app
// that owns a copy, and pastes an entry back into the previously focused app.
//
// Change detection is driven by the OS clipboard sequence/change counter rather
// than by diffing content. That matters: copying the same text twice does not
// change the content, but it is still two copies, and the UI reports a copy
// count per entry.
package clipboard

import (
	"context"
	"time"
)

// Kind is the payload type found on the clipboard.
type Kind int

const (
	KindNone Kind = iota
	KindText
	KindImage
)

// Snapshot is the clipboard state at one point in time.
type Snapshot struct {
	// Change is the OS-provided change counter. It increases on every copy,
	// including copies of identical content.
	Change int64

	Kind  Kind
	Text  string
	Image []byte // PNG

	// Concealed is set when the source app marked the payload confidential,
	// which is how password managers ask not to be recorded.
	Concealed bool
	// Transient is set when the source app marked the payload as temporary.
	Transient bool
}

// App identifies an application that owns or receives a copy.
type App struct {
	Name     string
	BundleID string
}

// ChangeCount returns the current clipboard change counter.
func ChangeCount() int64 { return changeCount() }

// Read returns the current clipboard contents.
func Read() (Snapshot, error) { return read() }

// WriteText places text on the clipboard and returns the resulting change
// counter, so a caller can recognise its own write and avoid re-recording it.
func WriteText(s string) (int64, error) { return writeText(s) }

// WriteImage places PNG bytes on the clipboard, returning the new change
// counter.
func WriteImage(png []byte) (int64, error) { return writeImage(png) }

// Frontmost returns the application currently in the foreground.
func Frontmost() App { return frontmost() }

// AppIconPNG returns the icon of an installed application as PNG bytes, sized
// for display in the list. Returns nil when unavailable.
func AppIconPNG(bundleID string, px int) []byte { return appIconPNG(bundleID, px) }

// RememberFrontmost records the foreground app so it can be refocused later.
// Call this before showing the popup, which necessarily steals focus.
func RememberFrontmost() { rememberFrontmost() }

// Paste refocuses the app captured by RememberFrontmost and sends the paste
// keystroke to it.
func Paste() error { return paste() }

// HasPastePermission reports whether the OS allows this process to synthesise
// keystrokes. When prompt is true the user may be asked to grant it.
func HasPastePermission(prompt bool) bool { return hasPastePermission(prompt) }

// Watcher polls the clipboard and reports each new copy.
type Watcher struct {
	// Interval between polls. Defaults to 300ms.
	Interval time.Duration

	// OnChange is called for every observed copy, on a background goroutine.
	OnChange func(Snapshot, App)

	// ignore holds change counters produced by our own writes.
	ignore   map[int64]struct{}
	ignoreCh chan int64
}

// NewWatcher builds a Watcher.
func NewWatcher(onChange func(Snapshot, App)) *Watcher {
	return &Watcher{
		Interval: 300 * time.Millisecond,
		OnChange: onChange,
		ignore:   make(map[int64]struct{}),
		ignoreCh: make(chan int64, 16),
	}
}

// Ignore tells the watcher to skip a change counter. Used for clipboard writes
// the app makes itself, which should not be recorded as new copies.
func (w *Watcher) Ignore(change int64) {
	if change == 0 {
		return
	}
	select {
	case w.ignoreCh <- change:
	default:
	}
}

// Run polls until ctx is cancelled.
func (w *Watcher) Run(ctx context.Context) {
	interval := w.Interval
	if interval <= 0 {
		interval = 300 * time.Millisecond
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Seed with the current counter so the clipboard's pre-existing contents
	// are not reported as a fresh copy on launch.
	last := changeCount()

	for {
		select {
		case <-ctx.Done():
			return
		case c := <-w.ignoreCh:
			w.ignore[c] = struct{}{}
		case <-ticker.C:
			// Drain pending ignores first so a write racing this tick counts.
			for {
				select {
				case c := <-w.ignoreCh:
					w.ignore[c] = struct{}{}
					continue
				default:
				}
				break
			}

			current := changeCount()
			if current == last {
				continue
			}
			last = current

			if _, skip := w.ignore[current]; skip {
				delete(w.ignore, current)
				continue
			}

			snap, err := read()
			if err != nil || snap.Kind == KindNone {
				continue
			}
			// Re-check: reading takes time and the clipboard may have moved on.
			snap.Change = current

			if w.OnChange != nil {
				w.OnChange(snap, frontmost())
			}
		}
	}
}
