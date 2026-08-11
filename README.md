# Geda Clipboard

[![CI](https://github.com/thienanblog/geda-clipboard/actions/workflows/ci.yml/badge.svg)](https://github.com/thienanblog/geda-clipboard/actions/workflows/ci.yml)

A menu bar / system tray clipboard manager for macOS and Windows, built with Go
and [Wails v2](https://wails.io). Inspired by [Maccy](https://github.com/p0deje/Maccy).

It keeps a searchable history of what you copy, **posts a notification each time
you copy or paste**, and pastes any earlier entry straight back into the app you
were working in.

## Features

- **Popup at the mouse pointer** — where you are already looking when you press
  the shortcut — with search, keyboard navigation and `⌘1`–`⌘9` accelerators.
  Preferences can anchor it under the tray icon instead.
- **Notifications on copy and paste**, showing the source app and a preview.
  Each can be toggled independently.
- **Text and image history** with thumbnails; images are stored as separate blob
  files so the index stays small.
- **Copy counting** — re-copying the same content bumps an existing entry and
  increments its counter rather than creating a duplicate. This is driven by the
  OS clipboard change counter, so an identical re-copy is still detected.
- **Provenance** — each entry records the app it came from, with its icon, plus
  first/last copy time.
- **Universal Clipboard** — copies handed over from an iPhone or iPad are
  recorded and notified like any other, labelled *Universal Clipboard* rather
  than blamed on whichever Mac app happened to be frontmost, and never dropped
  by that app's ignore-list entry. The payload comes across on demand, so the
  notification lands a second or so after the copy.
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
- **Launch at login** via `SMAppService` (macOS), which registers the bundle
  with the system and shows up under Login Items in System Settings, or the
  `Run` registry key (Windows).

## Requirements

To run:

- macOS 13 or later. Launch at login goes through `SMAppService`, and the app
  is sandboxed, which rules out the LaunchAgent that earlier versions wrote.
- Windows 10 or later.

To build:

- Go 1.25+ (see `go.mod`)
- Node 18+
- [Wails CLI v2](https://wails.io/docs/gettingstarted/installation)
- macOS: Xcode command line tools (the app uses cgo for AppKit)

## Install

Every [release](https://github.com/thienanblog/geda-clipboard/releases) has a
macOS bundle and Windows executables attached. They are unsigned — there is no
paid developer certificate behind this project — so each system asks once
before it will run them:

- **macOS** — unzip, move `Geda Clipboard.app` to `/Applications`, then clear
  the quarantine flag. Skip this and Gatekeeper claims the app is damaged,
  which is what an unsigned download looks like to it:

  ```bash
  xattr -dr com.apple.quarantine "/Applications/Geda Clipboard.app"
  ```

- **Windows** — unzip and run `Geda Clipboard.exe`. SmartScreen warns about an
  unknown publisher: choose **More info › Run anyway**.

Neither app opens a window: look for the icon in the menu bar or the
notification area.

Building from source avoids both prompts, since a locally built binary is
never quarantined:

```bash
git clone https://github.com/thienanblog/geda-clipboard.git
cd geda-clipboard
git checkout v0.4.0
wails build
```

See [CHANGELOG.md](CHANGELOG.md) for what changed in each release.

## Build and run

```bash
wails build
```

The bundle is written to `build/bin/`. On macOS:

```bash
open "build/bin/Geda Clipboard.app"
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

- macOS: `~/Library/Containers/com.geda.clipboard/Data/Library/Application Support/geda-clipboard/`.
  The app is sandboxed, so macOS redirects it there; the code still asks for
  `~/Library/Application Support`, which is where an unsandboxed build writes.
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
                           Windows: energye/systray. Also reports the pointer
                           and the work area around it, for popup placement.
  window/                  Moves the app's own window in global, all-display
                           coordinates, which Wails cannot do
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

### Three design decisions worth knowing

**The macOS tray is hand-written rather than using a systray library.** The usual
choice, `energye/systray`, calls `[[NSApplication sharedApplication]
setDelegate:]`, which takes over the delegate Wails relies on for window and
lifecycle handling. It also offers no way to query the icon's screen rectangle,
which the popup needs in order to hang underneath it. A native `NSStatusItem`
avoids both problems. Windows has neither issue, so it uses the library.

**The popup is positioned by moving the window directly, not through Wails.**
`WindowSetPosition` takes coordinates relative to the screen the window already
occupies, so asking it for the top-left of another display just moves the window
to the top-left of its own. `internal/window` finds the framework's window --
by AppKit class on macOS, by window class and process id on Windows -- and sets
its frame in the global space spanning every display. Every position is computed
both ways, so a failed lookup falls back to the Wails call, which is correct
whenever the target is the window's current screen.

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
- **Multi-monitor popup placement is unverified on Windows.** The popup is
  positioned in global coordinates and the window is moved there directly, so
  it should follow the pointer onto any display; only the macOS path has been
  tested against a second screen.

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
4. Tag and push. The [release workflow](.github/workflows/release.yml) takes it
   from there: it refuses a tag that disagrees with `wails.json`, builds a
   universal macOS bundle and x64/arm64 Windows executables, and publishes them
   with the changelog section for that version as the release notes.

   ```bash
   git tag -a v1.2.3 -m "v1.2.3"
   git push origin v1.2.3
   ```

Running the workflow manually from the Actions tab builds the same archives and
leaves them as artifacts without publishing anything, which is how to test a
packaging change without burning a tag.

`appVersion` can also be overridden at build time without editing the source,
which is useful for nightly builds:

```bash
wails build -ldflags "-X main.appVersion=1.2.3-nightly"
```
