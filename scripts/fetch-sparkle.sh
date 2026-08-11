#!/usr/bin/env bash
#
# Downloads Sparkle and unpacks the pieces the build needs:
#
#   build/darwin/Frameworks/Sparkle.framework   linked at build time, embedded
#                                               in the bundle at package time
#   build/darwin/sparkle-bin/                   sign_update, generate_appcast
#
# Both are gitignored. Run this before "wails build -tags sparkle" and before
# scripts/package-macos.sh; both fail with a clear message if it has not run.
#
# The version and its checksum are pinned. An update framework is about as
# sensitive as a dependency gets: it decides what code replaces the app on a
# user's machine. Taking whatever "latest" happens to serve today would mean
# the contents of a release could change without the release changing.

set -euo pipefail

SPARKLE_VERSION="2.9.5"
SPARKLE_SHA256="015336b601493e05c237964954bff6191370003d94edefe663724c88840d73cc"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
frameworks_dir="$repo_root/build/darwin/Frameworks"
tools_dir="$repo_root/build/darwin/sparkle-bin"

archive="Sparkle-$SPARKLE_VERSION.tar.xz"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

echo "==> Downloading Sparkle $SPARKLE_VERSION"
curl --fail --location --silent --show-error \
  --output "$work/$archive" \
  "https://github.com/sparkle-project/Sparkle/releases/download/$SPARKLE_VERSION/$archive"

echo "==> Verifying checksum"
actual="$(shasum -a 256 "$work/$archive" | cut -d' ' -f1)"
if [ "$actual" != "$SPARKLE_SHA256" ]; then
  echo "Checksum mismatch for $archive." >&2
  echo "  expected: $SPARKLE_SHA256" >&2
  echo "  actual:   $actual" >&2
  echo "Refusing to unpack. Either the pin is stale or the download is not what it claims to be." >&2
  exit 1
fi

echo "==> Unpacking"
tar -xf "$work/$archive" -C "$work"

rm -rf "$frameworks_dir/Sparkle.framework" "$tools_dir"
mkdir -p "$frameworks_dir" "$tools_dir"

# ditto, not cp: a framework is a tree of symlinks (Versions/Current and the
# top-level aliases), and cp -R flattens them into copies, which breaks both
# the code signature and the @rpath lookup at launch.
ditto "$work/Sparkle.framework" "$frameworks_dir/Sparkle.framework"
ditto "$work/bin/sign_update" "$tools_dir/sign_update"
ditto "$work/bin/generate_appcast" "$tools_dir/generate_appcast"

echo "==> Done"
echo "    framework: ${frameworks_dir#"$repo_root"/}/Sparkle.framework"
echo "    tools:     ${tools_dir#"$repo_root"/}/"
