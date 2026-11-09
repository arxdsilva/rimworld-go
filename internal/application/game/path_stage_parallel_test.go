package game

import (
	"testing"

	"rimworld-go/internal/application/game/pathstage"
	"rimworld-go/internal/platform/pool"
)

// The path stage on several lanes (3d, GAR-035): its answers, and so every
// tick's state, equal the serial stage's.

// penHerd is a 64 x 64 colony with 4 colonists and 40 animals penned in a
// fenced 10 x 10 field, each sent across it through the others (wild ones
// block one another, so walks jam and queue searches), the stage on p
// (nil: serial).
func penHerd(t *testing.T, p pathstage.Pool) *Game {
	t.Helper()
	g := New(Config{Seed: 11, Width: 64, Height: 64})
	bareGround(g)
	fenceRing(t, g, 20, 20, 31, 31)
	for i := range 4 {
		if _, err := g.SpawnPawn(10+2*i, 40); err != nil {
			t.Fatal(err)
		}
	}
	for i := range 40 {
		x, y := 21+i%10, 21+i/10*2
		id := wildAt(t, g, x, y)
		if i%4 == 3 {
			g.TameAnimal(id, "")
		}
		a, _ := g.animals.Get(id)
		g.animalGoTo(id, &a, 51-x, 51-y) // across the pen, through the herd
		g.animals.Set(id, a)
	}
	g.regs.paths.SetPool(p)
	return g
}

const parallelTicks = 120

// TestParallelEqualsSerial: on 1, 2 and 8 lanes, and 8 with one task
// claimed at a time in a shuffled order, every tick's state hash equals
// the serial run's; the first diverging tick is named. Run it under
// -race -cpu 1,4,8.
func TestParallelEqualsSerial(t *testing.T) {
	t.Parallel()
	ref := penHerd(t, nil)
	want := make([]uint64, parallelTicks)
	for i := range want {
		ref.Step(1)
		want[i] = ref.StateHash()
	}
	runs, n := ref.regs.paths.Stats()
	t.Logf("%d searches in %d stage runs", n, runs)
	if n < 20 || runs < 5 {
		t.Fatalf("the herd queued %d searches (%d runs) in %d ticks: too few to compare", n, runs, parallelTicks)
	}
	for _, c := range []struct {
		name string
		o    pool.Options
	}{
		{"1 lane", pool.Options{Lanes: 1, MinTasks: 1}},
		{"2 lanes", pool.Options{Lanes: 2, MinTasks: 1}},
		{"8 lanes", pool.Options{Lanes: 8, MinTasks: 1}},
		{"8 lanes shuffled", pool.Options{Lanes: 8, MinTasks: 1, Shuffle: 5}},
	} {
		p := pool.New(c.o)
		g := penHerd(t, p)
		for i := range parallelTicks {
			g.Step(1)
			if h := g.StateHash(); h != want[i] {
				p.Close()
				t.Fatalf("%s: tick %d diverged from serial (%#x, want %#x)", c.name, i+1, h, want[i])
			}
		}
		p.Close()
	}
}

// hashPool checks that a Run (the stage's Compute) changes no state.
type hashPool struct {
	*pool.Pool
	g    *Game
	t    *testing.T
	runs int
}

func (h *hashPool) Run(n int, task func(lane, i int)) {
	before := h.g.StateHash()
	h.Pool.Run(n, task)
	if after := h.g.StateHash(); after != before {
		h.t.Fatalf("Compute of %d searches changed the state: %#x -> %#x", n, before, after)
	}
	h.runs++
}

// TestPathStageComputeIsPure: the state hash is the same before and after
// every Compute (searches only read; their answers land in the stage).
func TestPathStageComputeIsPure(t *testing.T) {
	t.Parallel()
	p := pool.New(pool.Options{MinTasks: 1})
	defer p.Close()
	h := &hashPool{Pool: p, t: t}
	h.g = penHerd(t, h)
	h.g.Step(parallelTicks)
	if h.runs < 5 {
		t.Fatalf("%d Computes in %d ticks: too few to check", h.runs, parallelTicks)
	}
}

// TestConfigPoolReachesTheStage: a built colony's path stage runs on the
// Config's pool (compose's wiring).
func TestConfigPoolReachesTheStage(t *testing.T) {
	t.Parallel()
	p := pool.New(pool.Options{})
	defer p.Close()
	b := NewColonyBuilder(Config{Seed: 1, Width: 40, Height: 40, Pool: p})
	if _, _, done := b.Step(1); !done {
		t.Fatal("the legacy colony took more than one step")
	}
	if got := b.Game().regs.paths.Pool(); got != pathstage.Pool(p) {
		t.Fatalf("the stage runs on %v, want the Config's pool", got)
	}
}
