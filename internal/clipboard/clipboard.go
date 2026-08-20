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
	"errors"
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
	// Remote is set when the copy came from another device over Universal
	// Clipboard rather than from an app on this machine. The pasteboard says
	// only that much: never which device, and never an app to attribute it to.
	Remote bool
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

// RestoreFocus brings the app captured by RememberFrontmost back to the front,
// reporting whether it got there. It needs no permission on either platform,
// which is what makes it the fallback when Paste is unavailable: the entry is
// on the clipboard and the user presses the paste shortcut themselves, in the
// app they were working in rather than in whatever the window server picked.
func RestoreFocus() bool { return restoreFocus() }

// PasteSupported reports whether this build can paste an entry back by itself.
// It is false in the Mac App Store build, which is compiled without the
// axpaste tag because App Review forbids using Accessibility to automate other
// applications. Callers must offer the copy-and-refocus path instead rather
// than reporting a failure the user cannot fix.
func PasteSupported() bool { return pasteSupported() }

// Paste refocuses the app captured by RememberFrontmost and sends the paste
// keystroke to it. Returns ErrPasteUnsupported when the build has no keystroke
// path, and ErrNoPastePermission when the OS withholds permission for one.
func Paste() error { return paste() }

// ErrNoPastePermission reports that the OS withholds permission to synthesise
// the paste keystroke. Only macOS gates this, behind Accessibility; the
// Windows implementation never returns it.
var ErrNoPastePermission = errors.New("permission required to paste")

// ErrPasteUnsupported reports that this build has no keystroke path at all, so
// no permission would change the outcome.
var ErrPasteUnsupported = errors.New("this build cannot paste automatically")

// HasPastePermission reports whether the OS allows this process to synthesise
// keystrokes. When prompt is true the user may be asked to grant it.
func HasPastePermission(prompt bool) bool { return hasPastePermission(prompt) }

// OpenPastePermissionSettings reveals the settings pane where the user grants
// permission to synthesise keystrokes.
//
// This is not redundant with the prompt HasPastePermission can raise. macOS
// shows that prompt once per TCC record and silently does nothing every time
// after, so a button wired only to the prompt looks broken to exactly the user
// who needs it: the one who dismissed it the first time.
func OpenPastePermissionSettings() error { return openPastePermissionSettings() }

// pendingReadTicks is how many further polls a change gets when the counter has
// moved but nothing readable is on the clipboard yet.
//
// A Universal Clipboard payload is pulled from the other device on demand, so
// it is not on the pasteboard when the counter moves. Measured against an
// iPhone, the read simply blocks until the transfer finishes -- 1.3s for a
// short text, 0.4s for a 7MB photo -- and returns the payload on the first
// attempt, so this budget is rarely spent. It covers the case where the
// transfer fails instead and the read comes back empty: giving up on the first
// one would lose the entry for good, because the change counter has already
// been consumed and never repeats.
const pendingReadTicks = 10

// Watcher polls the clipboard and reports each new copy.
type Watcher struct {
	// Interval between polls. Defaults to 300ms.
	Interval time.Duration

	// OnChange is called for every observed copy, on a background goroutine.
	OnChange func(Snapshot, App)

	// OS access, indirected so the polling logic can be tested without a real
	// clipboard. Run fills in whichever are unset with the platform calls.
	counter func() int64
	reader  func() (Snapshot, error)
	source  func() App

	// ignore holds change counters produced by our own writes.
	ignore   map[int64]struct{}
	ignoreCh chan int64

	// Polling state, owned by the goroutine running Run.
	last          int64
	pendingChange int64
	pendingLeft   int
}

// NewWatcher builds a Watcher.
func NewWatcher(onChange func(Snapshot, App)) *Watcher {
	w := &Watcher{
		Interval: 300 * time.Millisecond,
		OnChange: onChange,
	}
	w.defaults()
	return w
}

// defaults fills in what a Watcher built as a struct literal leaves unset, so
// the zero value polls the real clipboard rather than panicking on a nil call.
func (w *Watcher) defaults() {
	if w.counter == nil {
		w.counter = changeCount
	}
	if w.reader == nil {
		w.reader = read
	}
	if w.source == nil {
		w.source = frontmost
	}
	if w.ignore == nil {
		w.ignore = make(map[int64]struct{})
	}
	if w.ignoreCh == nil {
		w.ignoreCh = make(chan int64, 16)
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
	w.defaults()

	interval := w.Interval
	if interval <= 0 {
		interval = 300 * time.Millisecond
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Seed with the current counter so the clipboard's pre-existing contents
	// are not reported as a fresh copy on launch.
	w.last = w.counter()

	for {
		select {
		case <-ctx.Done():
			return
		case c := <-w.ignoreCh:
			w.ignore[c] = struct{}{}
		case <-ticker.C:
			w.tick()
		}
	}
}

// tick performs one poll. It is the whole of the watcher's logic; Run only
// supplies the clock.
func (w *Watcher) tick() {
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

	if current := w.counter(); current != w.last {
		w.last = current
		// A newer copy supersedes any change still waiting for its payload.
		w.pendingChange = 0
		w.pendingLeft = 0

		if _, skip := w.ignore[current]; skip {
			delete(w.ignore, current)
			return
		}
		w.pendingChange = current
		w.pendingLeft = pendingReadTicks
	}

	if w.pendingChange == 0 {
		return
	}

	snap, err := w.reader()
	if err != nil || snap.Kind == KindNone {
		// Nothing readable behind the change. Worth retrying only when the
		// payload may still be on its way: the remote marker is on the
		// pasteboard from the moment the counter moves, ahead of the data
		// itself, so it is readable even now. Anything else -- a file copied
		// in Finder, say -- is simply not a payload this app records, and
		// re-reading it every tick for three seconds would be waste.
		if err == nil && !snap.Remote {
			w.pendingChange = 0
			w.pendingLeft = 0
			return
		}
		w.pendingLeft--
		if w.pendingLeft <= 0 {
			w.pendingChange = 0
		}
		return
	}

	snap.Change = w.pendingChange
	w.pendingChange = 0
	w.pendingLeft = 0

	// Remote copies need no dedupe of their own: macOS was measured to bump the
	// change counter exactly once per Universal Clipboard copy, so each one is a
	// copy the user actually made.
	if w.OnChange != nil {
		w.OnChange(snap, w.source())
	}
}
