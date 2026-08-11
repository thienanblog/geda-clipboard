# Repository instructions

## Scope and project context

- These instructions apply to the entire repository.
- Geda Clipboard is a menu bar/system tray clipboard manager for macOS and
  Windows, built with Go 1.25, Wails v2, Vue 3, and TypeScript.
- macOS is the primary development and manually tested platform. Windows builds
  and runs tests in CI, but desktop behavior is not manually verified; state
  that limitation when changing Windows-specific behavior.
- Read `README.md` for user-visible behavior and setup. For packaging, signing,
  release, appcast, Homebrew, or App Store work, read `scripts/README.md` first.

## Environment and commands

- CI uses Go 1.25, Node 20, Wails v2.12.0, and the npm lockfile in `frontend/`.
- Install frontend dependencies with `cd frontend && npm ci`.
- Run the application with `wails dev`; the frontend is also available at
  `http://localhost:34115` during development.
- Build with `wails build`; artifacts are written under `build/bin/`.
- Run frontend validation with `cd frontend && npm run build`. For a focused
  type check, run `cd frontend && npx vue-tsc --noEmit`.
- Run Go formatting validation with `gofmt -l .`; CI fails if it prints any Go
  source path outside dependency directories.
- Run `wails build -clean` before `go vet ./...` or `go test ./...`. The root
  package embeds the gitignored `frontend/dist`, so a fresh checkout cannot
  load the root package until the frontend has been built.
- Cross-check Windows-only internal packages from macOS or Linux with:
  - `GOOS=windows GOARCH=amd64 go vet ./internal/...`
  - `GOOS=windows GOARCH=arm64 go vet ./internal/...`

## Architecture and generated code

- `main.go` configures the hidden, frameless Wails application and owns
  `appVersion`.
- `app.go` owns the bound API, capture/paste orchestration, and popup placement.
- `internal/tray/` owns the menu bar/tray item, pointer position, and work area.
- `internal/window/` owns cross-display placement of the application window.
- `internal/clipboard/` owns clipboard polling, reads/writes, source-app
  detection, and paste keystrokes.
- `internal/store/` owns history deduplication, counts, pinning, eviction, and
  persistence; use it when creating fixtures so stored data follows the schema.
- `internal/settings/`, `internal/notify/`, `internal/imageutil/`,
  `internal/autostart/`, `internal/appdir/`, and `internal/updater/` own their
  corresponding platform services.
- `frontend/src/components/PopupView.vue` owns list, search, and keyboard
  behavior; `DetailPane.vue` owns the detail card; `SettingsView.vue` owns
  preferences.
- `frontend/wailsjs/` contains generated bindings. Never edit it by hand;
  regenerate it through Wails after changing the Go method set.

## Platform boundaries

- Keep OS-specific packages in the existing shape: a tag-free file declaring
  and documenting the API, plus `_darwin.go` and `_windows.go` implementations
  behind build constraints.
- AppKit integrations use a `.go`, `.h`, and `.m` trio. Preserve that structure
  when extending a macOS platform package.
- Do not put runtime platform checks in shared implementations when build-tagged
  files can express the boundary; shared platform checks can break the Windows
  cross-vet that protects releases.

## Repository invariants

- The version exists in `main.go` as `appVersion` and in `wails.json` as
  `info.productVersion`. Update both with the same bare semantic version, without
  a leading `v`; `version_test.go` and the release workflow enforce this.
- The `sparkle` build tag is additive. Without it, `internal/updater/` must
  compile to the no-updater stub. Do not invert this default because App Store
  builds must not include Sparkle.
- XML comments must never contain `--`. This applies especially to entitlements
  and `docs/appcast.xml`; signing tools can otherwise produce a bundle missing
  required entitlements while still exiting successfully.
- Sign nested code before the outer application bundle. Follow
  `scripts/package-macos.sh`; do not reproduce the signing sequence ad hoc.
- Do not assume `wails build -clean` empties `build/bin/`. App Store packaging
  must reject any Sparkle framework, linked library, updater symbol, or Sparkle
  metadata left by an earlier build.
- Preserve the public appcast URL. Installed copies depend on
  `https://thienanblog.github.io/geda-clipboard/appcast.xml` remaining stable.
- Keep `docs/shared.css` at that exact name; GitHub Pages/Jekyll drops files
  whose names begin with an underscore.

## Releases and distribution

- For a release, update both version sources and move `CHANGELOG.md` entries
  from `Unreleased` into the matching dated version section, including its link
  definitions.
- Do not tag or publish unless explicitly requested. Release tags use `v` plus
  the version from `wails.json`.
- A manual run of `.github/workflows/release.yml` builds artifacts without
  publishing; use it to validate packaging changes without consuming a tag.
- After publishing, regenerate and commit `docs/appcast.xml` using the exact
  process in `scripts/README.md`.
- The Homebrew cask source is the template in `scripts/update-cask.sh`, not the
  generated cask in the separate tap repository. Its checksum must come from
  the published archive, and prereleases must not update the tap.
- App Store `CFBundleVersion` values are single use. A released marketing
  version is also final; shipping another change requires a new marketing
  version in both version sources, not only a higher build number.
- Keep `build/appstore/metadata.md` synchronized with user-visible behavior and
  the public privacy, support, and terms pages under `docs/`.

## Secrets and sensitive data

- Signing certificates, provisioning profiles, App Store Connect keys, and the
  Sparkle private key live outside this repository. Never commit, copy into the
  tree, print, or paste their contents into logs or transcripts.
- Preserve secret handling through environment variables and repository
  secrets. Do not replace opt-in signing or publishing behavior with embedded
  credentials.
- The Sparkle EdDSA private key is not replaceable for installed copies. Treat
  any operation involving it as release-critical.
- App Store screenshots must use synthetic clipboard history. Follow the
  fixture workflow in `scripts/README.md` and verify that no real clipboard
  content is visible before capture.

## Style and verification

- Comments should explain why a decision exists, including the failure mode it
  prevents. Do not replace durable rationale with comments that restate code.
- Write Markdown and commit messages in plain, declarative English without
  marketing adjectives, exclamation marks, or emoji.
- Commit subjects are one imperative line from the reader's point of view; add
  a body only when the reasoning does not fit in the subject.
- For ordinary changes, run the focused checks for the affected area, then the
  applicable CI commands above. Packaging and platform changes require the
  corresponding script or host-specific CI coverage.
- Never claim Windows desktop behavior was manually verified unless it was
  actually exercised on Windows.
