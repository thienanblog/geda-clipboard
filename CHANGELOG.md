# Changelog

All notable changes to Geda Clipboard are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

While the major version is 0, the minor version is bumped for behavioural
changes and the patch version for fixes.

## [Unreleased]

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
- Memory is handed back to the operating system once the popup closes and after
  an image is captured, rather than being kept as a high-water mark. Decoding a
  full-screen screenshot to build a thumbnail is the app's largest allocation
  and it is needed for a moment only.
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

[Unreleased]: https://github.com/thienanblog/geda-clipboard/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/thienanblog/geda-clipboard/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/thienanblog/geda-clipboard/releases/tag/v0.1.0
