# Release scripts

## `fetch-sparkle.sh`

Downloads the pinned Sparkle release, checks it against a recorded SHA-256, and
unpacks two things that are both gitignored:

- `build/darwin/Frameworks/Sparkle.framework` — linked when building with
  `-tags sparkle`, embedded in the bundle at package time
- `build/darwin/sparkle-bin/` — `sign_update` and `generate_appcast`

Run it before either packaging script. Both fail with a clear message if it has
not run.

## `package-macos.sh` — the GitHub build

```bash
./scripts/fetch-sparkle.sh
SIGN_IDENTITY="Developer ID Application: An Vu (88BTYX26S4)" ./scripts/package-macos.sh
```

Omit `SIGN_IDENTITY` for an unsigned bundle, which is enough to check that the
app launches and nothing more: Sparkle refuses to install an update into an
unsigned app.

Notarization is not done here. CI does it, so an App Store Connect key never
has to sit on a developer machine.

This build is made with `-tags sparkle,axpaste`. Both tags are additive:
`sparkle` adds the updater and `axpaste` adds the Cmd+V keystroke that lands a
chosen entry back in the app the user came from. Neither belongs in an App
Store submission, and forgetting one yields a build without that feature rather
than a submission carrying it.

## `package-appstore.sh` — the Mac App Store build

```bash
BUILD_NUMBER=12 \
APP_IDENTITY="3rd Party Mac Developer Application: An Vu (88BTYX26S4)" \
INSTALLER_IDENTITY="3rd Party Mac Developer Installer: An Vu (88BTYX26S4)" \
PROFILE=~/path/Geda_Clipboard_MAS.provisionprofile \
  ./scripts/package-appstore.sh
```

This build is made with no tags at all, so it carries neither Sparkle nor the
keystroke path. The script then proves both are absent from the finished
binary, because App Review inspects the bundle rather than the source: an
updater framework is guideline 2.4.5, and so is using Accessibility to send
Cmd+V to another application, which is what rejected version 0.7.0. See
`internal/clipboard/clipboard_nopaste_darwin.go`.

`BUILD_NUMBER` has to increase on every upload, including uploads under a new
marketing version. App Store Connect rejects a `CFBundleVersion` that is not
higher than every build previously uploaded for the app.

Before it builds anything, the script runs `scripts/asc` against App Store
Connect and refuses a version that would be rejected. Set the API credentials
to enable it:

```bash
AC_API_KEY_P8=~/path/AuthKey_XXXXXXXXXX.p8 \
AC_API_KEY_ID=XXXXXXXXXX \
AC_API_ISSUER_ID=<uuid from Users and Access, Integrations> \
  ...
```

Without them the check prints a warning and skips, so a fork can still package.
With them it catches the two failures that are otherwise invisible until after
the upload and the processing pass: a build number that is not higher than
every previous upload, and a marketing version that has already been released.
The second one cannot be
fixed by raising the build number — a released version is spent, and shipping
a change means creating a new version in App Store Connect and bumping
`wails.json` and `main.go` to match.

`build/appstore/metadata.md` holds everything App Store Connect asks for in its
own fields, so a submission is copying rather than rewriting.

The script refuses to package if a Sparkle framework, a linked library, an
updater symbol or a Sparkle `Info.plist` key survived into the bundle. That is
not paranoia: `wails build -clean` does not empty `build/bin`, so a framework
left there by `package-macos.sh` really does persist into the next bundle, and
Apple scans the bundle rather than the source.

## `update-cask.sh` — the Homebrew cask

Renders `Casks/geda-clipboard.rb` into a checkout of
[thienanblog/homebrew-tap](https://github.com/thienanblog/homebrew-tap) and
commits it. Pushing is left to you, so a local run can be reviewed first.

```bash
git clone https://github.com/thienanblog/homebrew-tap ../homebrew-tap
TAP_DIR=../homebrew-tap ./scripts/update-cask.sh
git -C ../homebrew-tap show
git -C ../homebrew-tap push
```

The release workflow runs the same script after publishing a tag, so this is
normally only needed to repair a cask by hand. It is skipped for pre-releases:
a tap has one cask per app and no notion of a channel, so pushing `0.6.0-rc.1`
there would hand a release candidate to everyone running `brew upgrade`.

The checksum is taken from the archive attached to the GitHub release, never
from a local build. Two builds of one commit are not bit-identical — timestamps
and the signature see to that — and Homebrew rejects a download whose hash is
off by a byte.

`brew style --cask <path>` is worth running after a change to the template. The
`--online` audit is stricter still, but it insists on a current Xcode.

## `screenshot-fixture` — App Store screenshots

Store screenshots have to show a full history, and the one history to hand is
the developer's own. That is precisely the wrong thing to publish: a clipboard
manager's screenshots are where a real password or API key would end up on a
public store page. This writes a synthetic one instead.

```bash
go run ./scripts/screenshot-fixture -home /tmp/geda-shots
```

It builds the history through `internal/store`, so IDs, hashes, blob names and
the index layout come from the same code the app uses and cannot drift from the
schema. It refuses to run against your real home directory.

The app resolves its data directory from `HOME`, so in principle
`HOME=/tmp/geda-shots wails dev` runs it against the fixture. **It does not.**
`wails dev` relaunches the bundle through `open`, which starts it from launchd
with a clean environment, so `HOME` never arrives and the app loads the real
history. Check what is on screen before capturing anything.

The five images in the submission were taken from `frontend/dist` served
statically with the Wails bindings stubbed out from the fixture, then composed
onto 2880x1800 canvases with headless Chrome. That harness is not in the
repository: it has to know the shape of the bound API, and a copy of that here
would rot silently the first time a method changes. Rebuild it when screenshots
are next needed, or capture the real window and crop to one of the four sizes
App Store Connect accepts.

## Publishing an update to the appcast

After a release is published, download its macOS archive into an otherwise
empty folder, then:

```bash
./build/darwin/sparkle-bin/generate_appcast \
  --download-url-prefix "https://github.com/thienanblog/geda-clipboard/releases/download/v0.5.0/" \
  --link "https://github.com/thienanblog/geda-clipboard" \
  -o docs/appcast.xml \
  /path/to/that/folder
```

Commit `docs/appcast.xml`. GitHub Pages serves it from `main`, and that address
is what every installed copy polls, so it has to stay put across releases while
the archives themselves stay on Releases.

The EdDSA private key lives in the login keychain and is never in this
repository. Losing it ends updates permanently: Sparkle only accepts an archive
signed by the key matching the `SUPublicEDKey` already baked into the copy the
user is running, so there is no way to hand them a new one.

## A trap worth knowing

XML forbids `--` inside a comment. `plutil` tolerates it; the parser `codesign`
hands entitlements to does not, and it does something worse than fail — it
rejects the file, signs anyway with neither the entitlements nor the hardened
runtime, and exits zero. That bundle passes `codesign --verify` and fails
notarization for a reason that points somewhere else entirely.

Both entitlements files and `docs/appcast.xml` keep double hyphens out of their
comments for this reason, which is also why the appcast cannot quote the
command above. `package-macos.sh` reads the entitlements and the runtime flag
back out of the finished signature rather than trusting the exit status.
