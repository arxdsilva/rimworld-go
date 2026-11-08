# internal/adapters/inbound/cli/benchmark

## Purpose
The engine-scale benchmark tool behind `cmd/benchmark` (NG-1 §6.2): loads a grown or raid fixture per map size and speed, steps it and prints mean/p50/p95/p99/max per tick, frame, view and Step phase.

## Owns
`options.go` (flags), `run.go` (`Main`, the run loop, `measure`), `report.go` (percentiles and the table).

## Depends on
`application/game`, `application/scenario` (fixtures and `scenario.CacheDir`).

## Contracts
Wall-clock time is observation only (`game.PhaseObserver`); it never feeds the simulation. `run_test.go` pins parsing, the report and a cached-fixture run.

## Tests
`go test -race ./internal/adapters/inbound/cli/benchmark/`; the cached-fixture test writes a small colony under the 120 key so no fixture is built.
