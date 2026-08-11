#!/usr/bin/env bash
#
# Builds and packages the Mac App Store submission: no Sparkle, sandboxed,
# signed with Apple Distribution and wrapped in a signed .pkg.
#
# Usage:
#   BUILD_NUMBER=12 \
#   APP_IDENTITY="3rd Party Mac Developer Application: An Vu (88BTYX26S4)" \
#   INSTALLER_IDENTITY="3rd Party Mac Developer Installer: An Vu (88BTYX26S4)" \
#   PROFILE=~/path/Geda_Clipboard_MAS.provisionprofile \
#     ./scripts/package-appstore.sh
#
# Upload the resulting .pkg with Transporter.
#
# Newer accounts see these two certificates named "Apple Distribution" and
# "Mac Installer Distribution"; they are the same thing under later names.
# Take whichever spelling security find-identity -v actually prints.

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

app="build/bin/Geda Clipboard.app"
version="$(jq -r .info.productVersion wails.json)"
build_number="${BUILD_NUMBER:?set BUILD_NUMBER; App Store Connect rejects a CFBundleVersion it has seen before}"
app_identity="${APP_IDENTITY:?set APP_IDENTITY}"
installer_identity="${INSTALLER_IDENTITY:?set INSTALLER_IDENTITY}"
profile="${PROFILE:?set PROFILE to the Mac App Store provisioning profile}"
pkg="geda-clipboard-$version.pkg"

# Ask App Store Connect what it already has before spending ten minutes on a
# universal build. Both failures this catches are invisible until after the
# upload and the processing pass: a CFBundleVersion that has been seen before,
# and -- the expensive one -- a marketing version that has already been
# released, which no build number can reopen. Skipped, with a warning, when the
# API credentials are not in the environment.
echo "==> Checking the version against App Store Connect"
go run ./scripts/asc preflight \
  -bundle-id com.geda.clipboard \
  -version "$version" \
  -build "$build_number"

echo "==> Building (universal, no sparkle tag)"
wails build -clean -platform darwin/universal

# wails -clean does not empty build/bin, so a Sparkle.framework left behind by
# scripts/package-macos.sh survives into this bundle. The Go code that uses it
# is compiled out, the binary does not link it and nothing would load it, but
# Apple scans the bundle, not the source: an updater framework sitting in
# Contents/Frameworks reads as a mechanism for shipping code outside the store,
# and that is guideline 2.4.5. Remove it, then prove it is gone.
rm -rf "$app/Contents/Frameworks/Sparkle.framework"
[ -d "$app/Contents/Frameworks" ] && rmdir "$app/Contents/Frameworks" 2>/dev/null || true

echo "==> Checking the bundle carries no updater"
fail=0
if [ -e "$app/Contents/Frameworks/Sparkle.framework" ]; then
  echo "    Sparkle.framework is still in the bundle." >&2
  fail=1
fi
linked="$(otool -L "$app/Contents/MacOS/Geda Clipboard" 2>/dev/null || true)"
case "$linked" in
  *[Ss]parkle*) echo "    The executable links Sparkle." >&2; fail=1 ;;
esac
# The tag is what keeps these symbols out. If the build was made with it by
# mistake, this is the last place to notice before Apple does.
symbols="$(strings -a "$app/Contents/MacOS/Geda Clipboard" 2>/dev/null || true)"
case "$symbols" in
  *SPUStandardUpdater*|*SUFeedURL*|*sparkle-project*)
    echo "    Updater symbols are present in the executable." >&2
    fail=1
    ;;
esac
for key in SUFeedURL SUPublicEDKey SUEnableInstallerLauncherService; do
  if /usr/libexec/PlistBuddy -c "Print :$key" "$app/Contents/Info.plist" >/dev/null 2>&1; then
    echo "    Info.plist still declares $key." >&2
    fail=1
  fi
done
if [ "$fail" -ne 0 ]; then
  echo "Refusing to package: this bundle would be rejected." >&2
  exit 1
fi
echo "    clean"

echo "==> Embedding the provisioning profile"
# A Wails build has no Xcode to do this, and without it the upload is refused
# before review ever sees the app.
cp "$profile" "$app/Contents/embedded.provisionprofile"

echo "==> Setting the build number"
/usr/libexec/PlistBuddy -c "Set :CFBundleVersion $build_number" "$app/Contents/Info.plist"

echo "==> Stripping extended attributes"
# The provisioning profile is downloaded from developer.apple.com in a browser,
# so it arrives quarantined, and the quarantine flag propagates to the copy the
# line above makes -- cp -X does not help, because the kernel applies the flag
# to the new file rather than copying it across. productbuild then carries the
# attribute into the payload, and Transporter refuses the upload:
#
#   Invalid package contents. The package contains one or more files with the
#   com.apple.quarantine extended file attribute, such as
#   "com.geda.clipboard.pkg/Payload/Geda Clipboard.app/Contents/
#   embedded.provisionprofile". (91109)
#
# Nothing before the upload notices: the bundle launches, codesign --verify
# passes, and productbuild exits zero. Clear the attributes off the whole
# bundle rather than just the profile, since anything else fetched with a
# browser would land the same way. This has to happen before signing --
# com.apple.FinderInfo and resource forks make codesign itself fail, and
# clearing them afterwards would mean touching a signed bundle.
#
# com.apple.provenance survives; it is applied by the system and cannot be
# removed. It does not reach the payload and Apple does not object to it.
xattr -cr "$app"

echo "==> Signing"
# No --options runtime here: the hardened runtime is a Developer ID
# requirement, and the App Store does not ask for it.
codesign --force --timestamp \
  --entitlements build/darwin/entitlements.appstore.plist \
  --sign "$app_identity" "$app"

codesign --verify --strict --deep --verbose=2 "$app"

signed_entitlements="$(codesign -d --entitlements - --xml "$app" 2>/dev/null || true)"
case "$signed_entitlements" in
  *app-sandbox*) ;;
  *) echo "The sandbox entitlement did not make it into the signature." >&2; exit 1 ;;
esac
case "$signed_entitlements" in
  *temporary-exception*)
    echo "A temporary exception reached the App Store build." >&2
    exit 1
    ;;
esac
# WKWebView's helper processes will not launch under the sandbox without this,
# and the bundle that results looks healthy from the outside: it launches, it
# captures the clipboard, and its popup is an empty panel. Nothing downstream
# catches that, so check the signature says what the entitlements file says.
case "$signed_entitlements" in
  *network.client*) ;;
  *)
    echo "The outgoing network entitlement is missing; the popup will be blank." >&2
    exit 1
    ;;
esac

echo "==> Building the installer package"
rm -f "$pkg"
productbuild --component "$app" /Applications \
  --sign "$installer_identity" "$pkg"

echo "==> Checking the payload carries no quarantine"
# Read it back out of the finished package rather than trusting the strip
# above, for the same reason package-macos.sh reads the entitlements back out
# of the signature: the failure this guards against is silent locally and only
# shows up as a rejected upload. --expand-full unpacks the payload with its
# extended attributes intact, which is the only way to see what Transporter
# will see.
expanded="$(mktemp -d)"
trap 'rm -rf "$expanded"' EXIT
rm -rf "$expanded/pkg"
pkgutil --expand-full "$pkg" "$expanded/pkg" >/dev/null
# Captured rather than tested through a pipeline: under pipefail an xattr
# failure would mask a grep match and turn the check into a no-op.
quarantined="$(xattr -lr "$expanded/pkg" 2>/dev/null | grep com.apple.quarantine || true)"
if [ -n "$quarantined" ]; then
  echo "    Quarantined files in the payload:" >&2
  printf '%s\n' "$quarantined" >&2
  echo "Refusing the package: Transporter would reject it with 91109." >&2
  exit 1
fi
echo "    clean"

echo "==> Done: $pkg"
echo "    Upload it with Transporter."
