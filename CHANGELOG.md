# Changelog

All notable changes to Geda Clipboard are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

While the major version is 0, the minor version is bumped for behavioural
changes and the patch version for fixes.

## [Unreleased]

## [0.10.1] - 2026-08-27

### Fixed

- **Popup rows no longer reserve separate space for the numeric shortcut and
  pin control.** An unpinned row shows its `⌘1`–`⌘9` shortcut until the pointer
  reaches it, then replaces the shortcut with the pin button in the same fixed
  slot. Pinned rows keep the pin visible, and row content gains room without
  shifting when the control changes.

## [0.10.0] - 2026-08-27

### Added

- **Pinned entries can be managed from Preferences.** User-prioritised entries
  keep their chosen order above automatically ordered pins, and every popup row
  now has a direct pin control alongside the existing keyboard shortcut.
- **Image previews have Compact, Comfortable and Large size options.** History
  clearing keeps pinned entries by default, with an opt-in preference to clear
  them too.

## [0.9.0] - 2026-08-26

### Added

- **Preferences are organised into General, Clipboard, Privacy, Statistics and
  About tabs.** Statistics shows local copy totals for text, images and repeated
  content over a day, week, month or year. It stores only bounded hourly and
  daily counters, never clipboard content or per-copy events.

## [0.8.2] - 2026-08-24

### Fixed

- **Images copied from Chrome are no longer intermittently missed.** Chrome can
  advance the macOS pasteboard counter just before the new item's types become
  visible. Geda now treats that brief empty state as pending and retries the
  same change instead of discarding it before the image arrives.
- **Images copied from apps that lazily provide pasteboard data are no longer
  missed.** macOS now briefly retries a declared image whose bytes are not ready
  yet and accepts every image representation AppKit can decode, rather than
  relying only on PNG or TIFF.

## [0.8.0] - 2026-08-20

### Changed

- **The Mac App Store build no longer pastes an entry back by itself.** App
  Review rejected 0.7.0 under guideline 2.4.5: sending Cmd+V to another
  application needs Accessibility permission, and Accessibility may not be used
  to automate other apps. That build therefore ships without the keystroke
  path, asks for no Accessibility permission, and hides the preferences that
  offered it. Choosing an entry copies it and returns focus to the app the
  user came from, so pasting is one shortcut they press themselves. The
  Developer ID build published on GitHub and Homebrew is unchanged and still
  pastes automatically; the keystroke now lives behind the `axpaste` build tag,
  which packaging adds for that build only and verifies is absent from the App
  Store one.

### Fixed

- **The popup opens with the caret in the search field, and the first Down
  selects the newest entry.** The view switch that told the frontend to focus
  the search field was emitted before the window was shown and focused, so the
  web view handed the caret back to the document and typing was lost. The
  frontend now claims the field from a new event emitted once the window is on
  screen, and again whenever the window regains focus. The first Down after
  opening the popup, or after changing the search, also stepped past the row
  that was already highlighted; it now selects that row.

## [0.7.2] - 2026-08-15

### Fixed

- **Left- and right-clicking the Windows notification-area icon opens the
  popup.** The tray window and its Win32 message loop now stay on the same OS
  thread, so Windows can deliver both mouse-button callbacks. Its preview and
  shadow margins are now transparent, without a native border around the
  larger backing window.

- **A history file that cannot be read is kept instead of being replaced.**
  Any read or parse failure started the app with an empty history, and the next
  copy saved over the file, so one unreadable index destroyed the whole history
  with nothing to recover from. The file is now moved to
  `history.json.unreadable` before that first save.

## [0.7.1] - 2026-08-12

### Fixed

- **The Windows notification-area icon is visible.** The tray library asks
  Win32 to load ICO data, but the app supplied the monochrome PNG template used
  by macOS. Windows rejected it while the clipboard watcher kept running, so
  copy notifications worked even though there was no icon to click.

- **The Windows popup stays open and accepts keyboard input.** A hidden
  WebView2 window can briefly blur while Windows transfers focus from the
  native frame into the browser. That transient event was treated as an
  outside click, immediately hiding the popup. The app now restores WebView
  focus after showing and only dismisses a blurred popup after its native
  window has actually left the foreground.

## [0.7.0] - 2026-08-12

### Fixed

- **The keyboard shortcut works in the App Store build.** ⌘⇧V did nothing, and
  neither did any combination assigned in its place. The hotkey library
  implements macOS shortcuts as a keyboard event tap, which observes every
  keystroke in the session and is therefore refused unless the application is
  already trusted for Accessibility. The App Store build is a separate
  application as far as macOS privacy records are concerned, so nothing carried
  over from a Developer ID copy, and the app has no way to earn that trust
  before it has shown the user anything — the shortcut being the only way to
  show them anything. macOS shortcuts are now claimed through
  RegisterEventHotKey, which reserves one combination instead of watching the
  keyboard and needs no permission at all; it was verified registering and
  firing inside the sandbox with Accessibility withheld. Windows is unchanged:
  its side of the library already used RegisterHotKey.

- **A shortcut that cannot be registered says so.** The failure only reached
  the log, so an application whose one entry point is a shortcut looked simply
  broken. Preferences now reports it under the shortcut, including the case
  worth acting on: another application already owns the combination.

### Added

- **Accessibility permission can be reached in one click.** macOS shows the
  permission prompt once per application and silently does nothing on every
  later attempt, which left the existing button appearing dead for the user who
  had dismissed it. Preferences now also offers **Open Accessibility settings…**,
  which reveals the list directly, and re-checks the permission when the window
  returns to the front — macOS does not tell a running application that the
  grant landed, so the warning used to persist until the next launch.

## [0.6.2] - 2026-08-12

### Fixed

- **The App Store build shows its interface.** The popup opened as an empty
  panel: WKWebView's helper processes will not launch in a sandboxed app
  without the outgoing network entitlement, however local the content is, and
  the App Store entitlements left it out because the build opens no sockets.
  The app looked healthy from the outside, capture and its notifications
  included, which is why only a sandboxed run ever showed it. The build still
  sends nothing.

- **Closing preferences dismisses the window.** "Done" and Escape used to hand
  the window back to the history list, leaving a popup on screen that the user
  had not asked for; the window now goes away, as it does everywhere else.

- The App Store package no longer carries a quarantine flag. The provisioning
  profile is downloaded in a browser, so it is quarantined, and the flag
  propagates to the copy embedded in the bundle; Transporter rejects the upload
  over it (91109) after everything local has passed.
  `scripts/package-appstore.sh` now clears extended attributes before signing
  and reads the finished payload back to prove none survived.

## [0.6.1] - 2026-08-11

### Fixed

- **The popup can be driven with a screen reader.** The search field had no
  accessible name, the history was an unlabelled set of buttons with no list
  semantics, the highlighted row was never announced as selected, and an image
  row announced nothing at all — its only content was a thumbnail with an empty
  `alt`. The field is now a labelled combobox that publishes the highlight
  through `aria-activedescendant`, so the selection is spoken while the caret
  stays where you are typing, and every row carries a description that names an
  image by its size and each entry by the app it came from.
- **Reduce Motion is honoured.** Turning it on in System Settings now collapses
  the popup fade, the detail card's slide and the error toast.

## [0.6.0] - 2026-08-11

### Added

- **Homebrew cask.** `brew install --cask thienanblog/tap/geda-clipboard`
  installs the same signed, notarized bundle the release page serves. The cask
  declares `auto_updates true`, so Homebrew stands aside and lets Sparkle do
  the upgrading; `brew upgrade --cask --greedy` overrides that. The release
  workflow pushes an updated cask to
  [thienanblog/homebrew-tap](https://github.com/thienanblog/homebrew-tap)
  whenever a non-pre-release tag is published.
- **A project page, privacy policy, support page and terms**, served by GitHub
  Pages from `docs/` alongside the Sparkle appcast.
- **App Store Connect preflight.** `scripts/package-appstore.sh` now asks App
  Store Connect whether the version and build number are still free before it
  builds, and refuses a marketing version that has already been released —
  which raising the build number cannot reopen.

### Fixed

- The macOS bundle declares `LSApplicationCategoryType`. Without it App Store
  Connect refuses the upload, and only says so after the processing pass has
  run.

### Notes

- The Mac App Store build carries no updater, by construction: the Sparkle
  framework is added by `-tags sparkle` rather than removed by a tag, so the
  App Store package cannot ship one by forgetting a flag.

## [0.5.0] - 2026-08-11

### Upgrading from 0.4.0 or earlier on macOS

Earlier versions turned on **Launch at login** by writing a LaunchAgent into
`~/Library/LaunchAgents`. This one is sandboxed and cannot reach that folder,
so it can neither use that file nor clean it up. Left alone it keeps starting
the app at login whatever the preference says. Remove it once:

```bash
launchctl unload ~/Library/LaunchAgents/com.geda.clipboard.plist 2>/dev/null
rm -f ~/Library/LaunchAgents/com.geda.clipboard.plist
```

Then set **Launch at login** again if you want it; it is registered with the
system this time and appears under Login Items in System Settings.

History does not carry over. The sandbox gives the app its own container, so
what earlier versions wrote to `~/Library/Application Support/geda-clipboard`
is still on disk but out of reach. Copy it into
`~/Library/Containers/com.geda.clipboard/Data/Library/Application Support/`
before first launch to keep it.

### Added

- The macOS build is **signed and notarized**. Gatekeeper opens it with no
  prompt and no `xattr` incantation.
- **Software updates** through Sparkle, with a *Check for Updates…* button in
  Preferences › About. Updates are signed with a key separate from the one that
  signs the app, and are verified before anything is replaced.

### Changed

- **Launch at login** now goes through `SMAppService` instead of a LaunchAgent
  plist. The registration belongs to the bundle rather than to a path, so a
  rename or a move no longer breaks it, and it shows up under Login Items in
  System Settings where it can be turned off from outside the app.
- **macOS 13 or later** is required, which `SMAppService` sets the floor for.
- The app runs in the **App Sandbox**. Clipboard history, the global shortcut
  and pasting back into the previous app all work as before; only the storage
  location moves, into the app's container.

## [0.4.0] - 2026-08-11

### Fixed

- The crooked dark outline that framed the popup is gone. macOS derives a
  window's shadow from the alpha of its backing, and on a translucent WebView
  window it got that wrong: the result was a hard rim tracing wherever the page
  last had opaque pixels, so it stood around the panel, around the preview card
  and around shapes neither of them still had. The window now carries no shadow
  of its own — the panel and the card draw theirs in CSS, into transparent
  margins the window keeps around them.

### Changed

- The macOS bundle is now called `Geda Clipboard.app` rather than
  `geda-clipboard.app`, matching the name the app has everywhere else. If
  **Launch at login** was on, replacing the old bundle leaves the login item
  pointing at a path that no longer exists; the app now rewrites it on every
  launch, so starting the renamed app once repairs it.
- Releases are titled with their tag alone — `v0.4.0` rather than
  `Geda Clipboard 0.4.0`.

## [0.3.0] - 2026-08-11

### Added

- **Show icon in the Dock** in Preferences › General keeps or drops the macOS
  Dock tile, and takes effect immediately rather than on the next launch. It is
  off by default: the app is reached from the menu bar and the shortcut, so the
  tile only cost a slot. The popup still takes keyboard focus without it.
- **Open the popup at** in Preferences › Window chooses between the mouse
  pointer and the tray icon. At the pointer the popup opens like a context
  menu — top-left corner at the cursor, pulled back inside the work area near
  an edge.

### Changed

- The app no longer claims a Dock tile. `LSUIElement` was already set in the
  bundle, but Wails asks for the regular activation policy on launch and put
  the tile back; the policy is now applied from the app itself.
- Memory is handed back to the operating system after an image is captured,
  rather than being kept as a high-water mark. Decoding a full-screen
  screenshot to build a thumbnail is the app's largest allocation and it is
  needed for a moment only.
- The detail card no longer takes up room in the popup: it floats out to the
  left of the panel, level with the row under the pointer, and disappears again
  when the pointer leaves the list. Arrow-key navigation shows it too. The
  popup itself is now just the list, so the default width drops from 720 to
  420px; a stored width of 600px or more, which only the old docked column
  justified, is reset once on upgrade.
- The popup now opens at the mouse pointer by default. It used to always hang
  under the tray icon, and pressing the shortcut before the icon had ever been
  clicked centred the window on screen. Existing installations pick the new
  default up on upgrade; choose "the menu bar icon" to keep the old behaviour.

### Fixed

- Copies handed over from an iPhone or iPad by Universal Clipboard are no longer
  attributed to whichever Mac app happened to be frontmost, which also meant
  they were dropped silently whenever that app was on the ignore list — a list
  the copy had nothing to do with. They are labelled *Universal Clipboard*
  instead, recognised by the `com.apple.is-remote-clipboard` pasteboard type,
  and exempt from the ignore list. Verified against an iPhone: the type is
  present on Handoff copies of both text and images, and absent on local ones.
- A copy whose payload cannot be read yet is retried for a few seconds instead
  of being dropped on the first empty read. The change counter is consumed by
  then and never repeats, so a single empty read used to lose the entry for
  good. In practice a Universal Clipboard read blocks until the transfer
  completes rather than returning empty — measured at 1.3s for a short text and
  0.4s for a 7MB photo — so this is insurance against a transfer that fails.
- Pressing the shortcut before the tray icon has been clicked no longer centres
  the popup: the two placements now fall back to each other, and the window is
  only centred when neither the pointer nor the icon can be located.
- The popup opens on the display the pointer or the tray icon is on, instead of
  staying on whichever display it was last shown on. Positions are now computed
  in global coordinates and the window is moved there directly, because Wails'
  `WindowSetPosition` resolves its arguments against the window's own screen.
  Verified on macOS; the Windows path is written but untested.
- Windows reads the work area of the monitor under the pointer rather than the
  primary monitor's, so the popup is kept on screen against the right edges.

## [0.2.0] - 2026-07-31

### Added

- Release workflow: pushing a `v*` tag publishes a universal macOS bundle and
  x64/arm64 Windows executables, with the changelog section for that version as
  the release notes. Prebuilt downloads are documented in the README, including
  the first-launch step each system requires for an unsigned build.

### Changed

- The hover detail card is now a column docked beside the list instead of a
  card floating over it, which used to cover the very row it described. It
  follows the keyboard selection when the mouse is away, and hides itself on a
  popup too narrow to spare the space.
- History rows shorten long entries in the middle — head, ellipsis, tail —
  rather than dropping the tail, so the end of a path or URL stays visible. The
  budget follows the configured popup width.

## [0.1.0] - 2026-07-31

First release.

### Added

- Menu bar popup anchored under the tray icon, with search, keyboard
  navigation and `⌘1`–`⌘9` accelerators.
- Notifications on copy and on paste, each independently toggleable, showing
  the source application and a preview.
- Text and image history with thumbnails, pinning, deletion, copy counts and a
  configurable size limit. Pinned entries are kept regardless of the limit.
- Pastes a chosen entry back into the application the user came from, which is
  captured before the popup takes focus.
- Honours the conventions applications use to opt out of clipboard managers
  (`org.nspasteboard.*` on macOS, `CanIncludeInClipboardHistory` and
  `CanUploadToCloudClipboard` on Windows), plus a per-application ignore list.
- Preferences covering notifications, paste behaviour, the global shortcut,
  history size, privacy and popup dimensions.
- Continuous integration building and testing on macOS and Windows hosts.

### Notes

- The macOS tray is a native `NSStatusItem` written in cgo rather than
  `energye/systray`, which replaces the `NSApplication` delegate Wails relies
  on and exposes no way to read the status item's screen rectangle.
- Change detection polls the OS clipboard change counter rather than diffing
  content, so copying the same thing twice counts as two copies.
- The Windows platform layer is verified by CI compilation and tests, but has
  not been used interactively on a Windows desktop.
- Builds are unsigned. macOS Gatekeeper will need the app to be opened via
  right-click → Open the first time.

[Unreleased]: https://github.com/thienanblog/geda-clipboard/compare/v0.10.1...HEAD
[0.10.1]: https://github.com/thienanblog/geda-clipboard/compare/v0.10.0...v0.10.1
[0.10.0]: https://github.com/thienanblog/geda-clipboard/compare/v0.9.0...v0.10.0
[0.9.0]: https://github.com/thienanblog/geda-clipboard/compare/v0.8.2...v0.9.0
[0.8.2]: https://github.com/thienanblog/geda-clipboard/compare/v0.8.0...v0.8.2
[0.8.0]: https://github.com/thienanblog/geda-clipboard/compare/v0.7.2...v0.8.0
[0.7.2]: https://github.com/thienanblog/geda-clipboard/compare/v0.7.1...v0.7.2
[0.7.1]: https://github.com/thienanblog/geda-clipboard/compare/v0.7.0...v0.7.1
[0.7.0]: https://github.com/thienanblog/geda-clipboard/compare/v0.6.2...v0.7.0
[0.6.2]: https://github.com/thienanblog/geda-clipboard/compare/v0.6.1...v0.6.2
[0.6.1]: https://github.com/thienanblog/geda-clipboard/compare/v0.6.0...v0.6.1
[0.6.0]: https://github.com/thienanblog/geda-clipboard/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/thienanblog/geda-clipboard/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/thienanblog/geda-clipboard/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/thienanblog/geda-clipboard/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/thienanblog/geda-clipboard/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/thienanblog/geda-clipboard/releases/tag/v0.1.0
