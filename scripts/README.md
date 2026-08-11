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

## `package-appstore.sh` — the Mac App Store build

```bash
BUILD_NUMBER=12 \
APP_IDENTITY="3rd Party Mac Developer Application: An Vu (88BTYX26S4)" \
INSTALLER_IDENTITY="3rd Party Mac Developer Installer: An Vu (88BTYX26S4)" \
PROFILE=~/path/Geda_Clipboard_MAS.provisionprofile \
  ./scripts/package-appstore.sh
```

`BUILD_NUMBER` has to increase on every upload; App Store Connect rejects a
`CFBundleVersion` it has seen before, and a rejected review means uploading
again under the same marketing version.

The script refuses to package if a Sparkle framework, a linked library, an
updater symbol or a Sparkle `Info.plist` key survived into the bundle. That is
not paranoia: `wails build -clean` does not empty `build/bin`, so a framework
left there by `package-macos.sh` really does persist into the next bundle, and
Apple scans the bundle rather than the source.

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
