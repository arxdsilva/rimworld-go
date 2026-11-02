package game

import (
	"testing"

	"rimworld-go/internal/application/game/resize"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/roof"
)

// ax2Game is an open researched colony map for the AX-2 rules.
func ax2Game(t *testing.T) *Game {
	t.Helper()
	g := New(Config{Seed: 3, Width: 24, Height: 24})
	g.CompleteResearch()
	return g
}

// ax2Pen walls in (x0..x1, y0..y1) with piece k standing in the gap at gap.
func ax2Pen(t *testing.T, g *Game, x0, y0, x1, y1 int, gap [2]int, k construction.Kind) {
	t.Helper()
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			switch {
			case x != x0 && x != x1 && y != y0 && y != y1:
			case [2]int{x, y} == gap:
				placeBuilt(t, g, k, construction.South, x, y)
			default:
				placeBuilt(t, g, construction.Wall, construction.South, x, y)
			}
		}
	}
}

// TestAX2AnimalFlapLetsAnimalsThrough (AX-2): with the only way into a
// walled pen an animal flap, a colonist outside cannot reach in, a tame
// animal's path goes through, a wild one's does not; the flap bounds the
// room like a door. Regions on and off agree.
func TestAX2AnimalFlapLetsAnimalsThrough(t *testing.T) {
	t.Parallel()
	for _, off := range []bool{false, true} {
		g := ax2Game(t)
		g.regs.off = off
		ax2Pen(t, g, 6, 6, 12, 12, [2]int{6, 9}, construction.AnimalFlap)
		pid, err := g.SpawnPawn(2, 9)
		if err != nil {
			t.Fatal(err)
		}
		if g.reachable(pid, 9, 9) {
			t.Fatalf("regions off %v: a colonist reaches through the flap", off)
		}
		if _, ok := g.pathTo(pid, 9, 9); ok {
			t.Fatalf("regions off %v: a colonist's path goes through the flap", off)
		}
		if _, ok := pathing.Find(g.animalPathQuery(true, false), 2, 9, 9, 9); !ok {
			t.Fatalf("regions off %v: a tame animal has no path through the flap", off)
		}
		if _, ok := pathing.Find(g.animalPathQuery(false, false), 2, 9, 9, 9); ok {
			t.Fatalf("regions off %v: a wild animal walks through the flap", off)
		}
		g.ensureRooms()
		if g.roomAt(2, 9) == g.roomAt(9, 9) {
			t.Fatalf("regions off %v: the flap does not close the pen's room", off)
		}
	}
}

// TestAX2ColumnHoldsTheRoof (AX-2): a lone column supports a roof up to
// roof.SupportRange tiles off and splits no room.
func TestAX2ColumnHoldsTheRoof(t *testing.T) {
	t.Parallel()
	g := ax2Game(t)
	supported := func(x, y int) bool {
		g.ensureRooms()
		return g.roofs[y*g.grid.Width()+x].Has(roof.Supported)
	}
	if supported(12, 12) {
		t.Fatal("open ground is supported")
	}
	placeBuilt(t, g, construction.Column, construction.South, 10, 12)
	if !supported(13, 12) || supported(10+roof.SupportRange+2, 12) {
		t.Fatalf("column support: 3 off %v, %d off %v", supported(13, 12), roof.SupportRange+2, supported(10+roof.SupportRange+2, 12))
	}
	if g.roomAt(9, 12) != g.roomAt(11, 12) {
		t.Fatal("a column split a room")
	}
	if g.grid.Walkable(10, 12) {
		t.Fatal("a column is walked through")
	}
}

// TestAX2OrnateDoorReplacesTwoWalls (AX-2): an ornate door laid over a
// wall run marks both walls for deconstruction and waits for the second
// too; built, it is a 2-wide door that bounds rooms.
func TestAX2OrnateDoorReplacesTwoWalls(t *testing.T) {
	t.Parallel()
	g := ax2Game(t)
	for x := 8; x <= 12; x++ {
		placeBuilt(t, g, construction.Wall, construction.South, x, 5)
	}
	g.Enqueue(command.Command{Kind: command.DesignateBuild, Build: uint8(construction.OrnateDoor), Stuff: uint8(item.Wood), X: 9, Y: 5})
	g.Step(1)
	fr, ok := g.frames.Get(g.layerFrameAt(9, 5, construction.LayerObject))
	if !ok || fr.Kind != construction.OrnateDoor || g.layerFrameAt(10, 5, construction.LayerObject) == 0 {
		t.Fatal("no 2-wide ornate door blueprint over the walls")
	}
	for x := 9; x <= 10; x++ {
		if b, _ := g.bldgs.Get(g.layerBldgAt(x, 5, construction.LayerObject)); !b.Deconstruct {
			t.Fatalf("the wall at %d is not marked to come down", x)
		}
	}
	g.removeBuilding(g.layerBldgAt(9, 5, construction.LayerObject))
	if g.replaceWhy(fr) == "" {
		t.Fatal("the door may be built while its second wall stands")
	}
	g.removeBuilding(g.layerBldgAt(10, 5, construction.LayerObject))
	if g.replaceWhy(fr) != "" {
		t.Fatal("the door still waits with both walls gone")
	}
	g.completeFrame(g.layerFrameAt(9, 5, construction.LayerObject), fr)
	if !g.grid.Walkable(10, 5) || !g.isBoundary(10, 5) || !g.isBoundary(9, 5) {
		t.Fatal("the ornate door is not a walkable room boundary on both cells")
	}
}

// TestAX2WallHungLights (AX-2): a wall torch or lamp goes only in front
// of a wall, facing away from it; the lights' radii are the wiki's.
func TestAX2WallHungLights(t *testing.T) {
	t.Parallel()
	g := ax2Game(t)
	if ok, why := g.CanPlaceAt("wall lamp", "steel", "south", 10, 10); ok || why != ReasonNeedsWall {
		t.Fatalf("a wall lamp on open ground: %v %q", ok, why)
	}
	placeBuilt(t, g, construction.Wall, construction.South, 10, 9)
	if f := g.AutoFacingAt("wall lamp", "north", 10, 10); f != "south" {
		t.Fatalf("auto facing %q, want south (away from the wall north of it)", f)
	}
	if ok, why := g.CanPlaceAt("wall lamp", "steel", "south", 10, 10); !ok {
		t.Fatalf("in front of a wall: refused %q", why)
	}
	placeBuilt(t, g, construction.WallTorch, construction.West, 10, 10) // auto-faced south
	placeBuilt(t, g, construction.FloodLight, construction.South, 15, 15)
	for _, c := range []struct {
		k    construction.Kind
		x, y int
		r    int
	}{{construction.WallTorch, 10, 10, wallTorchRadius}, {construction.FloodLight, 15, 15, floodRadius}} {
		id := g.layerBldgAt(c.x, c.y, construction.LayerObject)
		b, _ := g.bldgs.Get(id)
		if _, r, _ := g.shining(id, b); b.Kind != c.k || r != c.r || !isLightSource(b.Kind) {
			t.Fatalf("%s: radius %d, want %d", c.k, r, c.r)
		}
		if c.k == construction.WallTorch && b.Facing != construction.South {
			t.Fatalf("wall torch faces %s", b.Facing)
		}
	}
	if wallTorchRadius != 6 || wallLampRadius != 8 || floodRadius != 17 {
		t.Fatal("wiki radii: wall torch 5.76, wall lamp 7.54, flood light 17.45")
	}
	if _, ok := fuelSpec(construction.WallTorch); !ok {
		t.Fatal("a wall torch burns wood")
	}
}

// TestAX2EggBoxIsTheLayingTarget (AX-2): a tame layer's egg goes to the
// nearest egg box within reach, into its storage; none past the reach.
func TestAX2EggBoxIsTheLayingTarget(t *testing.T) {
	t.Parallel()
	g := ax2Game(t)
	if _, _, ok := eggBoxNear(g, 5, 5); ok {
		t.Fatal("an egg box with none built")
	}
	placeBuilt(t, g, construction.EggBox, construction.South, 9, 5)
	if x, y, ok := eggBoxNear(g, 5, 5); !ok || x != 9 || y != 5 {
		t.Fatalf("egg box %d,%d %v", x, y, ok)
	}
	if _, _, ok := eggBoxNear(g, 9+eggBoxReach+1, 5); ok {
		t.Fatal("an egg box past its reach")
	}
	if _, _, c := g.storeView(g.layerBldgAt(9, 5, construction.LayerObject)); c == 0 {
		t.Fatal("the egg box stores nothing")
	}
}

// TestAX2TablesAndPotsOnTheView (AX-2): a table's seats, a couch's two,
// and a pot's plant, its growth and beauty reach the building view; the
// SetPotPlant command sows and empties a pot.
func TestAX2TablesAndPotsOnTheView(t *testing.T) {
	t.Parallel()
	g := ax2Game(t)
	placeBuilt(t, g, construction.Table2x4, construction.South, 2, 2)
	placeBuilt(t, g, construction.Couch, construction.South, 6, 2)
	placeBuilt(t, g, construction.PlantPot, construction.South, 9, 2)
	view := func(x, y int) BuildingView {
		for _, b := range g.World().Buildings {
			if b.X == x && b.Y == y {
				return b
			}
		}
		t.Fatalf("no building at %d,%d", x, y)
		return BuildingView{}
	}
	if s := view(2, 2).AX.Seats; s != 12 {
		t.Fatalf("2x4 table seats %d, want 12", s)
	}
	if s := view(6, 2).AX.Seats; s != 2 {
		t.Fatalf("couch seats %d, want 2", s)
	}
	pot := g.layerBldgAt(9, 2, construction.LayerObject)
	g.Enqueue(command.Command{Kind: command.SetPotPlant, Target: uint64(pot), Build: uint8(construction.PotDaylily)})
	g.Step(1)
	b, _ := g.bldgs.Get(pot)
	b.Growth = 500
	g.bldgs.Set(pot, b)
	if v := view(9, 2); v.AX.Plant != "daylily" || v.AX.PlantPct != 50 || v.Beauty != 9000 {
		t.Fatalf("pot view %+v beauty %d, want a half-grown daylily (beauty 9)", v.AX, v.Beauty)
	}
	g.Enqueue(command.Command{Kind: command.SetPotPlant, Target: uint64(pot), Build: uint8(construction.PotNone)})
	g.Step(1)
	if v := view(9, 2); v.AX.Plant != "" || v.AX.PlantPct != 0 {
		t.Fatalf("emptied pot view %+v", v.AX)
	}
}

// TestAX2OldTablesMigrate (AX-2): save v11 turns a 1x1 table into the
// 1x2 one where its second cell is free, and takes one with a wall in
// the way down for its material.
func TestAX2OldTablesMigrate(t *testing.T) {
	t.Parallel()
	g := ax2Game(t)
	placeBuilt(t, g, construction.Wall, construction.South, 5, 6)
	old := func(x, y int) entity.EntityID { // a table as a v10 save holds it: one cell
		id := g.entities.Next()
		g.bldgIDs = append(g.bldgIDs, id)
		g.bldgs.Set(id, construction.Building{Kind: construction.Table, Stuff: item.Wood, X: x, Y: y})
		g.structVer++
		return id
	}
	blocked, free := old(5, 5), old(12, 5)
	g.migrateBenches(resize.V11, resize.RefundV10)
	if g.bldgs.Has(blocked) || !g.bldgs.Has(free) {
		t.Fatalf("blocked table kept %v, free table kept %v", g.bldgs.Has(blocked), g.bldgs.Has(free))
	}
	if st, ok := g.stacks.Get(g.itemAt(5, 5)); !ok || st.Kind != item.Wood || st.Qty != construction.WoodCost(construction.Table) {
		t.Fatalf("refund %+v ok=%v", st, ok)
	}
	if !g.grid.Walkable(12, 6) {
		t.Fatal("a migrated table blocks its second cell")
	}
}

// TestAX2PreciousWalls (AX-2): a gold wall's blueprint needs ten times a
// wood wall's material (wiki: Walls) and gives ten times back.
func TestAX2PreciousWalls(t *testing.T) {
	t.Parallel()
	g := ax2Game(t)
	g.Enqueue(command.Command{Kind: command.DesignateBuild, Build: uint8(construction.Wall), Stuff: uint8(item.Gold), X: 4, Y: 4})
	g.Step(1)
	fr, ok := g.frames.Get(g.layerFrameAt(4, 4, construction.LayerObject))
	if !ok || fr.Stuff != item.Gold || fr.Need != 10*construction.WoodCost(construction.Wall) {
		t.Fatalf("gold wall frame %+v ok=%v", fr, ok)
	}
	if refundOf(construction.Building{Kind: construction.Wall, Stuff: item.Gold}) != 10*construction.Refund(construction.Wall) {
		t.Fatal("a gold wall's refund")
	}
}

// TestAX2GrowerSowsThePot (AX-2): a pot's chosen plant waits until a
// colonist with Growing work sows it; choosing another plant has it
// replanted.
func TestAX2GrowerSowsThePot(t *testing.T) {
	t.Parallel()
	g := ax2Game(t)
	placeBuilt(t, g, construction.PlantPot, construction.South, 10, 10)
	pot := g.layerBldgAt(10, 10, construction.LayerObject)
	if _, err := g.SpawnPawn(6, 10); err != nil {
		t.Fatal(err)
	}
	sown := func() bool { b, _ := g.bldgs.Get(pot); return b.Growth > 0 }
	g.Enqueue(command.Command{Kind: command.SetPotPlant, Target: uint64(pot), Build: uint8(construction.PotRose)})
	g.Step(1)
	if sown() {
		t.Fatal("the rose sowed itself")
	}
	for i := 0; i < 4000 && !sown(); i++ {
		g.Step(1)
	}
	if !sown() {
		t.Fatal("no grower sowed the pot")
	}
	g.Enqueue(command.Command{Kind: command.SetPotPlant, Target: uint64(pot), Build: uint8(construction.PotDaylily)})
	g.Step(1)
	for i := 0; i < 4000 && !sown(); i++ {
		g.Step(1)
	}
	if b, _ := g.bldgs.Get(pot); b.Plant != uint8(construction.PotDaylily) || b.Growth == 0 {
		t.Fatalf("not replanted with the daylily: %+v", b)
	}
}
