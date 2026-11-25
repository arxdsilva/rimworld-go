# internal/adapters/inbound/cli/benchmark

## Purpose
The engine-scale benchmark tool behind `cmd/benchmark` (NG-1 §6.2): loads a grown or raid fixture per map size, colonist count and speed, steps it and prints mean/p50/p95/p99/max per tick, frame, view and Step phase.

## Owns
`options.go` (flags), `run.go` (`Main`, the run loop, `measure`), `report.go` (percentiles and the table).

## Depends on
`application/game`, `application/scenario` (fixtures and `scenario.CacheDir`).

## Contracts
Wall-clock time is observation only (`game.PhaseObserver`); it never feeds the simulation. `run_test.go` pins parsing, the report and a cached-fixture run.

- `-colonists` (default 20, comma list, each >= `scenario.MinColonists` 3; ENG-010 timed 3 to 1,000). Fixtures build once per count and cache with `-c<N>` in the name (20 keeps its old name); table label `<fixture>/<side>/c<N>/x<speed>`.
- After each run an `allocs/tick N objects, B B` line: `runtime.ReadMemStats` once before and once after the timed loop, covering Step and the per-frame `World()` view; observation only.
- A fixture error (count too large for the map, `GrownColony` failure) prints `benchmark: grown colony: ...` and exits 1; a count below 3 is a flag error, also exit 1.

## Tests
`go test -race ./internal/adapters/inbound/cli/benchmark/`; the cached-fixture test writes a small colony under the 120 key so no fixture is built.
