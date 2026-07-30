# Changelog

All notable changes to Geda Clipboard are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

While the major version is 0, the minor version is bumped for behavioural
changes and the patch version for fixes.

## [Unreleased]

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

[Unreleased]: https://github.com/thienanblog/geda-clipboard/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/thienanblog/geda-clipboard/releases/tag/v0.1.0
