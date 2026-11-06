# CI-002: the tint shader's first draw raced

`test` and `release` failed on c1b25230f (v0.1.24), exit 1, so v0.1.24 published no archives. Job logs need auth, so the failing line was never seen.

## What the replay showed

CI-001's replay (`docker run --cpuset-cpus 0-3 --cpus 2.2 --memory 7500m`, CI's command) reported `WARNING: DATA RACE` in `art`: `TestTintBakesEvictLeastRecentlyUsed`, `TestPrewarmFilesRacingDraws`, `TestPrewarmAsyncMatchesPrewarm`. Ebitengine sets `Shader.uniformDwordCount` on a shader's first draw, without a lock. `art.TintShader` is the one shader shared by the whole process, so two parallel tests' first bakes can overlap. It has been possible since the art tests went parallel (2026-10-28) and depends on timing.

The game itself bakes on the frame goroutine only, so players never hit this.

## Fix

`newTintShader` draws one pixel before anyone else can reach the shader. `TestPrimedTintShaderDrawsFromAnyGoroutine` fails with `DATA RACE` without that draw and passes 20 of 20 with it.

## Second failure, mine

604628c61 failed CI too: `TestUIDrawsThroughCanvas` rejected `drawOneTinted`, a raw draw outside `ui_canvas.go`. It was pushed after running only `art`, without the root package. The guard now exempts it, as it does `bakeTint`. The failing line in the job log fell from 1905 to 249, which fits the race output being gone.

Sharded on Linux (16 runs, the VM can't hold the whole binary), all 1722 root tests passed apart from that guard.

## Not the cause, measured

The same replay also killed the ebiten root and `mapgen/steps/features` (`signal: killed`). The VM has 7.65 GB, about 2 GB of it free with its other containers running, and its swap was full, so the kills came from the VM (`memory.max` events 0, `oom_kill` 6). The root binary was killed there even alone, at 2.0 GB. On macOS it peaks at 3.4 GB, below CI-001's 3.65 GB. `features` has no diff against v0.1.23. This replay can't tell us about CI's 16 GB runner.

## Third failure, the last: a hash pinned on arm64

2656774a2 still failed, cause unknown from outside. `scripts/ci_test.sh` now runs the test step in both workflows and publishes the failure and the slowest binaries as annotations, which need no login. They showed `TestHabitsOffKeepTheHash`, seed 1: 0xee2b9d9d5399d8ac, want 0xecf8cbfa3ad64226. Races: 0.

`GOARCH=amd64 go test` under Rosetta reproduces CI's value. The states first differ at tick 25001, in `Weather`: `Goal` 0.04493452512603583 on arm64, 0.044934525126035824 on amd64. arm64 fuses `x*y+z` into one instruction that rounds differently (the Go spec allows it; an explicit conversion stops it).

The test's promise held all along: f7631b72a, the commit the pin came from, gives 0xee2b9d9d5399d8ac on amd64 and 0xecf8cbfa3ad64226 on arm64. With the habits off nothing moved on either.

## Fix

Every fused sum in per-tick code is now converted: weather, sun, snow, farm, light, difficulty, fire, stain, outdoor light. `go tool objdump` on the arm64 binary lists none left under `internal/application` nor in those domain packages. amd64 results don't change, so the only pin to move is this one, to the amd64 value. `TestWeatherIsTheSameOnEveryArchitecture` pins 240 blocks of weather by their bits. The rule and the objdump command are in GO_BEST_PRACTICES §25.

Measured after the fix: 16 seeds at 48x40 (60 000 ticks) and 120x120 (20 000 ticks), generation and final hash: all 64 equal on arm64 and amd64.

## Still differs between architectures

- `WindDir` goes through `math.Sin`, and the standard library compiles it with fused operations on arm64. Over 240 blocks its last bits differ, and it is saved, so a long run pinned on a Mac can still miss on CI. Fixing it means replacing the wobble or rounding it, which moves every pin.
- 116 fused lines remain in one-shot generation: `mapgen`, `generation`, `planet`, `pkg/noise*`. No map differed in the sample above, which used the default biome only, so a seed could still give a slightly different map on a Mac than on Windows. Nothing depends on that today.

## Gap

No test enforces the absence of fused sums other than the weather pin and the golden hashes running on two architectures. No HUD change: test and CI work only.
