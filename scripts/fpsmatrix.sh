#!/usr/bin/env bash
# fpsmatrix.sh: the GAR "always 60" probe matrix (PERF-004).
#
# Runs the grown 500x500 colony through every cell below with VSync ON
# (the display's 60 Hz is the target, and on the scaled 5K display VSync
# off has caused kernel panics: PERF-003), one cell at a time, and writes
#
#   bin/fpsmatrix-<tag>.log   every probe line, with a header per cell
#   bin/fpsmatrix-<tag>.csv   one row per cell from the "fps probe detail" line
#
#   scripts/fpsmatrix.sh <tag> [seconds]        (default 45 s per cell)
#   CELLS="quiet-z0.8-s3 daycross-s1" scripts/fpsmatrix.sh pick
#
# A cell is <scene>-z<zoom>-s<speed>; speed is the -capture-speed tier
# (1 = 1x, 2 = 3x, 3 = 6x). The daycross cells start at 23:00, so midnight
# (every daily and hourly site at once) falls inside the measured span at
# 1x and early in it at 6x. The autosave cell starts at 05:00, so the 06:00
# autosave falls about 40 s into the measured span at 1x; it starts on day 1,
# since the game never autosaves on day 0. Saves go to
# bin/fpsmatrix-saves (-save-dir), never the player's folder, and the log
# notes each cell's new save files. The first run after a build pays first-use
# bakes: run the matrix twice and keep the second. An fps probe is an
# "art"-class job (make agent-budget): run it alone, window focused.
set -euo pipefail
cd "$(dirname "$0")/.."

tag=${1:?usage: scripts/fpsmatrix.sh <tag> [seconds]}
seconds=${2:-45}
log=bin/fpsmatrix-$tag.log
csv=bin/fpsmatrix-$tag.csv

case " $* ${EXTRA:-} " in *vsync=false*) echo "fpsmatrix: VSync stays on (PERF-003)" >&2; exit 2 ;; esac

# Zooms are camera zooms: one cell is window height / 48 x zoom, held to
# 0.40-2.18 (camera.MinZoom, MaxZoom), so 0.4 is the farthest view and 1
# the default.
all="quiet-z0.4-s1 quiet-z0.4-s2 quiet-z0.4-s3
quiet-z1-s1 quiet-z1-s2 quiet-z1-s3
quiet-z0.8-s1 quiet-z0.8-s2 quiet-z0.8-s3
quiet-z2-s1 quiet-z2-s2 quiet-z2-s3
daycross-z0.8-s1 daycross-z0.8-s3
autosave-z0.8-s1"
cells=${CELLS:-$all}

saves=bin/fpsmatrix-saves
mkdir -p bin
go build -o bin/game ./cmd/game
mode=$(system_profiler SPDisplaysDataType 2>/dev/null | grep -E 'Resolution|UI Looks like' | tr -s ' ' | paste -sd ';' - || true)
echo "# $(date '+%Y-%m-%d %H:%M') $tag: $(git rev-parse --short HEAD) seconds=$seconds display=[$mode]" | tee -a "$log"
[ -f "$csv" ] || echo "cell,fps,low1,low01,frame_p99_ms,frames,over16_7,late20,hitch33,frame_max,update_max,draw_max,upd_per_draw,dropped_ticks,gc_cycles,gc_max_pause,allocs_per_frame,live_heap_mib" >"$csv"

for cell in $cells; do
  scene=${cell%%-*}
  zoom=$(sed -E 's/.*-z([0-9.]+)-.*/\1/' <<<"$cell")
  speed=${cell##*-s}
  hour=-1 day=0
  [ "$scene" = daycross ] && hour=23
  [ "$scene" = autosave ] && hour=5 day=1 # no autosave on day 0
  rm -rf "$saves" && mkdir -p "$saves"
  echo "## $cell" | tee -a "$log"
  out=$(bin/game -noaudio -scenario grown -seed 1 -width 500 -height 500 -hour "$hour" -day "$day" \
    -capture-speed "$speed" -capture-zoom "$zoom" -fpsprobe "$seconds" -save-dir "$saves" ${EXTRA:-} 2>&1 | grep 'fps probe' || true)
  if [ -z "$out" ]; then
    echo "fpsmatrix: $cell printed no probe line (crash?); stopping" | tee -a "$log" >&2
    exit 1
  fi
  echo "$out" | tee -a "$log"
  echo "saves written: $(ls "$saves" | wc -l | tr -d ' ')" | tee -a "$log"
  main=$(grep 'fps probe:' <<<"$out" | head -1)
  detail=$(grep 'fps probe detail:' <<<"$out" | head -1)
  fps=$(sed -E 's/.*avg ([0-9.]+) fps.*/\1/' <<<"$main")
  p99=$(sed -E 's/.*frame p99 ([0-9.]+[a-zµ]+)\).*/\1/' <<<"$main")
  kv() { sed -nE "s/.* $1=([^ ]+).*/\1/p" <<<"$detail"; }
  echo "$cell,$fps,$(kv low1),$(kv low01),$p99,$(kv frames),$(kv over16_7),$(kv late20),$(kv hitch33),$(kv frame_max),$(kv update_max),$(kv draw_max),$(kv upd_per_draw),$(kv dropped_ticks),$(kv gc_cycles),$(kv gc_max_pause),$(kv allocs_per_frame),$(kv live_heap_mib)" >>"$csv"
done
echo "fpsmatrix: $csv"
