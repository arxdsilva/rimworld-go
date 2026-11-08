# internal/platform/profiling

## Purpose
The developer profiles of `cmd/game` (`-cpuprofile`, `-trace`, `-memprofile`; ENG-A), moved out of `cmd/game` (2026-11, GAR). Observation only.

## Owns
`Start(cpuPath, tracePath)` returns the stop func; `WriteHeap(path)` writes a heap profile after a GC.

## Contracts
An empty path writes nothing; a file that can't be made is logged, never fatal (`TestProfilesAreWrittenWhereAsked`).

## Tests
The test leaves the CPU profile out: only one CPU profile may run per process, and `go test -cpuprofile` may hold it.
