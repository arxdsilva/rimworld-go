#!/usr/bin/env bash
# Package a built game binary with the files it loads at runtime, for a
# GitHub release (.github/workflows/release.yml).
#
#   scripts/package_release.sh <binary> <name> <zip|tar.gz|dir>
#   e.g. scripts/package_release.sh bin/rimworld-go rimworld-go-v0.1.0-macos-universal tar.gz
#
# Writes dist/<name>.<zip|tar.gz> holding <name>/: the binary, the runtime
# assets listed in packaging/assets.txt, README and licenses. "dir" leaves
# the folder dist/<name>/ unpacked for the installers to build from
# (scripts/package_{macos,linux}.sh, scripts/package_windows.ps1). The game
# finds assets/ next to its executable.
set -euo pipefail
if [ $# -ne 3 ]; then
  sed -n '2,6p' "$0" >&2
  exit 2
fi
bin=$1 name=$2 fmt=$3
case $fmt in
  tar.gz | zip | dir) ;;
  *) echo "package_release: format must be zip, tar.gz or dir" >&2; exit 2 ;;
esac
stage=dist/$name
rm -rf "$stage" "dist/$name.zip" "dist/$name.tar.gz"
mkdir -p "$stage"
cp "$bin" "$stage/"
# packaging/assets.txt: one repository path per line, # comments
while IFS= read -r entry || [ -n "$entry" ]; do
  entry=${entry%%#*}
  entry=$(printf '%s' "$entry" | tr -d '[:space:]')
  [ -n "$entry" ] || continue
  if [ ! -e "$entry" ]; then
    echo "package_release: $entry (packaging/assets.txt) is missing" >&2
    exit 1
  fi
  mkdir -p "$stage/$(dirname "$entry")"
  cp -R "$entry" "$stage/$(dirname "$entry")/"
done < packaging/assets.txt
cp README.md THIRD_PARTY_NOTICES.md "$stage/"
cp -R third_party "$stage/"
if [ "$fmt" = dir ]; then
  echo "package_release: $stage/ ($(du -sh "$stage" | cut -f1))"
  exit 0
fi
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
esac
rm -rf "$name"
echo "package_release: dist/$name.$fmt ($(du -h "$name.$fmt" | cut -f1))"
