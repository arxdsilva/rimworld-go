#!/usr/bin/env bash
# Pixel-diff the render paths over the capture scene list.
#   scripts/pixeldiff.sh OUTDIR [paths] [policy]
# paths: comma separated render.Paths names under test ("" = none, so the run
# only proves A == A', the scene is deterministic). policy: exact (default) | aa.
# Each scene renders old/new/old in one process (-capture-diff) and writes
# OUTDIR/<scene>/{A,B,heat}.png and report.json. Exits non-zero if any fails.
set -u
out=${1:?usage: pixeldiff.sh OUTDIR [paths] [policy]}
paths=${2:-}
policy=${3:-exact}
cd "$(dirname "$0")/.."
mkdir -p "$out"
go build -o bin/game ./cmd/game || exit 2
save=$(mktemp -d)
trap 'rm -rf "$save"' EXIT

# scene name | extra flags. Small maps keep each run to seconds.
scenes=(
  "grown-z1|-scenario grown -capture-zoom 1"
  "paths|-scenario grown -capture-ui paths"
  "lategame|-scenario grown -capture-ui lategame"
  "snow|-scenario grown -biome tundra -capture-ui weather"
  "combat|-scenario grown -capture-ui combat"
  "melee|-scenario grown -capture-ui melee"
  "options-graphics|-capture-ui options-graphics"
  "quests|-capture-ui quests"
)
fail=0
printf '%-18s %-6s %-6s %s\n' SCENE DETERM RESULT DETAIL
for s in "${scenes[@]}"; do
  name=${s%%|*}; flags=${s#*|}
  dir="$out/$name"
  # shellcheck disable=SC2086
  bin/game -noaudio -save-dir "$save" -seed 1 -width 120 -height 120 $flags \
    -capture "$dir/shot.png" -capture-diff "$dir" -capture-diff-paths "$paths" \
    -capture-diff-policy "$policy" >"$dir.log" 2>&1
  rc=$?
  if [ -f "$dir/report.json" ]; then
    det=$(grep -m1 '"deterministic"' "$dir/report.json" | grep -o 'true\|false')
    pass=$(grep -m1 '"pass"' "$dir/report.json" | grep -o 'true\|false')
    reason=$(grep -m1 '"reason"' "$dir/report.json" | sed 's/.*": "//; s/",*$//')
  else
    det=-; pass=false; reason=$(tail -1 "$dir.log")
  fi
  [ "$rc" -eq 0 ] && [ "$pass" = true ] && res=PASS || { res=FAIL; fail=1; }
  printf '%-18s %-6s %-6s %s\n' "$name" "$det" "$res" "$reason"
done
exit $fail
