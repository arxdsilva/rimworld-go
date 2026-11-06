package game

import (
	"slices"
	"sync"
	"testing"

	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// barOrderOf is every colonist on the bar, first to last, by the read model.
func barOrderOf(g *Game) []entity.EntityID {
	vs := g.Colonists()
	slices.SortFunc(vs, func(a, b PawnView) int { return a.BarOrder - b.BarOrder })
	out := make([]entity.EntityID, len(vs))
	for i, v := range vs {
		if v.BarOrder != i {
			panic("bar places are not 0..n-1")
		}
		out[i] = v.ID
	}
	return out
}

func moveOnBar(g *Game, id entity.EntityID, place int) {
	g.Enqueue(command.Command{Kind: command.SetColonistOrder, Pawn: uint64(id), Count: place})
	g.Step(1)
}

func orderColony(t *testing.T, names ...string) (*Game, []entity.EntityID) {
	t.Helper()
	g := New(Config{Seed: 11, Width: 24, Height: 24})
	ids := make([]entity.EntityID, len(names))
	for i, n := range names {
		id, err := g.SpawnNamedPawn(2+2*i, 3, n)
		if err != nil {
			t.Fatal(err)
		}
		ids[i] = id
	}
	g.Step(1) // rooms built, as a load builds them
	return g, ids
}

// TestColonistOrderDefault: unused, the bar is the colony's order.
func TestColonistOrderDefault(t *testing.T) {
	t.Parallel()
	g, ids := orderColony(t, "A", "B", "C")
	if got := barOrderOf(g); !slices.Equal(got, ids) {
		t.Fatalf("%v, want %v", got, ids)
	}
	if g.sk.bar.Export() != nil {
		t.Fatal("an order exists before the command")
	}
}

// TestColonistOrderMoves: the command moves the colonist; pawnIDs stays.
func TestColonistOrderMoves(t *testing.T) {
	t.Parallel()
	g, ids := orderColony(t, "A", "B", "C")
	moveOnBar(g, ids[2], 0)
	if got, want := barOrderOf(g), []entity.EntityID{ids[2], ids[0], ids[1]}; !slices.Equal(got, want) {
		t.Fatalf("%v, want %v", got, want)
	}
	if !slices.Equal(g.pawnIDs, ids) {
		t.Fatalf("pawnIDs moved: %v", g.pawnIDs)
	}
	moveOnBar(g, ids[2], 99) // clamped to the end
	if got, want := barOrderOf(g), []entity.EntityID{ids[0], ids[1], ids[2]}; !slices.Equal(got, want) {
		t.Fatalf("clamped: %v, want %v", got, want)
	}
}

// TestColonistOrderCachedSnapshotSees: a paused game's cached views take the
// new order without a tick of the clock.
func TestColonistOrderCachedSnapshotSees(t *testing.T) {
	t.Parallel()
	g, ids := orderColony(t, "A", "B")
	g.SetViewCache(true)
	if g.Snapshot()[1].BarOrder != 1 {
		t.Fatal("default place")
	}
	g.Enqueue(command.Command{Kind: command.SetColonistOrder, Pawn: uint64(ids[1]), Count: 0})
	g.ApplyPending()
	if got := g.Snapshot(); got[1].BarOrder != 0 || got[0].BarOrder != 1 {
		t.Fatalf("stale views: %d, %d", got[0].BarOrder, got[1].BarOrder)
	}
}

// TestColonistOrderRefuses: an unknown id or anyone but a colonist is a no-op.
func TestColonistOrderRefuses(t *testing.T) {
	t.Parallel()
	g, ids := orderColony(t, "A", "B")
	deer, err := g.SpawnAnimal("deer", 12, 12)
	if err != nil {
		t.Fatal(err)
	}
	raider := spawnRaiderAt(t, g, 14, 14, 1)
	before := g.StateHash()
	for _, id := range []entity.EntityID{0, 99999, deer, raider} {
		g.Enqueue(command.Command{Kind: command.SetColonistOrder, Pawn: uint64(id), Count: 0})
	}
	g.ApplyPending()
	if g.sk.bar.Export() != nil {
		t.Fatalf("recorded %v", g.sk.bar.Export())
	}
	if got := barOrderOf(g); !slices.Equal(got, ids) {
		t.Fatalf("%v, want %v", got, ids)
	}
	if g.StateHash() != before {
		t.Fatal("a refused command moved the hash")
	}
}

// TestColonistOrderNewcomerAndGone: a new colonist enters at the end; one
// who stops being a colonist leaves no hole.
func TestColonistOrderNewcomerAndGone(t *testing.T) {
	t.Parallel()
	g, ids := orderColony(t, "A", "B", "C")
	moveOnBar(g, ids[2], 0)
	id, err := g.SpawnNamedPawn(12, 3, "D")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := barOrderOf(g), []entity.EntityID{ids[2], ids[0], ids[1], id}; !slices.Equal(got, want) {
		t.Fatalf("newcomer: %v, want %v", got, want)
	}
	g.pawnIDs = withoutID(g.pawnIDs, ids[0]) // the way a stash or arrest takes one off
	if got, want := barOrderOf(g), []entity.EntityID{ids[2], ids[1], id}; !slices.Equal(got, want) {
		t.Fatalf("gone: %v, want %v", got, want)
	}
	g.pawnIDs = withID(g.pawnIDs, ids[0])
	if got, want := barOrderOf(g), []entity.EntityID{ids[2], ids[0], ids[1], id}; !slices.Equal(got, want) {
		t.Fatalf("back: %v, want %v", got, want)
	}
}

// TestColonistOrderSpansMaps: places count every open map, and the viewed one
// does not matter.
func TestColonistOrderSpansMaps(t *testing.T) {
	t.Parallel()
	g, bo, _, _, _, _ := twoMaps(t)
	ada := g.maps[0].pawnIDs[0]
	if got, want := barOrderOf(g), []entity.EntityID{ada, bo}; !slices.Equal(got, want) {
		t.Fatalf("default: %v, want %v", got, want)
	}
	moveOnBar(g, bo, 0)
	if got, want := barOrderOf(g), []entity.EntityID{bo, ada}; !slices.Equal(got, want) {
		t.Fatalf("moved: %v, want %v", got, want)
	}
	h := saveLoad(t, g)
	if got, want := barOrderOf(h), []entity.EntityID{bo, ada}; !slices.Equal(got, want) {
		t.Fatalf("loaded: %v, want %v", got, want)
	}
}

// TestColonistOrderSaveRoundTrip: the order survives a save and the hash holds
// through it, and an unused order saves nothing.
func TestColonistOrderSaveRoundTrip(t *testing.T) {
	t.Parallel()
	g, ids := orderColony(t, "A", "B", "C")
	if h := saveLoad(t, g); h.sk.bar.Export() != nil || h.StateHash() != g.StateHash() {
		t.Fatalf("an unused order came back as %v", h.sk.bar.Export())
	}
	moveOnBar(g, ids[1], 2)
	want := barOrderOf(g)
	h := saveLoad(t, g)
	if got := barOrderOf(h); !slices.Equal(got, want) {
		t.Fatalf("%v, want %v", got, want)
	}
	if h.StateHash() != g.StateHash() {
		t.Fatal("a save and load moved the hash")
	}
	if !slices.Equal(h.pawnIDs, ids) {
		t.Fatalf("pawnIDs moved: %v", h.pawnIDs)
	}
}

const orderRunTicks = 12000

// orderRun plays a started colony with seed, reordering the bar on the way
// when reorder is set.
func orderRun(t *testing.T, seed uint64, reorder bool) *Game {
	t.Helper()
	g := NewColony(Config{Seed: seed, Width: 60, Height: 60})
	ids := slices.Clone(g.pawnIDs)
	if len(ids) < 2 {
		t.Fatalf("%d colonists", len(ids))
	}
	for step := 0; step < 4; step++ {
		g.Step(orderRunTicks / 4)
		if reorder { // to the front and back, in another order each time
			id := ids[(step*3+1)%len(ids)]
			g.Enqueue(command.Command{Kind: command.SetColonistOrder, Pawn: uint64(id), Count: step * (len(ids) - 1) % len(ids)})
		}
	}
	g.Step(1)
	return g
}

// orderRunOut is what the order tests read from one finished orderRun.
type orderRunOut struct {
	pawnIDs []entity.EntityID
	ordered bool   // the bar holds an order
	hash    uint64 // StateHash at the end
	cleared uint64 // StateHash with the order cleared
}

// orderRunMemo holds orderRun for seeds 1 and 2, with and without the
// reordering, played once for both tests that read them.
var orderRunMemo [2][2]struct {
	once sync.Once
	out  orderRunOut
}

// sharedOrderRun is orderRun(seed, reorder) for seed 1 or 2, played once.
func sharedOrderRun(t *testing.T, seed uint64, reorder bool) orderRunOut {
	t.Helper()
	r := 0
	if reorder {
		r = 1
	}
	m := &orderRunMemo[seed-1][r]
	m.once.Do(func() {
		g := orderRun(t, seed, reorder)
		m.out = orderRunOut{pawnIDs: slices.Clone(g.pawnIDs), ordered: g.sk.bar.Export() != nil, hash: g.StateHash()}
		g.sk.bar.Import(nil)
		m.out.cleared = g.StateHash()
	})
	if m.out.pawnIDs == nil {
		t.Fatalf("orderRun(%d, %v) failed in the test that played it", seed, reorder)
	}
	return m.out
}

// TestColonistOrderOnlyTheView is the rule of the order: it enters the state
// hash (it is saved state, and commands replay deterministically), and nothing
// else moves. With the order cleared, a reordered colony hashes the same as
// one never reordered, tick for tick the same AI, work and RNG.
func TestColonistOrderOnlyTheView(t *testing.T) {
	t.Parallel()
	for _, seed := range []uint64{1, 2} {
		plain, moved := sharedOrderRun(t, seed, false), sharedOrderRun(t, seed, true)
		if !moved.ordered {
			t.Fatal("the run never reordered")
		}
		if plain.ordered {
			t.Fatal("the plain run has an order")
		}
		if !slices.Equal(plain.pawnIDs, moved.pawnIDs) {
			t.Fatalf("pawnIDs: %v, %v", plain.pawnIDs, moved.pawnIDs)
		}
		if plain.hash == moved.hash {
			t.Fatal("the order should enter the hash")
		}
		if a, b := plain.hash, moved.cleared; a != b {
			t.Fatalf("seed %d: the order changed the simulation: %x, %x", seed, a, b)
		}
	}
}

// TestColonistOrderSameSeedSameHash: with and without the command, a seed
// repeats (a second play against the one TestColonistOrderOnlyTheView reads).
func TestColonistOrderSameSeedSameHash(t *testing.T) {
	t.Parallel()
	for _, reorder := range []bool{false, true} {
		if a, b := sharedOrderRun(t, 1, reorder).hash, orderRun(t, 1, reorder).StateHash(); a != b {
			t.Fatalf("reorder %v: %x, %x", reorder, a, b)
		}
	}
}
