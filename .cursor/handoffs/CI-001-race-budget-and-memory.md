# CI-001: the `-race` suite within CI's time and memory

`test` and `release` failed on `main` after about 14 minutes (`FAIL`, exit 1). The failing line was never seen: job logs need auth, and the pasted tail doesn't show it. The two causes below are what reproduces in a throttled Linux replay, so confirm on the next real run.

## What was wrong
- **Time.** `application/game` ran 1042 tests serially (346 s at 4 cores), and CI's wall time is about suite CPU / 4 on slower cores. `game` and the ebiten root starved each other towards the 10-minute per-binary timeout.
- **Memory.** The ebiten root binary peaked at 15.09 GB RSS on a 16 GB runner. No test runs a game loop, so Ebitengine's atlas queue kept every draw and pixel copy forever, and its text cache (evicted on the loop's tick) never shrank.

## What changed
- `ebiten/test_main_test.go`: `TestMain` empties the atlas queue once a second through `go:linkname` (test binary only). `TestAtlasQueueDrains` fails if an upgrade renames those variables. The draws still run in full, including Ebitengine's checks.
- `gfx.ForgetFonts`, called every 256 `testApp`s: by count, because an allocation test must not meet an empty cache mid-loop.
- `t.Parallel()` on 1035 `game` tests. Skipped: tests measuring allocations, wall time or `SetIndexSelfCheck`.
- `TestPowerPaneAndGridAtUIScales`, `TestZonePaneAndGridAtUIScales`: one `chromeRects` per layout (`chromeHitIn`), and the full `colonyChromeHit` path once per region.
- `TestJourneyWorldAttackHostileBase`, `TestJourneyWorldDemandLetterAnswers`: `game.DevArrive` and `formCaravan` instead of walking. `TestJourneyWorldTravelAndReturn` still covers real travel.
- Both workflows: `go test -p 2`. `-race`, the 10-minute timeout and the assertions are unchanged.

## Measurements
Replay: `docker run --cpuset-cpus 0-3 --cpus 2.2 --memory 7500m`, CI's command. Calibrated on the pipeline package; `--cpus` alone doesn't lower nproc.

| | before | after |
|---|---|---|
| ebiten root peak RSS (macOS, GOMAXPROCS=4) | 15.09 GB | 3.65 GB |
| whole replay, cgroup peak | killed at the 7.5 GB limit | 4.2 GB |
| `game` in the replay | 527 s (root dead, so uncontended) | 404 s |
| ebiten root in the replay | killed at 230 s | 379 s |
| replay total | 10m36, FAIL | 11m13, PASS |

## Gaps
- `game` still uses about 570 s of CPU. At `-p 4` it took 567 s in the replay, so `-p 2` is what buys the margin. Next candidates, if it tightens: `TestViewCacheMatchesFresh` (48 s), `TestPopulationStableTenDays`. They were left alone because cutting them would reduce coverage.
- A cold build at `-p 2` wasn't timed (the replay's cache was warm); `test.yml`'s job limit is 25 minutes.
- No HUD change: this is test and CI work only.
