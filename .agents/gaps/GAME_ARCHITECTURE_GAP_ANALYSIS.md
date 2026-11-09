# Game architecture gap analysis (GAR-*)

Scope: game-architecture practices (scheduling, parallelism, events, render batching, frame pacing, runtime tuning) compared with ours, and the move to the architecture the user chose.
- Load only for GAR-* tasks.
- Handoffs go to `.agents/handoffs/ARCH.md` (architecture, `cmd`) and `PERF.md` (frame time).
- Decisions are ADRs `docs/adr/0001`–`0004`, and the diagrams are in `docs/architecture/`.
- Paths in the rows are under `internal/adapters/inbound/ebiten/` when they start with `render/`, `gfx/`, `uikit/`, `perf/` or a root file name; otherwise they are repo-relative.

## Target

**At least 60 fps, always**, on an Apple M2 Max (8 performance and 4 efficiency cores, 38 GPU cores, 64 GB). It drives a Dell S2721QS in a scaled mode: it looks like 2560×1440, with a 5120×2880 backing at 60 Hz, which is 14.7 Mpx a frame. VSync stays on in every measurement (PERF-003: this machine panicked with VSync off).

**Workload:** today's play. That is the grown 500×500 colony with about 20 colonists, every zoom, at 1x, 3x and 6x, raids included.

**Measured per 45 s probe cell** (`scripts/fpsmatrix.sh`, `make fps-matrix`):

| metric | target |
|---|---|
| 1% low | at least 60 fps |
| p99 frame gap | 16.7 ms or less (gaps are taken at Draw entry on the CPU; see GAR-085) |
| frames over 20 ms (one missed vsync) | 0.5% or less |
| frames over 33.3 ms | 0, day rollover and autosave included; loading screens exempt |
| work (Update + Draw CPU) | 11 ms or less on average, 14 ms or less at p99 |

**Frame budget** (16.67 ms):

| segment | budget |
|---|---|
| sim at 6x | 5.0 ms (worst tick 1.5 ms) |
| post-step | 0.7 ms |
| read models | 0.5 ms |
| UI | 1.0 ms |
| draw CPU | 4.0 ms |
| GC | 0.5 ms |
| slack | 5.0 ms |

## Decision record (user, 2026-11-08)

| axis | choice | ADR |
|---|---|---|
| Sim core | A systems scheduler over the existing `component.Store[T]` stores | [0001](../../docs/adr/0001-systems-scheduler.md) |
| Threading | Parallel pure stages: workers read a frozen view and return results; the single scheduler goroutine applies them in entity-ID order | [0002](../../docs/adr/0002-parallel-pure-stages.md) |
| Messaging | An event queue for every cross-system effect; systems read each other only through query ports | [0003](../../docs/adr/0003-event-queue.md) |
| Rendering | A render-command list: layers emit sort-keyed commands, merged into the fewest draw calls | [0004](../../docs/adr/0004-render-command-list.md) |

Also decided:
- Every `cmd/<tool>` is one wiring-only `main.go` (SOLID_GO.md §8a; done, ARCH-007).
- VSync stays on in every measurement.
- Determinism holds: the same seed and commands give the same state hash. The only planned re-pin is the scheduler stagger (GAR-005), with the reason "same per-entity rate, different tick".
- Disjoint-write parallel systems (two systems mutating different stores at once) are out of scope. They would break MASTER_SPEC §55 and need their own decision.

## Sources and method

- Five research fragments by web-research agents (R-sched, R-parallel, R-events, R-render, R-frame; 2026-11-08), 100 practice rows in all. Each row names who uses the practice, the problem it solves, how it works, its preconditions, its risks (determinism first) and its sources.
- A source is either *opened* (fetched and read) or *snippet-only* (seen only in search results). Snippet-only claims carry a mark, and nothing rests on them alone.
- The sources are public books, talks, blogs, papers, engine documentation and engine source, used for ideas only (CLEAN_ROOM.md, GAR row). Nothing comes from RimWorld's internals.
- Main sources:
  - Factorio Friday Facts #18, #67, #176, #204, #215, #324, #364, #421;
  - Bevy (schedules, run conditions, change detection, events and observers, render phases);
  - flecs pipelines; Unity DOTS (system groups, jobs, entity command buffers);
  - Victoria 3 Dev Diary #76;
  - Game Programming Patterns (Update Method, Event Queue, Observer, Command, Dirty Flag, Data Locality);
  - Acton, CppCon 2014; Fabian, *Data-Oriented Design*;
  - Gaffer On Games ("Fix Your Timestep", "Deterministic Lockstep");
  - Ericson, "Order your graphics draw calls around!"; Terdiman on radix sort;
  - Dear ImGui; Muratori, immediate-mode GUI (2005);
  - Unreal and Intel on dynamic resolution; GPU Gems 3;
  - oneTBB on grain size; Apple's dispatch and Metal guidance; Random123;
  - the Go GC guide, `runtime/metrics` and `runtime/trace`, and the Go 1.27 release notes;
  - the Ebitengine v2.10.6 source, read from the module cache with file:line.
- Code facts come from two read-only code-analysis agents (O-sched, O-render) and the lead, each with file:line. Numbers come from the probe matrix (Part C), `BenchmarkTickDayRollover500` and `BenchmarkSave500`.

## Our architecture, stated first

**Layers.** Hexagonal: `cmd/<tool>/main.go` (wiring only) calls `internal/compose/game` (the client's composition root), which builds:
- the inbound adapters `ebiten` (the client) and `cli/*` (tools);
- `internal/application/game` (`Game`, with 58 sub-packages, each a system behind a small host port);
- `internal/domain/*`, which is headless and deterministic;
- the outbound adapters (saves, settings, usage, audio) and `internal/platform/*`.

The `internal/arch` tests enforce the direction of dependencies, the size limits and the `cmd` rule.

**The sim:**
- One goroutine owns all mutation. External input enters as typed commands (`domain/sim/command`).
- A tick runs 15 phases from a fixed function list (`internal/application/game/phases.go`).
- About 40 periodic sites each test `now % P == 0` with offset 0.
- Entity data lives in sparse-set `component.Store[T]` stores.
- Read models are memoised on `[tick, viewGen]`.
- RNG comes in named streams. Saves are gob, with migrations. A state hash and golden tests guard determinism.

**The frame:**
- Ebitengine runs at 60 TPS. Since v2.10, after a slow frame it runs up to 5 Updates in one frame.
- `advanceColony` steps 1, 3 or 6 ticks per Update within a per-frame budget (`update_budget.go`, GAR-080).
- `Draw` interpolates the camera and pawns and calls the world pipeline: terrain, chunk bakes and LOD, grass, crop and zone batches, lit objects, creatures, overlays, weather. Then the HUD through `uikit` immediate-mode widgets.

## Already matching

| practice | ours |
|---|---|
| Hexagonal ports and adapters, inward dependencies | `internal/arch` direction and third-party guards; HEXAGONAL_ARCHITECTURE.md |
| Typed commands as the only external mutation | `internal/domain/sim/command` |
| Fixed timestep plus interpolation | 60 TPS; the renderer interpolates between ticks (C7, `app.go` `markUpdate`) |
| Named deterministic RNG streams | the domain RNG streams; no global RNG (AGENTS.md) |
| State hash, goldens, journeys | the state hash; `make test-journeys` |
| Versioned saves | gob saves with migrations (`internal/application/game/save.go`) |
| Sparse-set component stores | `internal/domain/sim/component` (12 stores in `Game`) |
| Memoised read models | `indexes.go` on `[tick, viewGen]` |
| Chunk bakes and LOD | 96 chunk images at 16 px a tile and map pages at 4 px a tile (`render/world/lod.go`) |
| Batched grass, crops, zones, dirt and blood, shadow blobs | one draw per batch (`render/env/grass_batch.go`, `crop_batch.go`, the zone and dirt atlases, the shadow-blob `DrawTriangles32`) |
| Lit data per vertex, frame-constant uniforms | `lit.kage`: sky, lamp, normal offset and blood in `Custom0..3` |
| Staggered or slept work | animals decide every 60 ticks through a bucket index (`animal_index.go`); plant, snow, mood and social work already partly staggered (see GAR-001 to GAR-029) |
| Reused A* state, region reachability | `paths.go` reuses its search state; region reach answers "can't get there" without a search |
| Allocation-free frame instruments | `perf/frames.go` (`TestProbeAllocFree`) |
| Shaders compiled before play | `light.SharedLit`, `env.SharedWater`, warmed on the boot screen (GAR E9, `3f4efd75e`) |

## Baseline (PERF-004)

`scripts/fpsmatrix.sh` on 2026-11-08 at `c27bd1512`. Grown 500×500 colony, seed 1, 45 s a cell, VSync on, HiDPI dev 2 (5120×2754 physical), UI scale 3, world detail 1. The matrix ran twice and the second pass is kept; the day-cross and autosave cells were rerun once their start-time jump worked (below). Zoom is the camera zoom (one cell = window height / 48 × zoom, held to 0.40–2.18), so the farthest view is 0.4. Files: `bin/fpsmatrix-baseline.csv`, `-timejump.csv`, `-zoom1.csv`, `-repro.csv`.

| cell | fps | 1% low | p99 frame | frames > 20 ms | > 33 ms | draw avg | work avg / p99 |
|---|---|---|---|---|---|---|---|
| quiet z0.4 1x / 3x / 6x | 17.4 / 17.9 / 16.8 | 13.1 / 14.3 / 13.0 | 76 / 70 / 77 ms | all | all | 41.7 ms | 45.0 / 55.1 ms |
| quiet z0.8 1x / 3x / 6x | 41.6 / 42.0 / 38.1 | 32.9 / 33.2 / 31.1 | 30.4 / 30.1 / 32.1 ms | 99.7% | 8 / 10 / 9 | 15.5 ms | 18.9 / 22.7 ms |
| quiet z1 (default) 1x / 3x / 6x | 53.5 / 53.9 / 48.3 | 39.1 / 39.9 / 37.1 | 25.6 / 25.1 / 26.9 ms | 21% / 19% / 54% | 6 / 0 / 0 | 11.6 ms | 15.0 / 19.3 ms |
| quiet z2 1x / 3x / 6x | 60.0 / 60.0 / 60.0 | 54.9 / 54.8 / 54.5 | 18.2 / 18.2 / 18.4 ms | 7 / 8 / 0 | 3 / 2 / 0 | 4.9 ms | 8.3 / 9.2 ms |
| day-cross z0.8 1x (23:00) | 51.5 | 44.6 | 22.4 ms | 22% | 0 (max 25.5 ms) | 13.1 ms | 15.0 / 17.6 ms |
| day-cross z0.8 6x | 45.8 | 33.7 | 29.7 ms | 63% | 3 (max 39.0 ms) | 14.4 ms | 16.9 / 22.3 ms |
| autosave z0.8 1x (day 1, 05:00) | 44.9 | 35.0 | 28.6 ms | 82% | 7 (max **63.3 ms**; Update max **42.7 ms**) | 14.8 ms | 17.6 / 22.2 ms |

A rerun of z0.4 and z0.8 at 1x (`-repro`) agreed within 1 fps.

**What it says, worst first:**
1. **Every cell misses the target except z2.** The frame is CPU-bound in Draw: the draw alone is 11.6–41.7 ms against its 4 ms budget, while Update is 1.9–4.1 ms.
2. **The farthest view is the most expensive one (GAR-155).** A bisect with exported builds (`git archive`, VSync on, z0.1 at 1x) puts the change at `049a90108` (UIS-2d): draw 5.7 ms at `1b9787ecd`, 37.7 ms at `049a90108`. The build of ARCH-006 (`b24b36709`) still holds 60 fps there, with a 4.6 ms draw. UIS-2d rebased the cell on the window's physical height, and the LOD tiers are still in physical pixels. On this 5K window, zoom 0.4 is 23 px a cell, so the whole view draws live; a 1080-pixel window showing the same cells is at 9 px and draws the map bake.
3. **Ebitengine v2.10.6 costs about 2 ms of draw (GAR-156).** The same code built with the pre-upgrade `go.mod` (v2.9.x), alternated twice: z0.8 draw 13.5 → 15.4 ms (48.5 → 42.0 fps); z0.4 38.3 → 40.6 ms.
4. **Autosave is a hitch (GAR-190).** One save took a 42.7 ms Update and a 63.3 ms frame, with 2 frames over 50 ms.
5. **The day tick shows at 6x (GAR-007).** 3 frames over 33 ms, max 39.0 ms; none at 1x.
6. **Allocations and GC.** 31,000–45,000 allocations a frame, 20–31 GC cycles per cell, worst pause 197 µs. Two cells (z1 1x, repro z0.4) logged one pause of 5.2–6.3 ms, which needs a trace (GAR-087).
7. **Pacing.** At z0.4 every Draw had 3 or more Updates and 6x dropped 4,652 ticks; z2 holds 60 with 1 Update per Draw. On-time frames sit at 18–19 ms in the histogram, so CPU-side gaps scatter about 1.5 ms around the vsync (GAR-085).

Not run yet: raid cells, the Work, Architect and world-view cells, the first frames after load, benchstat `-count 8`, a CPU profile of the worst cell, `ebitenginedebug` logs and Metal HUD runs (GAR-089).

## Rows

States: **done** (with its commit), **partial**, **missing**, **kept** (deliberately not adopted, with the reason).

### 001–029 Scheduler (ADR 0001)

Paths are in `internal/application/game/` unless another is given. Site numbers (§2a rows) refer to the periodic-site inventory in fragment O-sched.

| ID | practice (source) | ours | state | change | files | phase |
|---|---|---|---|---|---|---|
| GAR-001 | Phases as declared systems in a schedule (Bevy schedules, flecs pipelines, Unity system groups; R-sched) | The 15 phases are a fixed array of method values (`phases.go:41-62`), timed by an optional `PhaseObserver` | partial | `game/sched`: every phase becomes a `System` in today's order, with no behaviour change (the goldens stay equal) | `phases.go`, new `game/sched` | E2a |
| GAR-002 | Order constraints are data, each with its reason (Bevy `before`/`after`, ambiguity detection) | The order lives in comments: power before climate (`phases.go:9`); medical before needs (`bleed.go:23-25`); needs before AI (`tick.go:572-573`); AI before jobs (`tick.go:500-515`); jobs before social (`social.go:36`); animals before movement (`animals.go:124`); incidents after animals (`incidents.go:85`); world after incidents (`world_state.go:157`); movement last (`tick.go:1562-1564`) | missing | Each one becomes an `After`/`Before` edge with its reason. `TestScheduleMatchesPhaseOrder` checks the order, and a golden of the resolved order makes any change a reviewed one | `game/sched` | E2a |
| GAR-003 | Declared read and write sets per system (Bevy system access; flecs `[in]`/`[out]`; R-sched) | Only 12 `component.Store`s (`game.go:763-766, 848-858, 888, 900`); the rest of the state is about 47 `map[entity.EntityID]` side tables (`game.go:767-795, 816-822`) plus sub-package structs | missing | Register every side table as a store with a `StoreID` first, then declare `Reads` and `Writes`; `TestSystemsDeclareStores` | `game.go`, `game/sched`, `internal/arch` | E2b |
| GAR-004 | Run conditions (Bevy `run_if`) | Colony-wide work checks `primary(g)` by hand (`social.go:42`, `raids.go:63`, `incidents.go:92`, `world_state.go:164`); other gates are `if`s inside the phase | partial | `RunIf` per system: the primary map, a fire burning, a store changed | `game/sched` | E2a |
| GAR-005 | A system's scope: per map or colony-wide | `Step` runs `runPhases` once per open map, then `mapsFlush` (`game.go:1318-1331`) | partial | `Scope` on the descriptor; colony systems run once per tick | `game/sched` | E2a |
| GAR-006 | Frequency and stagger as scheduler features (Factorio FFF #204 update lists; GPP Update Method; R-sched) | 46 sites gate on a global `now % P` (O-sched §2a), and about 70 stagger by hand with five different formulas (`+id`, ×7, ×13, ×37, ×613, ×977; §2b) | missing | `Every`, `Offset` and `Stagger` on the descriptor: `(now + mix(id)) % Every == 0` over the system's entities. One rule replaces both lists. The goldens are re-pinned once, with the reason "same per-entity rate, different tick" | `game/sched`, the 116 gate lines | E2c |
| GAR-007 | No pile-up on the day tick | At `now % 60000 == 0`, 41 of the 46 global gates fire together (§2a rows 1–8, 10–19, 21, 23–43, 45). That tick walks every item at least three times (rows 3, 8, 35; four with a fire, row 24), every building twice (7, 13), every animal twice (10/12, 21) and every patient (15, 16). Its cost in frames is the day-cross cell (Baseline) | missing | Stagger rows 3, 7, 8, 10, 13, 21, 24 and 35 by entity. Give the daily-only jobs (rows 12, 16, 17, 19) fixed hour offsets. `TestDayTickUnderBudget`: the worst tick is 1.5 ms or less | `climate.go`, `furnish/furnish.go`, `animal_life.go`, `pens.go`, `trough_feed.go`, `wildlife.go`, `fire.go`, `craft/condition.go` | E2c, E2d |
| GAR-008 | No pile-up on the 250th tick or the hour | Every 250th tick runs rows 3, 5, 6, 8, 10, 13, 28, 32, 34 and 41 together; every hour adds 7, 15, 18, 21, 23, 29, 30, 35 and 36 | missing | Distinct `Offset`s so no two heavy systems share a slot, and a calendar test that adds up the declared cost of each slot | `game/sched` | E2c |
| GAR-009 | The rate per entity holds when staggered | (none) | missing | `Test<Site>RateUnchanged`: over 2 game days every entity fires as often as before | `game/sched` tests | E2c |
| GAR-010 | Sleeping entities skipped (Factorio FFF #204 sleep lists) | `movers()` merges three ID lists every tick and checks every mover's `NextMoveAt` (`tick.go:1567-1575`, `motion_store.go:83-98`); animals check `NextDecide` each tick (`animal_needs.go:166-168`) | missing | A wake-time heap for movers and animal decisions; creatures mid-step are not touched | `motion_store.go`, `animal_needs.go` | E2/E5 |
| GAR-011 | Change detection (Bevy `Changed<T>`; GPP Dirty Flag) | Version counters (`structVer`, `viewGen`, `plantVer`) and 8 invalidation port methods (`Changed`, `RoomsChanged`, `DoorChanged`, `EnsureRooms`, `SetThermoActive`, …) | partial | A change tick per store; `RunIf` on a store's change; invalidations become dirty events (GAR-058) | `component`, `game/sched` | E2b, E3 |
| GAR-012 | Per-system timing in a read model | `PhaseObserver` exists; the benchmark CLI uses it (`internal/adapters/inbound/cli/benchmark/run.go`) | partial | The scheduler times every system, and the debug HUD reads it (GAR-086) | `game/sched`, `perf/` | E2a |
| GAR-013 | Systems own their state (Bevy resources and components; R-sched) | `Game` has 1,783 methods and 64 fields, plus about 85 methods on the embedded `mapState` (`game.go:838-964`); the root is 174 files and 51,873 lines (`internal/arch/solid_budget.txt:11-18`) | missing | E11 moves the 13 method families (O-sched §7: animals 249, construction and power 222, environment 206, health 203, pawn AI 177, social 145, threats 136, kernel 110, movement 94, read models 91, world 85, hauling 61) into system packages; each move lowers `solid_budget.txt` | the root package | E11 |

### 030–049 Parallel pure stages (ADR 0002)

| ID | practice (source) | ours | state | change | files | phase |
|---|---|---|---|---|---|---|
| GAR-030 | One fixed worker pool behind a port (oneTBB arenas; Go issues #15110 and #54622; R-parallel) | The sim is one goroutine | missing | `internal/platform/pool`: 8 lanes (7 workers plus the owner), wired in `internal/compose/game`; `TestNoGoStatementsInSim` | `platform/pool`, `compose/game`, `internal/arch` | E4a |
| GAR-031 | A frozen view; caches built before workers start (R-parallel 4) | At least 14 "queries" build caches on first read: `creatures()` (`animal_index.go:129-135`), `plantIndex()` (`indexes.go:76`), `regionsNow()` (`regions.go:49`), `ensureRooms()` (`rooms.go:55`), `lightCells()` (`light.go:196`), `powerList`/`powerGraph` (`power.go:114, 157`), `mineList()` (`world.go:266`), `stoneLayers()` (`world.go:147`), `seedGrass()` (`grass.go:71`), `ensureRoofGrid()` (`roof.go:64`), `ensurePerson()` (`people.go:162`), `ensureBody()` (`body.go:46`), the occupancy rebuild (`occupancy.go:113-123`) and the food index (`animal_index.go:18-19`) | missing | A freeze step in `Plan` builds them all. A `frozen` epoch panics on any write during `Compute` in test and race builds. `rebuildCreatures` (`animals.go:93`) becomes that step | the listed files | E4b |
| GAR-032 | Decide all, then apply in ID order (Unity entity command buffers, sorted playback; R-parallel 2) | `startJob` claims reservations and plans the path in one call (`tick.go:674-745`), and 22 packages call `SetJob`/`StartJob`: first come, first served in pawn-ID order | missing | Work givers split into *propose* (target and score; pure, reads `FreeFor`) and *claim* (serial, in ID order, falling back to the next proposal on a conflict). This is a recorded behaviour change, made on one goroutine first; the no-flap tests stay | `tick.go`, `priorities.go`, the giver packages | E4c |
| GAR-033 | A counter-based RNG per task (Random123; SplitMix64) | `g.rng` is drawn 108 times in 33 files in loop order, and the host `Roll` ports draw from it (`combat_host.go:208`, `handling.go:81`, `haul_route.go:66`, `sense_host.go:95`). Named streams exist (`rng_streams.go:48-73`), and so do keyed rolls (`plant.Roll` at `plant_light.go:181`, `farm.Sprout` at `farm/farm.go:110`) | partial | Every draw inside a `Compute` is keyed on (seed, tick, stage, entity, purpose); the goldens are re-pinned with the stage that moves | `rng_streams.go`, the 33 files | E4d |
| GAR-034 | Search state per worker | One `pathing.Searcher` per map (`paths.go:15-23`, 4 MB at 500×500) | missing | One searcher per lane (at most 8 × 4 MB) | `paths.go` | E4c |
| GAR-035 | Path searches as a stage | At most 6 long job searches a tick (`paths.go:64`) plus unbounded short ones; 86 call sites in 42 files. Movement repaths read occupancy written earlier in the same loop (`tick.go:1601, 1636, 1660`) | missing | Job-start and animal searches queue as requests and run in parallel from about 8 requests; movement repaths stay serial | `paths.go`, `animals.go` | E4c |
| GAR-036 | Job and work scoring as a stage | `assignWorkByPriority` (`priorities.go:52-66`) per idle pawn, each at most every 30 ticks (`tick.go:596`), plus every job end | missing | After GAR-032; ships only at a gain of 0.2 ms or more and 1.3× or more | `priorities.go` | E4c |
| GAR-037 | Animal decisions | About 5 a tick (300 animals over 60 ticks), plus reflexes. Partly pure: works on a local copy (`animals.go:100, 124`), but `pickFeed` claims food (`animals.go:121-123`), `chase` strikes (`handling.go:267`), and wandering draws `g.rng` (`animals.go:212`) | kept serial | Decide and apply split on one goroutine, with keyed RNG. Parallel only if the matrix shows a gain; ADR 0002 expects it to stay serial | `animal_needs.go`, `animals.go`, `handling.go` | E4c |
| GAR-038 | Food search | `bestFood` is O(I) per hungry pawn (`food.go:269-281`) and reads `reserve.FreeFor`, which earlier pawns in the same loop write | missing | The index first (GAR-100), then decide all and apply | `food.go` | E5, E4c |
| GAR-039 | Measure memory-bound passes before threading them (Factorio FFF #421 rejected one) | Wealth (GAR-108); room heat, where the link exchange couples rooms in loop order (`thermo/step.go:44-53`); the plant chunk pass (`plant_growth.go:44`); the light grid, 250k cells on any build (`light.go:12-30, 50-68`); item rot and wear (§2a rows 3, 35); exposure (`heat.go:55-70`) | missing | Stagger first. Then parallel only if measured: a double-buffered heat exchange (results change, goldens re-pinned); per-lane light grids merged by max | `thermo/step.go`, `light.go` | E4c |
| GAR-040 | Merge in task order; integer reductions (R-parallel 7, 9) | (none) | missing | Merge in task-index order; integer or fixed-point sums; `TestParallelEqualsSerial` at 1 lane, 8 lanes, and 8 lanes shuffled | `game/sched` | E4 |
| GAR-041 | No false sharing (128-byte lines on Apple performance cores) | (none) | missing | Per-lane scratch padded to 128 bytes | `platform/pool` | E4a |
| GAR-042 | Counts, not timings, choose serial or parallel (R-parallel 11) | (none) | missing | `minParallelTasks` per stage; one lane in captures, replays and bisection | `game/sched` | E4a |

### 050–079 Events and query ports (ADR 0003)

The 57 sub-packages declare **1,238 port methods in 242 interfaces**. By name and return shape, about **418 are effects** and about **820 are queries** (each ±10%). The root package implements them with 1,233 methods on 187 host types.

| ID | practice (source) | ours | state | change | files | phase |
|---|---|---|---|---|---|---|
| GAR-050 | A typed event queue with no boxing (GPP Event Queue; Bevy events; R-events) | None: effects are host-port calls that mutate depth-first | missing | `internal/domain/sim/event`: a `Column[T]` per kind plus one spine; an exhaustive switch; a kind without a handler, codec or log format fails `make arch` | new `domain/sim/event`, `internal/arch` | E3a |
| GAR-051 | The queue is empty at each tick's end; caps on generation depth and events per tick | (none) | missing | `TestQueueEmptyAtTickEnd`; caps turn a feedback loop into a test failure | `domain/sim/event` | E3a |
| GAR-052 | A golden of the dispatch table | (none) | missing | Any reordering is a reviewed change | `domain/sim/event` | E3a |
| GAR-053 | 0 allocations for an emit and drain | (none) | missing | `AllocsPerRun` | `domain/sim/event` | E3a |
| GAR-054 | Commands become events (GPP Command) | One entry point, `applyOne` (`tick.go:61-140`), with about 40 kinds through `applyHostCommand` (`tick.go:79-94`) | partial | The commands system emits events; this is the first seam | `tick.go` | E3b |
| GAR-055 | Leaf effects move first: messages, letters, log (30 methods, 18 packages) | Order matters for display only | missing | Emitted in sequence; UI and audio read a per-tick outbox | the 18 packages | E3c |
| GAR-056 | Commutative effects next: skills (6, 5 packages), needs and mood (16, 8) | Commutative within a tick, except clamping | missing | Events; clamping at drain | the 13 packages | E3c |
| GAR-057 | Environment effects (15, 6 packages: `Ignite`, `Douse`, `Stain`, `AddDirt`, `KillPlant`, `SetPlant`, `SetTerrain`, `DigRock`) | Ignitions are order-free; terrain changes bump regions | missing | Events; terrain changes emit a dirty event | combat, farm, industry, raid, sense, snow | E3c |
| GAR-058 | Derived-index invalidation as dirty events (8, 8 packages) | `Changed`, `RoomsChanged`, `DoorChanged`, `EnsureRooms`, `SetThermoActive`; buildings and frames (21, 10) bump `structVer`, which rooms, regions and light read lazily later in the same tick | missing | A `StructureChanged` event, drained before the next reader | the 18 packages | E3c |
| GAR-059 | Items (58, 19 packages) | Order matters: merges and quantities; two takers of the last unit | missing | Events applied in ID order, each re-validated | the 19 packages | E3c |
| GAR-060 | Health (29, 12 packages) | Order matters: a kill mid-loop changes what later actors see (`chase` re-reads its record after a strike, `handling.go:267-270`) | missing | Events; handlers check alive and generation | the 12 packages | E3c |
| GAR-061 | Spawns: raiders, factions, world (27, 11 packages) and animals (10, 4) | Entity IDs are allocated in call order (`entity.Allocator`, `game.go:754`) | missing | Spawn events allocate IDs in `Seq` order | the 15 packages | E3c |
| GAR-062 | Jobs (60, 22 packages) and reservations (27, 17) last | The main serial dependency: first come, first served in pawn-ID order | missing | A single-handler intent event, re-validated in ID order; reservations stay an owner call inside `Apply` | the 22 packages | E3c |
| GAR-063 | No hidden effects inside a turn | `phaseAI` calls `care`, `mind`, `combat.Guard`, `rest`, `sense` and `dose` in the middle of one pawn's turn (`tick.go:492-655`); `phaseMovement` calls `care.FollowCarriers` and `combat.Traps.Underfoot` (`tick.go:1582-1583`) | missing | The last seams, each in its own commit with the reason if results change | `tick.go` | E3c |
| GAR-064 | Read-only query ports (CQRS; R-events 8) | 754 distinct port names; the most shared are `Pos` (21 packages), `Now` (18), `Job` (16), `Building` (16), `BuildingIDs` (14) and `Roll` (12). `Roll` is a "query" that advances the RNG | missing | Query ports of at most 8 methods per system (O-sched §5); `Roll` moves to keyed RNG (GAR-033); `TestQueryPortsDoNotMutate` with a shrinking allowlist | the 57 sub-packages | E3d |
| GAR-065 | Queries never build caches | The lazy builders in GAR-031 run behind query ports | missing | Built at system boundaries (shared with GAR-031) | the listed files | E3d |
| GAR-066 | A saved timer heap (R-events 7) | Cross-tick work rides ad-hoc countdown fields (`nextThreat`, `NextNature`, `NextDecide`, `pathRetry`, the `noFood` back-off, …) | missing | A typed generic heap ordered by (Due, Seq), with a save migration | `save.go`, `domain/sim/event` | E3e |
| GAR-067 | A derived event log (StarCraft II tracker events; R-events 10) | Only the state hash | missing | An injected sink, off by default and free when off; the log feeds `cli/replay` (GAR-201) | `domain/sim/event`, `cli/replay` | E3f |

### 080–099 Frame loop and pacing

| ID | practice (source) | ours | state | change | files | phase |
|---|---|---|---|---|---|---|
| GAR-080 | One sim budget per frame, shared by every Update in it (R-frame 1; Gaffer's clamp) | A 5 ms budget per Update over `Step`, at 3x and up | done (`e0e4d4f9c`) | `tickDebt.left` spans all Updates between two Draws and counts post-step time | `update_budget.go` | E1 |
| GAR-081 | The budget applies at every speed (R-frame 1) | `nominal > 1` exempts 1x. Ebitengine v2.10.6 runs up to 5 Updates after a slow frame (`internal/clock/clock.go:31, 204-209`), where v2.9 resynced | missing | Budget 1x too. Every Update after the first in a frame steps 0 ticks once the frame budget is spent | `update_budget.go` | E1 |
| GAR-082 | Cap ticks per frame; drop the debt after a hitch (R-frame 2) | The owed debt is capped at one Update's nominal ticks; no cap per frame; the debt survives a hitch | partial | At most `ceil(speed × 1.5)` ticks per frame; clear the debt after any frame over 33 ms, on pause and on a speed change | `update_budget.go` | E1 |
| GAR-083 | Dropped ticks are visible (R-frame 2; AGENTS.md read-only HUD) | Counted in the probe (`dropped_ticks`) | partial | The HUD speed control shows ticks per second achieved against the target, from a read model | `update_budget.go`, HUD speed control | E1 |
| GAR-084 | Speed is ticks per Update, never `SetTPS` or `SyncWithFPS` (R-frame 3) | Ticks per Update at 60 TPS | done | (none) | `update_budget.go` | (none) |
| GAR-085 | Frame stats with a jitter allowance; 1% and 0.1% lows; histogram; Updates per Draw; GC (R-frame 13; CapFrameX) | The probe reports avg, 1% low and p99 only | done (`da341bcdc`) | `perf/frames.go`: histogram, `late20`, `hitch33`, 0.1% low, Updates per Draw, GC window. A gap of 16.7–20 ms counts as on time: CPU-side Draw-to-Draw gaps scatter about 1.5 ms around a vsync | `perf/frames.go`, `perf/probe.go` | C2 |
| GAR-086 | A per-phase cost in the frame (R-frame 7) | Not attributed | missing | The probe keeps per-phase avg, p99 and max through `game.SetPhaseObserver`, with the boundary tick | `perf/`, `internal/application/game` | C2 |
| GAR-087 | A flight recorder dumps spikes (R-frame 7; Go `trace.FlightRecorder`) | `-trace` records a whole run | missing | `-spiketrace <dir>`: a 600-frame ring buffer, plus `FlightRecorder.WriteTo` on a gap over 25 ms or work over 16.7 ms, at most once per 10 s, tagged with its cause | `internal/platform/profiling`, `cli/game`, `perf/` | C2 |
| GAR-088 | A VSync-on probe matrix that crosses midnight and 06:00 (R-frame 13) | `make fps` turns VSync off; a 60 s probe at 6x never crossed a day | done (`64b1ed7bf`, `c27bd1512`, PERF-004) | `scripts/fpsmatrix.sh`: 15 cells at the camera's real zooms (0.4, 0.8, 1, 2), refuses `vsync=false`; `-save-dir` keeps the player's saves untouched; `-hour` now applies to the grown colony too, and `-day 1` puts the autosave cell past day 0 (the game never autosaves on day 0) | `scripts/fpsmatrix.sh`, `Makefile` | C3 |
| GAR-089 | Tell CPU-bound from GPU-bound before optimising (R-frame 12; Metal HUD, Instruments Game Performance) | Not done | missing | Run the worst matrix cells again with `MTL_HUD_ENABLED=1 MTL_HUD_LOG_ENABLED=1`, and record GPU time against interval | `scripts/fpsmatrix.sh` (`EXTRA` env) | C3 |
| GAR-090 | Windowed and fullscreen are measured apart (R-frame 2.4: `presentsWithTransaction` only windowed with VSync) | The probe runs windowed | missing | A fullscreen pass of the matrix's worst cells | `scripts/fpsmatrix.sh` | F |
| GAR-091 | Interpolate from the frame's place in the tick stream, not the last Update's wall time (R-frame 1; Gaffer "Fix Your Timestep": alpha is the accumulator's remainder) | `frameAlpha` measures from the last `markUpdate`. Ebitengine runs a frame's Updates back to back, so below 60 fps, and at 60 when its clock drifts against VSync, walkers advance 2 ticks in one frame and 1 or 0 in the next. Unpaced per-frame displacement CV: 0.13 at 59 fps, 0.30 at 53, 0.35 at 42, 0.51 for a 2-then-0 pair | done | `render/motion/pace.go`: a drawn clock that advances by the measured tick rate (1.5 s average, re-seeded on a speed change or after a pause) and pulls toward the interpolation point (τ 0.25 s), clamped to [tick − 2 Updates, tick] and monotonic. Paced CV: 0.02 to 0.06 | `render/motion`, `app.go` Draw | E1 |

### 100–119 Sim fixes

| ID | practice (source) | ours | state | change | files | phase |
|---|---|---|---|---|---|---|
| GAR-100 | An index by item kind and by free stock (AGENTS.md "no global scans") | The ground index is per cell (`indexes.go:580-586`, `haul/ground.go`). Nothing indexes by kind or by free food or material, so every "nearest X" walks all items: `hasEdible` (`tick.go:805`), `bestFood` (`food.go:272`), `haulToStockpile` (`tick.go:847, 881-892`), `planDelivery` (`haul_route.go:147`), `haulCands` (`tick.go:976`), `nearestFree` (`orders.go:332`), the craft planner (`craft/gather.go:101, 240, 271`), drugs (`dose/drugs.go:337`), `fillTroughs` (`trough_feed.go:215`), `stockOf` (`craft_host.go:56`), `freeWood` (`burial.go:235`), books (`books/books.go:83`) | missing | An `ItemsByKind` index (def to IDs) plus free-food and free-material counts, kept on store writes. Consumers move one at a time, each with an equality test against the scan | `indexes.go`, the listed callers | E5 |
| GAR-101 | A back-off that engages (AGENTS.md no-flap rule) | A hungry pawn with no food reachable: `seekFood` calls `hasEdible` up to 4 times a tick (`tick.go:587, 595, 605, 622`). The `noFood` back-off is set only in `assignEat` (`tick.go:835, 841`), which never runs when `hasEdible` is false. In a famine, every hungry pawn walks every item up to 4 times every tick | missing | Set `noFood` when `hasEdible` is false (or answer from GAR-100's count). `TestHungryNoFoodBacksOff`, plus a no-flap test: food that appears is eaten within the retry, and food that vanishes backs off once | `tick.go` | E5 |
| GAR-102 | Reachability before a search | `animalGoTo` calls `searcher().Find` directly (`animals.go:515-516`). Pawns ask the regions first (`paths.go:25-53`, `tick.go:759-766`). An unreachable wander or feed goal floods the animal's whole component (up to 250k nodes at 500×500), for up to 300 animals, retried every `pathRetry` = 90 ticks (`handling.go:31`, `animals.go:517-521`) | missing | `Reachable` first, so an unreachable goal costs no search. `TestAnimalUnreachableGoalSkipsSearch` | `animals.go` | E5 |
| GAR-103 | A node budget on every search, time-sliced in work units (R-frame 9; Allen Chou on time slicing) | The search has no node or expansion limit (`internal/domain/ai/pathing/pathing.go:101-132`). Only long job searches are budgeted, at 6 a tick (`paths.go:62-65, 115-130`). Movement repaths (`tick.go:1601, 1636, 1660`) and non-job `pathTo` calls are never budgeted | missing | A node budget per search and per tick; a search resumes on the next tick over a versioned passability bitset. `TestPathBudgetResumes` | `pathing.go`, `paths.go` | E5, E4c |
| GAR-104 | One multi-goal search for a blocked target | For a target that is not walkable (rock, wall, bench), a job start searches to each of its 8 neighbours (`tick.go:769-799`) | missing | One search whose goal is any walkable neighbour | `tick.go`, `pathing.go` | E5 |
| GAR-105 | Haul candidates from an index | `assignHaul` passes `g.itemIDs` (`tick.go:847`), and `haulToStockpile` walks it up to `workTries` = 8 times (`tick.go:881-892`, `postpone.go:31`), per idle hauler at every idle recheck (30 ticks) | missing | A haulable list from GAR-100, ranked in one pass | `tick.go`, `postpone.go` | E5 |
| GAR-106 | A cell lookup, not a scan | A plant cell holding two plants walks every plant (`plant_growth.go:72-77`, in the chunk pass) | missing | Look the cell up in the plant index | `plant_growth.go` | E5 |
| GAR-107 | No allocations in a periodic step | While any fire burns, `fireItems` walks every item and allocates 2 maps every 30 ticks (`fire.go:216-219, 300`) | missing | Reused maps; only items near burning cells, from the ground index | `fire.go` | E5 |
| GAR-108 | An incremental colony wealth | `Wealth()` walks every item, building and animal plus packs (`raids.go:370-393`). It runs 2–4 times per threat (`story.go:107, 201`; `raids.go:120, 420`; `incidents.go:203, 248`; `quest_host.go:226`), once per map in `threatTarget` (`world_state.go:904`), and in the daily recreation tolerance (`recreate/state.go:70-71`) | missing | A total kept on store writes. `TestWealthCacheMatchesFullScan` | `raids.go`, the stores | E5 |
| GAR-109 | Retarget on a cadence, reachable targets only | An enraged animal with no target runs `nearestColonist` over every colonist every tick, without a reachability check (`handling.go:242-245`, `raids.go:225-239`) | missing | Retarget every 15 ticks, staggered, reachable colonists only. `TestEnragedRetargetsEvery15` | `handling.go`, `raids.go` | E5 |
| GAR-110 | Cheap per-tick indexes stay | `rebuildCreatures` every tick, O(P+R+A), about 320 entries (`animals.go:93`, `animal_index.go:48-57, 138-163`) | kept | It becomes the explicit freeze step (GAR-031) | `animal_index.go` | E4b |

### 120–139 Read models

| ID | practice (source) | ours | state | change | files | phase |
|---|---|---|---|---|---|---|
| GAR-120 | Views versioned on what they read (GPP Dirty Flag) | `World()` and `Snapshot()` are memoised on `[tick, viewGen]` (`indexes.go:389-400`); every Step bumps the key, so on a tick that runs, every view is rebuilt | partial | A version per view, keyed on the stores it reads; a view whose stores did not change is reused | `indexes.go` | E6 |
| GAR-121 | No allocation per rebuild | `animalViews` allocates a fresh slice each rebuild and calls `youngByMother` (`animals.go:736-740`); each animal's `trainViews` builds two slices (`train_animals.go:297-299`, from `bond_views.go:51`) | missing | Reused slices; training rows versioned on the animal's record | `animals.go`, `train_animals.go`, `bond_views.go` | E6 |
| GAR-122 | A memo check that does not redo the work | `cropWiltFlipped` recomputes the weather for every crop of the memo on every check (`plant_view.go:42, 118-130`) | missing | Re-check only when the air, light or roof version on a crop's tile changed | `plant_view.go` | E6 |
| GAR-123 | A budget on the changed tick | Not measured | missing | `BenchmarkWorldViewChangedTick` at 0.5 ms or less, and `TestWorldViewQuietTickAllocs` at 120 or fewer | `indexes.go` tests | E6 |
| GAR-124 | Views updated from events, not scans (R-events) | The views scan the stores | missing | After E3: dirty sets from events drive the rebuilds | `indexes.go` | E6 |

### 140–169 Render (ADR 0004)

| ID | practice (source) | ours | state | change | files | phase |
|---|---|---|---|---|---|---|
| GAR-140 | A per-frame command list with sort keys and a single submit (Ericson; Bevy render phases) | Each layer draws directly to its target | missing | `render/cmdlist`: arena, `pass:4\|layer:8\|sub:32\|seq:20` keys, an "already sorted?" check then radix, merging as Ebitengine would, `Submit` | new `render/cmdlist` | E7a |
| GAR-141 | Sheet-level normal maps, not per-sprite pairs (R-render 5, 6) | One pair image (sprite plus normal map) per lit sprite, made with `NewImage` and two `DrawImage`s on first use: a target switch mid-frame; an LRU of 256–1,024 deallocates evicted pairs (`render/light/light.go:808-835`). Every lit draw has a different source, so none merge | missing | Pair pages built at load by the art pipeline (atlasgen normal pages beside colour pages); lit draws from one page merge | `render/light/light.go`, `cli/atlasgen`, art skill | E7a |
| GAR-142 | Water in the list; no region uniforms (R-render 2.2, 5) | One draw per water tile through `water.kage`, inside the ground loop (`render/env/water_draw.go:148`, `render/world/base.go:157-159`): each water tile breaks the lit chain on both sides. `water.kage` reads `imageSrc0Origin` and `Size` (`:25-27, 49`). The `Time` float goes into the uniform map per tile (`water_draw.go:163`; not verified to allocate) | missing | Water tiles emit into their own sub-layer and merge. The tile's rect moves to vertex data or a whole-sheet source. `Time` is set once a frame | `render/env/water_draw.go`, `water.kage` | E7b |
| GAR-143 | Default-shader draws grouped apart from lit draws (R-render 3) | Overlay sprites (`render/sprite/sprite.go:84-93`), hit flash (`render/creature/anim.go:57-65`) and white-pixel rects (`render/creature/set.go:149`) interleave with lit draws in the object pass | missing | Each kind gets its own key bits; the strict order holds where pixels overlap | `render/sprite`, `render/creature` | E7b |
| GAR-144 | Crop quads not cut by other objects (R-render 3) | `FlushCrops` runs before every non-plant object (`render/world/objects.go:21-23`), cutting crop batches at each row that has another object | missing | Crops emit into the list at their y; merging is done in the list | `render/world/objects.go`, `render/env/crop_batch.go` | E7b |
| GAR-145 | Chunk bakes, tint bakes and pixel writes in a prepare phase before the frame's draws (R-render 2.4; Metal load and store per target) | Up to 24 chunk bakes a frame (`BakeBudget`, `render/world/lod.go:123`), each a target switch. Snow, wet and fog `WritePixels` per chunk mid-frame. Tint bakes `DrawRectShader` into a new image on first use (`art/memory.go:384-402`) | partial | A prepare pass before any world draw holds every bake and write, so world draws to the screen stay contiguous | `render/world/lod.go`, `art/memory.go`, render/env veils | E7b |
| GAR-146 | Half-resolution soft passes (GPU Gems 3; R-render 7) | Shadows are blob quads, not a full-resolution pass (O-render). Weather is up to 3 full-screen `vector.FillRect`s (veil, fog, sky event; `render/env/environment.go:1000-1006`), plus lightning (`:1078`) | partial | The weather tint folds into the composite or the final-screen pass; no full-screen fills | `render/env/environment.go` | E7c |
| GAR-147 | Skip the full-screen clear when terrain covers the screen (R-render 8; `SetScreenClearedEveryFrame`) | `screen.Fill` opens each frame (`renderer.go:494`) | missing | Skip it when the visible map covers the screen; keep it at the map's edge | `renderer.go` | E7c |
| GAR-148 | Render-scale composite without extra switches (R-render 2.5) | Below world scale 1, a `BlendCopy` composite plus 2 target switches (`gfx/worldpass.go:112-117`, `app.go:1015-1031`) | partial | A `world` pass in the key; one composite | `gfx/worldpass.go` | E7c |
| GAR-149 | Auto world detail with hysteresis (Unreal and Intel dynamic resolution; R-render 12) | Fixed world detail (Options › Graphics) | missing | EWMA of the frame gap; drop at 1.05× the budget for 30 frames, raise below 0.80× for 120, at least 60 frames between changes; never while CPU-bound; steps 1 to 0.5; no-flap test; an Options row via `rimworld-go-ui-screens` | `world_scale.go`, options | E7d |
| GAR-150 | Shaders compiled before play (R-render 11; R-frame 11) | Lazy compile in the first lit or water frame | done (`3f4efd75e`) | `light.SharedLit` and `env.SharedWater` compile and draw once off-screen in a `sync.OnceValues`; the boot and menu screens build the screen pipeline (zero-area triangle). Release `.metallib`s through `exp/shaderprecomp` are a later option | `render/light/warm.go`, `render/env/water_draw.go`, `app.go` | E9 |
| GAR-151 | A shader lint for list shaders (R-render 5) | `blood_draw_test.go` bans region helpers for one shader | partial | Generalise it to every shader the list uses | `render/` tests | E7a |
| GAR-152 | Draw-call and batch stats in a read model (R-render 14) | `Lit.Calls` counts world draws for benchmarks only | partial | The list's per-frame snapshot (commands, batches, sheets, targets and target switches, sort ran) goes to the debug HUD | `render/cmdlist`, debug HUD | E7a |
| GAR-153 | No per-frame rebuild of fixed sets (GPP Dirty Flag) | Two `map[[2]int]bool` of creature cells rebuilt every frame (`draw_art.go:156-164`) | missing | Versioned on the snapshot's tick; reused slices | `draw_art.go` | E8 |
| GAR-154 | Frame-constant uniforms set once a frame | `SyncUniforms` copies 21 floats on every `Lit.Draw` (`render/light/light.go:1062`) | missing | Once per frame, before the first lit draw | `render/light/light.go` | E7a |
| GAR-155 | The LOD tier follows the cells in view, not physical pixels (NG-1 §6.7; R-render 7) | Since UIS-2d (`049a90108`) a cell is the window's physical height / 48 × zoom (`camera/camera.go:147-154`), but the tiers still switch at 20 and 12 **physical** px (`render/env/tiles.go:17-50`, `render/world/lod.go:18-30`). At HiDPI dev 2 on this 5K window the farthest zoom (0.40) is 23 px, so the whole 120-cell-tall view draws live: draw 41.7 ms, 17 fps (Baseline 2), against 5.7 ms before UIS-2d | done (PERF-005) | `render.LODTile` = tile / max(1, dev): the tiers read points, so the same cells pick the same tier at dev 1 and 2 (`TestTierFollowsTheCellsInView`); dev 1 is unchanged. A/B at 1x, alternated twice: z0.4 17.7 → 60.0 fps (draw 41 → 5.0 ms), z0.6 32.1 → 58.9 fps (draw 21.2 → 9.5 ms), z0.8 unchanged (still live). Trade-off: below about zoom 0.7 a HiDPI window draws bakes at 2× their texels, the detail a 1440p screen shows. If that reads too soft, bake at dev × texels (chunk 32, map 8 px a tile; memory ×4) | `render/env/tiles.go`, `render/world/lod.go`, the tier callers | E7 (first) |
| GAR-156 | Engine upgrades are measured (R-frame 13) | Ebitengine v2.9 → v2.10.6 (`36d0eaf56`) added about 2 ms of draw at z0.8 (Baseline 3); nobody measured it at the time | partial (PERF-008) | Likely cause found: v2.10 renders anti-aliased `vector` paths through a stencil atlas per target, 0.82 of 1.62 s of Draw CPU in `DrawArt500/z1`; `gfx/aashape` replaced them (4.44 → 2.24 ms/op). Still to do: an upgrade lands with a back-to-back matrix A/B of the old and new `go.mod`; find the 2 ms with an `ebitenginedebug` comparison (command counts, merge breaks) | `go.mod`, `scripts/fpsmatrix.sh` | E7 |
| GAR-157 | The draw pass does not allocate per frame for game lookups (SOLID_GO §7) | After PERF-008, `DrawArt500/z1` allocates 6383 objects a frame: half are Ebitengine's `atlas.DrawTriangles`, then `construction.Footprint` (12 %) and `game.Buildables` (10 %), `AnimalBaseKey`, `StoneItem` and `pathing.reconstruct` (about 2 % each) | missing | Footprints and buildables come from cached read models, built when their version changes; `DrawArt500/z1` ≤ 6000 allocs/op | read models feeding `render/world`, `construction` | E7 |
| GAR-158 | The pixel-diff edge mask sees anti-aliased fringes (PERF-007) | `pixeldiff.EdgeMask` marks only pixels with a luma or alpha step ≥ 48, so faint AA fringes (rings, thin lines) count as non-edge and fail the `aa` policy's Δ ≤ 1, though nothing visible changed (PERF-008: non-edge max 2–19 in all 8 scenes) | done (PERF-009) | Any pixel with 0 < alpha < 255 in A or B, or within 1 px of a step, is an edge; re-run the PERF-008 scenes, which should then pass `aa` | `render/pixeldiff` | E7 |

### 170–189 Text, UI, allocations, GC

| ID | practice (source) | ours | state | change | files | phase |
|---|---|---|---|---|---|---|
| GAR-170 | Width cache and advance tables (Dear ImGui `IndexAdvanceX`; R-render 9) | `Measure` shapes every call | missing | An LRU keyed by (face, string), cleared on a face or scale change; an advance table for digits and numbers composed without `fmt.Sprintf` | `gfx/text.go` | E8 |
| GAR-171 | Binary-search ellipsis | `Ellipsize` makes O(n) `Measure` calls, each allocating a string (`gfx/text.go:310-322`), every frame from the button paint through `FitText` (`uikit/kit.go:366, 395`) | missing | A binary search over rune offsets, plus caching per (label, width) | `gfx/text.go`, `uikit/kit.go` | E8 |
| GAR-172 | No lock or allocation per text call | `fontMu` is locked in `Text`, `Measure`, `LineH` and `CenterTop` (`gfx/text.go:172, 212, 226, 246`); `&text.DrawOptions{}` is allocated per `Text` call (`:175`; escape not verified) | missing | A reused options block; the lock only where text is drawn off the game goroutine (none in play) | `gfx/text.go` | E8 |
| GAR-173 | Immediate-mode widgets without closures (Muratori 2005; Dear ImGui) | Each `Button`, `Label` and `Panel` allocates 1–2 closures per Update (`uikit/kit.go:329, 350, 374, 379`), which `Kit.Paint` replays each Draw. World labels below scale 1 are closures (`gfx/worldpass.go:44-110`) | missing | Paint values (kind, rect, label ID, state) replayed by a switch; `TestKitPaintNoClosureAllocs` | `uikit/kit.go`, `gfx/worldpass.go` | E8 |
| GAR-174 | Glyphs rasterised before the frame's draws (R-render 9d; Ebitengine #3455) | Glyphs rasterise lazily inside the UI pass | missing | `CacheGlyphs` for the UI sizes at load; new strings realise before submit | `gfx/text.go` | E8 |
| GAR-175 | 0 allocations per steady Step and Draw (R-frame 9) | About 31,000 allocations a frame in the 5 s probe at zoom 0.8 (`allocs_per_frame`) | missing | `AllocsPerRun` gates for one steady `Step` and one fixed-view `Draw`; then the sites one by one | `internal/application/game`, the renderer | E8 |
| GAR-176 | GOGC and GOMEMLIMIT set after measuring (Go GC guide; R-frame 8) | Defaults; Green Tea GC is on (Go 1.26+). The 5 s probe: 4 cycles, 35 ms of GC CPU, worst pause 49 µs | kept until measured | Set only on an A/B in the matrix: `SetGCPercent(200–400)` plus a memory limit, in compose and only when the environment does not set them | `internal/compose/game` | E8 |
| GAR-177 | PGO (Go; R-frame 10) | None | missing | `cmd/game/default.pgo` from three 30 s 6x profiles and one probe run, refreshed at each release. The `cmd` guard allows the file once the rule names it | `cmd/game`, `internal/arch/cmd_test.go` | E8 |

### 190–199 Hitches

| ID | practice (source) | ours | state | change | files | phase |
|---|---|---|---|---|---|---|
| GAR-190 | An async autosave: snapshot at a tick boundary, encode and write on a worker (R-frame 5) | Synchronous on Update at 06:00 (`options/saves.go:55-77`, calling `internal/application/game/save.go:496-590`): encode 14.6 ms (`BenchmarkSave500`), plus gzip and the write | missing | `game/savejob`: a deep-copy snapshot of at most 2 ms at the tick boundary; one worker does gob, gzip, a temp file, fsync and rename; one save in flight; "Saving…" in the HUD; failures reported | `internal/application/game/savejob`, `options/saves.go`, `filesave` | E9 |
| GAR-191 | Saves compared by canonical decode (R-frame 6: gob map order, Go 1.27 flate) | Save tests compare decoded state | kept | (none) | (none) | (none) |
| GAR-192 | Tints and the visible set baked before play (R-render 11) | Tint bakes on first use, in the frame (`art/memory.go:384-402`) | partial | Pre-bake the tints the start colony uses behind the loading screen | `art/memory.go` | E9 |
| GAR-193 | Snow cache bounded (LRU) | Per-chunk snow images | missing | An LRU with a bound and `TestSnowCacheLRUBound` | render/env snow | E9 |
| GAR-194 | The App never calls GLFW off the main thread | `monitorScale` called `ebiten.Monitor()`, which in v2.10 starts GLFW on the calling goroutine (macOS SIGTRAP in tests) | done (`49465e3ce`) | `AppConfig.DeviceScale` from `display.Screen` | `ui_scale.go`, `internal/compose/game/run.go` | A |

### 200–219 Tooling

| ID | practice (source) | ours | state | change | files | phase |
|---|---|---|---|---|---|---|
| GAR-200 | Every `cmd/<tool>` is one wiring-only `main.go` (SOLID_GO §8a) | `cmd/game/main.go` was 1,213 lines; generators held 700–2,000 | done (ARCH-007) | `internal/arch/cmd_test.go`; `cmdTODO` empty | `cmd/*`, `cli/*`, `internal/compose/game` | A |
| GAR-201 | Replay from save plus commands; report the first diverging tick and event (R-events 10; Factorio, Riot) | `cmd/replay` is a stub | missing | `cli/replay` re-simulates, compares the per-tick hash and dumps a diff | `cli/replay`, event log | E10 |
| GAR-202 | Content lint (defs, mods) | `cmd/contentlint` is a stub | missing | `cli/contentlint` validates defs and mods | `cli/contentlint` | E10 |
| GAR-203 | A perf gate in CI on work time (R-frame 14) | Benchmarks without caps | missing | `make perf-gate`: worst tick vs median tick around the rollover, the snapshot time, allocations per tick; run without `-race` in its own job | `Makefile`, CI | F |
| GAR-204 | Diagrams from data (Archify 3.2.0) | None | partial | `docs/architecture/*.json` to `.html` with `make diagrams`; CI never runs it | `docs/architecture`, `Makefile` | D |
