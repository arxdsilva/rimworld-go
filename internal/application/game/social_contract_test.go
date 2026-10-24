package game

import (
	"bytes"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// refineSetup is a sandbox doctor beside a workbench with a Refine
// industrial medicine bill and herbal medicine to hand; only work type t
// is on.
func refineSetup(t *testing.T, on work.Type) (*Game, entity.EntityID) {
	t.Helper()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	id, _ := g.SpawnNamedPawn(1, 1, "Ada")
	for wt := work.Type(0); wt < work.NumTypes; wt++ {
		g.setPriority(id, wt, 0)
	}
	g.setPriority(id, on, 1)
	placeBuilt(t, g, construction.Workbench, construction.South, 5, 2)
	g.SpawnStack(item.HerbalMedicine, 4, 1, 5)
	g.Enqueue(command.Command{Kind: command.AddBill, X: 5, Y: 2, Recipe: uint8(recipe.RefineMedicine), Mode: uint8(recipe.DoCount), Count: 1})
	g.Step(1)
	return g, id
}

func industrialMedicine(g *Game) int {
	n := 0
	for _, it := range g.World().Items {
		if it.Kind == item.IndustrialMedicine.String() {
			n += it.Qty
		}
	}
	return n
}

// TestDoctorRefinesMedicine (ENG-003 doctor bill fix): Refine industrial
// medicine is a Doctor bill, so a doctor with nothing to tend works it at
// the workbench; a crafter never does.
func TestDoctorRefinesMedicine(t *testing.T) {
	g, id := refineSetup(t, work.Doctor)
	for range 40 {
		g.SetNeeds(id, needs.Full, needs.Full)
		g.Step(TicksPerCell * 10)
	}
	if n := industrialMedicine(g); n != 1 {
		t.Fatalf("doctor refined %d industrial medicine, want 1", n)
	}
	g, id = refineSetup(t, work.Craft)
	for range 40 {
		g.SetNeeds(id, needs.Full, needs.Full)
		g.Step(TicksPerCell * 10)
		if j, _ := g.jobs.Get(id); j.Kind == job.Bill {
			t.Fatal("a crafter took the doctor's bill")
		}
	}
	if n := industrialMedicine(g); n != 0 {
		t.Fatalf("crafter refined %d industrial medicine", n)
	}
}

// TestWardenOffByDefaultAndMigrated (ENG-003): new colonists have Warden
// off, priorities round-trip as slices, and a save written with 12 work
// types (Prios) loads with Warden off and the rest kept.
func TestWardenOffByDefaultAndMigrated(t *testing.T) {
	g := New(Config{Seed: 4, Width: 10, Height: 10})
	id, _ := g.SpawnNamedPawn(2, 2, "Ada")
	if p := g.prioOf(id); p[work.Warden] != 0 || p[work.Haul] != work.DefaultPriority {
		t.Fatalf("priorities = %v", p)
	}
	g.setPriority(id, work.Warden, 2)
	var buf bytes.Buffer
	if err := g.Save(&buf, "t"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if p := h.prioOf(id); p[work.Warden] != 2 || p[work.Haul] != work.DefaultPriority {
		t.Fatalf("round trip: %v", p)
	}
	// A 12-type save: Prios filled, PrioList absent.
	var old priorities12
	old[work.Cook], old[work.Handle] = 1, 4
	h.prio = map[entity.EntityID]work.Priorities{}
	h.migrateWarden(map[entity.EntityID]priorities12{id: old})
	if p := h.prio[id]; p[work.Cook] != 1 || p[work.Handle] != 4 || p[work.Warden] != 0 {
		t.Fatalf("migrated = %v", p)
	}
	if got := importPrios(map[entity.EntityID][]uint8{id: {0, 2}}); got[id][work.Doctor] != 2 || got[id][work.Warden] != 0 {
		t.Fatalf("short list = %v", got[id])
	}
}

// TestRNGStreamsSavedAndLazy (ENG-003 C0): a stream is created on first use
// from the base seed, survives a save, and continues exactly; using it
// never touches the main RNG.
func TestRNGStreamsSavedAndLazy(t *testing.T) {
	g := New(Config{Seed: 9, Width: 10, Height: 10})
	main := g.StateHash()
	a := g.stream(streamSocial)
	first := a.IntN(1 << 30)
	if g.stream(streamSocial) != a {
		t.Fatal("stream not reused")
	}
	var buf bytes.Buffer
	if err := g.Save(&buf, "t"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if x, y := g.stream(streamSocial).IntN(1<<30), h.stream(streamSocial).IntN(1<<30); x != y {
		t.Fatalf("social stream diverged after load: %d vs %d", x, y)
	}
	if x, y := g.stream(streamBond).IntN(1<<30), h.stream(streamBond).IntN(1<<30); x != y {
		t.Fatalf("a stream first used after load diverged: %d vs %d", x, y)
	}
	if fresh := New(Config{Seed: 9, Width: 10, Height: 10}).stream(streamSocial).IntN(1 << 30); fresh != first {
		t.Fatal("stream not seeded from the base seed")
	}
	if New(Config{Seed: 9, Width: 10, Height: 10}).StateHash() != main {
		t.Fatal("unused streams changed the state hash")
	}
}

// TestSocialStubsAreNoOps (ENG-003 C3): the contract's hooks, read models
// and save fields change nothing yet — a colony steps, saves and loads to
// the same state, and the read models are empty.
func TestSocialStubsAreNoOps(t *testing.T) {
	g := NewColony(Config{Seed: 3, Width: 48, Height: 40})
	g.Step(600)
	id := g.pawnIDs[0]
	if _, ok := g.Social(id); ok {
		t.Fatal("social view before M4")
	}
	if _, ok := g.Health(id); ok {
		t.Fatal("health view before M1")
	}
	if !g.inArea(id, 0, 0) {
		t.Fatal("the area stub restricts")
	}
	if _, ok := g.Prisoner(id); ok || len(g.Areas()) != 0 || len(g.History()) != 0 || len(g.WorldMap().Caravans) != 0 {
		t.Fatal("non-empty stub read model")
	}
	var buf bytes.Buffer
	if err := g.Save(&buf, "t"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if h.StateHash() != g.StateHash() {
		t.Fatal("save round trip changed the state")
	}
	g.Step(300)
	h.Step(300)
	if h.StateHash() != g.StateHash() {
		t.Fatal("loaded game diverged")
	}
}
