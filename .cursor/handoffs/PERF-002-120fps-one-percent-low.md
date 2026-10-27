# PERF-002: the 1 % low on 500×500 maps (issue #6)

Issue #6 profiled the grown 500×500 colony at 3× speed: 117 fps average, a 1 % low of 68 (ENG-001's ten-minute probe on an M3 Pro). The average is near 120; the work is the 1 % low. Eight items: the six hot spots the issue names, the Update time budget it lists as a follow-up, and an in-game performance overlay (user request) so the result is visible in play.

## Protocol
- `make fps` (`scripts/fpsprobe.sh <tag> [seconds]`): the grown 500×500 colony, seed 1, 3× speed, 60 s, zoom 0.1 and 0.8, **vsync off** (`-vsync=false`, new) so the display's refresh does not cap the number; `RENDER_SCALE=0.75` sets the map's render scale (`-render-scale`, new). The log is `bin/fps-<tag>.log`. A probe is an "art"-class job: it runs alone.
- `go test -run '^$' -bench 'ReadModel|WorldView|DrawArt500|GroundCover' -benchmem` and `go run ./cmd/benchmark -sizes 500 -speeds 3 -ticks 1500` after each item.
- One commit per item; the commit message carries its before/after line.

## The display caveat

The development machine's 5K display runs at **30 Hz**, and macOS throttles an occluded window to about 9 fps (ENG-004), so a probe here never reports more than 30 fps whatever the renderer does. Two consequences:
- the probe runs in a floating window the size of the display, never fullscreen (`main.go`: `SetWindowFloating`, Fullscreen overridden off), so it is never occluded;
- the report gains a **work** line, `work avg/p99 = update + draw` CPU time per frame: the display-independent number. 8.3 ms is the 120 fps budget. The GPU share (hot spots 3 and 6) does not show in it; those are judged by pixel area and by a run on a fast display.

The fps and 1 % low columns below are therefore meaningful only from runs on a 120 Hz display with the window focused (the README's 117/68 came from such a run). The work columns compare everywhere.

## Results

| State | Zoom | avg fps | 1 % low | frame p99 | update avg / p99 | draw avg / p99 | work avg / p99 | heap MiB |
|---|---|---|---|---|---|---|---|---|
| baseline `c8e5709ae` (30 Hz display) | 0.1 | 30.0 | 15.6 | 64.2 ms | 3.42 / 4.56 ms | 5.21 / 8.65 ms | 8.6 / 13.2 ms | 555 |
| baseline `c8e5709ae` (30 Hz display) | 0.8 | 30.0 | 27.5 | 36.3 ms | 3.17 / 4.28 ms | 5.89 / 7.39 ms | 9.1 / 11.7 ms | 215 |

Benchmarks (baseline `c8e5709ae`, 12 cores):
- `BenchmarkReadModel/500/tick` 1.25 ms/op, 2.30 MB/op, 1156 allocs/op (`World()`+`Snapshot()` after one Step); `/500/cached` 60 ns.
- `BenchmarkDrawArt500`: z1 0.98 ms / 439 draw calls / 847 KB; z0.5 1.21 ms / 591 / 1.29 MB; z0.25 3.09 ms / 1019 / 3.18 MB; zmin 1.23 ms / 268 / 1.78 MB.
- `cmd/benchmark` grown/500/x3: tick mean 0.21 ms p99 0.50 max 6.99; frame mean 0.65 p99 1.23 max 7.80 (ai max 6.87). raid/500/x3: tick p99 0.70 max 2.46; frame p99 1.91 max 4.06 (movement max 2.23, threats max 1.96).

## Items
_filled per wave_
