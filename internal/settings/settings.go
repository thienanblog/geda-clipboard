// Package settings holds user preferences and persists them as JSON.
package settings

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"geda-clipboard/internal/appdir"
)

// Placement values for Settings.PopupPlacement.
const (
	// PlacementCursor opens the popup at the mouse pointer, the way a context
	// menu does. This is the default: the pointer is where the user is already
	// looking when they press the shortcut.
	PlacementCursor = "cursor"
	// PlacementMenuBar hangs the popup under the menu bar / tray icon.
	PlacementMenuBar = "menubar"
)

// Settings mirrors the preferences surface exposed in the UI. Field names are
// lowerCamelCase in JSON so the frontend can use them directly.
type Settings struct {
	// MaxItems caps how many unpinned entries are kept. Pinned entries are
	// always kept regardless of this limit.
	MaxItems int `json:"maxItems"`

	// NotifyOnCopy posts a notification each time a new entry is captured.
	NotifyOnCopy bool `json:"notifyOnCopy"`
	// NotifyOnPaste posts a notification each time an entry is pasted back.
	NotifyOnPaste bool `json:"notifyOnPaste"`

	// PasteOnSelect pastes into the previously focused app on selection. When
	// false, selecting an entry only puts it on the clipboard.
	PasteOnSelect bool `json:"pasteOnSelect"`

	// Hotkey toggles the popup, e.g. "cmd+shift+v" or "ctrl+shift+v".
	Hotkey string `json:"hotkey"`

	// LaunchAtLogin registers the app as a login item.
	LaunchAtLogin bool `json:"launchAtLogin"`

	// IgnoredApps lists application names whose copies are not recorded.
	IgnoredApps []string `json:"ignoredApps"`

	// IgnoreConcealed skips entries marked confidential by the source app,
	// which is how password managers ask to be excluded.
	IgnoreConcealed bool `json:"ignoreConcealed"`
	// IgnoreTransient skips entries the source app marked as transient.
	IgnoreTransient bool `json:"ignoreTransient"`

	// CaptureImages records images as well as text.
	CaptureImages bool `json:"captureImages"`

	// PopupWidth/PopupHeight size the visible popup panel in logical pixels.
	// The window itself is wider when previews are on: it carries a transparent
	// gutter for the preview card (see previewGutter in app.go).
	PopupWidth  int `json:"popupWidth"`
	PopupHeight int `json:"popupHeight"`

	// PopupPlacement decides where the popup opens: PlacementCursor or
	// PlacementMenuBar.
	PopupPlacement string `json:"popupPlacement"`

	// PreviewOnHover floats the detail card beside the list for the entry under
	// the cursor.
	PreviewOnHover bool `json:"previewOnHover"`

	// LayoutVersion records which popup layout the stored sizes were chosen
	// for, so a layout change can adjust them once. The app owns it: whatever
	// arrives from the preferences UI is overwritten on save.
	LayoutVersion int `json:"layoutVersion"`
}

// layoutFlyout is the layout in which the detail column moved out of the panel
// and became a card floating beside it. Widths picked for the docked column are
// far wider than a list-only panel needs.
const layoutFlyout = 1

// Defaults returns the settings a fresh install starts with.
func Defaults() Settings {
	return Settings{
		MaxItems:        200,
		NotifyOnCopy:    true,
		NotifyOnPaste:   true,
		PasteOnSelect:   true,
		Hotkey:          defaultHotkey,
		LaunchAtLogin:   false,
		IgnoredApps:     []string{},
		IgnoreConcealed: true,
		IgnoreTransient: true,
		CaptureImages:   true,
		PopupWidth:      420,
		PopupHeight:     520,
		PopupPlacement:  PlacementCursor,
		PreviewOnHover:  true,
		LayoutVersion:   layoutFlyout,
	}
}

// migrate brings a stored configuration up to the current layout. It runs once:
// normalise stamps the current layout onto everything that is saved afterwards,
// so a width the user picks under the new layout is never touched again.
func (s *Settings) migrate() {
	if s.LayoutVersion < layoutFlyout {
		// Only the docked detail column ever justified a popup this wide.
		if s.PopupWidth >= 600 {
			s.PopupWidth = Defaults().PopupWidth
		}
	}
}

// normalise clamps values that would break the UI or the store.
func (s *Settings) normalise() {
	d := Defaults()
	if s.MaxItems < 10 {
		s.MaxItems = 10
	}
	if s.MaxItems > 2000 {
		s.MaxItems = 2000
	}
	if s.Hotkey == "" {
		s.Hotkey = d.Hotkey
	}
	// Bounds match the min/max the preferences UI advertises. The upper ones
	// matter: a number field does not enforce max for a typed value, and a
	// popup larger than the screen cannot be dismissed from its own footer.
	if s.PopupWidth < 300 {
		s.PopupWidth = d.PopupWidth
	}
	if s.PopupWidth > 1600 {
		s.PopupWidth = 1600
	}
	if s.PopupHeight < 240 {
		s.PopupHeight = d.PopupHeight
	}
	if s.PopupHeight > 1200 {
		s.PopupHeight = 1200
	}
	// An unknown placement -- a hand-edited file, or one written by a build that
	// knew a mode this one does not -- falls back to the default rather than
	// leaving the popup with nowhere to go.
	switch s.PopupPlacement {
	case PlacementCursor, PlacementMenuBar:
	default:
		s.PopupPlacement = d.PopupPlacement
	}
	if s.IgnoredApps == nil {
		s.IgnoredApps = []string{}
	}
	// Stamped rather than trusted: the layout a configuration was written for
	// is the app's own bookkeeping, not a preference.
	s.LayoutVersion = layoutFlyout
}

// Manager loads, serves and saves the settings.
type Manager struct {
	mu      sync.RWMutex
	current Settings
	path    string

	// onChange is invoked after every successful save so the app can react to
	// e.g. a new hotkey or history limit.
	onChange func(Settings)
}

// Load reads settings from disk, falling back to defaults for a fresh install
// or an unreadable file. The Manager is always usable, even when an error is
// returned: callers report the problem but keep running on defaults.
func Load() (*Manager, error) {
	m := &Manager{current: Defaults()}

	dir, err := appdir.Data()
	if err != nil {
		// No data directory at all. Serve defaults from memory; Save will
		// report that it has nowhere to write.
		return m, err
	}
	m.path = filepath.Join(dir, "settings.json")

	raw, err := os.ReadFile(m.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return m, nil
		}
		return m, err
	}

	// Unmarshal over the defaults so keys absent from an older file keep their
	// default rather than becoming zero.
	loaded := Defaults()
	// A file predating the field has to read back as version 0, which the
	// default would otherwise hide.
	loaded.LayoutVersion = 0
	if err := json.Unmarshal(raw, &loaded); err != nil {
		// Corrupt file: keep defaults rather than refusing to start.
		return m, nil
	}
	loaded.migrate()
	loaded.normalise()
	m.current = loaded
	return m, nil
}

// OnChange registers a callback invoked after each save.
func (m *Manager) OnChange(fn func(Settings)) {
	m.mu.Lock()
	m.onChange = fn
	m.mu.Unlock()
}

// Get returns a copy of the current settings.
func (m *Manager) Get() Settings {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c := m.current
	c.IgnoredApps = append([]string(nil), c.IgnoredApps...)
	return c
}

// Save validates, stores and persists s.
func (m *Manager) Save(s Settings) (Settings, error) {
	s.normalise()

	m.mu.Lock()
	m.current = s
	path := m.path
	cb := m.onChange
	m.mu.Unlock()

	if path == "" {
		return s, errors.New("no data directory: preferences cannot be saved")
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return s, err
	}
	if err := appdir.WriteAtomic(path, data, 0o600); err != nil {
		return s, err
	}
	if cb != nil {
		cb(s)
	}
	return s, nil
}

// IsIgnored reports whether copies from app should be dropped.
func (s Settings) IsIgnored(app string) bool {
	if app == "" {
		return false
	}
	for _, ignored := range s.IgnoredApps {
		if equalFold(ignored, app) {
			return true
		}
	}
	return false
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if 'A' <= ca && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if 'A' <= cb && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
