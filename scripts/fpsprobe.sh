#!/usr/bin/env bash
# fpsprobe.sh: the repeatable uncapped frame-rate run behind PERF-002.
#
# Runs the grown 500x500 colony at 3x speed for SECONDS (default 60) at
# zoom 0.1 (the whole map) and 0.8 (close in), with vsync off so the
# display's refresh does not cap the number, and appends the probe lines
# to bin/fps-<tag>.log. The first run after a build pays the first-use
# tint bakes (ENG-001), so run it twice and keep the second.
#
#   scripts/fpsprobe.sh <tag> [seconds]
#   RENDER_SCALE=0.75 scripts/fpsprobe.sh hs6-075
#
# An fps probe is an "art"-class job (make agent-budget): run it alone.
set -euo pipefail
cd "$(dirname "$0")/.."

tag=${1:?usage: scripts/fpsprobe.sh <tag> [seconds]}
seconds=${2:-60}
scale=${RENDER_SCALE:-0}
log=bin/fps-$tag.log

mkdir -p bin
go build -o bin/game ./cmd/game
{
  echo "# $(date '+%Y-%m-%d %H:%M') $tag: $(git rev-parse --short HEAD) seconds=$seconds render-scale=$scale"
  for z in 0.1 0.8; do
    echo "## zoom $z"
    bin/game -noaudio -scenario grown -seed 1 -width 500 -height 500 \
      -capture-speed 2 -capture-zoom "$z" -fpsprobe "$seconds" -vsync=false \
      -render-scale "$scale" 2>&1 | grep 'fps probe' || true
  done
} | tee -a "$log"
