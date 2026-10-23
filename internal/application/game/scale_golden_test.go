package game

import (
	"fmt"
	"testing"
	"time"

	"rimworld-go/internal/domain/sim/entity"
)

// Golden state hashes (NG-1 §6.2): the complete state after 20 000 ticks
// of a generated colony, pinned before the engine-scale work (E1) so every
// behaviour-preserving optimisation is proven bit-identical. An intended
// behaviour change re-pins these, with the reason in the handoff.
//
// Re-pin: go test -run TestScaleGolden -v ./internal/application/game/
// and copy the "got" values.
//
// Re-pinned: save format (research slice), behaviour unchanged. StateHash
// became format-independent (normalised fields) at the same time; these
// values are identical when computed on the contract commit (fbc76fd6)
// and on the new save format.
var scaleGolden = map[string]uint64{
	"48x40/seed1":    0xde829987be0450d,
	"48x40/seed7":    0x33b15f6cbd436f80,
	"48x40/seed42":   0x1b69004746143ca3,
	"120x120/seed1":  0x5fce5c24ecd8eec0,
	"120x120/seed7":  0xbfb1360ca07b408b,
	"120x120/seed42": 0x3bfa12d561682348,
}

const scaleGoldenTicks = 20000

func TestScaleGolden(t *testing.T) {
	if testing.Short() || raceOn {
		t.Skip("long golden run (runs in the determinism gate without -race)")
	}
	for _, size := range [][2]int{{48, 40}, {120, 120}} {
		for _, seed := range []uint64{1, 7, 42} {
			name := fmt.Sprintf("%dx%d/seed%d", size[0], size[1], seed)
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				g := NewColony(Config{Seed: seed, Width: size[0], Height: size[1]})
				g.Step(scaleGoldenTicks)
				got := g.StateHash()
				t.Logf("%q: %#x,", name, got)
				if want := scaleGolden[name]; got != want {
					t.Fatalf("state hash = %#x, want %#x", got, want)
				}
			})
		}
	}
}

func TestStateHashStable(t *testing.T) {
	g := NewColony(Config{Seed: 3, Width: 48, Height: 40})
	g.Step(300)
	a, b := g.StateHash(), g.StateHash()
	if a != b {
		t.Fatalf("StateHash not stable: %x vs %x", a, b)
	}
	g.Step(1)
	if g.StateHash() == a {
		t.Fatal("StateHash unchanged after a tick")
	}
}

// TestPhaseObserverDoesNotChangeState: an observed Step is the same Step.
func TestPhaseObserverDoesNotChangeState(t *testing.T) {
	a := NewColony(Config{Seed: 5, Width: 48, Height: 40})
	b := NewColony(Config{Seed: 5, Width: 48, Height: 40})
	seen := map[string]int{}
	b.SetPhaseObserver(func(p string, _ time.Duration) { seen[p]++ })
	a.Step(500)
	b.Step(500)
	if a.StateHash() != b.StateHash() {
		t.Fatal("observed run diverged")
	}
	for _, p := range Phases() {
		if seen[p] != 500 {
			t.Fatalf("phase %s observed %d times, want 500", p, seen[p])
		}
	}
	b.SetPhaseObserver(nil)
	a.Step(10)
	b.Step(10)
	if a.StateHash() != b.StateHash() {
		t.Fatal("diverged after removing the observer")
	}
}

// Test-only hooks for the external scale benchmarks (scale_bench_test.go,
// package game_test, which needs the scenario fixture and so cannot be
// an internal test).

// BenchColonist is the first colonist's id.
func (g *Game) BenchColonist() entity.EntityID { return g.pawnIDs[0] }

// BenchReachable is reachable(id, x, y).
func (g *Game) BenchReachable(id entity.EntityID, x, y int) bool { return g.reachable(id, x, y) }

// BenchPath is the length of pawn id's path to (x, y), or -1.
func (g *Game) BenchPath(id entity.EntityID, x, y int) int {
	p, ok := g.pathTo(id, x, y)
	if !ok {
		return -1
	}
	return len(p)
}

// BenchRebuildRooms rebuilds rooms as a wall change does and returns the
// room count.
func (g *Game) BenchRebuildRooms() int {
	g.roomDirty = true
	g.ensureRooms()
	return len(g.rooms)
}

// Every test in this package runs with the per-cell indexes checked
// against the original scans (indexes.go); the external benchmarks turn
// it off.
// Under -race (about ten times slower) one lookup in 32 is checked.
func init() {
	indexSelfCheck = true
	if raceOn {
		selfCheckEvery = 32
	}
}

// SetIndexSelfCheck switches the index self-check (benchmarks: off).
func SetIndexSelfCheck(on bool) { indexSelfCheck = on }
