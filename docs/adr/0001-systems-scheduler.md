# ADR 0001: a systems scheduler over the component stores

Status: accepted (2026-11-08). Gap rows: GAR-001 to GAR-029 (`.agents/gaps/GAME_ARCHITECTURE_GAP_ANALYSIS.md`).

## Context

A tick runs 15 phases in a fixed function list (`internal/application/game/phases.go`). On top of that, about 40 periodic sites each test `now % P == 0` with offset 0. Every one of them fires on the day tick (the least common multiple of their periods), which therefore costs about 6 times an average tick (`BenchmarkTickDayRollover500`: 1.68 ms against 0.28 ms). At 6x speed that tick lands inside a frame that already runs 6 ticks. The phase order is implicit, nothing declares what a phase reads or writes, and every new feature edits the central list.

Entity data already lives in sparse-set `component.Store[T]` stores (`internal/domain/sim/component`), and 58 sub-packages already follow "a system behind a small host port". The missing parts are the schedule, the declarations and the timing.

## Options

| | Modular monolith (keep the phase list) | **Systems scheduler (chosen)** | Full archetype ECS |
|---|---|---|---|
| pros | No refactor | Builds on the stores, phases and sub-packages; an explicit, testable order; one timing mechanism replaces the 40 `now % P` sites; declared access opens parallel stages (ADR 0002) | Best locality; queries and change detection built in |
| cons | `Game` stays the hub; the day-tick spike stays | A medium refactor of the tick; declarations need enforcing; goldens re-pin once, for the stagger | A near-rewrite of our own ECS, which our scale does not need |

## Decision

1. **One descriptor per system**, registered at the composition root with no `init()`:
   - `Name`: stable and unique. It is the tie-break key, so a rename is a behaviour change.
   - `Phase`: one of today's 15 phases, kept as coarse ordered sets.
   - `Reads`, `Writes`: typed store keys.
   - `After`, `Before`: edges inside the one scheduler only.
   - `Every`, `Offset`, and an optional per-entity `Stagger`.
   - `RunIf`: a pure predicate that declares its own reads.
   - `AmbiguousWith`: each entry carries a reason.
   - `Run`: receives the tick context and only the handles its declarations name.
2. **A deterministic order, built once.** Kahn's algorithm over a ready heap keyed by (phase rank, name). The path has no map iteration and no unstable sort. A cycle fails with its whole chain. The resolved order is a golden file, so a schedule change shows up as a reviewed diff.
3. **A conflict without an order is an error.** A pair that can run on the same tick, where one writes what the other reads or writes, and with no path between them, fails the build unless `AmbiguousWith` gives a reason. A seeded order-shuffle test must reproduce the golden state hash.
4. **Access is checked twice.** Handles `R[T]` (read) and `W[T]` (write, stamping row versions) come only from the descriptor. An arch analyzer (`TestSystemsDeclareStores`) flags store access that does not go through a handle.
5. **Timing:** `due(t) ⇔ t mod Every == Offset`. Offsets are set on purpose in a table, frozen in the golden. No tick may host more than a budgeted number of heavy systems over one LCM cycle.
6. **Per-entity stagger keeps each rate exact.** An entity gets a slot `s ∈ [0, P)` when it joins. Tick `t` runs only bucket `t mod P`, through an index, never a filter over all N. The slot and `lastRun` are saved. Integrated amounts use `t − lastRun`, never the constant P. Rebalancing happens only at load.
7. **The day rollover splits in two.** The O(1) calendar edge stays on the day tick. Per-entity daily work (aging, social day, upkeep) moves to the stagger with P = one day, so each entity gets its own midnight.
8. **Change detection by versions.** Each row carries a version stamped by `W[T]`; nothing scans to find changes. Writing an equal value does not stamp. Removal lists live until every declared reader has run.
9. **Sleeping sets.** Spawns, despawns and wake-ups wait in a buffer applied at the phase boundary, sorted by (entity ID, kind). Every threshold that puts something to sleep keeps the repo's hysteresis rule and ships with a no-flap test.
10. **Observable.** The scheduler's `Snapshot()` holds the resolved order, per-system mean and max cost, and the heaviest tick of the last day. A debug overlay and `cli/benchmark` read it.

## Adoption (strangler; the golden state hash is identical at every step unless a step says otherwise)

1. A replay harness: a seeded colony over several day ticks, hashing the state every N ticks.
2. **Wrap.** The 15 phases become 15 systems with `Reads = Writes = {Legacy}`, chained in today's order. Golden identical.
3. **Lift.** Each periodic site becomes its own system with `Every = P`, `Offset = 0`, at the position it runs today. Golden identical; the spike is unchanged.
4. **Declare.** Real reads and writes, one system per commit. Turn on checks 3 and 4. Golden identical.
5. **Spread.** Offsets, then per-entity stagger, then the day split. These are deliberate behaviour changes: each commit re-pins the golden with the reason "same per-entity rate, different tick", and records the day-tick benchmark before and after.

## Consequences

- The day tick drops to its calendar edge. Target: worst tick ≤ 1.5 ms (`TestDayTickUnderBudget`).
- Saves gain each entity's stagger slot and `lastRun` (a save migration).
- The phase list and the `now % P` tests disappear from the tick. Each feature registers its own systems.
- New code lives in `internal/application/game/sched`. The game root is at its size ceiling.

## Sources

Research fragment R-sched, with every URL; summarised in the gap file. Bevy schedules and run conditions; the flecs pipeline and rate filters; Unity DOTS system groups; Factorio Friday Facts #18, #67, #324, #364, #421 (sleeping, targeters, deterministic merge of wake-ups, update buckets); Victoria 3 Dev Diary #76 (tiered tick tasks, superset spikes); Game Programming Patterns (Update Method, Dirty Flag, Data Locality); Acton, CppCon 2014.
