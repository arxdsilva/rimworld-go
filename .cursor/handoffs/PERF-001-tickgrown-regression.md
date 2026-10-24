# Task Handoff

Task ID: PERF-001-tickgrown-regression (after ENG-003)
Status: complete (race-tested)

## Summary
`BenchmarkTickGrown` had regressed against `f957fc8a` (the commit before ENG-003). It is now back to that level or better at every size, and simulation outcomes are unchanged.

## Cause
The grown fixtures built by HEAD are a different colony: more colonists are idle, so the per-tick cost of failed work searches (`assignWorkByPriority` with no job found) roughly doubled (26 failed searches per 50 ticks against 8 at 120x120). New per-tick work added to that: social opinions computed cold after a load (`tendEdges` -> `opinion`), the animal job givers' `anyAnimal` copying every `animal.State`, which had grown, and `thoughts()` allocating on every mood check.

## Changes (all behaviour-preserving)
- `component.Store.AnyRef`: stops at the first match and reads through a pointer. `anyAnimal` and its predicates (`collectReady`, `trainable`, `needsVet`) take `*animal.State`.
- Hot read-only scans use `Store.Ref` instead of copying with `Get`: hauling, stock counts, troughs, bills, repair, refuel, construct/deconstruct, wear, food, ingredients, haul routes, `itemAt`/`otherStackAt`/merge, `plantAtIndexed`, plant candidates, `woodLow`, `hasTraps`, train/tend loops.
- Postponed work (`postpone.go`): `postState.kinds` lists the keys per (pawn, job kind). It is maintained by `put`/`drop`/`reindex` and never saved. `postponedFor` returns early when the list is empty; a self-check verifies this. `postponedTargets` lets `haulToStockpile` and `plantCands` skip `postponedFor` for targets that are not put off, where it would return false and prune nothing, and run the cheap stockpile filter first. Targets that are put off keep the original filter order, so lazy pruning (which is hashed) is identical.
- `zone.Map` keeps zone records in an ID-indexed slice instead of a `map[ID]*rec`. `allows` is table-driven (`stockCats`, computed from the original switch).
- Social: `social.OpinionValue`, a value-only path with no term list, which the simulation's `opinion()` uses. The edge cache keeps terms only after a view asks for them (`termsOK`), and a view never overwrites a cached value. The ancestor walk and the opinion relation lists use stack buffers. Memory terms are computed in one pass.
- `thoughtLevel` (`moodOf`, `stepMood`) builds the thought list in a reused buffer (`mood.AppendActive`). `thoughts()` for views still returns a fresh slice.

## Verification
- `TestScaleGolden`, `TestGrownGolden`, and `Determin|SameSeed|Hash|Golden` at -count=3, with and without -race: unchanged.
- Extra check: StateHash every 500 ticks for 5,000 ticks from the 120/250/500 grown fixtures is identical between HEAD and this tree. Freshly generated 120/250 fixtures have identical StateHash. Save bytes are not comparable: gob writes maps in random order.
- New tests: `TestAnyRefStopsAtFirstMatchInInsertionOrder`, `TestOpinionValueMatchesOpinion`, `TestMemoryTermsMatchPerKindScan`, `TestPostKindsTrackEntries`, `TestPostponedTargetsCoverPostponedFor`.

## Benchmarks
`BenchmarkTickGrown`, 50x, median of 9 runs, µs/op (allocs/op):

| size | f957fc8a | HEAD 5791b03b | this change |
|---|---|---|---|
| 120 | 81.8 (3) | 122.9 (22) | 84.8 (2) |
| 250 | 185.4 (2) | 180.9 (23) | 129.0 (2) |
| 500 | 179.9 (2) | 211.1 (21) | 166.8 (3) |

Steady state at 120 (3000x): 175 → 90 B/tick (f957fc8a: 113). `BenchmarkSocialPhase50/200`: 454 → 435 ns, 15 → 0 B/op; 2190 → 2070 ns, 59 → 1 B/op.

## Determinism Impact
None. Goldens are not re-pinned.

## UI
None: an internal optimisation with no player-facing effect.

## Known Limitations
`thoughts()` calls `g.memories[id].Prune(now)`, which compacts the slice in place but does not store the result. When an expired memory comes before a live one, `g.memories[id]` keeps its old length with a duplicated tail. This predates this task and the goldens pin it, so it was left alone and `thoughts()` is still called exactly as often as before. Fixing it changes behaviour and needs a re-pin.
