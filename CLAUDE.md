# CLAUDE.md

Guidance for AI agents working in this repository. [README.md](README.md) is
written for users and describes what the app does; this file covers what is
easy to get wrong while changing it.

## What this is

A menu bar / system tray clipboard manager for macOS and Windows. Go 1.25 +
[Wails v2](https://wails.io) with a Vue 3 + TypeScript frontend. The macOS
platform layer is cgo against AppKit; the Windows one is pure Go against
`golang.org/x/sys/windows` and `energye/systray`.

macOS is the developed and tested platform. Windows compiles and passes tests
in CI, but nobody has used it on a Windows desktop — treat Windows changes as
unverified and say so rather than claiming they work.

## Commands

```bash
wails build                 # bundle into build/bin/
wails dev                   # hot reload, UI also at http://localhost:34115
go test ./...               # requires a prior wails build (see below)
go vet ./...                # same
gofmt -l .                  # CI fails on any output
```

```bash
GOOS=windows GOARCH=amd64 go vet ./internal/...    # cross-check from a Mac
GOOS=windows GOARCH=arm64 go vet ./internal/...
```

```bash
cd frontend && npx vue-tsc --noEmit    # frontend type check, also run by CI
```

**`wails build` must run before `go vet ./...` or `go test ./...`.** The root
package embeds `frontend/dist` with `//go:embed all:frontend/dist`, and that
directory is gitignored — it does not exist in a fresh checkout, so the root
package cannot be loaded at all until the frontend has been built. This is why
the Windows cross-check above is scoped to `./internal/...`.

## Layout

```
main.go                 Wails setup: frameless, always-on-top, starts hidden.
                        Holds appVersion.
app.go                  The bound API (40 methods on *App), orchestration,
                        capture and paste flow, popup placement
popup_position_test.go  Placement maths, tested without a display
version_test.go         Keeps appVersion and wails.json in step
bundle_metadata_test.go Guards the Info.plist keys App Store Connect requires,
                        and the no-double-hyphen rule on every XML comment

internal/
  tray/        Menu bar item. macOS: native NSStatusItem via cgo. Windows:
               energye/systray. Also reports the pointer and the work area
               around it, which popup placement needs.
  window/      Moves the app's own window in global, all-display coordinates,
               which Wails cannot do
  clipboard/   Change-counter polling, read/write, source-app detection,
               paste keystroke synthesis
  store/       History: dedupe, copy counting, pinning, eviction, persistence
  settings/    Preferences with validation and change callbacks
  notify/      Native notifications (UNUserNotifications / Windows toast)
  imageutil/   Thumbnail generation, bounded at 360x150
  autostart/   Launch at login (SMAppService / Run registry key)
  appdir/      Per-user paths, atomic writes
  updater/     Sparkle, behind a build tag

frontend/src/
  App.vue                    Routes between the two views
  components/PopupView.vue   The list, search, keyboard handling
  components/DetailPane.vue  The floating detail card
  components/SettingsView.vue
  lib/format.ts, lib/keys.ts
  wailsjs/                   Generated bindings. Never hand-edit; wails build
                             regenerates them from the Go method set.

build/darwin/    Info.plist templates, entitlements, appcast example
build/appstore/  The App Store Connect submission sheet
scripts/         Release tooling, each with a header comment
  asc/                 Asks App Store Connect whether a version and build
                       number are still free, and refuses the package if not
  screenshot-fixture/  Writes a synthetic history into a throwaway home, so
                       store screenshots never show a real clipboard
docs/            Served by GitHub Pages from main
  appcast.xml          The live Sparkle feed
  index.html           Landing page
  privacy.html         Privacy policy, linked from App Store Connect
  support.html         Support page, linked from App Store Connect
  terms.html           Terms of use and the MIT licence
  shared.css           Styling for the four pages. Not _shared.css: Jekyll
                       drops anything starting with an underscore, and the
                       page would ship unstyled.
```

### Platform layers

Every package with OS-specific behaviour follows the same shape: a
tag-free file declaring the API and documenting it, then `_darwin.go` and
`_windows.go` implementations behind `//go:build`. macOS files that touch
AppKit come as a `.go` / `.h` / `.m` trio. Keep that shape when adding
anything — a platform check inside a shared file breaks the cross-compile
vet, which is the only thing standing between a Windows change and a broken
release.

## Invariants worth knowing

**The version lives in two files.** `main.go`'s `appVersion` and `wails.json`'s
`info.productVersion` must match; `version_test.go` fails if they do not, and
the release workflow refuses a tag that disagrees with `wails.json`. Bare
semver, no leading `v`.

**`-tags sparkle` is additive, not subtractive.** Without it, `internal/updater`
compiles to a stub and no framework is linked. This direction is deliberate:
a forgotten tag yields a build with no updater (a missing menu item) rather
than an App Store submission carrying an auto-updater, which is a guideline
2.4.5 rejection that is invisible in the source. Do not invert it.

**XML forbids `--` inside a comment.** `plutil` tolerates it; the parser
`codesign` hands entitlements to does not, and it fails in the worst possible
way — it rejects the file, signs anyway with neither the entitlements nor the
hardened runtime, and exits zero. That bundle passes `codesign --verify` and
fails notarization pointing somewhere else entirely. Both entitlements files
and `docs/appcast.xml` keep double hyphens out of their comments for this
reason. `package-macos.sh` reads the entitlements and the runtime flag back out
of the finished signature rather than trusting the exit status.

**Signing is inside out.** Nested code first, outer bundle last. Signing the
outer bundle first seals a hash of code that is about to change, and the
failure points at the app rather than at the order.

**The appcast URL can never move.** `https://thienanblog.github.io/geda-clipboard/appcast.xml`
is baked into every installed copy. The archives live on Releases, where each
URL names its version; only the feed lives on Pages, because a feed needs one
address forever.

**The Sparkle EdDSA private key is irreplaceable.** Sparkle only accepts an
archive signed by the key matching the `SUPublicEDKey` already in the copy the
user is running, so a lost key ends updates permanently for everyone already
installed.

**`wails build -clean` does not empty `build/bin`.** A Sparkle framework left
there by `package-macos.sh` really does survive into the next bundle, which is
why `package-appstore.sh` refuses to package if a framework, a linked library,
an updater symbol or a Sparkle `Info.plist` key made it in. Apple scans the
bundle, not the source.

**Quarantine propagates into the App Store package.** The provisioning profile
is downloaded in a browser, so it carries `com.apple.quarantine`, and the flag
lands on the copy embedded in the bundle — the kernel applies it to the new
file, so `cp -X` does not avoid it. `productbuild` then writes it into the
payload and Transporter rejects the upload with 91109, after the bundle has
launched, `codesign --verify` has passed and everything local has exited zero.
`package-appstore.sh` clears extended attributes before signing and expands the
finished package to prove none survived. Anything else fetched with a browser
and dropped into the bundle arrives the same way.

## Signing material

Certificates and keys live outside the repository, in
`/Users/anvu/simple-projects/geda-secrets/`: the Developer ID `.p12`, the App
Store Connect `AuthKey_*.p8`, the provisioning profile, and the Sparkle private
key. `.gitignore` matches these by extension rather than by path, so a stray
copy anywhere in the tree is caught. Never commit them, never paste their
contents into a file in this repository, and never echo them into a shell
transcript. A leaked `.p12` or App Store Connect key has to be revoked; a
leaked Sparkle key means every installed copy can be handed a forged update.

In CI the same material comes from repository secrets: `MACOS_CERT_P12_BASE64`,
`MACOS_CERT_PASSWORD`, `MACOS_KEYCHAIN_PASSWORD`, `MACOS_SIGN_IDENTITY`,
`AC_API_KEY_P8_BASE64`, `AC_API_KEY_ID`, `AC_API_ISSUER_ID`, and
`HOMEBREW_TAP_TOKEN`. Every one of them is opt-in: the workflow degrades to an
unsigned build or a skipped cask push rather than failing, so a fork still gets
a working release.

## Releasing

1. Bump `appVersion` and `wails.json` together.
2. Move the `Unreleased` entries in `CHANGELOG.md` under the new version with
   today's date; the link definitions at the bottom need updating too.
3. Merge to `main`, wait for CI on both platforms.
4. `git tag -a v1.2.3 -m "v1.2.3" && git push origin v1.2.3`.

The [release workflow](.github/workflows/release.yml) then verifies the tag
against `wails.json`, builds a universal signed and notarized macOS bundle plus
x64/arm64 Windows executables, publishes them with that version's changelog
section as the release notes, and pushes an updated Homebrew cask.

Running the workflow by hand from the Actions tab builds the same archives as
artifacts without publishing anything. That is how to test a packaging change
without burning a tag.

After the release, regenerate `docs/appcast.xml` and commit it — see
[scripts/README.md](scripts/README.md) for the exact invocation.

### The Homebrew cask

The cask lives in [thienanblog/homebrew-tap](https://github.com/thienanblog/homebrew-tap),
not here. `scripts/update-cask.sh` renders it into a checkout of that repo from
the project version and the SHA-256 of the archive attached to the GitHub
release; the `homebrew` job in the release workflow runs that same script and
pushes. Do not hand-edit the cask in the tap — the next release overwrites it.
Change the template in `update-cask.sh` instead.

Three things about it are load-bearing:

- The checksum comes from the **published archive**, never a local build. Two
  builds of one commit differ by timestamps and signature alone, and Homebrew
  rejects a download whose hash is off by a byte.
- `auto_updates true`, because Sparkle updates the app in place. Without it
  Homebrew treats a self-updated copy as broken and reinstalls the pinned
  version over the top, walking users backwards.
- Pre-releases are skipped. A tap has one cask per app and no notion of a
  channel.

Validate a template change with `brew style --cask <path>`. The `--online`
audit is stricter but insists on a current Xcode.

### The Mac App Store

`scripts/package-appstore.sh` builds the submission, and it will not package a
version App Store Connect would reject. Two rules, both enforced by
`scripts/asc` before the build starts:

**A `CFBundleVersion` is single use.** App Store Connect rejects a build number
it has already seen for the same marketing version. `BUILD_NUMBER` must be
strictly higher than the highest already uploaded for that version.

**A released marketing version can never be submitted again, and bumping only
the build number does not reopen it.** Once `CFBundleShortVersionString` has
reached `READY_FOR_SALE`, that version is spent. Shipping a further change
means *creating a new version in App Store Connect* and bumping the app to
match — `appVersion` in `main.go` and `info.productVersion` in `wails.json`,
together, then a changelog entry and a tag. `BUILD_NUMBER` may restart at 1 for
the new version. If a change is ever proposed that raises only the build
number against an already-released version, that is the mistake this paragraph
exists to prevent: say so and bump the marketing version instead.

The check needs `AC_API_KEY_P8` (or `AC_API_KEY_P8_BASE64`), `AC_API_KEY_ID`
and `AC_API_ISSUER_ID`. Without them it prints a warning and skips, the same
way signing is opt-in, so a fork can still package. With them it is the only
thing standing between a wrong version number and a day lost to a rejection.

Everything App Store Connect asks for in its own fields — description,
keywords, copyright, the privacy questionnaire, the accessibility label, the
reviewer notes — is written out in [build/appstore/metadata.md](build/appstore/metadata.md).
Keep it in step with the app: the description promises behaviour, and a
reviewer checks.

The privacy policy, support and terms pages it points at are `docs/*.html`,
served by GitHub Pages from `main` alongside the appcast. A privacy policy URL
that 404s is an automatic rejection, so a change to `docs/` is a change to the
submission.

## House style

The comments in this repository explain **why**, at some length, and are the
main reason it is navigable. They record the failure that motivated the code:
what was tried, what broke, and what the symptom looked like. Match that. A
comment restating what the next line plainly does is worse than none, and a
change that quietly drops an existing explanation loses the only record of a
bug someone already paid for.

Prose in Markdown and in commit messages is plain and declarative, in the same
register — no marketing adjectives, no exclamation marks, no emoji.

Commit messages are a single imperative line describing the change from the
reader's point of view ("Sign and notarize the macOS release", "Drop the
quarantine workaround from the install steps"), with a body only when the
reasoning does not fit.
