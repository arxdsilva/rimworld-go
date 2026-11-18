# Test flakiness gap analysis (TFL-001..)

Scope: every Go test package in the repo (`go list ./...` = 294 at 58dc5a4b9: 292 before this run, plus the new `scalegolden` and `testkit/allocs`). What was checked, what was fixed, what was ruled out, what stays on watch. Source run: `.loops/runs/20261118-1734-follow-ups-ally-band-beacon-power-golden` (`research/flaky.md`, `audit/scan.md`, lane reports and QA files). The summary row is in `handoffs/OPS.md`; this file holds the per-package table.

## Decision record

| Item | Decision |
|---|---|
| Method | Three sources per package: CI history (`gh`, test.yml 42 runs, release.yml 40 runs, 2026-10-22 to 11-18), a static scan (`audit/scan.sh`: sleeps, wall clock, `Gosched`, `NumGoroutine`, `AllocsPerRun`, `t.Parallel`, shared package vars and `sync.Once` fixtures, map ranges in assertions, `linkname`, env gates, build tags), and a sweep (`-race -shuffle=on -count=3`, 268 packages, 207 ok, 60 no test files, 1 FAIL that is not a flake). |
| Fix rule | A flake gets a named cause, a fix that keeps the assertion (no raised threshold, no retry, no skip), a before/after pass rate at `-count` 50 to 200 with `-cpu 1,2(,4)`, and a bite proof (a mutation that must fail). |
| Not evidenced | Stated per row. Globe was fixed by reasoning, never reproduced. Misc comment-only rows have no bite proof because no assertion changed. |
| Not a test flake | Release run 42669968005 (v0.1.34, 11-09): test job SUCCESS, no build job ran. Cause unknown, needs the job log. |
| Hashes | 02b37c56b features; cb5e5abbb pool, usagehttp, filesave, globe; d53afbf69 aashape and light/overlay allocations; 659c5ea45 ebiten root (W1b-flaky-ebiten); 3c431e8a0 scale golden and CI (W2-golden); a3746ef50 game fog, planet, day budget (W3-flaky-game). |

## Findings

- **TFL-001 Time budget (largest class).** 15 of 28 failed runs (test 18, release 10) hit `-timeout 10m` in `application/game` (about 333 s last green) or the ebiten root (about 310 s). Last on 11-08 (run 42526626027), none since 11-09. Not reproducible by the sweep (both packages excluded as heavy). The root now runs alone at 150.7 s (W1b-flaky-ebiten, race, count 1, -p 1). Status: watch; `game` is W3-flaky-game. CI job used 23 of 25 minutes at bd41e9500.
- **TFL-002 Allocation checks against Ebitengine (aashape, light, overlay).** The +1 allocation is Ebitengine's `sync.Pool` refill under race, not the code under test. `AllocsPerRun` averages hid it when the average fell under the integer cut-off. Fix: the fewest allocations of single calls, no slack (d53afbf69, power_cords in 02b37c56b). Status: fixed. Caveat: min-of-single-calls hides an intermittent allocation in the code under test exactly as the old average did. It catches a steady one: a per-call allocation fails 600/600 and 1800/1800.
- **TFL-003 Golden drift (`TestHabitsOffKeepTheHash`, scale golden).** Five failures on 11-04, all the same hash: deterministic per commit, not random. The golden moved in 6837041ac. Separately all six scale pins had been stale since cc8ba7a0d (EAI-014) because the golden did not run in CI. W2-golden re-pinned once with causes and moved it into `scalegolden` (own CI step under race, est. 40 s). Status: fixed.
- **TFL-004 Art and UI races (11-03/04).** Five art tests and `TestUIDrawsThroughCanvas` fixed by 604628c61 and 2656774a2. Status: fixed earlier; art re-swept ok.
- **TFL-005 Filesave delete race.** Fixed by 2f8f97867, loop bounded and failure no longer hangs in cb5e5abbb. Status: fixed.
- **TFL-006 Wall-clock and goroutine-count tests (usagehttp, pool).** Replaced by an injected timer and a pprof-labelled goroutine count (cb5e5abbb). Status: fixed.
- **TFL-007 Ebiten root.** `TestJourneySaveAsyncLoadsTheSavedTick` flapped once (release 42610487265, 11-09, no fix commit before this run). Cause found: the layout Update polled the save before the click cleared the hint (the old body fails 5/5 with a Wait there). The test now has no timing dependence, and the process-wide atlas drain goroutine (candidate 4) is gone too (W1b-flaky-ebiten, 659c5ea45). Status: fixed.
- **TFL-008 Globe `TestGlobeNoAllocSteadyState`.** Fixed by reasoning, not reproduction (cb5e5abbb). Status: fixed, unproven by a red run; watch.
- **TFL-009 Not flakes.** Lint and gofmt failures 11-06 to 11-09 were real findings. `lang` `TestCatalogIsCurrent` fails every run until `assets/lang/en.json` is regenerated (lane fallout).

## Candidate mapping (`research/flaky.md` B1-B13)

| # | Candidate | Disposition |
|---|---|---|
| 1 | aashape `alloc_test.go` | fixed, d53afbf69 |
| 2 | game `sense_joy_test.go` golden | fixed by W2-golden (pins, causes) |
| 3 | filesave `Gosched` loop | fixed, cb5e5abbb |
| 4 | ebiten `TestMain` drain goroutine | fixed, W1b-flaky-ebiten |
| 5 | ebiten `saves_async_test.go` | fixed, W1b-flaky-ebiten |
| 6 | game `fog_test.go:392-406` 5 ms threshold | W3-flaky-game, a3746ef50: the bound counts flooded cells (401 per reveal), no wall clock |
| 7 | usagehttp `waitFor`, `TestIDs` | fixed, cb5e5abbb |
| 8 | pool `NumGoroutine` | fixed, cb5e5abbb |
| 9 | shared fixtures | ruled out: worldview (cb5e5abbb), ebiten root (47 users, count 10), `worldinfo`, `game/maps` `walker` read-only; game `worldTestOnce`: W3-flaky-game |
| 10 | alloc thresholds | perf, gfx, sheet, sense/joy ruled out with comments; globe fixed; perf_overlay and hud_resources fixed in W1b-flaky-ebiten; 38 other hits benign (pure code, exact 0) |
| 11 | ebitenaudio device | ruled out: build tag `audiodevice`, not in CI |
| 12 | low-risk list | rechecked by scan: fake clocks, seeded `rand` only, no fixed ports, temp files in `t.TempDir()` |
| 13 | map iteration order | 175 heuristic hits, 19 with order-sensitive bodies read one by one: all benign |

## Notes

1. Build tags: `race_off_test.go` (`!race`) is not compiled in CI, so tests that skip under `raceOn` (`population_test.go:255`, `day_budget`) are invisible there. W3-flaky-game.
2. Opt-in gates not run in CI: `testing.Short` skips, `RIMWORLD_PROBE`, `GLOBE_CAPTURE_DIR`, `LANG_GUARD_LIST`.
3. Stress: light and overlay count 20 ok and count 10 `-cpu 1,2` ok before the fix; count 50 `-cpu 1,2,4` after.
4. Watch: the 11 `watch` rows below are feature-lane packages that were being edited and were not swept (W1a-ui, W1a-domain-trade, W1a-domain-faction, W1a-beacon-app). The lead swept them after the lanes landed: `-race -shuffle=on -count=10`, all ok (tradeview has no tests).
5. Open: whether the game package fits 10 min after the golden left it; a failing flake that appears only on linux/amd64 CI cannot be reproduced on the arm64 dev machine (amd64 was run under Rosetta, not linux).

## Per-package table (293 rows)

Columns: package, `t.Parallel()` calls, what was checked, disposition, evidence. "Sweep" is `-race -shuffle=on -count=3`.

| package | par | checked | disposition | evidence |
|---|---|---|---|---|
| `cmd/atlasgen` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `cmd/audiogen` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `cmd/benchmark` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `cmd/contentlint` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `cmd/game` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `cmd/iconpack` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `cmd/mapgen` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `cmd/normalgen` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `cmd/rambudget` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `cmd/replay` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `cmd/sfxgen` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `cmd/simulate` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/adapters/inbound/cli` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/adapters/inbound/cli/atlasgen` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/adapters/inbound/cli/audiogen` | 29 | scan, sweep | ruled out | map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; benign: package vars are read-only tables; swept ok. |
| `internal/adapters/inbound/cli/benchmark` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/adapters/inbound/cli/contentlint` | 3 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/adapters/inbound/cli/game` | 5 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/adapters/inbound/cli/mapgen` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/adapters/inbound/cli/mapgen/render` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/adapters/inbound/cli/normalgen` | 0 | scan, sweep | ruled out | time.Now().Add(1h) mtime ordering for Chtimes (normalgen_test.go:85), no wall-clock race; swept ok. |
| `internal/adapters/inbound/cli/rambudget` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/adapters/inbound/cli/sfxgen` | 36 | scan, sweep | ruled out | map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; benign: package vars are read-only tables; swept ok. |
| `internal/adapters/inbound/cli/simulate` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/adapters/inbound/debugui` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/adapters/inbound/ebiten` | 1587 | CI history, scan, lane runs | fixed, lane W1b-flaky-ebiten 659c5ea45 | Drain goroutine removed (only test goroutines drain, `test_main_test.go`); async-save test has no timing dependence; perf overlay queue regrowth removed. Root alone, race, shuffle, count 1, -p 1: ok 150.7 s. Touched tests count 10 ok, saves count 50 ok. Fixtures worldFix, realArtOnce, atlasArtOnce read-only (47 users, count 10, ok). |
| `internal/adapters/inbound/ebiten/architect` | 29 | scan only (lane was editing it) | swept after the lanes landed: -race -shuffle=on -count=10 ok (lead) | Owner W1a-ui, a feature lane, not a flake lane. No flake seen, none tested. Scan note: OWNED-BY-LANE: W1a-ui |
| `internal/adapters/inbound/ebiten/art` | 23 | CI history (5 races), scan, sweep | fixed earlier, 604628c61 | Tint shader races fixed 11-04. Sweep (race, shuffle, count 3) ok; tier 2 count 5 ok. |
| `internal/adapters/inbound/ebiten/camera` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/adapters/inbound/ebiten/devtools` | 16 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/adapters/inbound/ebiten/dialogs` | 15 | scan only (lane was editing it) | swept after the lanes landed: -race -shuffle=on -count=10 ok (lead) | Owner W1a-ui, a feature lane, not a flake lane. No flake seen, none tested. Scan note: OWNED-BY-LANE: W1a-ui |
| `internal/adapters/inbound/ebiten/display` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/adapters/inbound/ebiten/dossier` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/adapters/inbound/ebiten/dossier/sheet` | 55 | scan, lane | ruled out (comment), cb5e5abbb | `DockedFor` is integer arithmetic over value copies. Count 50, -cpu 1,2 ok. |
| `internal/adapters/inbound/ebiten/gfx` | 0 | scan, lane | ruled out (comment), cb5e5abbb | `textrun_test.go`: hit path only, 78 runs under RunCap 256, serial. Count 50, -cpu 1,2 ok. |
| `internal/adapters/inbound/ebiten/gfx/aashape` | 0 | CI history (2 fails), scan, stress | fixed, d53afbf69 | Cause: Ebitengine sync.Pool refill under race (Put drops 1 in 4), not the Painter. Min of 40 single-call samples, no slack. Before 32/600 FAIL, after 600/600 PASS on arm64 and amd64; bite via `-overlay` 600/600 FAIL. |
| `internal/adapters/inbound/ebiten/globe` | 34 | scan, lane | fixed by reasoning, not reproduced, cb5e5abbb | Never seen failing. Cause by reasoning: frame buffers grow to the peak camera view, so a later angle could allocate inside the measured runs; warm-up now walks the measured sweep. Count 5 ok (190 s). |
| `internal/adapters/inbound/ebiten/globemath` | 13 | scan, sweep | ruled out | imports math/rand but only seeded rand.New sources (no top-level rand.X( calls anywhere in tests); swept ok. |
| `internal/adapters/inbound/ebiten/hud` | 82 | scan only (lane was editing it) | swept after the lanes landed: -race -shuffle=on -count=10 ok (lead) | Owner W1a-ui, a feature lane, not a flake lane. No flake seen, none tested. Scan note: OWNED-BY-LANE: W1a-ui |
| `internal/adapters/inbound/ebiten/hudlayout` | 10 | scan, sweep | ruled out | map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; swept ok. |
| `internal/adapters/inbound/ebiten/ids` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/adapters/inbound/ebiten/infocard` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/adapters/inbound/ebiten/inputframe` | 4 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/adapters/inbound/ebiten/inspector` | 67 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/adapters/inbound/ebiten/inspector/gear` | 10 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/adapters/inbound/ebiten/lang` | 11 | scan, sweep | not a flake: lane fallout, catalog regenerated in 02b37c56b | `TestCatalogIsCurrent` failed 3/3 and 20/20 with a fixed shuffle seed: `assets/lang/en.json` stale (2952 keys in code) after the W1a-ui text changes. Fix: run its `-update` flag, then re-run the package. |
| `internal/adapters/inbound/ebiten/learn` | 8 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/adapters/inbound/ebiten/mainmenu` | 19 | scan, sweep | ruled out | `time.Now()` is only a reference for relative slot ages (`screen_test.go:44`), no timing assertion. |
| `internal/adapters/inbound/ebiten/mapnav` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/adapters/inbound/ebiten/options` | 32 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/adapters/inbound/ebiten/options/saves` | 38 | scan, sweep | ruled out | package vars are read-only tables; swept ok. |
| `internal/adapters/inbound/ebiten/orders` | 22 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/adapters/inbound/ebiten/perf` | 10 | scan, lane | ruled out (comment), cb5e5abbb | `TestProbeAllocFree`: fixed arrays and counters, no growth. Count 50, -cpu 1,2 ok. |
| `internal/adapters/inbound/ebiten/plugins` | 10 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/adapters/inbound/ebiten/pointer` | 1 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/adapters/inbound/ebiten/ports` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/adapters/inbound/ebiten/render` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/adapters/inbound/ebiten/render/creature` | 25 | scan, sweep | ruled out | AllocsPerRun == 0 on pure Dress; swept ok. |
| `internal/adapters/inbound/ebiten/render/cue` | 27 | scan, sweep | ruled out | AllocsPerRun on pure code (exact 0, or <=4 at wave3_sounds_test.go:47), not parallel; swept ok. |
| `internal/adapters/inbound/ebiten/render/env` | 37 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/adapters/inbound/ebiten/render/light` | 33 | scan, stress, lane | fixed (preventive), d53afbf69 | Same mechanism as aashape; `fewestAllocs(100, f)`, +1/+2 slack removed. 1800/1800 PASS under race; a +1 bite fails 1800/1800. Count 50, -cpu 1,2,4 ok. |
| `internal/adapters/inbound/ebiten/render/minimap` | 2 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/adapters/inbound/ebiten/render/motion` | 13 | scan, sweep | ruled out | package vars are read-only tables; swept ok. |
| `internal/adapters/inbound/ebiten/render/overlay` | 70 | scan, stress, lane | fixed (preventive, power_cords), 02b37c56b | `fewestAllocs`, slack removed in `power_cords_test.go` (landed in the feature commit through the shared checkout; content correct). 600/600 PASS; count 50, -cpu 1,2,4 ok. Other overlay tests are W1a-ui lane files, see note 4. |
| `internal/adapters/inbound/ebiten/render/pixeldiff` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/adapters/inbound/ebiten/render/sprite` | 22 | scan, sweep | ruled out | AllocsPerRun == 0 on pure lookups, not parallel; swept ok. |
| `internal/adapters/inbound/ebiten/render/world` | 19 | scan, sweep | ruled out | AllocsPerRun == 0 on pure sort/sprite code, not parallel; swept ok. |
| `internal/adapters/inbound/ebiten/selection` | 15 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/adapters/inbound/ebiten/uikit` | 30 | scan, sweep | ruled out | map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; swept ok. |
| `internal/adapters/inbound/ebiten/units` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/adapters/inbound/ebiten/windows` | 68 | scan only (lane was editing it) | swept after the lanes landed: -race -shuffle=on -count=10 ok (lead) | Owner W1a-ui, a feature lane, not a flake lane. No flake seen, none tested. Scan note: OWNED-BY-LANE: W1a-ui |
| `internal/adapters/inbound/ebiten/wizard` | 10 | scan only (lane was editing it) | swept after the lanes landed: -race -shuffle=on -count=10 ok (lead) | Owner W1a-ui, a feature lane, not a flake lane. No flake seen, none tested. Scan note: OWNED-BY-LANE: W1a-ui |
| `internal/adapters/inbound/ebiten/worldinfo` | 0 | scan, sweep | ruled out | `sync.Once` planet fixture (`info_test.go:20`) built once and read-only; swept ok. |
| `internal/adapters/inbound/ebiten/worldview` | 31 | scan, lane | ruled out (proof), cb5e5abbb | `testPlanet`, shared by parallel tests, is read-only (worldview never calls `Planet.Hops`; `HopsWith` has its own scratch). Comment says use `Fork()` if that changes. Count 5 ok; no -cpu sweep. |
| `internal/adapters/inbound/future_net` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/adapters/outbound/cardpresetfile` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/adapters/outbound/ebitenaudio` | 22 | scan | ruled out | `device_test.go` is `//go:build audiodevice`, opt-in manual smoke, not in CI. `load_test.go` temp file is inside `t.TempDir()`. |
| `internal/adapters/outbound/filesave` | 12 | CI history (1 fail), scan, lane | fixed, 2f8f97867 then cb5e5abbb | Race fixed by 2f8f97867 (153/600 before, 0 after). cb5e5abbb bounds the remaining `Gosched` loop with a 10 s deadline and releases the gate so a failure no longer hangs. Count 200, -cpu 1,2 ok; bite fails in 10.03 s instead of hanging. |
| `internal/adapters/outbound/modloader` | 1 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/adapters/outbound/scenariofile` | 6 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/adapters/outbound/settingsfile` | 3 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/adapters/outbound/sfxapi` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/adapters/outbound/slogmetrics` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/adapters/outbound/starlarkhost` | 9 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/adapters/outbound/updatecheck` | 6 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/adapters/outbound/usagehttp` | 17 | scan, lane | fixed, cb5e5abbb | No wall-clock assertion left: injected close timer (`sender.go` seam, nil means the real timer); `TestCloseIsBounded` proves close blocks until the timer fires. Both mutations (ignore timer, ask 2x) fail. Count 50, -cpu 1,2,4 ok. |
| `internal/application/benchmark` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/application/commands` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/application/devscene` | 4 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/application/game` | 2350 | CI history (time budget, golden), scan, lane | fixed: W3-flaky-game a3746ef50; golden moved to scalegolden by W2-golden 3c431e8a0 | Heavy binary, last green 333 s of the 10 min budget. Candidates: `fog_test.go:392-406` 5 ms wall threshold, Once fixtures, 12 package vars, `race_off_test.go` skips. W3-flaky-game (a3746ef50): the fog bound counts 401 flooded cells per reveal; a test proves the shared planet read-only; the wall-clock day budget is opt-in (`RIMWORLD_TIMING=1`), as it already skipped under -race. Whole package once at CI pressure (`GOMAXPROCS=2 -p 1 -race -shuffle=on`): ok in 325.9 s (product proof AC-7). |
| `internal/application/game/scalegolden` | 0 | new package, lane | fixed, lane W2-golden 3c431e8a0 | Six pins stale since cc8ba7a0d (EAI-014), unseen because the golden never ran in CI. Re-pinned; moved out of the heavy binary to run under race in its own job (est. 40 s). amd64 equals arm64. |
| `internal/application/game/actionview` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/application/game/allow` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/application/game/animalview` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/application/game/aqua` | 14 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/application/game/axview` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/application/game/beacon` | 5 | scan, sweep | ruled out | AllocsPerRun == 0 on pure Reach, not parallel; swept ok. |
| `internal/application/game/bodytemp` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/application/game/books` | 2 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/application/game/cadence` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/application/game/care` | 9 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/application/game/combat` | 139 | scan, sweep | ruled out | imports math/rand but only seeded rand.New sources (no top-level rand.X( calls anywhere in tests); benign: package vars are read-only tables; swept ok. |
| `internal/application/game/contentid` | 4 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/application/game/craft` | 101 | scan, sweep | ruled out | map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; benign: package vars are read-only tables; swept ok. |
| `internal/application/game/crops` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/application/game/diet` | 9 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/application/game/dose` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/application/game/dossierview` | 5 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/application/game/evlog` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/application/game/factionview` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/application/game/farm` | 7 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/application/game/floors` | 1 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/application/game/forage` | 4 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/application/game/framestatus` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/application/game/furnish` | 3 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/application/game/gather` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/application/game/gearfacts` | 4 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/application/game/gearview` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/application/game/haul` | 10 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/application/game/herd` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/application/game/hit` | 5 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/application/game/idindex` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/application/game/industry` | 1 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/application/game/landing` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/application/game/letters` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/application/game/mapfields` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/application/game/maps` | 7 | scan, sweep | ruled out | package var `walker` (questsite_test.go:62) is shared by parallel tests but questSite only reads it (questsite.go:41); the sweep runs it under -race; swept ok. |
| `internal/application/game/mapview` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/application/game/memo` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/application/game/mind` | 6 | scan, sweep | ruled out | map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; benign: package vars are read-only tables; swept ok. |
| `internal/application/game/orders` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/application/game/outfit` | 5 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/application/game/pathstage` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/application/game/postpone` | 1 | scan, sweep | ruled out | imports math/rand but only seeded rand.New sources (no top-level rand.X( calls anywhere in tests); swept ok. |
| `internal/application/game/prison` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/application/game/quest` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/application/game/quest/kinds` | 81 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/application/game/rage` | 3 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/application/game/raid` | 21 | scan, sweep | ruled out | map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; swept ok. |
| `internal/application/game/recreate` | 16 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/application/game/research` | 9 | scan, sweep | ruled out | AllocsPerRun asserts exact 0/small on pure code, not parallel; swept ok. |
| `internal/application/game/resize` | 3 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/application/game/rest` | 0 | scan, sweep | ruled out | AllocsPerRun asserts exact 0/small on pure code, not parallel; benign: map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; swept ok. |
| `internal/application/game/roster` | 11 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/application/game/savefmt` | 22 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/application/game/sched` | 0 | scan, sweep | ruled out | AllocsPerRun == 0 on pure code, not parallel; swept ok. |
| `internal/application/game/sense` | 52 | scan, lane | ruled out (comment), cb5e5abbb | `joy_test.go`: one map, one key, slot reused after the warm insert. Count 50, -cpu 1,2 ok. Rest of the package: W3-flaky-game. |
| `internal/application/game/setupview` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/application/game/ship` | 10 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/application/game/snow` | 17 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/application/game/sociallog` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/application/game/spatial` | 0 | scan, sweep | ruled out | AllocsPerRun asserts exact 0/small on pure code, not parallel; swept ok. |
| `internal/application/game/store` | 12 | scan, sweep | ruled out | package vars are read-only tables; swept ok. |
| `internal/application/game/tabview` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/application/game/thermo` | 0 | scan, sweep | ruled out | imports math/rand but only seeded rand.New sources (no top-level rand.X( calls anywhere in tests); swept ok. |
| `internal/application/game/tradeview` | 0 | scan only (lane was editing it) | swept after the lanes landed: -race -shuffle=on -count=10 ok (lead) | Owner W1a-beacon-app, a feature lane, not a flake lane. No flake seen, none tested. Scan note: OWNED-BY-LANE: W1a-beacon-app |
| `internal/application/game/travel` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/application/game/visit` | 2 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/application/game/wealth` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/application/modding` | 33 | scan, sweep | ruled out | imports math/rand but only seeded rand.New sources (no top-level rand.X( calls anywhere in tests); benign: map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; swept ok. |
| `internal/application/replay` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/application/savegame` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/application/scenario` | 0 | scan, sweep | ruled out | `time.Now` at `grown_test.go:51-58` feeds `t.Logf` only; env gates are opt-in skips. Slowest swept package (155 s at count 3). |
| `internal/application/scripting` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/application/session` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/arch` | 16 | scan, sweep | ruled out | package vars are read-only tables; tests read the repo tree, no writes; swept ok. |
| `internal/compose/game` | 21 | scan, sweep | ruled out | package vars are read-only tables; swept ok. |
| `internal/domain/ai/intent` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/domain/ai/job` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/domain/ai/movement` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/domain/ai/pathing` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/domain/ai/reservation` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/domain/ai/work` | 1 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/domain/animal` | 44 | scan, sweep | ruled out | map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; swept ok. |
| `internal/domain/animal/training` | 4 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/domain/building/aqua` | 11 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/domain/building/construction` | 24 | scan only (lane was editing it) | swept after the lanes landed: -race -shuffle=on -count=10 ok (lead) | Owner W1a-domain-trade, a feature lane, not a flake lane. No flake seen, none tested. Scan note: OWNED-BY-LANE: W1a-domain-trade |
| `internal/domain/building/door` | 4 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/domain/building/power` | 37 | scan only (lane was editing it) | swept after the lanes landed: -race -shuffle=on -count=10 ok (lead) | Owner W1a-domain-trade, a feature lane, not a flake lane. No flake seen, none tested. Scan note: OWNED-BY-LANE: W1a-domain-trade |
| `internal/domain/building/zone` | 18 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/domain/combat/armor` | 18 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/domain/combat/attack` | 12 | scan, sweep | ruled out | package vars are read-only tables; swept ok. |
| `internal/domain/combat/cover` | 7 | scan, sweep | ruled out | map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; swept ok. |
| `internal/domain/combat/damage` | 2 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/domain/combat/explosion` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/domain/combat/kit` | 6 | scan, sweep | ruled out | imports math/rand but only seeded rand.New sources (no top-level rand.X( calls anywhere in tests); benign: map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; swept ok. |
| `internal/domain/combat/projectile` | 5 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/domain/combat/weapon` | 17 | scan, sweep | ruled out | map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; benign: package vars are read-only tables; swept ok. |
| `internal/domain/difficulty` | 11 | scan, sweep | ruled out | map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; swept ok. |
| `internal/domain/economy/book` | 3 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/domain/economy/item` | 40 | scan, sweep | ruled out | map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; benign: package vars are read-only tables; swept ok. |
| `internal/domain/economy/production` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/domain/economy/quality` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/domain/economy/recipe` | 31 | scan, sweep | ruled out | AllocsPerRun asserts exact 0/small on pure code, not parallel; benign: map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; swept ok. |
| `internal/domain/economy/storage` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/domain/economy/trade` | 9 | scan only (lane was editing it) | swept after the lanes landed: -race -shuffle=on -count=10 ok (lead) | Owner W1a-domain-trade, a feature lane, not a flake lane. No flake seen, none tested. Scan note: OWNED-BY-LANE: W1a-domain-trade |
| `internal/domain/faction` | 17 | scan only (lane was editing it) | swept after the lanes landed: -race -shuffle=on -count=10 ok (lead) | Owner W1a-domain-faction, a feature lane, not a flake lane. No flake seen, none tested. Scan note: OWNED-BY-LANE: W1a-domain-faction |
| `internal/domain/gameplay` | 4 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/domain/incident` | 6 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/domain/pawn/backstory` | 0 | scan, sweep | ruled out | map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; swept ok. |
| `internal/domain/pawn/body` | 37 | scan, sweep | ruled out | imports math/rand but only seeded rand.New sources (no top-level rand.X( calls anywhere in tests); benign: map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; swept ok. |
| `internal/domain/pawn/drug` | 1 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/domain/pawn/exposure` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/domain/pawn/health` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/domain/pawn/infection` | 8 | scan, sweep | ruled out | imports math/rand but only seeded rand.New sources (no top-level rand.X( calls anywhere in tests); benign: package vars are read-only tables; swept ok. |
| `internal/domain/pawn/inventory` | 7 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/domain/pawn/look` | 0 | scan, sweep | ruled out | map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; swept ok. |
| `internal/domain/pawn/medcare` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/domain/pawn/memory` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/domain/pawn/mentalbreak` | 0 | scan, sweep | ruled out | map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; swept ok. |
| `internal/domain/pawn/mood` | 8 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/domain/pawn/needs` | 9 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/domain/pawn/pawngen` | 0 | scan, sweep | ruled out | map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; swept ok. |
| `internal/domain/pawn/prisoner` | 3 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/domain/pawn/recreation` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/domain/pawn/schedule` | 2 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/domain/pawn/skill` | 0 | scan, sweep | ruled out | map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; swept ok. |
| `internal/domain/pawn/social` | 0 | scan, sweep | ruled out | imports math/rand but only seeded rand.New sources (no top-level rand.X( calls anywhere in tests); benign: map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; swept ok. |
| `internal/domain/pawn/stat` | 1 | scan, sweep | ruled out | map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; swept ok. |
| `internal/domain/pawn/trait` | 0 | scan, sweep | ruled out | map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; swept ok. |
| `internal/domain/quest` | 0 | scan only (lane was editing it) | swept after the lanes landed: -race -shuffle=on -count=10 ok (lead) | Owner W1a-domain-faction, a feature lane, not a flake lane. No flake seen, none tested. Scan note: OWNED-BY-LANE: W1a-domain-faction |
| `internal/domain/raid` | 5 | scan, sweep | ruled out | map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; swept ok. |
| `internal/domain/research` | 18 | scan, sweep | ruled out | map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; swept ok. |
| `internal/domain/scenario` | 10 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/domain/sim/clock` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/domain/sim/command` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/domain/sim/component` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/domain/sim/entity` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/domain/sim/event` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/domain/sim/scheduler` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/domain/sim/snapshot` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/domain/sim/statehash` | 0 | scan, sweep | ruled out | map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; swept ok. |
| `internal/domain/storyteller` | 8 | scan, sweep | ruled out | map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; swept ok. |
| `internal/domain/world/area` | 3 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/domain/world/biome` | 3 | scan, sweep | ruled out | map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; swept ok. |
| `internal/domain/world/caravan` | 5 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/domain/world/chunk` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/domain/world/filth` | 5 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/domain/world/fire` | 0 | scan, sweep | ruled out | map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; swept ok. |
| `internal/domain/world/generation` | 25 | scan, sweep | ruled out | map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; benign: package vars are read-only tables; swept ok. |
| `internal/domain/world/geology` | 22 | scan, sweep | ruled out | package vars are read-only tables; swept ok. |
| `internal/domain/world/grid` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/domain/world/lighting` | 3 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/domain/world/mapgen` | 4 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/domain/world/mapgen/pipeline` | 1 | scan, sweep | ruled out | package vars are read-only tables; swept ok. |
| `internal/domain/world/mapgen/steps/features` | 2 | scan, sweep | ruled out | map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; swept ok. |
| `internal/domain/world/mapgen/steps/ground` | 0 | scan, sweep | ruled out | map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; swept ok. |
| `internal/domain/world/mapgen/steps/landform` | 4 | scan, sweep | ruled out | package vars are read-only tables; swept ok. |
| `internal/domain/world/mapgen/steps/resources` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/domain/world/mapgen/steps/roads` | 0 | scan, sweep | ruled out | map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; swept ok. |
| `internal/domain/world/mapgen/steps/start` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/domain/world/mapgen/steps/vegetation` | 19 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/domain/world/mapgen/steps/water` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/domain/world/planet` | 0 | scan, sweep | ruled out | map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; swept ok. |
| `internal/domain/world/plant` | 32 | scan, sweep | ruled out | imports math/rand but only seeded rand.New sources (no top-level rand.X( calls anywhere in tests); benign: map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; swept ok. |
| `internal/domain/world/region` | 1 | scan, sweep | ruled out | map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; swept ok. |
| `internal/domain/world/roof` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/domain/world/room` | 16 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/domain/world/site` | 0 | scan, sweep | ruled out | map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; swept ok. |
| `internal/domain/world/stain` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/domain/world/temperature` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/domain/world/terrain` | 5 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/domain/world/vault` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/domain/world/weather` | 4 | scan, sweep | ruled out | map ranges are per-key table assertions (order only changes which failure prints first) or sorted first; swept ok. |
| `internal/platform/assetroot` | 3 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/platform/atomicfile` | 6 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/platform/buildinfo` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/platform/config` | 17 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/platform/input` | 9 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/platform/logfile` | 1 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `internal/platform/openfolder` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/platform/pool` | 0 | scan, lane | fixed, cb5e5abbb | No `NumGoroutine` delta and no "runs first" order: counts goroutines carrying a unique pprof label. Leak mutation fails; count 50, -cpu 1,2 ok. |
| `internal/platform/profiling` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/platform/sysinfo` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/ports/inbound` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/ports/outbound` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/testkit/allocs` | 1 | new (58dc5a4b9), sweep | ruled out | `Fewest` takes the minimum of single calls; its test fails an averaging version; `-race -shuffle=on -count=3` ok. |
| `internal/testkit/deepeq` | 3 | scan, sweep | ruled out | imports math/rand but only seeded rand.New sources (no top-level rand.X( calls anywhere in tests); swept ok. |
| `internal/testkit/fakecontent` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/testkit/fixtures` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/testkit/replaytest` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `internal/testkit/simtest` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `pkg/appicon` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `pkg/audiomanifest` | 3 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `pkg/flatyaml` | 6 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `pkg/mathx` | 2 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `pkg/msgfmt` | 4 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `pkg/musicsynth` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `pkg/noise` | 5 | scan, sweep | ruled out | t.Parallel only, no shared-state hit; swept ok. |
| `pkg/noise2` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `pkg/noise3` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `pkg/randx` | 1 | scan, sweep | ruled out | AllocsPerRun asserts exact 0/small on pure code, not parallel; benign: package vars are read-only tables; swept ok. |
| `pkg/sfxsynth` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
| `pkg/spritenormals` | 0 | scan, sweep | ruled out | No pattern hit; swept (race, shuffle, count 3) ok. |
