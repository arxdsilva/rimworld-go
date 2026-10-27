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
| wave 1 `be4b924fd` (1, 4, 5, 8) | 0.1 | 30.0 | 24.5 | 40.8 ms | 2.57 / 3.88 ms | 5.54 / 9.86 ms | 8.1 / 13.7 ms | — |
| wave 1 `be4b924fd` (1, 4, 5, 8) | 0.8 | 30.0 | 27.8 | 36.0 ms | 2.33 / 3.59 ms | 6.14 / 7.59 ms | 8.5 / 11.2 ms | — |
| wave 2 `b74e56459` (2, 3, 7), 30 s A/B | 0.1 | 29.9 | 13.5 | 74 ms | 2.53 / 3.90 ms | 5.54 / 9.2 ms | 8.07 / 13.1 ms | — |
| wave 1 `be4b924fd`, same 30 s A/B | 0.1 | 30.0 | 13.7 | 73 ms | 2.52 / 3.70 ms | 5.55 / 10.0 ms | 8.07 / 13.7 ms | — |

| final `6f8c0a478`, world 1 | 0.1 | 30.0 | 23.6 | 42.5 ms | 2.55 / 3.83 ms | 5.52 / 9.98 ms | 8.06 / 13.8 ms | — |
| final `6f8c0a478`, world 1 | 0.8 | 30.0 | 27.2 | 36.7 ms | 2.33 / 3.64 ms | 6.15 / 7.56 ms | 8.48 / 11.2 ms | — |
| final `6f8c0a478`, world 0.75 | 0.1 | 30.0 | 26.6 | 37.6 ms | 2.56 / 3.87 ms | 5.49 / 8.56 ms | 8.05 / 12.4 ms | — |
| final `6f8c0a478`, world 0.75 | 0.8 | 30.0 | 27.2 | 36.7 ms | 2.34 / 3.63 ms | 6.23 / 7.74 ms | 8.57 / 11.4 ms | — |
| final `6f8c0a478`, world 0.5 | 0.1 | 30.0 | 26.8 | 37.4 ms | 2.55 / 3.87 ms | 5.38 / 9.07 ms | 7.92 / 12.9 ms | — |
| final `6f8c0a478`, world 0.5 | 0.8 | 30.0 | 27.8 | 35.9 ms | 2.33 / 3.56 ms | 6.17 / 7.78 ms | 8.50 / 11.3 ms | — |

Reading the table on this 30 Hz display: update fell from 3.4 to 2.5 ms per frame (hot spot 1) and the display-bound 1 % low at zoom 0.1 rose from 15.6 to 23.6 (fewer GC-long frames); the render scale leaves the CPU work alone (it is GPU fill) and lifts the 1 % low to 26.6–26.8. The budget held 0 Updates on the grown colony (no raid). Final benchmarks: `BenchmarkDrawArt500` z1 0.95 ms / 439 draw calls / 780 KB / 3749 allocs, z0.25 3.09 ms / 6436 allocs, zmin 1.20 ms / 2251 allocs (baseline 847 KB / 4164, 7117, 2538), snow 0 bakes and 0 draws (the fixture has no snow); `BenchmarkShadowsHiDPI` 256,000 layer px; `BenchmarkGroundCover` 0 bakes/op, 55–310 µs; `cmd/benchmark` grown/500/x3 frame p99 1.17 ms max 1.41, raid p99 1.92 max 4.06, `frames > 5ms: 0` for both, `views` row 0.36 ms mean.

**Still to measure:** the true avg fps and 1 % low need one `make fps` run on a 120 Hz display with the game window focused (the README's 117 / 68 came from such a run on the M3 Pro's own panel); the `work` line predicts a frame of ≈8 ms average, ≈13 ms p99 at zoom 0.1 before GPU time.

The first 60 s wave-2 probe read 9 fps with update 5.5 ms and draw 10 ms; a back-to-back A/B of the wave-2 and wave-1 binaries (above, two wave-2 runs bracketing the wave-1 one) showed them identical, so that run was the machine busy (a freshly extracted repo copy being indexed), not the code. A 30 s run has a lower 1 % low than a 60 s one because the first-use tint bakes are a larger share of it. The grown fixture has no snow and little wetness at the probe's weather, so hot spot 2 does not show in its draw time; `BenchmarkGroundCover` carries that number. Hot spot 3 is GPU fill, invisible to the CPU-side work line. `cmd/benchmark raid/500/x3` after wave 2: frame p99 2.02 ms, max 4.64 ms, `frames > 5ms: 0`.

Benchmarks (baseline `c8e5709ae`, 12 cores):
- `BenchmarkReadModel/500/tick` 1.25 ms/op, 2.30 MB/op, 1156 allocs/op (`World()`+`Snapshot()` after one Step); `/500/cached` 60 ns.
- `BenchmarkDrawArt500`: z1 0.98 ms / 439 draw calls / 847 KB; z0.5 1.21 ms / 591 / 1.29 MB; z0.25 3.09 ms / 1019 / 3.18 MB; zmin 1.23 ms / 268 / 1.78 MB.
- `cmd/benchmark` grown/500/x3: tick mean 0.21 ms p99 0.50 max 6.99; frame mean 0.65 p99 1.23 max 7.80 (ai max 6.87). raid/500/x3: tick p99 0.70 max 2.46; frame p99 1.91 max 4.06 (movement max 2.23, threats max 1.96).

## Items

### 1. `World()` keeps identity on quiet ticks (`be4b924fd`)
- `GrowState`/`RipeIn` left `PlantView` for `Game.PlantGrowth(id)` (`PlantGrowthView`), read by the plant inspector only; the per-plant `growState` over 12k plants was most of the CPU.
- Plants: `plantVer` (bumped by `setPlant`/`dropPlant`, the only writers) + `zone.Map.Ver()` + a crop-only `Wilted` re-check give the fast path; the slow path builds into a private scratch list and keeps the old slice when `slices.Equal`. A `selfCheck` panics in tests on a raw `g.plants.Set`.
- Buildings, Items, Frames: scratch-build and field-by-field compare (a reflection test guards every field); `Bills`, `MovingTo`, `Haulers`, the item totals and `FoodByKind` are reused when equal. `ShotAge` saturates at 120 ticks. Messages, Schedule, Letters, Research and Locked are memoised. `construction.Footprint` is cached per kind and facing (it allocated per building per tick); `shareLayer` compares `~uint8` layers as bytes.
- `World()`+`Snapshot()` after a step: **1.25 ms / 2.30 MB / 1156 allocs → 0.50 ms / 304 KB / 480 allocs**. `World()` alone (`BenchmarkWorldView/500`): night 323 µs / 256 KB / 184 allocs, day 329 µs / 262 KB / 185 allocs. Lists kept identity on 597/600 daytime ticks (Plants), 600 (Buildings), 596 (Items). `cmd/benchmark` gained a `views` row (0.37 ms mean, 1.07 p99 at 500²).
- `TestWorldViewQuietTickAllocs` pins ≤ 250 allocs and ≤ 256 KB per quiet tick on a 200² colony (measured 157 / 192 KB); the remainder is `animalViews` (≈175 KB, 70 allocs), the next read-model target. `TestViewCache` now asserts identity iff equal plus forced refreshes.
- Goldens unchanged (views only).

### 5. Camera stepped per Draw (`be4b924fd`)
- `applyCamera` (Update) only collects input and clamps; `stepCamera(dt)` runs from `Draw` with the real elapsed time, clamped to 0.1 s, and skips an idle smoother so a camera placed by code stays exactly put (`TestStepCameraLeavesStillCameraAlone`; the Draw-time clamp had moved `TestFrameInvariance`'s scene). Captures keep the instant path. Input mapping sees the previous Draw's camera: one frame of lag.

### 4. Zone outlines in one batch (`be4b924fd`)
- Dashes, grow strokes and fence rails/posts append to `r.zoneBatch` (a `triBatch`), culled per dash and clipped to the first on-screen dash; one `DrawTriangles32` per frame after the fills. `BenchmarkDrawArt500/z0.25`: 3.29 → 3.23 ms, 7117 → 6436 allocs/op (the fixture's zones are small; the saving grows with zone count). The fence rail is no longer anti-aliased. Follow-ups: `drawZoneHighlight`, `drawPaths`, `drawWeather`'s full-screen `FillRect`s.

### 2. Snow and wet cover as a lit per-chunk cache
- `snow_cache.go`: 32-tile chunks at 4 px/tile with a 1-texel pad (130×260 images: the snow mask on top, the open-ground mask for wetness below). Texels are the old veil's corner alphas (`snowField` over `snowCorner`) interpolated across the tile and zeroed where the ground is not open; the pad samples the neighbours, so chunks show no seams. A chunk rebakes only when its own snow bytes (plus a 1-tile ring) differ, the ground changed (`StructVer`, and the `RoomGrid`/`RoofCells` layer identity, since roofs don't bump `StructVer`) or the legacy uniform depth moved. `drawSnowLayer` draws a wet pass and a snow pass per visible chunk through the lit shader (`drawBakedColor` at live/chunk zoom with sky and lamp per corner, one flat quad per chunk at map zoom), so the veil is now dimmed at night (it was drawn unlit before). `drawGroundCover`'s per-tile pass 1 is gone; the decal pass stays.
- `BenchmarkGroundCover` (500², a snowy quarter, wetness 0.5): 0 bakes/op in steady state; 89 µs at 64 px, 320 µs at 32 px (the decal pass), 99 µs at 16 px, 57 µs at 4 px for the whole visible map, against a 250k-tile loop before. The grown fixture has no snow, so `BenchmarkDrawArt500` is unchanged.
- Risks: a quarter-tile feather at open-to-closed edges at live zoom; ≈135 KB of GPU memory per chunk, ≈33 MB for a fully seen 500² map, no LRU yet.

### 3. Shadow layer at half resolution on 2× displays
- `shadowLayerScale(dev)`: 0.5 at `dev >= 2`, else 1. The layer and scratch are allocated at that size, `triBatch` carries a `k` scale on destination coordinates (zero = none, so the zone batch is untouched), `castCreature` scales its projection, the blur radii follow the layer's zoom, and the composite scales up with `FilterLinear`. `BenchmarkShadowsHiDPI`: 1,024,000 → 256,000 layer px per frame at 1280×800 logical (a quarter of the fill for the three shadow passes).

### 6. World render scale ("World detail")
- One coordinate space: `uiScaleState.world` is `Graphics.RenderScale` (clamped to at least `1/dev`, so a 1× screen always draws at Full; captures force 1 unless `-render-scale` is explicit), and the renderer's `dev` becomes `dev·world`. The map draws into `a.wr`, an offscreen of `ceil(pw·world)` pixels (`world_scale.go`), composited to the screen by exactly `1/world` with linear filtering. `mapCursor`, `mapViewport`, the hand conversions in the architect, zone tools, zone selection, room stats and multiselect, and the probe's centring all carry `world`; `TestMapPixelsComeFromMapCursor` (an AST test) keeps new code honest.
- Labels, icons and text the world passes draw through the canvas are **deferred**: inside the world pass `uiScale()` reports `s·world`, each canvas helper queues the draw when its target is the offscreen, and `flushWorldOverlay` replays the queue on the screen after the composite, so they render at native resolution in the same places. A queued draw whose tile is unexplored waits until fog has drawn. The HUD-pass map drawing (selection marks, ghosts, range rings, the drag box, room overlay) moved into `drawMapOverlays`, drawn into the map target before the composite — at Full the only visible change is that those now sit under the colonist bar and tabs instead of over them. `measure`/`lineH` use the HUD scale.
- Config: `Graphics.RenderScale` ∈ {1, 0.75, 0.5}, `GraphicsRows` has `renderscale`, presets High 1, Medium 0.75 on `devScale ≥ 2`, Low 1; Options › Graphics "World detail" (Full / 3/4 / Half, a preset row, applied live). `-render-scale` overrides it for a run; `-graphics` now applies its tier at the screen's device scale. The probe prints `world %g`.
- Left open: labels draw above the weather at reduced scale (only fog is handled); a Medium settings file from before this change keeps Full until a preset is picked again; a change is one frame late like HiDPI.

### 7. Update time budget at 3× and faster
- `update_budget.go`: `updateBudget = 5 ms`. `advanceColony` runs the tier's ticks plus any owed, one `Step(1)` at a time, and stops once the Update has spent the budget; the remainder (capped at one Update's nominal) carries to the next Update, `held` counts the stops. A speed change, pause or background pause clears the debt. `r.mo.stepped` is the count actually run, so interpolation stays continuous. Captures and 1× never split. Reported by the probe (`budget held N updates, max M ticks owed`), the overlay (`held`) and the dev inspector; `cmd/benchmark` prints `frames > 5ms`.
- Follow-up: Ebiten can run two Updates before one Draw after a slow frame, so the budget bounds an Update, not a frame; a per-Draw share is the next step if the probe shows it.

## Left open (next perf pass)
- `animalViews`/`trainViews` rebuild ≈175 KB and 70 allocs per tick (≈27 % of `World()`); `cropWiltFlipped` re-reads the air over every crop each tick (≈13 %); stains, dirt and pen labels allocate a little per tick; `chunkKey` still includes `viewKey`, so the chunk index re-hashes per tick.
- The Update budget bounds an Update, not a frame: after a slow frame Ebiten can run two Updates before one Draw. A per-Draw share is the next step if a probe on a raid shows it.
- A snow-cache LRU (≈33 MB once a 500² map has been seen); the quarter-tile mask feather at live zoom.
- `drawZoneHighlight`, `drawPaths` and `drawWeather`'s two full-screen `FillRect`s still go through the vector path.
- Render scale: labels draw above the weather at reduced scale; old Medium settings files keep Full until a preset is picked again; a hotkey for the overlay; the sparkline draws through the vector path (≈5k allocs per frame under `-race` are Ebiten's, not ours).
- The probe cannot exceed the display's refresh here (30 Hz) and the "vsync off" override shows no effect on Metal; a run on a 120 Hz display is the only way to the headline number.

## Verify
- `go test -race -count=1 ./...` passes after every wave; `TestScaleGolden` (no race) and `TestGrownGolden` are at their pins throughout (views only, no sim change).
- `make fps`, `go run ./cmd/benchmark -sizes 500 -speeds 3 -ticks 1500`, `go test -run '^$' -bench 'ReadModel|WorldView|DrawArt500|GroundCover|Shadows' -benchmem ./internal/...` reproduce the tables above.
- Captures reviewed: the overlay at UI scale 1 and 2 (`-capture-ui perf`), snow by day and night against the previous build (day identical but for the weather dimming now applied to the veil; night lit instead of flat grey), shadows at dev 2, the map at world 0.5 (soft map, crisp HUD and labels), the Graphics tab with the two new rows.

### 8. Performance overlay (`be4b924fd`)
- Options › Graphics "Performance overlay" (`Graphics.PerfOverlay`, off by default). A panel right of the resource list: `117 fps · 1% low 71` / `frame 8.6 ms · p99 14.1` (amber over 16.7 ms, red over 33) / `update 2.3 ms · draw 4.2 ms` / `sim 3× · 0.8 ms/tick · held 0` / `heap 412 MiB · gc 12/min` and a 120-frame sparkline. Always-on ring timing (600 samples), stats and strings refreshed twice a second, heap from `runtime/metrics` once a second; the fps probe's `fpsReport` computes through the same `frameStats`. Drawn with the UI hidden too. `-capture-ui perf` seeds a fixed series. A hotkey is a follow-up.
