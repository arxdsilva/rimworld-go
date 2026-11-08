# ADR 0002: parallel pure stages under one writer

Status: accepted (2026-11-08). Gap rows: GAR-030 to GAR-049.

## Context

The sim runs on one goroutine. AGENTS.md and MASTER_SPEC §55 make that goroutine the only owner of the simulation and forbid a goroutine per pawn or job. At today's scale (a 500×500 map, about 20 colonists) an average tick costs 0.28 ms, but searches (paths, job and food scoring) are where the time goes at raid and 6x peaks. The M2 Max has 8 performance cores and 4 efficiency cores, and the sim uses one of them.

## Options

| | Single thread, budgeted | Sim pipelined beside render | **Parallel pure stages (chosen)** |
|---|---|---|---|
| pros | Simplest | The most headroom | The sim scales with cores; keeps the one-writer rule |
| cons | One core's 16.7 ms | Input latency; every live read in Draw refactored | Needs the scheduler (ADR 0001); false sharing; small gains at our scale |

## Decision

1. **The scheduler goroutine stays the only writer.** A stage has three parts:
   - `Plan`, serial: emits tasks sorted by entity ID, each with a dense index.
   - `Compute`, pure, in parallel: a function of (frozen view, task, RNG stream) that writes only `out[task index]`.
   - `Apply`, serial: walks `out` in index order and issues the same typed mutations a serial loop would.

   `Compute` never sees a mutator. Results are values (IDs, coordinates, scores, intents), never pointers into state.
2. **Simultaneous semantics come first, on one goroutine.** A serial loop in which entity 2 sees entity 1's fresh choice is a different simulation from "decide all, then apply all". Each loop first becomes Plan → Compute all → Apply all, still serial. That behaviour change is recorded, and threshold behaviour keeps its no-flap tests. Only then does turning on parallelism become a change that leaves the state hash unchanged.
3. **One fixed pool, a `Pool` port**, implemented in `internal/platform/pool` and built in `internal/compose/game`:
   - 8 lanes: 7 workers plus the owner, which runs chunks while it waits.
   - `GOMAXPROCS` stays at its default, so render, audio and the GC keep their Ps.
   - No goroutine per task or per stage: no `errgroup` or `WaitGroup.Go` on the tick path.
   - `go` statements appear only in the pool (`TestNoGoStatementsInSim`).
4. **Chunks are fixed by the data, claimed dynamically.**
   - Chunk size depends only on the task count and a per-stage constant, never on the lane count.
   - Lanes claim chunk indices from an atomic counter, so an efficiency-core lane just claims fewer.
   - Aim for at least 3 × lanes chunks of at least about 40 µs each.
5. **Frozen by construction, guarded in tests.** No writes happen while a stage runs: the owner is inside the stage. A `frozen` epoch on the world panics on any mutation during `Compute` in test and race builds. Caches that fill lazily are warmed in `Plan`, never filled in `Compute`. There is no copy-on-write.
6. **Buffers belong to the stage.**
   - Results go in preallocated, index-addressed buffers, and variable-length output (a path) in a per-chunk arena.
   - Per-lane scratch is padded to 128 bytes (Apple silicon's line size), fully reset per task, and never visible in results.
7. **Merge in task-index order (entity-ID order), never completion order.** Conflicts keep the serial rule: the lower ID wins and the loser re-plans.
8. **RNG per task** is a stream keyed by (world seed, tick, stage, entity, sub-task) mixed through SplitMix64. It seeds a `math/rand/v2` PCG held by value. The global sim RNG is never touched in `Compute`, and no (key, counter) pair is ever reused.
9. **Reductions are integer or fixed-point**, or per-chunk partials combined in chunk order.
10. **Panics stay on the owner.** Each lane recovers a panic and records it; after `Wait`, the owner re-panics with the lowest failing task index.
11. **Serial or parallel is decided by counts, never by timings.** A stage runs inline below `minParallelTasks`, with one lane, in captures and replays, and in debug bisection. A stage ships only if it gains at least 0.2 ms in its worst matrix cell and speeds up at least 1.3×.

## Tests

- `TestParallelEqualsSerial` runs the same seed and commands with 1 lane, 8 lanes, and 8 lanes with a shuffled claim order and chunk size 1. It compares the state hash every tick and names the first tick and stage that diverge. It also runs at the forced-serial and forced-parallel thresholds, under `go test -race -cpu 1,4,8`.
- A purity test hashes the view before and after `Compute`.
- Race fixtures stay small (a 64×64 map, 4 pawns, 120 ticks) but still give at least 2 chunks per lane. Long soaks stay out of CI.

## Expected stages (each measured before it ships)

- Path searches, parallel at about 8 or more queued requests. They are time-sliced with a node budget, so a search can span ticks over a versioned passability bitset.
- Job and food scoring, by pawn.
- Animal decisions stay serial: the 60-tick bucket gives about 5 animals per tick.
- Wealth, heat and plant growth are probably memory-bound, so they are measured first.

## Consequences

The wording of the one-owner rule changes (AGENTS.md, MASTER_SPEC §55): "the scheduler is the only writer; pure stages read a frozen view and return results it applies in ID order". Disjoint-write parallel systems stay out of scope and would need their own decision.

## Sources

Research fragment R-parallel, with every URL:
- Factorio Friday Facts #215, #364 and #421: shared writes made it slower; a deterministic wake-up merge; a memory-bound stage was rejected.
- Unity entity command buffers: sorted playback.
- oneTBB grain size; Apple's dispatch guidance.
- Go issues #15110 and #54622: the cost of waking a parked worker.
- Random123 and SplitMix64.
- The Age of Empires IV and *Realms of Ruin* GDC talks.
