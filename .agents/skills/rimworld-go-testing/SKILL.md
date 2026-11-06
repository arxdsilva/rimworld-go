---
name: rimworld-go-testing
description: Use when writing or running rimworld-go tests: unit, system, deterministic, fuzz, benchmark, replay, race and long-run simulation tests.
---

# Testing & Benchmarking

## Load

- `MASTER_SPEC` §57 Testing Pyramid, §58 Golden and Long-Run Simulation Tests, §59 Simulation Test DSL, §61 CI
- `PARALLEL_WORK` §5 (gate commands)

## Always `-race`

Every `go test` runs with `-race` (`make test`, CI, agents, one-off runs). The answer to a slow suite is cheaper tests, never dropping the flag. Keep the suite inside CI's 10-minute `-race` budget.

CI-speed tips:

- Tests that need real art use the lazy helpers `realArt`/`testArt`/`atlasArt` (`portrait_test.go`) in the ebiten package, never `loadArtCatalog`, unless they test the loader.
- CI wall time is about suite CPU / 4 on slower cores; `-p 2` keeps big test binaries from starving each other. Under `-race`, pprof's `runtime._ExternalCode`/`__tsan_*` is the TSan runtime (CPU, not waiting).
- Replay CI with `docker run --cpuset-cpus 0-3 --cpus 2.2` (`--cpus` alone does not lower nproc). The Docker VM has 8.2 GB, so `signal: killed` can be the container's limit.
- Live heap x GC headroom x race shadow is about 5x RSS. Headless tests never run a game loop, so Ebitengine retains every draw; `ebiten/test_main_test.go` drains it. Check peak RSS as well as time.

## No-flap tests

Every need-, threat- or threshold-driven behaviour ships a no-flap test (root `AGENTS.md`; model: `internal/application/game/sleep_test.go`).

## Pyramid

Use:

- unit tests for pure rules;
- system tests for interacting domain components;
- integration tests across real architectural boundaries;
- replay/golden tests for determinism;
- fuzz tests for parsers/state transitions;
- benchmarks for hot paths;
- long-run simulations for leaks/invariants.

## Test behavior, not implementation trivia

Prefer:

```go
TestReservation_RejectsExclusiveConflict
```

over tests tied to private helper call counts.

## Headless first

Domain/application tests must not require renderer initialization.

Asset/metadata validators (`assettool`, `contentlint`) should run without launching the renderer. Visual QA (screenshot scenes, contact sheets, in-game review) is additional for art tasks per `.agents/specs/ART_DIRECTION_AND_ASSET_PIPELINE.md` §29. Screen/controller tests should cover view-model behavior without requiring a full GPU path where practical; named screens also need a visual acceptance screenshot/scenario per `.agents/specs/MAIN_UI_SCREENS_SPECIFICATION.md` §59.

## Sim test DSL

Use/extend `internal/testkit/simtest` so scenarios can be expressed compactly and consistently.

A test fixture should make seed/tick/entity setup explicit.

## Determinism suite

Include fixed seeds and state hashes.

Run scenarios repeatedly.

For concurrent compute paths, vary worker counts/`GOMAXPROCS`.

## Fuzz targets

High-value candidates:

- save decoder/migrations;
- content parser;
- item quantity transfers;
- pathing invariants;
- commands;
- geometry;
- reservation transitions.

## Benchmarks

Every key hot system should have representative scale benchmarks.

Call `b.ReportAllocs()`.

Separate setup from timed work.

## Long-run checks

Nightly/periodic scenarios should detect:

- state-hash divergence;
- orphan reservations;
- invalid entity references;
- negative stacks;
- runaway heap growth;
- deadlocks/races;
- impossible state transitions.

## Regression rule

For a bug:

1. reproduce;
2. add failing test or replay fixture;
3. fix;
4. verify test;
5. retain regression artifact.
