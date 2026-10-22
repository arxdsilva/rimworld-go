package game

import (
	"bytes"
	"reflect"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/terrain"
)

// built places a finished kind at (x, y) and returns its id.
func built(t *testing.T, g *Game, kind, facing string, x, y int) entity.EntityID {
	t.Helper()
	if !g.BuildNow(kind, "wood", facing, x, y) {
		t.Fatalf("could not build %s at %d,%d", kind, x, y)
	}
	return g.bldgIDs[len(g.bldgIDs)-1]
}

// moveCmd issues MoveBuilding through the command gateway.
func moveCmd(g *Game, bid entity.EntityID, f construction.Facing, x, y int) {
	g.Apply(command.Command{Kind: command.MoveBuilding, Target: uint64(bid), X: x, Y: y, Facing: uint8(f)})
	g.ApplyPending()
}

// reinstallFrames lists the reinstall frames for bid.
func reinstallFrames(g *Game, bid entity.EntityID) []entity.EntityID {
	var out []entity.EntityID
	for _, fid := range g.frameIDs {
		if fr, ok := g.frames.Get(fid); ok && fr.Reinstall == uint64(bid) {
			out = append(out, fid)
		}
	}
	return out
}

// finishMove completes bid's pending move at once (no builder).
func finishMove(t *testing.T, g *Game, bid entity.EntityID) {
	t.Helper()
	fids := reinstallFrames(g, bid)
	if len(fids) != 1 {
		t.Fatalf("want one reinstall frame for %d, got %v", bid, fids)
	}
	fr, _ := g.frames.Get(fids[0])
	g.completeFrame(fids[0], fr)
}

// builder is a rested, fed colonist who will take construct work.
func builder(t *testing.T, g *Game, x, y int) entity.EntityID {
	t.Helper()
	id, err := g.SpawnNamedPawn(x, y, "Bob")
	if err != nil {
		t.Fatal(err)
	}
	g.SetNeeds(id, needs.Full, needs.Full)
	return id
}

// runMove steps until bid's move is done (or fails the test).
func runMove(t *testing.T, g *Game, bid entity.EntityID, pawns ...entity.EntityID) {
	t.Helper()
	for i := 0; i < 400; i++ {
		if g.movingTo(bid) == nil {
			return
		}
		for _, p := range pawns {
			g.SetNeeds(p, needs.Full, needs.Full)
		}
		g.Step(25)
	}
	t.Fatalf("move of %d never finished", bid)
}

func TestMoveBedKeepsOwner(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 12})
	bid := built(t, g, "bed", "south", 3, 3)
	owner := builder(t, g, 1, 1)
	b, _ := g.bldgs.Get(bid)
	g.bldgs.Set(bid, b.Claim(uint64(owner)))

	// the builder reinstalls it through the normal construct chain
	moveCmd(g, bid, construction.East, 9, 6)
	if mt := g.movingTo(bid); mt == nil || *mt != [2]int{9, 6} {
		t.Fatalf("movingTo = %v, want 9,6", mt)
	}
	runMove(t, g, bid, owner)

	b, ok := g.bldgs.Get(bid)
	if !ok || b.X != 9 || b.Y != 6 || b.Facing != construction.East {
		t.Fatalf("bed %+v %v, want same id at 9,6 east", b, ok)
	}
	if b.Claimant != uint64(owner) {
		t.Fatalf("owner lost: %+v", b)
	}
	if len(g.frameIDs) != 0 {
		t.Fatalf("frames left: %v", g.frameIDs)
	}
	n := 0
	for _, id := range g.bldgIDs {
		if id == bid {
			n++
		}
	}
	if n != 1 || len(g.bldgIDs) != 1 {
		t.Fatalf("building list %v", g.bldgIDs)
	}
	for _, bv := range g.World().Buildings {
		if bv.ID == bid && bv.MovingTo != nil {
			t.Fatal("finished move still shows MovingTo")
		}
	}
}

func TestMoveBenchKeepsBills(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 12})
	bid := built(t, g, "butcher table", "south", 3, 3)
	if len(g.bills[bid]) == 0 {
		t.Fatal("butcher table should start with bills")
	}
	g.bills[bid][0].Done = 2
	want := append(g.bills[bid][:0:0], g.bills[bid]...)
	moveCmd(g, bid, construction.South, 8, 5)
	if !reflect.DeepEqual(g.bills[bid], want) {
		t.Fatal("bills changed while pending")
	}
	finishMove(t, g, bid)
	if !reflect.DeepEqual(g.bills[bid], want) {
		t.Fatalf("bills %v, want %v", g.bills[bid], want)
	}
	if b, _ := g.bldgs.Get(bid); b.X != 8 || b.Y != 5 {
		t.Fatalf("bench at %d,%d", b.X, b.Y)
	}
}

func TestMoveCampfireKeepsFuel(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 12})
	bid := built(t, g, "campfire", "south", 3, 3)
	g.FuelCampfire(3, 3)
	fuel := g.fuel[bid]
	b, _ := g.bldgs.Get(bid)
	b.Damage = 7
	g.bldgs.Set(bid, b)
	moveCmd(g, bid, construction.South, 10, 4)
	finishMove(t, g, bid)
	b, _ = g.bldgs.Get(bid)
	if g.fuel[bid] != fuel || b.Damage != 7 || b.X != 10 || b.Y != 4 {
		t.Fatalf("fuel %d (want %d), building %+v", g.fuel[bid], fuel, b)
	}
}

func TestMoveOneTileOverlapAllowed(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 12})
	bid := built(t, g, "butcher table", "south", 4, 4) // cells 4,4 and 5,4
	if ok, why := g.CanMoveTo(bid, "south", 5, 4); !ok {
		t.Fatalf("one-tile nudge refused: %s", why)
	}
	moveCmd(g, bid, construction.South, 5, 4)
	finishMove(t, g, bid)
	if !g.grid.Walkable(4, 4) || g.grid.Walkable(5, 4) || g.grid.Walkable(6, 4) {
		t.Fatalf("walkability after nudge: 4=%v 5=%v 6=%v",
			g.grid.Walkable(4, 4), g.grid.Walkable(5, 4), g.grid.Walkable(6, 4))
	}
}

func TestMovedBlockerUpdatesWalkability(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 12})
	bid := built(t, g, "campfire", "south", 3, 3)
	pid := builder(t, g, 10, 4)
	sp, err := g.SpawnPlantSpecies("berry_bush", 10, 4, 0)
	if err != nil {
		t.Fatal(err)
	}
	moveCmd(g, bid, construction.South, 10, 4)
	// a pending reinstall frame blocks nothing yet
	if !g.grid.Walkable(10, 4) || g.grid.Walkable(3, 3) {
		t.Fatal("pending move changed walkability")
	}
	finishMove(t, g, bid)
	if !g.grid.Walkable(3, 3) || g.grid.Walkable(10, 4) {
		t.Fatal("old cell should be free, new cell blocked")
	}
	if pos, _ := g.positions.Get(pid); pos.X == 10 && pos.Y == 4 {
		t.Fatal("pawn not ejected from the new cell")
	}
	if _, ok := g.plants.Get(sp); ok {
		t.Fatal("plant under the new footprint survived")
	}
}

func TestCannotMoveWallOrGrave(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 12})
	wall := built(t, g, "wall", "south", 2, 2)
	grave := built(t, g, "grave", "south", 5, 5)
	chair := built(t, g, "chair", "south", 8, 8)
	for _, bid := range []entity.EntityID{wall, grave, 999} {
		if ok, why := g.CanMoveTo(bid, "south", 10, 2); ok || why != "can't move this" {
			t.Fatalf("%d: ok=%v why=%q", bid, ok, why)
		}
		moveCmd(g, bid, construction.South, 10, 2)
	}
	if len(g.frameIDs) != 0 {
		t.Fatalf("unmovable got frames: %v", g.frameIDs)
	}
	g.markDeconstruct(8, 8, 8, 8, true)
	if ok, why := g.CanMoveTo(chair, "south", 10, 2); ok || why != "can't move this" {
		t.Fatalf("deconstructing chair: ok=%v why=%q", ok, why)
	}
}

func TestMoveBlockedReason(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 12})
	bid := built(t, g, "butcher table", "south", 4, 4)
	built(t, g, "chair", "south", 9, 9)
	g.setTerrain(12, 2, terrain.Rock)
	cases := []struct {
		name   string
		facing string
		x, y   int
		want   string
	}{
		{"off the map", "south", 15, 4, "out of bounds"},
		{"negative", "south", -1, 4, "out of bounds"},
		{"onto another building", "south", 8, 9, "blocked"},
		{"onto rock", "south", 11, 2, "blocked"},
		{"same place", "south", 4, 4, "already there"},
	}
	for _, c := range cases {
		if ok, why := g.CanMoveTo(bid, c.facing, c.x, c.y); ok || why != c.want {
			t.Errorf("%s: ok=%v why=%q want %q", c.name, ok, why, c.want)
		}
	}
	// a planned frame also blocks
	g.placeFrameStuff(construction.Chair, item.Wood, construction.South, 1, 9)
	if ok, why := g.CanMoveTo(bid, "south", 0, 9); ok || why != "blocked" {
		t.Errorf("onto a frame: ok=%v why=%q", ok, why)
	}
	// a rotated move in place is not "already there"
	if ok, why := g.CanMoveTo(bid, "east", 4, 4); !ok {
		t.Errorf("rotate in place refused: %s", why)
	}
	// blocked commands do nothing
	moveCmd(g, bid, construction.South, 15, 4)
	if len(reinstallFrames(g, bid)) != 0 {
		t.Fatal("blocked move made a frame")
	}
}

func TestBuildingUsableWhileMovePending(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 12})
	bid := built(t, g, "bed", "south", 3, 3)
	pid, _ := g.SpawnNamedPawn(5, 5, "Ada")
	// tired (not collapsed: that sleeps where it drops) in hours that allow
	// sleep, so it goes to bed before any building
	sleepAnytime(g)
	g.SetNeeds(pid, needs.Full, needs.CriticalRest-500)
	moveCmd(g, bid, construction.South, 10, 6)
	slept := false
	for i := 0; i < 200 && !slept; i++ {
		g.Step(5)
		j, _ := g.jobs.Get(pid)
		slept = j.Kind == job.Sleep && j.Target == bid && j.State == job.Do
	}
	if !slept {
		t.Fatal("bed with a pending move was not slept in")
	}
	if b, _ := g.bldgs.Get(bid); b.SlotOf(uint64(pid)) < 0 {
		t.Fatal("bed not claimed while move pending")
	}
	// the move completes under the sleeper: they wake up
	finishMove(t, g, bid)
	if j, _ := g.jobs.Get(pid); j.Kind == job.Sleep && j.Target == bid && j.State == job.Do {
		t.Fatal("sleeper not woken by the move")
	}
	if b, _ := g.bldgs.Get(bid); b.SlotOf(uint64(pid)) < 0 || b.X != 10 {
		t.Fatalf("claim lost or not moved: %+v", b)
	}
}

func TestCancelMove(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 12})
	bid := built(t, g, "chair", "south", 3, 3)
	moveCmd(g, bid, construction.South, 9, 6)
	if len(g.World().Frames) != 1 || !g.World().Frames[0].Moving {
		t.Fatal("reinstall frame not shown as moving")
	}
	g.Apply(command.Command{Kind: command.DesignateCancel, X: 9, Y: 6, X2: 9, Y2: 6})
	g.ApplyPending()
	if len(g.frameIDs) != 0 || g.movingTo(bid) != nil {
		t.Fatal("cancel left the move pending")
	}
	b, _ := g.bldgs.Get(bid)
	if b.X != 3 || b.Y != 3 {
		t.Fatal("cancelled move moved the chair")
	}
	// a new move after cancel works
	moveCmd(g, bid, construction.South, 9, 6)
	if mt := g.movingTo(bid); mt == nil {
		t.Fatal("move after cancel refused")
	}
}

func TestMoveDroppedWhenBuildingGone(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 12})
	bid := built(t, g, "chair", "south", 3, 3)
	moveCmd(g, bid, construction.South, 9, 6)
	g.removeBuilding(bid)
	if g.movingTo(bid) != nil {
		t.Fatal("gone building still reports a move")
	}
	pid := builder(t, g, 1, 1)
	for i := 0; i < 100 && len(g.frameIDs) > 0; i++ {
		g.SetNeeds(pid, needs.Full, needs.Full)
		g.Step(25)
	}
	if len(g.frameIDs) != 0 || len(g.bldgIDs) != 0 {
		t.Fatalf("frames %v buildings %v", g.frameIDs, g.bldgIDs)
	}

	// marked for deconstruction before completion: the frame is dropped
	c := built(t, g, "chair", "south", 3, 3)
	moveCmd(g, c, construction.South, 9, 6)
	b, _ := g.bldgs.Get(c)
	b.Deconstruct = true
	g.bldgs.Set(c, b)
	finishMove(t, g, c)
	if len(g.frameIDs) != 0 {
		t.Fatal("frame kept")
	}
	if b, _ := g.bldgs.Get(c); b.X != 3 || b.Y != 3 {
		t.Fatal("deconstructing chair moved")
	}
}

func TestSecondMoveReplacesFirst(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 12})
	bid := built(t, g, "butcher table", "south", 3, 3)
	moveCmd(g, bid, construction.South, 8, 5)
	// the second destination overlaps the first plan: still allowed
	moveCmd(g, bid, construction.South, 9, 5)
	fids := reinstallFrames(g, bid)
	if len(fids) != 1 || len(g.frameIDs) != 1 {
		t.Fatalf("frames %v", g.frameIDs)
	}
	if mt := g.movingTo(bid); mt == nil || *mt != [2]int{9, 5} {
		t.Fatalf("movingTo = %v", mt)
	}
	fr, _ := g.frames.Get(fids[0])
	want := max(60, construction.WorkFor(construction.ButcherTable, fr.Stuff)/2)
	if fr.Need != 0 || fr.Wood != 0 || !fr.Ready() || fr.WorkLeft != want {
		t.Fatalf("frame %+v, work want %d", fr, want)
	}
	for _, bv := range g.World().Buildings {
		if bv.ID == bid && (bv.MovingTo == nil || *bv.MovingTo != [2]int{9, 5}) {
			t.Fatalf("view MovingTo %v", bv.MovingTo)
		}
	}
}

func TestMoveSaveRoundTrip(t *testing.T) {
	g := New(Config{Seed: 3, Width: 16, Height: 12})
	bid := built(t, g, "butcher table", "south", 3, 3)
	builder(t, g, 1, 1)
	moveCmd(g, bid, construction.West, 9, 6)
	var buf bytes.Buffer
	if err := g.Save(&buf, "move"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if mt := h.movingTo(bid); mt == nil || *mt != [2]int{9, 6} {
		t.Fatalf("loaded movingTo %v", mt)
	}
	sameState(t, "after load", g, h)
	for i := 0; i < 8; i++ {
		g.Step(250)
		h.Step(250)
		sameState(t, "running on", g, h)
	}
	if b, _ := h.bldgs.Get(bid); b.X != 9 || b.Y != 6 || b.Facing != construction.West {
		t.Fatalf("loaded game did not finish the move: %+v", b)
	}
}

func TestMoveDeterministic(t *testing.T) {
	run := func() *Game {
		g := NewColony(Config{Seed: 5, Width: 40, Height: 32})
		cx, cy := g.Width()/2, g.Height()/2
		bid := built(t, g, "butcher table", "south", cx-4, cy+4)
		bed := built(t, g, "bed", "south", cx+4, cy+4)
		moveCmd(g, bid, construction.North, cx-4, cy+6)
		moveCmd(g, bed, construction.East, cx+5, cy+6)
		g.Step(3000)
		return g
	}
	a, b := run(), run()
	sameState(t, "two runs", a, b)
}
