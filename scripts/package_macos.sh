#!/usr/bin/env bash
# The macOS installer: a disk image holding rimworld-go.app and a link to
# /Applications (.github/workflows/release.yml, make dist-macos).
#
#   scripts/package_macos.sh <universal-binary> <tag>   → dist/rimworld-go-<tag>-macos.dmg
#   scripts/package_macos.sh smoke <dmg> <tag>          mount, copy the app out, run -version and -selfcheck
#
# The app is ad-hoc signed unless MACOS_SIGN_IDENTITY names a Developer ID
# certificate in the keychain (the release workflow imports one when its
# secrets exist; notarization happens there). The icon comes from
# packaging/icons/rimworld-go.icns; ALLOW_NO_ICON=1 builds without it, for
# local tests only.
set -euo pipefail
# shellcheck source=packaging/smoke.sh
. packaging/smoke.sh
usage() { sed -n '2,6p' "$0" >&2; exit 2; }
app_name=rimworld-go

# version_of v0.3.0-rc1 → 0.3.0: Info.plist versions take digits and dots only.
version_of() {
  local v=${1#v}
  v=${v%%[-+]*}
  if [[ ! $v =~ ^[0-9]+(\.[0-9]+){0,2}$ ]]; then
    v=0.0.0
  fi
  printf '%s' "$v"
}

build() {
  local bin=$1 tag=$2
  local icon=packaging/icons/$app_name.icns
  local version stage root app dmg
  version=$(version_of "$tag")
  if [ ! -f "$icon" ] && [ "${ALLOW_NO_ICON:-}" != 1 ]; then
    echo "package_macos: $icon is missing (make app-icon); ALLOW_NO_ICON=1 skips it for a local test" >&2
    exit 1
  fi
  lipo -info "$bin"
  stage=dist/$app_name-$tag-macos-stage
  scripts/package_release.sh "$bin" "$app_name-$tag-macos-stage" dir
  root=dist/$app_name-$tag-macos-dmg
  app=$root/$app_name.app
  rm -rf "$root"
  mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
  mv "$stage/$(basename "$bin")" "$app/Contents/MacOS/$app_name"
  mv "$stage/assets" "$stage/README.md" "$stage/THIRD_PARTY_NOTICES.md" "$stage/third_party" "$app/Contents/Resources/"
  rm -rf "$stage"
  if [ -f "$icon" ]; then
    cp "$icon" "$app/Contents/Resources/"
  fi
  sed -e "s/@VERSION@/$version/g" -e "s/@TAG@/$tag/g" packaging/macos/Info.plist.in > "$app/Contents/Info.plist"
  plutil -lint "$app/Contents/Info.plist"
  printf 'APPL????' > "$app/Contents/PkgInfo"

  local id=${MACOS_SIGN_IDENTITY:--}
  if [ "$id" = - ]; then
    codesign --force --deep --sign - "$app"
  else
    codesign --force --options runtime --timestamp --sign "$id" "$app"
  fi
  codesign --verify --deep --strict --verbose=1 "$app"

  ln -s /Applications "$root/Applications"
  dmg=dist/$app_name-$tag-macos.dmg
  rm -f "$dmg"
  hdiutil create -volname "$app_name" -srcfolder "$root" -fs HFS+ -format ULFO -ov "$dmg" >/dev/null
  if [ "$id" != - ]; then
    codesign --force --timestamp --sign "$id" "$dmg"
  fi
  rm -rf "$root"
  echo "package_macos: $dmg ($(du -h "$dmg" | cut -f1))"
}

# smoke checks what a player gets: the image mounts, the app copies out and
# its executable answers -version and finds every file inside the bundle.
# It runs from / so the working directory can't lend it the repository's
# assets, and it opens no window.
smoke() {
  local dmg=$1 tag=$2 work mnt app out
  work=$(cd dist && pwd -P)/smoke-macos
  rm -rf "$work"
  mkdir -p "$work/mnt" "$work/apps"
  mnt=$work/mnt
  hdiutil attach -nobrowse -readonly -noautoopen -mountpoint "$mnt" "$dmg" >/dev/null
  trap 'hdiutil detach "$mnt" >/dev/null 2>&1 || true' EXIT
  [ -L "$mnt/Applications" ] || { echo "smoke: no /Applications link in the image" >&2; exit 1; }
  cp -R "$mnt/$app_name.app" "$work/apps/"
  hdiutil detach "$mnt" >/dev/null
  trap - EXIT
  app=$work/apps/$app_name.app
  codesign --verify --deep --strict "$app"
  out=$(cd / && "$app/Contents/MacOS/$app_name" -version)
  [ "$out" = "$tag" ] || { echo "smoke: -version printed '$out', want '$tag'" >&2; exit 1; }
  out=$(cd / && "$app/Contents/MacOS/$app_name" -selfcheck) || { echo "$out"; echo "smoke: -selfcheck failed" >&2; exit 1; }
  echo "$out"
  check_inside "$out" "$app/Contents/Resources/assets"
  rm -rf "$work"
  echo "smoke: $dmg ok"
}

case ${1:-} in
  smoke) [ $# -eq 3 ] || usage; smoke "$2" "$3" ;;
  "") usage ;;
  *) [ $# -eq 2 ] || usage; build "$1" "$2" ;;
esac
