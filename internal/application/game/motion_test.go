package game

import (
	"bytes"
	"os"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// corridor is a w-wide, 1-tall walkable strip on row 1 (rows 0 and 2 are
// walled), so the only route between two cells of it is straight.
func corridor(w int) *Game {
	g := New(Config{Seed: 1, Width: w, Height: 3})
	for x := 0; x < w; x++ {
		g.grid.SetWalkable(x, 0, false)
		g.grid.SetWalkable(x, 2, false)
	}
	return g
}

// orderMove drafts a fresh pawn at (x, y) and orders it to (tx, ty).
func orderMove(t *testing.T, g *Game, x, y, tx, ty int) entity.EntityID {
	t.Helper()
	id, err := g.SpawnPawn(x, y)
	if err != nil {
		t.Fatal(err)
	}
	g.Enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(id), Enabled: true})
	g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: uint64(id), X: tx, Y: ty})
	return id
}

// stepTicks runs the game tick by tick and returns the ticks at which
// entity id entered a new cell, with the cells entered.
func stepTicks(g *Game, id entity.EntityID, ticks int) ([]uint64, []Position) {
	var at []uint64
	var cells []Position
	pos, _ := g.positions.Get(id)
	for i := 0; i < ticks; i++ {
		now := uint64(g.Tick())
		g.Step(1)
		p, _ := g.positions.Get(id)
		if p != pos {
			at = append(at, now)
			cells = append(cells, p)
			pos = p
		}
	}
	return at, cells
}

func assertIntervals(t *testing.T, at []uint64, steps, want int) {
	t.Helper()
	if len(at) != steps {
		t.Fatalf("took %d steps (at %v), want %d", len(at), at, steps)
	}
	for i := 1; i < len(at); i++ {
		if d := int(at[i] - at[i-1]); d != want {
			t.Fatalf("step %d took %d ticks, want %d (steps at %v)", i, d, want, at)
		}
	}
}

func TestStraightStepTakes15Ticks(t *testing.T) {
	t.Parallel()
	g := corridor(12)
	id := orderMove(t, g, 1, 1, 7, 1)
	at, cells := stepTicks(g, id, 200)
	assertIntervals(t, at, 6, TicksPerCell)
	if last := cells[len(cells)-1]; last != (Position{7, 1}) {
		t.Fatalf("ended at %+v, want (7,1)", last)
	}
}

func TestDiagonalStepTakes21Ticks(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 8, Height: 8})
	id := orderMove(t, g, 1, 1, 5, 5)
	at, cells := stepTicks(g, id, 200)
	assertIntervals(t, at, 4, TicksPerCellDiag)
	for i, c := range cells {
		if c.X != 2+i || c.Y != 2+i {
			t.Fatalf("step %d entered %+v, want a pure diagonal", i, c)
		}
	}
	// the last diagonal step also reads 21 ticks long
	if v := pawn(t, g, id); v.StepTicks != TicksPerCellDiag {
		t.Fatalf("view StepTicks %d, want %d", v.StepTicks, TicksPerCellDiag)
	}
}

// walkAnimal gives a calm animal a fixed path (no thinking in the way).
func walkAnimal(t *testing.T, g *Game, species string, x, y int, path []pathing.Point) entity.EntityID {
	t.Helper()
	id, err := g.SpawnAnimal(species, x, y)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := g.animals.Get(id)
	a.Path, a.PathI = path, 0
	a.GoalX, a.GoalY = path[len(path)-1].X, path[len(path)-1].Y
	a.NextThink = 1 << 40
	g.animals.Set(id, a)
	return id
}

func TestAnimalDiagonalStepScaled(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 10, Height: 10})
	diag := []pathing.Point{{X: 2, Y: 2}, {X: 3, Y: 3}, {X: 4, Y: 4}, {X: 5, Y: 5}}
	id := walkAnimal(t, g, "dustbuck", 1, 1, diag)
	at, _ := stepTicks(g, id, 300)
	d := animal.DefOf(animal.Dustbuck).StepTicks
	assertIntervals(t, at, 4, (d*1414+500)/1000) // round(18 * 1.414) = 25

	h := New(Config{Seed: 1, Width: 10, Height: 10})
	straight := []pathing.Point{{X: 2, Y: 1}, {X: 3, Y: 1}, {X: 4, Y: 1}}
	sid := walkAnimal(t, h, "dustbuck", 1, 1, straight)
	at, _ = stepTicks(h, sid, 300)
	assertIntervals(t, at, 3, d)
}

func animalView(t *testing.T, g *Game, id entity.EntityID) AnimalView {
	t.Helper()
	for _, a := range g.World().Animals {
		if a.ID == id {
			return a
		}
	}
	t.Fatalf("animal %d not in view", id)
	return AnimalView{}
}

func TestSnapshotExposesStepTiming(t *testing.T) {
	t.Parallel()
	g := corridor(10)
	id := orderMove(t, g, 1, 1, 4, 1)
	// walk until the first step
	var entered uint64
	for i := 0; i < 60; i++ {
		now := uint64(g.Tick())
		g.Step(1)
		if pawn(t, g, id).X == 2 {
			entered = now
			break
		}
	}
	v := pawn(t, g, id)
	if v.X != 2 || v.StepTick != entered || v.StepTicks != TicksPerCell {
		t.Fatalf("mid-walk view %+v, want X=2 StepTick=%d StepTicks=%d", v, entered, TicksPerCell)
	}
	if !v.HasNext || v.NextX != 3 || v.NextY != 1 || v.BlockedBy != 0 {
		t.Fatalf("mid-walk next (%d,%d) has=%v blocked=%d, want (3,1)", v.NextX, v.NextY, v.HasNext, v.BlockedBy)
	}
	g.Step(3 * TicksPerCell)
	v = pawn(t, g, id)
	if v.X != 4 || v.HasNext || v.StepTicks != TicksPerCell || v.StepTick != entered+2*TicksPerCell {
		t.Fatalf("arrived view %+v, want X=4 no next, StepTick=%d", v, entered+2*TicksPerCell)
	}

	// animals expose the same fields
	h := New(Config{Seed: 1, Width: 10, Height: 10})
	aid := walkAnimal(t, h, "ridgewolf", 1, 1, []pathing.Point{{X: 2, Y: 2}, {X: 3, Y: 2}})
	start := uint64(h.Tick())
	h.Step(1)
	av := animalView(t, h, aid)
	if diag := (animal.DefOf(animal.TimberWolf).StepTicks*1414 + 500) / 1000; av.X != 2 || av.Y != 2 || av.StepTick != start || av.StepTicks != diag {
		t.Fatalf("animal view %+v, want a %d-tick diagonal step at tick %d", av, diag, start)
	}
	if !av.HasNext || av.NextX != 3 || av.NextY != 2 {
		t.Fatalf("animal next (%d,%d) has=%v, want (3,2)", av.NextX, av.NextY, av.HasNext)
	}
}

func TestSaveRoundTripKeepsMotion(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 5, Width: 12, Height: 12})
	id := orderMove(t, g, 1, 1, 9, 6)
	walkAnimal(t, g, "dustbuck", 10, 10, []pathing.Point{{X: 9, Y: 9}, {X: 8, Y: 8}, {X: 8, Y: 7}})
	g.Step(TicksPerCellDiag + 3) // mid-step for both
	var buf bytes.Buffer
	if err := g.Save(&buf, "motion"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if v := pawn(t, h, id); v.StepTick == 0 || v.StepTicks == 0 || !v.HasNext {
		t.Fatalf("loaded mid-walk view lost its motion: %+v", v)
	}
	for i := 0; i < 200; i++ {
		sameState(t, "motion round trip", g, h)
		g.Step(1)
		h.Step(1)
	}
}

func TestOldSaveWithoutMotionLoads(t *testing.T) {
	t.Parallel()
	// Written by Save before motion records existed: a drafted pawn walking
	// (1,1) -> (8,1), a second pawn and a dustbuck, at tick 20020.
	data, err := os.ReadFile("testdata/premotion_v1.sav")
	if err != nil {
		t.Fatal(err)
	}
	g, info, err := Load(bytes.NewReader(data))
	if err != nil || info.Version != SaveVersion {
		t.Fatalf("load: %v (info %+v)", err, info)
	}
	walker := g.pawnIDs[0]
	m := g.motionOf(walker)
	if m.NextMoveAt != uint64(g.Tick()) || m.StepTicks != TicksPerCell || m.BlockedTicks != 0 {
		t.Fatalf("defaulted motion %+v, want NextMoveAt=%d StepTicks=%d", m, g.Tick(), TicksPerCell)
	}
	before := pawn(t, g, walker)
	g.Step(1) // due right away
	if after := pawn(t, g, walker); after.X != before.X+1 || after.StepTick != uint64(g.Tick())-1 {
		t.Fatalf("walker did not step on the first tick: %+v -> %+v", before, after)
	}
	g.Step(10 * TicksPerCell)
	if v := pawn(t, g, walker); v.X != 8 || v.Y != 1 {
		t.Fatalf("walker at (%d,%d), want (8,1)", v.X, v.Y)
	}
	if j, _ := g.jobs.Get(walker); j.State == job.Fail {
		t.Fatalf("walk failed: %v", j.Fail)
	}
}

func TestMoversAscendingAcrossKinds(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 2, Width: 30, Height: 30})
	p1, _ := g.SpawnPawn(5, 5)
	a1, _ := g.SpawnAnimal("dustbuck", 20, 20)
	g.Raid(2)
	p2, _ := g.SpawnPawn(6, 5)
	a2, _ := g.SpawnAnimal("ridgewolf", 22, 20)
	got := g.movers()
	want := append([]entity.EntityID{p1, a1}, g.raiderIDs...)
	want = append(want, p2, a2)
	if len(got) != len(want) {
		t.Fatalf("movers %v, want %v", got, want)
	}
	for i := range got {
		if got[i].id != want[i] || got[i].animal != (want[i] == a1 || want[i] == a2) {
			t.Fatalf("movers %v, want %v", got, want)
		}
	}
}

func TestMotionRecordsOfRemovedCreaturesArePruned(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 10, Height: 10})
	id := walkAnimal(t, g, "dustbuck", 1, 1, []pathing.Point{{X: 2, Y: 1}})
	g.Step(1)
	if _, ok := g.motion.Get(id); !ok {
		t.Fatal("a step leaves a motion record")
	}
	g.removeAnimal(id)
	g.Step(1)
	if _, ok := g.motion.Get(id); ok || g.motion.Len() != 0 {
		t.Fatalf("record of a removed animal kept (%d records)", g.motion.Len())
	}
	if out := g.exportMotion(); len(out) != 0 {
		t.Fatalf("exported %v for an empty map", out)
	}
}
