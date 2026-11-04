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

## Gap

Without the job log, neither cause is read directly from CI: the green run after both fixes is the evidence. No HUD change: test and CI work only.
