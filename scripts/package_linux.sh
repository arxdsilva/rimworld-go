#!/usr/bin/env bash
# The Linux installers (.github/workflows/release.yml, make dist-linux-pkgs):
#
#   scripts/package_linux.sh <binary> <tag>        all three below
#   scripts/package_linux.sh appimage <binary> <tag>  → dist/rimworld-go-<tag>-x86_64.AppImage
#   scripts/package_linux.sh pkgs <binary> <tag>      → dist/rimworld-go-<tag>-linux-amd64.deb, -linux-x86_64.rpm
#   scripts/package_linux.sh smoke <tag>              install the .deb, run the AppImage, list the .rpm
#
# The binary must be the cgo build from Ubuntu 22.04 (its glibc is the
# floor players need). appimagetool and its runtime are pinned releases
# checked by sha256; nfpm runs through go run at a pinned version. The
# icons come from packaging/icons; ALLOW_NO_ICON=1 skips them for a local
# layout test only.
set -euo pipefail
# shellcheck source=packaging/smoke.sh
. packaging/smoke.sh
usage() { sed -n '2,7p' "$0" >&2; exit 2; }
app=rimworld-go
nfpm_pkg=github.com/goreleaser/nfpm/v2/cmd/nfpm@v2.48.0
appimagetool_url=https://github.com/AppImage/appimagetool/releases/download/1.9.1/appimagetool-x86_64.AppImage
appimagetool_sha=ed4ce84f0d9caff66f50bcca6ff6f35aae54ce8135408b3fa33abfc3cb384eb0
runtime_url=https://github.com/AppImage/type2-runtime/releases/download/20251108/runtime-x86_64
runtime_sha=2fca8b443c92510f1483a883f60061ad09b46b978b2631c807cd873a47ec260d
icon_sizes="48 128 256 512"

need_icons() {
  local s missing=""
  for s in $icon_sizes; do
    [ -f "packaging/icons/$app-$s.png" ] || missing="$missing packaging/icons/$app-$s.png"
  done
  if [ -n "$missing" ] && [ "${ALLOW_NO_ICON:-}" != 1 ]; then
    echo "package_linux: missing$missing (make app-icon); ALLOW_NO_ICON=1 skips them for a local test" >&2
    exit 1
  fi
}

# stage_dir copies the binary and packaging/assets.txt into dist/<name>/.
stage_dir() {
  local bin=$1 name=$2
  scripts/package_release.sh "$bin" "$name" dir >&2
  if [ "$(basename "$bin")" != "$app" ]; then
    mv "dist/$name/$(basename "$bin")" "dist/$name/$app"
  fi
  printf 'dist/%s' "$name"
}

# libs records what the binary links and the newest glibc symbol it needs.
libs() {
  local bin=$1
  if command -v ldd >/dev/null && ldd "$bin" >/dev/null 2>&1; then
    echo "package_linux: ldd $bin"
    ldd "$bin"
    echo "package_linux: newest glibc symbol: $(objdump -T "$bin" | grep -o 'GLIBC_[0-9.]*' | sort -uV | tail -1)"
  fi
}

# fetch downloads url to dist/tools/<name> once and checks its sha256.
fetch() {
  local url=$1 sha=$2 name=$3 out=dist/tools/$3
  mkdir -p dist/tools
  if [ ! -f "$out" ]; then
    curl -fsSL --retry 3 -o "$out.part" "$url"
    mv "$out.part" "$out"
  fi
  if ! echo "$sha  $out" | sha256sum -c - >/dev/null 2>&1 && ! echo "$sha  $out" | shasum -a 256 -c - >/dev/null 2>&1; then
    rm -f "$out"
    echo "package_linux: $name does not match its pinned sha256" >&2
    exit 1
  fi
  chmod +x "$out"
}

appimage() {
  local bin=$1 tag=$2 stage dir out s
  need_icons
  stage=$(stage_dir "$bin" "$app-$tag-appdir-stage")
  dir=dist/$app.AppDir
  rm -rf "$dir"
  mkdir -p "$dir/usr/lib" "$dir/usr/share/applications"
  mv "$stage" "$dir/usr/lib/$app"
  cp packaging/linux/AppRun "$dir/AppRun"
  chmod 0755 "$dir/AppRun" "$dir/usr/lib/$app/$app"
  cp packaging/linux/$app.desktop "$dir/$app.desktop"
  cp packaging/linux/$app.desktop "$dir/usr/share/applications/"
  for s in $icon_sizes; do
    if [ -f "packaging/icons/$app-$s.png" ]; then
      mkdir -p "$dir/usr/share/icons/hicolor/${s}x$s/apps"
      cp "packaging/icons/$app-$s.png" "$dir/usr/share/icons/hicolor/${s}x$s/apps/$app.png"
    fi
  done
  if [ -f "packaging/icons/$app-256.png" ]; then
    cp "packaging/icons/$app-256.png" "$dir/$app.png"
    ln -sf "$app.png" "$dir/.DirIcon"
  fi
  if [ "$(uname -s)-$(uname -m)" != Linux-x86_64 ]; then
    echo "package_linux: AppDir laid out at $dir; appimagetool runs on x86_64 Linux only, so no AppImage here"
    return 0
  fi
  fetch "$appimagetool_url" "$appimagetool_sha" appimagetool
  fetch "$runtime_url" "$runtime_sha" runtime-x86_64
  out=dist/$app-$tag-x86_64.AppImage
  rm -f "$out"
  APPIMAGE_EXTRACT_AND_RUN=1 ARCH=x86_64 dist/tools/appimagetool --no-appstream \
    --runtime-file dist/tools/runtime-x86_64 "$dir" "$out"
  rm -rf "$dir"
  echo "package_linux: $out ($(du -h "$out" | cut -f1))"
}

pkgs() {
  local bin=$1 tag=$2 stage fmt out
  need_icons
  stage=$(stage_dir "$bin" "$app-linux-pkg-stage") # the path packaging/linux/nfpm.yaml reads
  chmod 0755 "$stage/$app"
  libs "$stage/$app"
  for fmt in deb rpm; do
    case $fmt in
      deb) out=dist/$app-$tag-linux-amd64.deb ;;
      rpm) out=dist/$app-$tag-linux-x86_64.rpm ;;
    esac
    rm -f "$out"
    VERSION=${tag#v} go run "$nfpm_pkg" package \
      --config packaging/linux/nfpm.yaml --packager "$fmt" --target "$out"
  done
  rm -rf "$stage"
}

# smoke installs the .deb with apt (its dependencies must resolve), runs
# the installed game and the AppImage from /, then removes the package;
# the .rpm's file list must hold the same layout. It opens no window.
smoke() {
  local tag=$1 deb rpm img out list f root
  deb=dist/$app-$tag-linux-amd64.deb
  rpm=dist/$app-$tag-linux-x86_64.rpm
  img=dist/$app-$tag-x86_64.AppImage
  sudo apt-get install -y "./$deb"
  out=$(cd / && $app -version)
  [ "$out" = "$tag" ] || { echo "smoke: -version printed '$out', want '$tag'" >&2; exit 1; }
  out=$(cd / && $app -selfcheck) || { echo "$out"; echo "smoke: .deb -selfcheck failed" >&2; exit 1; }
  echo "$out"
  check_inside "$out" /opt/$app/assets
  for f in /usr/share/applications/$app.desktop /usr/share/icons/hicolor/256x256/apps/$app.png; do
    [ -f "$f" ] || { echo "smoke: the .deb did not install $f" >&2; exit 1; }
  done
  sudo apt-get remove -y $app
  if [ -e /opt/$app ] || [ -e /usr/bin/$app ]; then
    echo "smoke: removing the .deb left files behind" >&2
    exit 1
  fi

  chmod +x "$img"
  img=$(cd "$(dirname "$img")" && pwd -P)/$(basename "$img")
  out=$(cd / && "$img" --appimage-extract-and-run -version)
  [ "$out" = "$tag" ] || { echo "smoke: AppImage -version printed '$out', want '$tag'" >&2; exit 1; }
  out=$(cd / && "$img" --appimage-extract-and-run -selfcheck) || { echo "$out"; echo "smoke: AppImage -selfcheck failed" >&2; exit 1; }
  echo "$out"
  root=$(printf '%s\n' "$out" | sed -n 's|^ok       art: \(.*/usr/lib/rimworld-go/assets\)$|\1|p')
  [ -n "$root" ] || { echo "smoke: the AppImage did not find its own assets" >&2; exit 1; }
  check_inside "$out" "$root"

  list=$(rpm -qpl "$rpm")
  for f in /opt/$app/$app /opt/$app/assets/lang /usr/bin/$app /usr/share/applications/$app.desktop \
    /usr/share/icons/hicolor/512x512/apps/$app.png; do
    grep -qx "$f" <<< "$list" || { echo "smoke: $rpm lacks $f" >&2; exit 1; }
  done
  rpm -qpR "$rpm"
  echo "smoke: linux packages ok"
}

case ${1:-} in
  appimage) [ $# -eq 3 ] || usage; appimage "$2" "$3" ;;
  pkgs) [ $# -eq 3 ] || usage; pkgs "$2" "$3" ;;
  smoke) [ $# -eq 2 ] || usage; smoke "$2" ;;
  "" | -h | --help) usage ;;
  *) [ $# -eq 2 ] || usage; pkgs "$1" "$2"; appimage "$1" "$2" ;;
esac
