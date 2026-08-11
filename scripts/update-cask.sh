#!/usr/bin/env bash
#
# Renders Casks/geda-clipboard.rb into a checkout of the Homebrew tap and
# commits it. Pushing is left to the caller, so a local run can be reviewed
# before it becomes the thing every `brew upgrade` sees.
#
# Usage:
#   git clone https://github.com/thienanblog/homebrew-tap ../homebrew-tap
#   TAP_DIR=../homebrew-tap ./scripts/update-cask.sh
#   git -C ../homebrew-tap show      # review
#   git -C ../homebrew-tap push
#
# VERSION defaults to wails.json, which is the same field the release workflow
# checks the tag against, so the cask cannot end up describing a version this
# repository never released.
#
# The checksum is taken from the archive attached to the GitHub release, not
# from a local build. Two builds of the same commit are not bit-identical --
# timestamps and the signature see to that -- and Homebrew rejects a download
# whose hash disagrees by a single byte. The only checksum worth writing is the
# one belonging to the file users will actually download.

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

tap_dir="${TAP_DIR:-}"
version="${VERSION:-$(jq -r .info.productVersion wails.json)}"

slug="thienanblog/geda-clipboard"
archive="geda-clipboard-$version-macos-universal.zip"

if [ -z "$tap_dir" ]; then
  echo "TAP_DIR is not set. Point it at a checkout of thienanblog/homebrew-tap." >&2
  exit 1
fi

if [ ! -d "$tap_dir/.git" ]; then
  echo "$tap_dir is not a git checkout." >&2
  exit 1
fi

echo "==> Fetching $archive from the v$version release"
# A directory of its own, thrown away on exit: gh release download refuses to
# overwrite, and hashing whatever an earlier run left behind is exactly the
# mistake this script exists to avoid.
workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT

gh release download "v$version" \
  --repo "$slug" \
  --pattern "$archive" \
  --dir "$workdir"

# macOS ships shasum and no sha256sum; a Linux runner is the other way round
# often enough that picking one and hoping turns a release into a failed job at
# the last step, after the release itself has already gone out.
if command -v shasum >/dev/null 2>&1; then
  sha256="$(shasum -a 256 "$workdir/$archive" | cut -d' ' -f1)"
else
  sha256="$(sha256sum "$workdir/$archive" | cut -d' ' -f1)"
fi
echo "    sha256 $sha256"

echo "==> Writing the cask"
mkdir -p "$tap_dir/Casks"

# Unquoted heredoc: $version and $sha256 are substituted, while Ruby's own
# #{version} passes through untouched -- the brace form is meaningless to the
# shell without a leading dollar. The cask interpolates it itself so the URL
# and the version stanza can never disagree.
cat > "$tap_dir/Casks/geda-clipboard.rb" <<EOF
cask "geda-clipboard" do
  version "$version"
  sha256 "$sha256"

  url "https://github.com/$slug/releases/download/v#{version}/geda-clipboard-#{version}-macos-universal.zip"
  name "Geda Clipboard"
  desc "Menu bar clipboard manager with copy and paste notifications"
  homepage "https://github.com/$slug"

  # The Sparkle feed the app itself polls, so Homebrew learns about a release
  # from the same place, at the same moment, as an already-installed copy.
  livecheck do
    url "https://thienanblog.github.io/geda-clipboard/appcast.xml"
    strategy :sparkle
  end

  # Sparkle updates the app in place. Without this, Homebrew would consider a
  # self-updated copy to be a broken install and reinstall the pinned version
  # over the top of it, quietly walking users backwards.
  auto_updates true
  # Ventura is macOS 13, the LSMinimumSystemVersion in the bundle. The bare
  # symbol is the minimum; Homebrew 6 deprecated the ">= :ventura" string form
  # that used to be needed to say so.
  depends_on macos: :ventura

  app "Geda Clipboard.app"

  # It has no window and no Dock tile, so a running copy would otherwise sit
  # there holding the old bundle open while the new one is written underneath.
  uninstall quit: "com.geda.clipboard"

  # The app is sandboxed, so its real data lives under Containers. The
  # Application Support path is what an unsandboxed build writes, and copies
  # built from source before the sandbox landed left one behind.
  zap trash: [
    "~/Library/Application Support/geda-clipboard",
    "~/Library/Caches/com.geda.clipboard",
    "~/Library/Containers/com.geda.clipboard",
    "~/Library/HTTPStorages/com.geda.clipboard",
    "~/Library/Preferences/com.geda.clipboard.plist",
    "~/Library/Saved Application State/com.geda.clipboard.savedState",
  ]
end
EOF

# Staged before the comparison, not after: on the very first run the cask is
# untracked, and an unstaged diff of a file git has never seen is empty. That
# reads as "nothing changed" and skips the one commit that matters.
git -C "$tap_dir" add Casks/geda-clipboard.rb

# Scoped to the cask, both here and in the commit below. A tap checkout is
# somebody's working directory too, and sweeping up whatever else happened to
# be staged in it would put unrelated edits into a release commit.
if git -C "$tap_dir" diff --cached --quiet -- Casks/geda-clipboard.rb; then
  echo "==> Cask already describes $version with this checksum; nothing to commit"
  exit 0
fi

echo "==> Committing"
git -C "$tap_dir" commit -m "Update geda-clipboard to $version" \
  -- Casks/geda-clipboard.rb

echo "==> Done. Review with 'git -C $tap_dir show', then push."
