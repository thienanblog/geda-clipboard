#!/usr/bin/env bash
#
# Builds and packages the Developer ID bundle: the one published on GitHub,
# with Sparkle in it. The App Store bundle is a different script and does not
# go through here.
#
# Usage:
#   ./scripts/fetch-sparkle.sh          # once, or when the pin moves
#   ./scripts/package-macos.sh          # unsigned, for a local smoke test
#   SIGN_IDENTITY="Developer ID Application: An Vu (88BTYX26S4)" \
#     ./scripts/package-macos.sh        # signed
#
# security find-identity -v -p codesigning prints the exact string. It is the
# Developer ID one; the 3rd Party Mac Developer certificate next to it belongs
# to the App Store build and produces a bundle the notary service rejects.
#
# Notarization is not done here. CI does it, and doing it locally would mean
# holding an App Store Connect key on a developer machine for no gain.

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

app="build/bin/Geda Clipboard.app"
framework_src="build/darwin/Frameworks/Sparkle.framework"
identity="${SIGN_IDENTITY:-}"

# Where Sparkle looks for the appcast, and the public half of the key it checks
# every download against. The private half lives in the login Keychain and is
# never in this repository.
feed_url="https://thienanblog.github.io/geda-clipboard/appcast.xml"
public_ed_key="mbp25RgkuCaUTgYkpi2o3HHyw0CYvPNLrox5Hv9NPJA="

if [ ! -d "$framework_src" ]; then
  echo "Sparkle.framework is missing. Run ./scripts/fetch-sparkle.sh first." >&2
  exit 1
fi

echo "==> Building (universal, -tags sparkle)"
# Wails generates its TypeScript bindings by linking a throwaway binary in
# /tmp and running it. That binary pulls in Sparkle along with the rest of the
# app, but it is nowhere near the framework and has no rpath of its own, so it
# dies in dyld before it can emit anything and takes the build with it. This
# tells dyld where to look for that one run; the shipped binary finds Sparkle
# through the rpath added further down instead, and carries no absolute path
# from this machine.
export DYLD_FRAMEWORK_PATH="$repo_root/build/darwin/Frameworks"

wails build -clean -platform darwin/universal -tags sparkle

echo "==> Embedding Sparkle"
mkdir -p "$app/Contents/Frameworks"
rm -rf "$app/Contents/Frameworks/Sparkle.framework"
ditto "$framework_src" "$app/Contents/Frameworks/Sparkle.framework"

framework="$app/Contents/Frameworks/Sparkle.framework"

# The Downloader service exists so an app can fetch updates without the network
# entitlement. This build has that entitlement, on Sparkle's own advice, so the
# service is dead weight: it ships a deprecated WebView and is one more nested
# bundle to sign. The Installer service stays; a sandboxed app cannot install
# an update without it.
rm -rf "$framework/Versions/B/XPCServices/Downloader.xpc"

echo "==> Adding the runtime search path"
# Sparkle's install name is @rpath relative, so the executable needs to know
# that @rpath means Contents/Frameworks. This is done here rather than through
# cgo LDFLAGS because cgo rejects -Wl,-rpath unless every build exports
# CGO_LDFLAGS_ALLOW, and a flag that has to be remembered is one that gets
# missed. It must happen before signing: it rewrites the Mach-O header and
# invalidates any signature already there.
install_name_tool -add_rpath @executable_path/../Frameworks "$app/Contents/MacOS/Geda Clipboard" 2>/dev/null \
  || echo "    (rpath already present)"

echo "==> Writing Sparkle's Info.plist keys"
plist="$app/Contents/Info.plist"
set_plist() {
  # Add fails if the key exists and Set fails if it does not, so try both and
  # let whichever applies win. Keeps the script safe to run over a stale build.
  /usr/libexec/PlistBuddy -c "Add :$1 $2 $3" "$plist" 2>/dev/null \
    || /usr/libexec/PlistBuddy -c "Set :$1 $3" "$plist"
}
set_plist SUFeedURL string "$feed_url"
set_plist SUPublicEDKey string "$public_ed_key"
# Required for a sandboxed app: without it Sparkle cannot reach the installer
# that does the work outside the sandbox, and updates fail at the last step.
set_plist SUEnableInstallerLauncherService bool true

if [ -z "$identity" ]; then
  echo "==> SIGN_IDENTITY not set; leaving the bundle unsigned"
  echo "    Sparkle will refuse to install an update into an unsigned app."
  echo "    Fine for checking that the app launches, not for anything else."
  exit 0
fi

echo "==> Signing"
# Inside out. Signing the outer bundle first would seal a hash of nested code
# that is about to change, and the result fails verification in a way that
# points at the app rather than at the order it was signed in.
sign() {
  codesign --force --options runtime --timestamp \
    --sign "$identity" "$@"
}

sign "$framework/Versions/B/XPCServices/Installer.xpc"
sign "$framework/Versions/B/Updater.app"
sign "$framework/Versions/B/Autoupdate"
sign "$framework"
codesign --force --options runtime --timestamp \
  --entitlements build/darwin/entitlements.plist \
  --sign "$identity" "$app"

echo "==> Verifying"
codesign --verify --strict --deep --verbose=2 "$app"

# codesign reports success even when it could not parse the entitlements file,
# having signed with neither them nor the hardened runtime. Read both back.
#
# Captured into variables rather than piped into grep -q. Under pipefail, a
# matching grep -q is the failure case: it exits at the first hit, codesign
# takes SIGPIPE on the closed pipe, and the pipeline reports the non-zero
# status of the upstream command. The check then fails precisely when the
# signature is correct.
signature_info="$(codesign -dvvv "$app" 2>&1)"
case "$signature_info" in
  *flags=*runtime*) ;;
  *)
    echo "Signed without the hardened runtime; notarization would reject this." >&2
    exit 1
    ;;
esac

signed_entitlements="$(codesign -d --entitlements - --xml "$app" 2>/dev/null || true)"
case "$signed_entitlements" in
  *app-sandbox*) ;;
  *)
    echo "Entitlements did not make it into the signature." >&2
    exit 1
    ;;
esac

echo "==> Done: $app"
