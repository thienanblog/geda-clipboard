package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"geda-clipboard/internal/autostart"
	"geda-clipboard/internal/clipboard"
	"geda-clipboard/internal/hotkeys"
	"geda-clipboard/internal/imageutil"
	"geda-clipboard/internal/notify"
	"geda-clipboard/internal/settings"
	"geda-clipboard/internal/store"
	"geda-clipboard/internal/tray"
	"geda-clipboard/internal/window"
)

const (
	// Thumbnails only ever render at ~74px tall in the list and ~110px in the
	// detail card, so these bounds are already 2x the largest display size.
	// Going larger just inflates the history index, which is rewritten whenever
	// the history changes.
	thumbMaxW = 360
	thumbMaxH = 150

	// Size of the source-app icon captured alongside each entry.
	appIconPx = 32

	// Provenance shown for a copy that arrived from another device over
	// Universal Clipboard. The pasteboard never says which device it was, so
	// this is as specific as the label can honestly be, and there is no icon.
	remoteSourceName = "Universal Clipboard"

	// The settings view needs more room than the popup list.
	settingsWidth  = 760
	settingsHeight = 640

	// previewGutter is the width of the transparent strip the popup window keeps
	// to the left of the list, so the hover preview card has somewhere to appear
	// beside the row it describes. The list panel itself stays PopupWidth wide;
	// the gutter only costs window area, not visible chrome.
	previewGutter = 300

	// panelRadius matches --radius-panel in the stylesheet: the frosted window
	// material is rounded to the same corners the panel draws for itself.
	panelRadius = 12

	// The native window shadow is unusable on a translucent WebView window (see
	// internal/window), so the panel casts its own in CSS -- and a shadow has
	// nowhere to fall unless the window is bigger than the panel. These are the
	// transparent margins the window keeps for it, sized to what the shadow
	// actually reaches: 0 8px 24px spreads half its blur, 12px, outwards, and
	// 8px of that is spent moving down. Keeping the top margin as small as it
	// can be is what lets the popup still sit close under the menu bar.
	// Matches --shadow-top, --shadow-side and --shadow-bottom in the stylesheet.
	shadowTop    = 8
	shadowSide   = 16
	shadowBottom = 24
)

// View identifies which screen the single window is currently showing.
type View string

const (
	ViewPopup    View = "popup"
	ViewSettings View = "settings"
)

// App is the Wails-bound application object. Every exported method is callable
// from the frontend.
type App struct {
	ctx    context.Context
	cancel context.CancelFunc

	trayIcon []byte

	settings *settings.Manager
	store    *store.Store
	watcher  *clipboard.Watcher
	hotkeys  *hotkeys.Manager

	mu        sync.Mutex
	visible   bool
	view      View
	anchor    tray.Anchor
	iconCache map[string]string
}

// NewApp constructs the application. Settings and history are loaded eagerly so
// the window can be sized correctly before Wails starts.
func NewApp(trayIcon []byte) *App {
	a := &App{
		trayIcon:  trayIcon,
		view:      ViewPopup,
		iconCache: make(map[string]string),
	}

	// Load always returns a usable Manager, so an error here means "running on
	// defaults, changes will not persist" rather than "cannot start".
	sm, err := settings.Load()
	if err != nil {
		log.Println("settings: falling back to defaults:", err)
	}
	a.settings = sm

	cfg := a.settings.Get()

	st, err := store.Open(cfg.MaxItems)
	if err != nil {
		log.Println("history: unavailable, running without persistence:", err)
	}
	a.store = st

	return a
}

// PopupWidth is the configured popup width, used to size the window at startup.
func (a *App) PopupWidth() int { return a.settings.Get().PopupWidth }

// PopupHeight is the configured popup height.
func (a *App) PopupHeight() int { return a.settings.Get().PopupHeight }

// PopupGutter is the width of the transparent strip the popup window reserves
// on its left for the hover preview card, or 0 when previews are off. The
// frontend needs it to lay the panel out flush with the right of the window.
func (a *App) PopupGutter() int {
	if !a.settings.Get().PreviewOnHover {
		return 0
	}
	return previewGutter
}

// OnStartup wires up the tray, clipboard watcher and hotkey once Wails is ready.
func (a *App) OnStartup(ctx context.Context) {
	a.ctx = ctx

	watchCtx, cancel := context.WithCancel(ctx)
	a.cancel = cancel

	cfg := a.settings.Get()

	if err := tray.Start(a, a.trayIcon, "Geda Clipboard"); err != nil {
		log.Println("tray:", err)
	}

	a.watcher = clipboard.NewWatcher(a.onClipboardChange)
	go a.watcher.Run(watchCtx)

	a.hotkeys = hotkeys.New(a.TogglePopup)
	if err := a.hotkeys.Register(cfg.Hotkey); err != nil {
		log.Println("hotkey:", err)
	}

	a.settings.OnChange(a.onSettingsChanged)

	// The login item records where the bundle is, so anything that moves the
	// app -- a rename, a drag out of Downloads -- leaves it pointing at a path
	// that no longer exists, and the app silently stops starting at login.
	// Rewriting it on launch keeps it aimed at whatever is actually running.
	if cfg.LaunchAtLogin {
		if err := autostart.Set(true); err != nil {
			log.Println("autostart:", err)
		}
	}

	window.SetDockIconVisible(cfg.ShowDockIcon)

	notify.RequestPermission()
}

// OnShutdown releases resources.
func (a *App) OnShutdown(ctx context.Context) {
	if a.cancel != nil {
		a.cancel()
	}
	if a.hotkeys != nil {
		a.hotkeys.Unregister()
	}
	tray.Stop()
	if a.store != nil {
		if err := a.store.Close(); err != nil {
			log.Println("history: final save failed:", err)
		}
	}
}

func (a *App) onSettingsChanged(cfg settings.Settings) {
	if a.store != nil {
		a.store.SetMaxItems(cfg.MaxItems)
	}

	if a.hotkeys != nil && a.hotkeys.Spec() != cfg.Hotkey {
		if err := a.hotkeys.Register(cfg.Hotkey); err != nil {
			log.Println("hotkey:", err)
		}
	}

	if err := autostart.Set(cfg.LaunchAtLogin); err != nil {
		log.Println("autostart:", err)
	}

	window.SetDockIconVisible(cfg.ShowDockIcon)
}

// ---------------------------------------------------------------------------
// Clipboard capture
// ---------------------------------------------------------------------------

func (a *App) onClipboardChange(snap clipboard.Snapshot, source clipboard.App) {
	if a.store == nil {
		return
	}
	cfg := a.settings.Get()

	if cfg.IgnoreConcealed && snap.Concealed {
		return
	}
	if cfg.IgnoreTransient && snap.Transient {
		return
	}
	if snap.Remote {
		// A copy handed over from another device has no app behind it on this
		// machine. Whatever happens to be frontmost here did not produce it, so
		// naming it would be a lie -- and matching it against the ignore list
		// would drop an unrelated copy.
		source = clipboard.App{Name: remoteSourceName}
	} else if cfg.IsIgnored(source.Name) {
		return
	}
	if snap.Kind == clipboard.KindImage && !cfg.CaptureImages {
		return
	}

	capture := store.Capture{
		SourceApp:     source.Name,
		SourceIconKey: source.BundleID,
		SourceIcon:    a.appIcon(source.BundleID),
		At:            time.Now(),
	}

	switch snap.Kind {
	case clipboard.KindText:
		capture.Kind = store.KindText
		capture.Text = snap.Text
	case clipboard.KindImage:
		thumb, w, h, err := imageutil.Thumbnail(snap.Image, thumbMaxW, thumbMaxH)
		if err != nil {
			log.Println("thumbnail:", err)
			return
		}
		capture.Kind = store.KindImage
		capture.Image = snap.Image
		capture.Thumb = thumb
		capture.ImageW = w
		capture.ImageH = h
		// The decode above is the app's single largest allocation.
		defer releaseMemory()
	default:
		return
	}

	item, isNew, err := a.store.Add(capture)
	if err != nil {
		log.Println("history add:", err)
		return
	}

	a.emitHistoryChanged()

	if cfg.NotifyOnCopy {
		a.notifyCopied(item, isNew)
	}
}

func (a *App) notifyCopied(item *store.Item, isNew bool) {
	title := "Copied"
	if !isNew && item.CopyCount > 1 {
		title = fmt.Sprintf("Copied again (%d×)", item.CopyCount)
	}

	subtitle := ""
	if item.SourceApp != "" {
		subtitle = "from " + item.SourceApp
	}

	body := notify.Summarise(item.Preview(), 120)
	if item.Kind == store.KindImage {
		body = fmt.Sprintf("Image · %d × %d", item.ImageW, item.ImageH)
	}

	if err := notify.Send(notify.Notification{
		Title:    title,
		Subtitle: subtitle,
		Body:     body,
	}); err != nil {
		log.Println("notify:", err)
	}
}

// appIcon returns a cached data URL for an app's icon.
func (a *App) appIcon(bundleID string) string {
	if bundleID == "" {
		return ""
	}

	a.mu.Lock()
	cached, ok := a.iconCache[bundleID]
	a.mu.Unlock()
	if ok {
		return cached
	}

	url := imageutil.DataURL(clipboard.AppIconPNG(bundleID, appIconPx))
	if url == "" {
		// The lookup can fail for reasons that pass: an app on a volume that is
		// briefly unavailable, or one Launch Services has not indexed yet.
		// Caching the miss would hide that app's icon until the next restart.
		return ""
	}

	a.mu.Lock()
	a.iconCache[bundleID] = url
	a.mu.Unlock()

	return url
}

func (a *App) emitHistoryChanged() {
	if a.ctx == nil {
		return
	}
	wruntime.EventsEmit(a.ctx, "history:changed")
}

// ---------------------------------------------------------------------------
// Tray handling
// ---------------------------------------------------------------------------

// OnLeftClick toggles the popup, anchored under the tray icon.
func (a *App) OnLeftClick(anchor tray.Anchor) {
	a.mu.Lock()
	a.anchor = anchor
	visible := a.visible && a.view == ViewPopup
	a.mu.Unlock()

	if visible {
		a.HidePopup()
		return
	}
	a.showPopupAt(anchor)
}

// OnRightClick behaves the same as a left click; the popup itself carries the
// menu actions rather than a separate context menu.
func (a *App) OnRightClick(anchor tray.Anchor) {
	a.OnLeftClick(anchor)
}

// TogglePopup shows or hides the popup. Bound to the global hotkey and callable
// from the frontend.
func (a *App) TogglePopup() {
	a.mu.Lock()
	visible := a.visible && a.view == ViewPopup
	anchor := a.anchor
	a.mu.Unlock()

	if visible {
		a.HidePopup()
		return
	}
	a.showPopupAt(anchor)
}

func (a *App) showPopupAt(anchor tray.Anchor) {
	if a.ctx == nil {
		return
	}

	// Capture the app the user was working in *before* our window takes focus,
	// so a later paste goes back to the right place.
	clipboard.RememberFrontmost()

	cfg := a.settings.Get()
	w, h := cfg.PopupWidth, cfg.PopupHeight
	gutter := 0
	if cfg.PreviewOnHover {
		gutter = previewGutter
	}

	a.mu.Lock()
	a.view = ViewPopup
	a.visible = true
	if anchor.Work.W != 0 {
		a.anchor = anchor
	} else {
		anchor = a.anchor
	}
	a.mu.Unlock()

	wruntime.EventsEmit(a.ctx, "view:changed", string(ViewPopup))
	wruntime.WindowSetSize(a.ctx, w+gutter+2*shadowSide, h+shadowTop+shadowBottom)
	// The frosted material has to stop where the panel starts, or the gutter
	// and the shadow margins show as a grey slab instead of being see-through.
	window.SetPanelInset(gutter+shadowSide, shadowTop, shadowSide, shadowBottom, panelRadius)

	place, ok := popupPosition(cfg.PopupPlacement, anchor, w, h, gutter+shadowSide, shadowTop)
	switch {
	case !ok:
		// Neither the pointer nor the tray icon could be located.
		wruntime.WindowCenter(a.ctx)
	case moveWindow(place.GlobalX, place.GlobalY):
		// Placed in global coordinates, so the popup can open on whichever
		// display the pointer or the icon is on.
	default:
		// The window could not be found natively. Wails resolves these
		// coordinates against the screen the window already occupies, which is
		// right whenever that is also the screen being aimed at.
		wruntime.WindowSetPosition(a.ctx, place.RelX, place.RelY)
	}

	wruntime.WindowShow(a.ctx)
}

// placement is a resolved popup position, given both ways: relative to the work
// area of the screen it was computed for, which is what Wails takes, and in the
// global space spanning every display, which is what moveWindow takes.
type placement struct {
	RelX, RelY       int
	GlobalX, GlobalY int
}

// Both are variables so the placement logic can be tested without a window
// server to ask for the pointer or a window to move.
var (
	cursorAnchor = tray.CursorAnchor
	moveWindow   = window.MoveTo
)

// popupPosition resolves where the popup should open for the given placement
// mode. Each mode falls back to the other when its own source is unavailable:
// the tray anchor is unknown until the icon has been clicked once, and the
// pointer cannot be read on every platform. ok is false only when neither can
// answer, which leaves the caller to centre the window.
//
// w and h are the size of the visible list panel; insetX and insetY are how far
// inside the window the panel's top-left corner sits -- the preview gutter plus
// the side shadow margin across, the top shadow margin down. The placement is
// computed for the panel and then shifted back by those insets, so the list
// still lands where the user aimed -- unless that would push the window off the
// top or left edge, in which case the window stops there and the panel sits
// inset-deep in from where it was aimed.
func popupPosition(mode string, anchor tray.Anchor, w, h, insetX, insetY int) (placement, bool) {
	resolve := func(a tray.Anchor, at func(int, int) (int, int)) placement {
		x, y := at(w, h)
		if x -= insetX; x < 0 {
			x = 0
		}
		if y -= insetY; y < 0 {
			y = 0
		}
		globalX, globalY := a.Global(x, y)
		return placement{RelX: x, RelY: y, GlobalX: globalX, GlobalY: globalY}
	}

	atCursor := func() (placement, bool) {
		cursor, ok := cursorAnchor()
		if !ok {
			return placement{}, false
		}
		return resolve(cursor, cursor.CursorPopupPosition), true
	}

	atIcon := func() (placement, bool) {
		if anchor.Work.W == 0 {
			return placement{}, false
		}
		return resolve(anchor, anchor.PopupPosition), true
	}

	if mode == settings.PlacementCursor {
		if p, ok := atCursor(); ok {
			return p, true
		}
		return atIcon()
	}

	if p, ok := atIcon(); ok {
		return p, true
	}
	return atCursor()
}

// releasing keeps at most one release in flight, so a burst of copies cannot
// queue up a collection per item.
var releasing atomic.Bool

// releaseMemory hands the Go heap's free spans back to the operating system.
//
// Reserved for capturing an image, which is the one point where this app
// allocates enough for the difference to show: decoding a full-screen
// screenshot to build a thumbnail costs tens of megabytes for a moment, and the
// collector is in no hurry to return that arena, so a background utility ends
// up holding a high-water mark it will never need again. It is deliberately not
// called when the popup closes -- that path allocates little, and it closes
// often enough that a forced stop-the-world collection each time would cost
// more than the handful of bytes it recovers.
func releaseMemory() {
	if !releasing.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer releasing.Store(false)
		debug.FreeOSMemory()
	}()
}

// HidePopup hides the window.
func (a *App) HidePopup() {
	if a.ctx == nil {
		return
	}
	a.mu.Lock()
	a.visible = false
	a.view = ViewPopup
	a.mu.Unlock()

	wruntime.WindowHide(a.ctx)
}

// OnWindowBlur is called by the frontend when the window loses focus. The popup
// dismisses itself, matching how menu bar popovers behave; the settings view
// stays put so the user can switch to another app while editing preferences.
func (a *App) OnWindowBlur() {
	a.mu.Lock()
	shouldHide := a.visible && a.view == ViewPopup
	a.mu.Unlock()

	if shouldHide {
		a.HidePopup()
	}
}

// ---------------------------------------------------------------------------
// Frontend API: history
// ---------------------------------------------------------------------------

// List returns history entries matching query, pinned entries first.
func (a *App) List(query string) []*store.Item {
	if a.store == nil {
		return []*store.Item{}
	}
	items := a.store.List(query)
	if items == nil {
		return []*store.Item{}
	}
	return items
}

// Count returns the number of stored entries.
func (a *App) Count() int {
	if a.store == nil {
		return 0
	}
	return a.store.Count()
}

// FullImage returns the full-size PNG for an image entry as a data URL.
func (a *App) FullImage(id string) (string, error) {
	if a.store == nil {
		return "", fmt.Errorf("history unavailable")
	}
	raw, err := a.store.ImageBytes(id)
	if err != nil {
		return "", err
	}
	return imageutil.DataURL(raw), nil
}

// Select puts an entry on the clipboard and, depending on the preference,
// pastes it into the app the user came from. This is the primary action.
func (a *App) Select(id string) error {
	return a.use(id, a.settings.Get().PasteOnSelect)
}

// CopyOnly puts an entry on the clipboard without pasting.
func (a *App) CopyOnly(id string) error {
	return a.use(id, false)
}

// PasteItem puts an entry on the clipboard and always pastes it.
func (a *App) PasteItem(id string) error {
	return a.use(id, true)
}

func (a *App) use(id string, pasteBack bool) error {
	if a.store == nil {
		return fmt.Errorf("history unavailable")
	}
	item, ok := a.store.Get(id)
	if !ok {
		return fmt.Errorf("entry not found")
	}

	var (
		change int64
		err    error
	)
	switch item.Kind {
	case store.KindText:
		change, err = clipboard.WriteText(item.Text)
	case store.KindImage:
		var raw []byte
		raw, err = a.store.ImageBytes(id)
		if err != nil {
			return fmt.Errorf("read image: %w", err)
		}
		change, err = clipboard.WriteImage(raw)
	default:
		return fmt.Errorf("unsupported entry kind %q", item.Kind)
	}
	if err != nil {
		return fmt.Errorf("write clipboard: %w", err)
	}

	// Our own write must not be recorded as a fresh copy.
	if a.watcher != nil {
		a.watcher.Ignore(change)
	}

	// Hide first: the popup holds focus, and the paste needs to land in the
	// user's app.
	a.HidePopup()

	cfg := a.settings.Get()

	if !pasteBack {
		if cfg.NotifyOnPaste {
			a.notifyUsed(item, "Copied to clipboard", "")
		}
		return nil
	}

	// Give the window a moment to actually give up focus.
	time.Sleep(80 * time.Millisecond)

	if err := clipboard.Paste(); err != nil {
		// Content is on the clipboard even when the keystroke could not be
		// sent, so report the shortfall rather than failing silently.
		notify.Send(notify.Notification{
			Title: "Copied, but could not paste",
			Body:  pasteFailureHint(err),
		})
		return fmt.Errorf("paste: %w", err)
	}

	if cfg.NotifyOnPaste {
		subtitle := ""
		if target := clipboard.Frontmost(); target.Name != "" {
			subtitle = "to " + target.Name
		}
		a.notifyUsed(item, "Pasted", subtitle)
	}
	return nil
}

// pasteFailureHint explains what the user can do about a failed paste. Only
// macOS gates synthetic keystrokes behind a permission; elsewhere a failure
// means the target window would not take focus, which retrying usually fixes.
func pasteFailureHint(err error) string {
	if errors.Is(err, clipboard.ErrNoPastePermission) {
		return "Grant Accessibility permission in System Settings › Privacy & Security › Accessibility to paste automatically."
	}
	return "The entry is on your clipboard — press " + pasteChord() + " to paste it yourself."
}

func pasteChord() string {
	if runtime.GOOS == "darwin" {
		return "⌘V"
	}
	return "Ctrl+V"
}

func (a *App) notifyUsed(item *store.Item, title, subtitle string) {
	body := notify.Summarise(item.Preview(), 120)
	if item.Kind == store.KindImage {
		body = fmt.Sprintf("Image · %d × %d", item.ImageW, item.ImageH)
	}
	if err := notify.Send(notify.Notification{
		Title:    title,
		Subtitle: subtitle,
		Body:     body,
	}); err != nil {
		log.Println("notify:", err)
	}
}

// TogglePin pins or unpins an entry and returns the new state.
func (a *App) TogglePin(id string) (bool, error) {
	if a.store == nil {
		return false, fmt.Errorf("history unavailable")
	}
	pinned, err := a.store.TogglePin(id)
	if err != nil {
		return false, err
	}
	a.emitHistoryChanged()
	return pinned, nil
}

// Delete removes a single entry.
func (a *App) Delete(id string) error {
	if a.store == nil {
		return fmt.Errorf("history unavailable")
	}
	if err := a.store.Delete(id); err != nil {
		return err
	}
	a.emitHistoryChanged()
	return nil
}

// ClearAll removes every entry.
func (a *App) ClearAll() error {
	if a.store == nil {
		return fmt.Errorf("history unavailable")
	}
	a.store.Clear()
	a.emitHistoryChanged()
	return nil
}

// ---------------------------------------------------------------------------
// Frontend API: settings and app control
// ---------------------------------------------------------------------------

// GetSettings returns the current preferences.
func (a *App) GetSettings() settings.Settings { return a.settings.Get() }

// SaveSettings validates and persists preferences, returning the stored values.
func (a *App) SaveSettings(s settings.Settings) (settings.Settings, error) {
	return a.settings.Save(s)
}

// DefaultSettings returns a pristine preference set, for the "reset" action.
func (a *App) DefaultSettings() settings.Settings { return settings.Defaults() }

// ShowSettings switches the window to the preferences view.
func (a *App) ShowSettings() {
	if a.ctx == nil {
		return
	}
	a.mu.Lock()
	a.view = ViewSettings
	a.visible = true
	a.mu.Unlock()

	wruntime.EventsEmit(a.ctx, "view:changed", string(ViewSettings))
	wruntime.WindowSetSize(a.ctx, settingsWidth+2*shadowSide, settingsHeight+shadowTop+shadowBottom)
	// Preferences fill the window bar the shadow margins, and so does the
	// material.
	window.SetPanelInset(shadowSide, shadowTop, shadowSide, shadowBottom, panelRadius)
	wruntime.WindowCenter(a.ctx)
	wruntime.WindowShow(a.ctx)
}

// ShowPopupView switches back from settings to the list.
func (a *App) ShowPopupView() {
	a.mu.Lock()
	anchor := a.anchor
	a.mu.Unlock()
	a.showPopupAt(anchor)
}

// Environment describes the host, for UI that differs per platform.
type Environment struct {
	Platform     string `json:"platform"`
	ModifierName string `json:"modifierName"`
	Version      string `json:"version"`
	// CanPaste reports whether automatic paste-back is currently permitted.
	CanPaste bool `json:"canPaste"`
	// NotificationStatus is one of "authorized", "denied", "notDetermined" or
	// "unknown". Copy/paste alerts are silently dropped unless authorized.
	NotificationStatus string `json:"notificationStatus"`
}

// Env returns host details for the frontend.
func (a *App) Env() Environment {
	mod := "Ctrl"
	if runtime.GOOS == "darwin" {
		mod = "⌘"
	}
	return Environment{
		Platform:           runtime.GOOS,
		ModifierName:       mod,
		Version:            appVersion,
		CanPaste:           clipboard.HasPastePermission(false),
		NotificationStatus: string(notify.Permission()),
	}
}

// NotificationStatus reports whether notifications will be delivered.
func (a *App) NotificationStatus() string { return string(notify.Permission()) }

// RequestNotificationPermission re-prompts for notification permission. macOS
// only shows the prompt once, so when already denied this opens the settings
// pane instead.
func (a *App) RequestNotificationPermission() string {
	if notify.Permission() == notify.StatusDenied {
		if err := notify.OpenSettings(); err != nil {
			log.Println("notify: open settings:", err)
		}
		return string(notify.StatusDenied)
	}
	notify.RequestPermission()
	return string(notify.Permission())
}

// SendTestNotification posts a sample notification so the user can confirm the
// copy/paste alerts will actually reach them.
func (a *App) SendTestNotification() error {
	return notify.Send(notify.Notification{
		Title:    "Geda Clipboard",
		Subtitle: "Notifications are working",
		Body:     "You will see an alert like this each time you copy or paste.",
	})
}

// RequestPastePermission asks the OS for permission to synthesise keystrokes,
// prompting the user if necessary. Returns the resulting state.
func (a *App) RequestPastePermission() bool {
	return clipboard.HasPastePermission(true)
}

// Quit shuts the application down.
func (a *App) Quit() {
	if a.ctx == nil {
		return
	}
	wruntime.Quit(a.ctx)
}
