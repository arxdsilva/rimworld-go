# ENG-001 Engine scale: 200 to 500 tiles a side

Part of NG-1 (`.cursor/NG1_NEW_GAME_PLAN.md` §6; decision D6, 60 fps at every size). Commits: `4d68b9aa` (E0/E1), `049759a4` (profiling flags), `f815d545` (E2, E3 with the setup work), `a9ca527c` (E4, E5, saves), `07f74a0c` (E5 follow-up), `46d1110d` (FPS probe).

## Status
Engine budgets and the final frame-rate gate pass on the numbers recorded below. `game.MaxEnabledSide` (`internal/application/game/ng1_types.go`) is **500** (commit ce78855a): Advanced offers every size from 200x200 to 500x500, 350 and above marked "needs a capable computer".

## What changed

### E0 Measure first
- **Fixture** `internal/application/scenario.GrownColony(seed, w, h, opts)` (also `-scenario grown` in the game): 20 colonists, a fenced pen of tame animals, wildlife at the cap, about 400 buildings in walled rooms, 1,000 stacks, grow zones, mine designations, blueprints, dirt and stains, a two-day warm-up; built through public dev hooks, cached as bytes (`RIMWORLD_FIXTURE_DIR`).
- **Benchmarks:** `BenchmarkTickGrown` (120, 250, 500), `BenchmarkTickRaid`, `BenchmarkReadModel`, `BenchmarkGenerate500`, `BenchmarkSave500`, `BenchmarkLoad500`, `BenchmarkReachable500`, `BenchmarkPath500Long`, `BenchmarkPathUnreachable500`, `BenchmarkRoomsAfterWall500`, `BenchmarkSearchOpen120/500`, `BenchmarkChunkLayer500`, `BenchmarkMinimapLayer500`, `BenchmarkDrawArt500`, `BenchmarkDrawArt250`, `BenchmarkGlobeFrame`, `BenchmarkPlanetBuild30/100`, `BenchmarkGenerateSite500`.
- **Runner:** `go run ./cmd/benchmark -sizes 120,250,500 -ticks 3000 -speeds 1,3 -cache <dir>` prints p50/p95/p99 per tick, per frame and per Step phase through `game.PhaseObserver` (wall-clock time is observation only and never feeds the sim).
- **Dev flags** in `cmd/game`: `-cpuprofile`, `-memprofile`, `-trace` (`049759a4`), and `-fpsprobe N` (`46d1110d`): runs the colony at `-capture-speed` and `-capture-zoom` for N seconds of real frames and logs fps, 1 % low and update/draw times; observation only.
- **Golden state hashes** (`scale_golden_test.go`, `TestScaleGolden`): `Game.StateHash()` is FNV-64 of normalised save fields (format independent), seeds 1, 7 and 42 at 48x40 and 120x120 after 20,000 ticks. Pins and every re-pin, with the reason:

  | When | Why |
  |---|---|
  | Before E1 | Pinned from the contract commit (`fbc76fd6`); the save format gained the research slice and the hash became format independent, with identical values on both. |
  | E1 (`4d68b9aa`) | **Unchanged.** Every E1 change is behaviour preserving. |
  | S3 (`f815d545`) | **Re-pinned once:** colonists now save their bios and skills (new state). Every other saved field is identical to the previous pins, checked field by field against `4d68b9aa`. |
  | E4 (`a9ca527c`) | **Re-pinned once:** plants grow per chunk once per 250 ticks (`plant.GrowSpan`, one rate per window) instead of every tick, and frost kills in the same pass. Checked field by field against `f815d545` after 1, 250, 1,000, 5,000 and 20,000 ticks of all six colonies: only plant `Growth` differs (ids, species, positions and marks are equal). |
  | E5 follow-up (`07f74a0c`) | **Unchanged.** |

### E1 Costs that grew with area (behaviour identical, goldens unchanged)
Walkability as a `[]uint64` bitset; per-cell indexes for plants, buildings and frames replace linear scans (this also removes the quadratic plant spawning that blocked 500x500 generation); building cell sets cached on the structure version; cached weather seed and mine list; batched in-order removals for rot and frost; shared copy-on-write terrain, room, grass, home and roof layers and an opt-in per-tick view cache. A guard stops a stack bouncing forever between two unmergeable stacks. 500x500 grown colony: **102 to 38 ms per tick**, generation 604 to 21 ms, rooms after a wall 3.9 s to 9 ms, allocations 6.1 MB to 142 KB per tick.

### E2 Chunks, regions, rooms
16x16 chunks (`domain/world/chunk`) and 4-connected walk regions per chunk (`domain/world/region`) with movement-layer labels (colonist, hostile, tame, wild); doors, fences, gates and traps are one-cell regions. Reachability is a label comparison; unreachable targets fail at once instead of flooding the component. Rooms are incremental: only dirty chunks are re-flooded and re-labelled. The region map is derived state, rebuilt after load and never saved (`game/regions.go`). Tests compare region answers with A* over random edits.

### E3 Pathfinding
Unreachable targets are rejected by label before A*. Paths longer than 96 tiles route through the region corridor then run tile A* inside it; shorter paths stay exact. At most a fixed number of long searches run per tick, the rest deferred in entity-id order (deterministic). Hive exits and threat scans are cached or indexed. Follow-up (`07f74a0c`): a per-game searcher with packed keys in a 4-ary heap (identical paths, 30 % less search time) replaces a GC-emptied pool; long crowd replans use the corridor too.

### E4 Staggered work and job indexes
Plants grow per chunk every 250 ticks; job givers read maintained indexes (ready frames, deconstruct marks, harvest and cut marks, haulable stacks, refuel and feed targets) instead of scanning; the item bucket grid is incremental; `Load` and the colony builder pre-build rooms and regions. **Wildlife cap:** linear up to 120x120, then by √area, capped at 300. A setup colony's largest raid is 8 plus area/40,000 raiders, at most 24. 500x500 grown colony: 38.6 to 1.4 ms per tick (S3), then 1.49 to 0.74 ms with worst three ticks 22.9 to 3.5 ms (S4).

### E5 Renderer (`ebiten/lod.go`, `chunk_cache.go`, `minimap.go`)
- Chunk read models (`game.ChunkView`, `Simulation.ChunkLayer`, `PlantsIn`, `MinimapLayer`); the renderer draws only visible chunks with plain draw records and no per-frame full-map loops.
- **Zoom tiers by on-screen tile size:** 48 px or more live; 16 to 48 px baked chunk images at 16 px per tile (an LRU of about 24 MB; terrain, water, smoothed stone, floors, carpets, rebaked when the chunk version moves, a few per frame); under 16 px a whole-map bake at 4 px per tile in pages of at most 4096 px. Bakes are unlit and lit per tile at draw time, so day, night, roofs and lamps read the same at every zoom. Per-chunk lighting.
- Atlas pages are decoded once and dropped; the minimap for maps over 150 tiles; the view cache on.
- Follow-up (`07f74a0c`): atlas pages cut in the background, off-screen overlays and particles culled, solid health bars, merged baked quads, batched zones.
- 500x500 draw: 15 to 41 ms before, 1.5 to 3.7 ms after S4; draw calls from 35,000 to under 1,400.

### Saves (E6)
Save version 2 (v1 still loads, `testdata/premotion_v1.sav` and `ng1_v2.sav`): header carries commitment, storyteller, difficulty, map size and biome; atomic writes with fsync; state that had made loaded games drift is now saved.

## Measured results (grown colony, 500x500)

| Measure | Plan budget | Recorded |
|---|---|---|
| Tick, average | 1.5 ms | **0.52 ms** (p99 0.87; worst three ticks 2.8 ms) |
| Tick during a raid | same | **0.55 ms** |
| Allocation per tick | 2 KB | **0.5 to 0.7 KB** |
| Draw (CPU) | 6 ms | **1.5 to 3.2 ms** (5.5 ms with the sim running at zoom 0.25) |
| Draw calls | 6,000 | under 1,400 |
| Legacy 48x40 `BenchmarkColonyHour` | within 10 % of 17.5 ms per hour | not re-measured in the docs pass |
| Read model per frame | 0.5 ms and 64 KB | not recorded here (`BenchmarkReadModel`) |

Numbers are from the commit messages (`07f74a0c`, `a9ca527c`, `f815d545`, `4d68b9aa`).

## Known limitations
- The 500x500 gate run used the `-fpsprobe` grown colony (seed 1) at the farthest zoom, not a hand-played session on seed "amber-hollow" as plan §6.9 described. First-use pawn tint bakes cost frames in the first ~25 s at a new zoom; a pre-warm would remove them.
- Slow ground (water, mud, marsh) only adds path cost on crowd repaths (SIM-033).
- The final planet rebuild on load (plan §5.8) and the worker-goroutine save snapshot (plan §6.8) were not separately benchmarked in this pass.

## Final frame-rate gate (passed 2026-10-23)
Measured by the lead on an Apple M3 Pro after ENG-E and ENG-F. Probe runs use the grown colony (`-scenario grown`), 3 ticks per frame, a 3 s warm-up after the first colony frame; the display caps at ~120 fps.

| Check | Budget | Result | Date | Notes |
|---|---|---|---|---|
| `TickGrown/250` average tick | 1.0 ms | 0.15 ms (p99 0.36) | 2026-10-22 | ENG-E |
| `TickRaid/250` | 1.0 ms average, 4 ms p99 | not run separately; `TickRaid/500` 0.55 ms avg, p99 0.90, worst three ticks 5.1-5.3 ms | 2026-10-22 | ENG-E |
| `DrawArt` at 250x250, every zoom | 6 ms, 6,000 calls | 1.9-4.2 ms paused; real frames 4.3 ms avg at z0.25 | 2026-10-23 | at most 1,368 draw calls at any zoom |
| `TickGrown/500` average tick | 1.5 ms | 0.52 ms | 2026-10-22 | ENG-E |
| `TickGrown/500` p99 and worst three ticks | 4 ms and 7 ms | 0.87 ms and 2.8 ms; 0.5 KB alloc per tick | 2026-10-22 | ENG-E |
| `BenchmarkReadModel` at 500 | 0.5 ms, 64 KB | 47 ns, 0 B when unchanged (cached) | 2026-10-22 | ENG-E; a changed tick still rebuilds parts |
| `-fpsprobe` 250x250 at 3x, grown colony | at least 59 fps, 1 % low at least 60 | z0.25: 118.5 avg, 81.0 low (ENG-F: z1 85.8, z0.5 83.9, z0.25 75.0, z0.1 78.2 low) | 2026-10-23 | update 1.3 ms, draw 4.3 ms avg |
| 10-minute run, 500x500, 3x, farthest zoom (0.1) | at least 59 fps, 1 % low at least 60 | 117.5 avg, 68.4 low (frame p99 14.6 ms), 70,520 frames | 2026-10-23 | probe run on the grown colony (seed 1), not a hand-played session; 25 s runs at z1/0.5/0.25/0.1: 72.0/71.7/60.8/58.3 low (first-use pawn tint bakes in the first seconds) |
| `BenchmarkColonyHour` 48x40 | within +10 % of 17.5 ms per hour | 13.8 ms (faster) | 2026-10-23 | |
| `MaxEnabledSide` raised to | 300 after the 250 gate, 500 after the 500 gate | 500 (commit ce78855a) | 2026-10-23 | |
