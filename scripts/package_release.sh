#!/usr/bin/env bash
# Package a built game binary with the files it loads at runtime, for a
# GitHub release (.github/workflows/release.yml).
#
#   scripts/package_release.sh <binary> <name> <zip|tar.gz>
#   e.g. scripts/package_release.sh bin/rimworld-go rimworld-go-v0.1.0-macos-universal tar.gz
#
# Writes dist/<name>.<zip|tar.gz> holding <name>/: the binary, the original
# art, sounds and metadata, palettes, the menu splash, README and licenses.
# The vendor sprite pack (a fallback for missing art) and review images are
# left out. The game finds assets/ next to its executable.
set -euo pipefail
if [ $# -ne 3 ]; then
  sed -n '2,6p' "$0" >&2
  exit 2
fi
bin=$1 name=$2 fmt=$3
stage=dist/$name
rm -rf "$stage" "dist/$name.zip" "dist/$name.tar.gz"
mkdir -p "$stage/assets/generated" "$stage/assets/source/raster/ui"
cp "$bin" "$stage/"
cp -R assets/generated/world assets/generated/ui assets/generated/audio "$stage/assets/generated/"
cp -R assets/metadata assets/palettes "$stage/assets/"
cp assets/source/raster/ui/main_menu_colonist.png "$stage/assets/source/raster/ui/"
cp README.md THIRD_PARTY_NOTICES.md "$stage/"
cp -R third_party "$stage/"
cd dist
case $fmt in
  tar.gz) tar -czf "$name.tar.gz" "$name" ;;
  zip)
    if command -v zip >/dev/null; then
      zip -qr "$name.zip" "$name"
    else
      7z a -bso0 "$name.zip" "$name"
    fi
    ;;
  *) echo "package_release: format must be zip or tar.gz" >&2; exit 2 ;;
esac
rm -rf "$name"
echo "package_release: dist/$name.$fmt ($(du -h "$name.$fmt" | cut -f1))"
