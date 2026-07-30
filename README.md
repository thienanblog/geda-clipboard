# Geda Clipboard

A menu bar / system tray clipboard manager for macOS and Windows, built with Go
and [Wails v2](https://wails.io). Inspired by [Maccy](https://github.com/p0deje/Maccy).

It keeps a searchable history of what you copy, **posts a notification each time
you copy or paste**, and pastes any earlier entry straight back into the app you
were working in.

## Features

- **Menu bar popup** anchored under the tray icon, with search, keyboard
  navigation and `⌘1`–`⌘9` accelerators.
- **Notifications on copy and paste**, showing the source app and a preview.
  Each can be toggled independently.
- **Text and image history** with thumbnails; images are stored as separate blob
  files so the index stays small.
- **Copy counting** — re-copying the same content bumps an existing entry and
  increments its counter rather than creating a duplicate. This is driven by the
  OS clipboard change counter, so an identical re-copy is still detected.
- **Provenance** — each entry records the app it came from, with its icon, plus
  first/last copy time.
- **Pin** entries to keep them past the history limit; **delete** individually
  or clear everything.
- **Paste back into the previous app**: the app you came from is remembered
  before the popup steals focus, then refocused and sent the paste keystroke.
- **Privacy** — honours the conventions password managers use to opt out
  (`org.nspasteboard.ConcealedType` on macOS,
  `ExcludeClipboardContentFromMonitorProcessing` on Windows), plus a per-app
  ignore list.
- **Global hotkey** (default `⇧⌘V` on macOS, `Ctrl+Shift+V` on Windows),
  rebindable in preferences.
- **Launch at login** via a LaunchAgent (macOS) or the `Run` registry key
  (Windows).

## Requirements

- Go 1.23+
- Node 18+
- [Wails CLI v2](https://wails.io/docs/gettingstarted/installation)
- macOS: Xcode command line tools (the app uses cgo for AppKit)

## Build and run

```bash
wails build
```

The bundle is written to `build/bin/`. On macOS:

```bash
open build/bin/geda-clipboard.app
```

For development, with hot reload and a browser-accessible UI at
<http://localhost:34115>:

```bash
wails dev
```

Cross-compile check for Windows from any host:

```bash
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...
```

Run the tests:

```bash
go test ./...
```

## Permissions (macOS)

Two OS permissions gate the headline features. The app detects both and tells
you in **Preferences** when either is missing.

| Feature | Permission | Where |
| --- | --- | --- |
| Notifications on copy/paste | Notifications | System Settings › Notifications › Geda Clipboard |
| Pasting automatically | Accessibility | System Settings › Privacy & Security › Accessibility |

Without Accessibility the app still copies the entry to the clipboard — you just
paste it yourself. Without Notifications, history still works but the alerts are
silently dropped, which is why Preferences surfaces the status and offers a
**Send a test notification** button.

The app is a menu bar accessory (`LSUIElement`), so it has no Dock icon.

> **Note:** if you use Bartender or a similar menu bar manager, a newly added
> icon is usually hidden by default — unhide "Geda Clipboard" there to see it.

## Keyboard shortcuts

| Action | Shortcut |
| --- | --- |
| Toggle the popup | `⇧⌘V` (configurable) |
| Navigate | `↑` / `↓`, `Home` / `End` |
| Choose the selected entry | `Return` |
| Choose entry 1–9 | `⌘1` … `⌘9` |
| Copy without pasting | `⌘C` |
| Pin / unpin | `⌥P` |
| Delete entry | `⌥⌫` |
| Clear all | `⌥⇧⌘⌫` |
| Preferences | `⌘,` |
| Clear search, then close | `Esc` |
| Quit | `⌘Q` |

## Where data lives

- macOS: `~/Library/Application Support/geda-clipboard/`
- Windows: `%AppData%\geda-clipboard\`

```
settings.json    preferences
history.json     history index (text, thumbnails, metadata)
icons.json       source-app icons, one copy per app
blobs/           full-size PNGs for image entries
```

History is written atomically (temp file + rename) and saves are coalesced, so
rapid copying does not thrash the disk.

Because the index is rewritten whenever the history changes, two things are kept
out of it: full-size images live in `blobs/`, and app icons live once per app in
`icons.json` rather than being repeated on every entry. Thumbnails are bounded at
360×150 — twice the largest size they are ever displayed at. An index written by
an older build, with icons inline, is migrated automatically on first load.

## Architecture

```
main.go                    Wails setup: frameless, always-on-top, starts hidden
app.go                     Bound API, orchestration, capture and paste flow
internal/
  tray/                    Menu bar item. macOS: native NSStatusItem via cgo.
                           Windows: energye/systray.
  clipboard/               Change-counter polling, read/write, source-app
                           detection, paste keystroke synthesis
  store/                   History: dedupe, copy counting, pinning, eviction,
                           persistence
  settings/                Preferences with validation and change callbacks
  notify/                  Native notifications (UNUserNotifications / toast)
  imageutil/               Thumbnail generation
  autostart/               Launch at login
  appdir/                  Per-user paths, atomic writes
frontend/                  Vue 3 + TypeScript
```

### Two design decisions worth knowing

**The macOS tray is hand-written rather than using a systray library.** The usual
choice, `energye/systray`, calls `[[NSApplication sharedApplication]
setDelegate:]`, which takes over the delegate Wails relies on for window and
lifecycle handling. It also offers no way to query the icon's screen rectangle,
which the popup needs in order to hang underneath it. A native `NSStatusItem`
avoids both problems. Windows has neither issue, so it uses the library.

**Change detection uses the OS clipboard change counter, not content diffing.**
Copying the same text twice does not change the content but is still two copies,
and the UI reports a per-entry copy count. Tray creation is also dispatched
asynchronously to the main queue, because Wails invokes `OnStartup` on a
goroutine that races `[NSApp run]`.

## Known gaps

- **Preferences share the popup's window.** Wails v2 supports a single window,
  so preferences swap the view and resize rather than opening a second window.
- **No source-app icon on Windows.** Extracting an executable's icon needs
  `HICON`-to-bitmap conversion; the UI omits the icon when it is unavailable.
- **The Windows tray anchor is approximate.** Windows exposes no supported way
  to query a notification icon's rectangle, so the cursor position at click time
  is used instead.
- **Windows is compile-verified only.** The Windows platform layer builds and
  vets cleanly via cross-compilation but has not been run on a Windows host.
- **Multi-monitor popup placement** follows the screen the window is currently
  on, which can differ from the screen holding the tray icon.
