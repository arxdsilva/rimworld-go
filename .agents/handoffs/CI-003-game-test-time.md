# CI-003: `application/game` test time under `-race`

CI timed out on `internal/application/game` under `-race`. The aim was to cut its summed test time without weakening any test.

## What was wrong
- **Index self-checks.** Every index lookup sampled a self-check through one package-level `atomic.Uint64`. Under `-race`, each atomic syncs vector clocks, so about 2000 parallel test goroutines all hitting that one counter cost roughly 28% of the package's CPU.
- **Read-model comparisons.** `reflect.DeepEqual` on the full read models ran 4 times per tick for 1500 ticks in `TestViewCacheMatchesFresh`; with checkptr on, every reflect field access is slow.
- **Fixed run lengths.** Some tests stepped a fixed number of ticks long after the event they assert had already happened.
- **Hop-scratch race.** The world tests shared one planet whose `Hops` writes a scratch buffer on the planet, so the full run reported data races.

## What changed
- `0d6286a48`: `checkIndex(g)` counts self-check samples per game (`indexes.checkN`, no atomic). `haulHost.SelfCheck` keeps the old `selfCheck()`. The benchmark state hashes are unchanged.
- `0322b6959`: `planet.Fork()` makes a shallow copy with its own hop scratch. Test games get a fork, and `fork_test.go` shows two forks running `Hops` at once under `-race`. Hashes are unchanged.
- `internal/testkit/deepeq`: a `Comparer` that gives the same answers as `reflect.DeepEqual`. It compiles per-type comparisons and compares pointer-free values with `==` in chunks. Its tests check it against DeepEqual on edge cases, 3000 random trees and every chunk boundary. `TestViewCacheMatchesFresh` uses it and still compares every tick of 1500.
- Tests that now stop at their asserted event (same upper bound):
  - `TestGrowZoneSowsGrowsAndHarvests` stops when the yield is in;
  - `TestFirstPlayableHaulBedEatSleep` stops when the bed, stockpiled food, eating and sleeping all hold, checked every 2000 ticks up to 80000;
  - `TestPhaseWeatherAdvancesDeterministically` stops at the first block boundary where the two seeds' weather differs, up to 40 blocks.
- `TestColonistOrder*`: each seed is played once, through a `sync.Once` fixture, and shared. `SameSeedSameHash` still compares two independent runs of one seed (now seed 1), using 6 runs instead of 8.
- `standingCreatures` (blocking scenarios) builds the carried list once per call instead of once per creature.

## Measurements
`go test -race -count=1 -json`, macOS, 12 cores. Every run shared the machine with other agents (load 13–22), so the numbers are noisy. "Effective" adds up the leaf tests, because parallel subtests report their parent as about 0 s.

| | baseline | after |
|---|---|---|
| top-level sum | 939.4 s | 551.2 s |
| leaf (effective) sum | 1148.1 s | 733.2 s |
| `TestViewCacheMatchesFresh` | 76.7 | 28.3 |
| `TestGrowZoneSowsGrowsAndHarvests` | 22.1 | 7.1 |
| `TestFirstPlayableHaulBedEatSleep` | 8.9 | 1.8 |
| `TestPhaseWeatherAdvancesDeterministically` | 9.2 | 0.6 |
| `TestColonistOrderSameSeedSameHash` | 13.2 | 4.7 |
| `TestColonistOrderOnlyTheView` | 13.0 | 9.0 |

## Gaps
- The 350 s top-level target is not met: 551 s, under contention. What remains is mostly simulation the tests are there to run:
  - `TestPopulationStableTenDays`: about 80 s, ten game days;
  - `TestPackBillSavesMidway`: about 63 s, a save-hash pair every tick;
  - `TestRegionsDoNotChangeState`: about 27 s.
- Production hot spots left untouched (pure perf, which needs the lead's approval):
  - `StateHash` does a gob+gzip save and a decode, about 22 ms per call under race;
  - `Guard.ThreatTo`/`NearestThreat` copy `animal.State` by value per raider;
  - the `plantAtScan`/`layerBldgAtScan` fallbacks.
- No HUD change: this is test and CI work only.
