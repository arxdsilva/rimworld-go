#!/usr/bin/env bash
# Summarises an Ebitengine command log (PERF-007). Build the game with
# `go build -tags ebitenginedebug -o bin/game-ebdebug ./cmd/game`, run a scene
# with stdout to a file, then:
#   scripts/ebdebug.sh <log> [frames to skip, default 120]
# It prints, per frame after the warm-up: executed draw commands (after
# Ebitengine merged them), destination switches, write-pixels, new images,
# and the source lines that started the most commands (each one is a call
# the previous command could not merge with). Counts only: the debug build's
# logging makes its timings meaningless.
set -euo pipefail

log=${1:?usage: scripts/ebdebug.sh <log> [skip]}
skip=${2:-120}
root=$(cd "$(dirname "$0")/.." && pwd)

awk -v skip="$skip" -v root="$root/" '
/^----$/ { frame++; next }
frame <= skip { next }
/^  draw-triangles: / {
	n++; draws++
	dst = $3
	if (dst != lastdst) { switches++ }
	lastdst = dst
	pending = 1
	next
}
/^    first-caller: / && pending {
	c = $2; sub(root, "", c); sub(/ \(internal\)$/, "", c)
	callers[c]++
	pending = 0
	next
}
/^  write-pixels/ { wp++; pending = 0; next }
/^  new-image/ { ni++; pending = 0; next }
/^  / { pending = 0 }
END {
	f = frame - skip
	if (f <= 0) { print "no frames after the warm-up of " skip; exit 1 }
	printf "frames %d\n", f
	printf "draw commands/frame   %8.1f\n", draws / f
	printf "dst switches/frame    %8.1f\n", switches / f
	printf "write-pixels/frame    %8.1f\n", wp / f
	printf "new-image/frame       %8.1f\n", ni / f
	print "top first-callers (commands/frame):"
	for (c in callers) printf "%10.1f  %s\n", callers[c] / f, c | "sort -rn | head -20"
}' "$log"
