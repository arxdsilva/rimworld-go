// Package benchmark is the engine-scale benchmark tool (NG-1 §6.2): for each
// fixture x map size x speed it loads a grown colony, steps it frame by
// frame and prints p50/p95/p99 per tick, per frame and per Step phase.
//
//	go run ./cmd/benchmark -sizes 120,250,500 -ticks 3000 -speeds 1,3 -cache /tmp/fx
//
// Wall-clock time is observation only (game.PhaseObserver); it never
// feeds the simulation.
package benchmark
