# Geda Clipboard

[![CI](https://github.com/thienanblog/geda-clipboard/actions/workflows/ci.yml/badge.svg)](https://github.com/thienanblog/geda-clipboard/actions/workflows/ci.yml)

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

- Go 1.25+ (see `go.mod`)
- Node 18+
- [Wails CLI v2](https://wails.io/docs/gettingstarted/installation)
- macOS: Xcode command line tools (the app uses cgo for AppKit)

## Install

There are no prebuilt downloads: releases ship source only, because the
binaries are unsigned and macOS Gatekeeper would block them without any useful
explanation. Build from a tagged release, or from `main` for the latest work:

```bash
git clone https://github.com/thienanblog/geda-clipboard.git
cd geda-clipboard
git checkout v0.1.0
wails build
```

See [CHANGELOG.md](CHANGELOG.md) for what changed in each release.

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

Cross-compile check for the Windows sources from any host:

```bash
GOOS=windows GOARCH=amd64 go vet ./internal/...
```

This deliberately excludes the root package, which embeds `frontend/dist` and
therefore cannot be loaded until the frontend has been built.

Run the tests (after a `wails build`, for the same reason):

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

The search field keeps focus while the popup is open, so two of these defer to
it when there is text to act on: `⌥⌫` deletes the previous word of a non-empty
query rather than an entry, and `⌘C` copies a selection in the field rather than
the highlighted entry. With the query empty they act on the list as listed
above.

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
- **Windows has no interactive testing.** CI builds and tests it on a Windows
  host, but nobody has used it on a Windows desktop. The `INPUT` struct
  SendInput depends on is pinned by compile-time size and
  offset assertions, since getting it wrong fails silently at runtime.
- **Multi-monitor popup placement** follows the screen the window is currently
  on, which can differ from the screen holding the tray icon.

## Cutting a release

The version appears in two files and a test fails if they disagree:

| File | Field |
| --- | --- |
| `main.go` | `appVersion` — reported in the About panel |
| `wails.json` | `info.productVersion` — templated into the macOS `Info.plist` and the Windows resource block |

1. Bump both to the same bare semver (`1.2.3`, no leading `v`).
2. Move the `Unreleased` entries in [CHANGELOG.md](CHANGELOG.md) under the new
   version with today's date, and update the link definitions at the bottom.
3. Merge to `main` and wait for CI to pass on both platforms.
4. Tag and publish:

   ```bash
   git tag -a v1.2.3 -m "v1.2.3"
   git push origin v1.2.3
   gh release create v1.2.3 --title "v1.2.3" --notes-file <(sed -n '/## \[1.2.3\]/,/## \[/p' CHANGELOG.md)
   ```

`appVersion` can also be overridden at build time without editing the source,
which is useful for nightly builds:

```bash
wails build -ldflags "-X main.appVersion=1.2.3-nightly"
```
