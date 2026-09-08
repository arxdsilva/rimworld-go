---
name: rimworld-go-testing
description: Design and run rimworld-go unit, system, integration, deterministic, fuzz, benchmark, replay, race, and long-run simulation tests.
---

# Testing & Benchmarking

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

Asset/metadata validators (`assettool`, `contentlint`) should run without launching the renderer. Visual QA (screenshot scenes, contact sheets, in-game review) is additional for art tasks per `.cursor/ART_DIRECTION_AND_ASSET_PIPELINE.md` §29. Screen/controller tests should cover view-model behavior without requiring a full GPU path where practical; named screens also need a visual acceptance screenshot/scenario per `.cursor/MAIN_UI_SCREENS_SPECIFICATION.md` §59.

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
