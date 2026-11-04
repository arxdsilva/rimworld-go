# CI-002: the tint shader's first draw raced

`test` and `release` failed on c1b25230f (v0.1.24), exit 1, so v0.1.24 published no archives. Job logs need auth, so the failing line was never seen.

## What the replay showed

CI-001's replay (`docker run --cpuset-cpus 0-3 --cpus 2.2 --memory 7500m`, CI's command) reported `WARNING: DATA RACE` in `art`: `TestTintBakesEvictLeastRecentlyUsed`, `TestPrewarmFilesRacingDraws`, `TestPrewarmAsyncMatchesPrewarm`. Ebitengine sets `Shader.uniformDwordCount` on a shader's first draw, without a lock. `art.TintShader` is the one shader shared by the whole process, so two parallel tests' first bakes can overlap. It has been possible since the art tests went parallel (2026-10-28) and depends on timing.

The game itself bakes on the frame goroutine only, so players never hit this.

## Fix

`newTintShader` draws one pixel before anyone else can reach the shader. `TestPrimedTintShaderDrawsFromAnyGoroutine` fails with `DATA RACE` without that draw and passes 20 of 20 with it.

## Not the cause, measured

The same replay also killed the ebiten root and `mapgen/steps/features` (`signal: killed`). Alone, in the same container: root 2.0 GB, `game` 0.6 GB, `features` 2.1 GB, all passing. `features` has no diff against v0.1.23. The Docker VM has 7.65 GB and also runs other containers, so this replay can't tell us about CI's 16 GB runner.

## Gap

Without the log, the race is the only failure the replay could confirm, not a proven match for CI's. The next CI run on `main` is the proof. No HUD change: test and CI work only.
