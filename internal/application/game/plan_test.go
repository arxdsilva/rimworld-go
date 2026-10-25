package game

import (
	"bytes"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
)

func planCount(w WorldView) int {
	n := 0
	for _, v := range w.Plan {
		n += int(v)
	}
	return n
}

func planAt(g *Game, x, y int) bool {
	w := g.World()
	return len(w.Plan) > 0 && w.Plan[y*w.TerrainW+x] == 1
}

func planCmd(g *Game, x, y, x2, y2 int, on bool) {
	g.Enqueue(command.Command{Kind: command.DesignatePlan, X: x, Y: y, X2: x2, Y2: y2, Enabled: on})
	g.Step(1)
}

func TestPlanPaintErase(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	w0 := g.World()
	if len(w0.Plan) != 0 || w0.PlanVer != 0 {
		t.Fatalf("fresh map has plan: %d tiles, ver %d", len(w0.Plan), w0.PlanVer)
	}
	planCmd(g, 5, 4, 2, 2, true) // reversed corners are normalised
	w1 := g.World()
	if planCount(w1) != 12 || w1.PlanVer == w0.PlanVer || !planAt(g, 3, 3) {
		t.Fatalf("painted %d tiles, ver %d -> %d", planCount(w1), w0.PlanVer, w1.PlanVer)
	}
	planCmd(g, 3, 3, 3, 4, false)
	w2 := g.World()
	if planCount(w2) != 10 || w2.PlanVer == w1.PlanVer || planAt(g, 3, 3) {
		t.Fatalf("erased to %d tiles", planCount(w2))
	}
}

func TestPlanOvalRowRuns(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	// an oval arrives as one command per row run
	for _, r := range [][3]int{{0, 5, 7}, {1, 4, 8}, {2, 5, 7}} {
		planCmd(g, r[1], r[0]+3, r[2], r[0]+3, true)
	}
	w := g.World()
	if planCount(w) != 3+5+3 || planAt(g, 4, 3) || !planAt(g, 4, 4) || !planAt(g, 6, 3) {
		t.Fatalf("oval painted %d tiles", planCount(w))
	}
}

func TestPlanClampsAtEdges(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	g.paintPlan(-5, -5, 1, 1, true)
	if planCount(g.World()) != 4 {
		t.Fatalf("clamped paint gave %d tiles", planCount(g.World()))
	}
	v := g.World().PlanVer
	g.paintPlan(100, 100, 120, 120, true)
	g.paintPlan(-9, -9, -3, -3, true)
	if g.World().PlanVer != v || planCount(g.World()) != 4 {
		t.Fatal("off-map paint changed the plan")
	}
	w, h := g.grid.Width(), g.grid.Height()
	g.paintPlan(w-2, h-2, w+4, h+4, true)
	if planCount(g.World()) != 8 || !planAt(g, w-1, h-1) {
		t.Fatalf("far-edge paint gave %d tiles", planCount(g.World()))
	}
}

func TestPlanVersionOnlyOnRealChange(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	g.paintPlan(0, 0, 5, 5, false) // erasing nothing
	if g.World().PlanVer != 0 {
		t.Fatal("erasing nothing bumped the version")
	}
	g.paintPlan(2, 2, 5, 4, true)
	v := g.World().PlanVer
	g.paintPlan(2, 2, 5, 4, true)
	g.paintPlan(10, 10, 12, 12, false)
	if g.World().PlanVer != v {
		t.Fatal("a no-op bumped the version")
	}
	g.paintPlan(2, 2, 6, 4, true) // one new column
	if g.World().PlanVer == v {
		t.Fatal("a real change kept the version")
	}
}

func TestPlanViewAliasesUntilChanged(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	g.paintPlan(2, 2, 5, 4, true)
	a, b := g.World().Plan, g.World().Plan
	if &a[0] != &b[0] {
		t.Fatal("an unchanged plan was copied")
	}
	g.paintPlan(0, 0, 0, 0, true)
	c := g.World().Plan
	if &a[0] == &c[0] || a[0] != 0 || c[0] != 1 {
		t.Fatal("a change mutated the shared view")
	}
}

func TestPlanClearedByFinishedWallAndBed(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	g.paintPlan(2, 2, 12, 12, true)
	before := planCount(g.World())
	placeBuilt(t, g, construction.Wall, construction.South, 4, 4)
	if planAt(g, 4, 4) || planCount(g.World()) != before-1 {
		t.Fatal("a finished wall kept its plan mark")
	}
	placeBuilt(t, g, construction.DoubleBed, construction.South, 8, 8)
	cells, _, _ := construction.Footprint(construction.DoubleBed, construction.South, 8, 8)
	for _, c := range cells {
		if planAt(g, c[0], c[1]) {
			t.Fatalf("bed footprint %v kept its mark", c)
		}
	}
	if planCount(g.World()) != before-1-len(cells) {
		t.Fatalf("marks outside the footprints changed: %d", planCount(g.World()))
	}
}

func TestPlanKeptUnderFloor(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	g.paintPlan(6, 6, 6, 6, true)
	v := g.World().PlanVer
	placeBuilt(t, g, construction.Floor, construction.South, 6, 6)
	if g.layerBldgAt(6, 6, construction.LayerFloor) == 0 {
		t.Fatal("the floor was not built")
	}
	if !planAt(g, 6, 6) || g.World().PlanVer != v {
		t.Fatal("a finished floor cleared a plan mark")
	}
}

func TestPlanKeptThroughSwap(t *testing.T) {
	t.Parallel()
	g := swapGame(t)
	if !g.BuildNow("wall", "wood", "south", 5, 5) {
		t.Fatal("no wall")
	}
	g.paintPlan(5, 5, 5, 5, true) // marked again over the standing wall
	v := g.World().PlanVer
	g.Enqueue(commandBuild(construction.Wall, item.Steel, 5, 5))
	g.Step(1)
	fid, _ := swapFrameOn(t, g, 5, 5, construction.LayerObject)
	finishFrame(g, fid)
	if !planAt(g, 5, 5) || g.World().PlanVer != v {
		t.Fatal("a material swap changed the plan")
	}
}

func TestPlanClearedByMove(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 16, Height: 12})
	bid := built(t, g, "bed", "south", 3, 3)
	g.paintPlan(8, 5, 10, 7, true)
	moveCmd(g, bid, construction.South, 9, 6)
	finishMove(t, g, bid)
	if planAt(g, 9, 6) {
		t.Fatal("a moved building kept the mark under its new spot")
	}
	if !planAt(g, 8, 5) {
		t.Fatal("a move cleared marks outside the new footprint")
	}
}

func TestPlanDoesNotBlockPlacementOrPathing(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	g.paintPlan(4, 4, 8, 8, true)
	if !g.grid.Walkable(6, 6) {
		t.Fatal("a plan mark blocks walking")
	}
	if ok, _ := g.canPlaceAt(construction.Wall, construction.South, 6, 6); !ok {
		t.Fatal("a plan mark blocks placement")
	}
	g.paintHome(4, 4, 8, 8, true) // zones and Home paint over marks freely
	if !g.inHome(tileIdx(g, 6, 6)) || !planAt(g, 6, 6) {
		t.Fatal("Home and plan interfere")
	}
}

func TestPlanSaveRoundTrip(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	var empty bytes.Buffer
	if err := g.Save(&empty, "t"); err != nil {
		t.Fatal(err)
	}
	if g.exportPlan() != nil {
		t.Fatal("an empty plan must export nil")
	}
	g.paintPlan(2, 2, 5, 3, true)
	g.paintPlan(9, 9, 9, 9, true)
	var buf bytes.Buffer
	if err := g.Save(&buf, "t"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	a, b := g.World(), h.World()
	if planCount(b) != planCount(a) || !bytes.Equal(a.Plan, b.Plan) || b.PlanVer == 0 {
		t.Fatalf("round trip: %d tiles -> %d", planCount(a), planCount(b))
	}
	// an old save (no Plan) loads with no marks
	l, _, err := Load(&empty)
	if err != nil {
		t.Fatal(err)
	}
	if w := l.World(); len(w.Plan) != 0 || w.PlanVer != 0 {
		t.Fatalf("empty save loaded %d marks", planCount(w))
	}
	l.importPlan([]int32{-1, 1 << 30}) // out-of-range indexes are ignored
	if planCount(l.World()) != 0 {
		t.Fatal("bad indexes were imported")
	}
}
