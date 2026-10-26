package game

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/fire"
	"rimworld-go/internal/domain/world/terrain"
)

// PLAY-021 (INFRA): the conduit layer in the places that enumerate layers:
// the cell indexes, placement under walls, the order menu, deconstruction,
// delivery to a frame under a wall, fire and bridges.

// stepChunks steps the game in TicksPerCell chunks until done or limit
// chunks have run; it reports whether done held.
func stepChunks(g *Game, limit int, done func() bool) bool {
	for i := 0; i < limit; i++ {
		if done() {
			return true
		}
		g.Step(TicksPerCell)
	}
	return done()
}

func conduitGame() *Game { return New(Config{Seed: 1, Width: 16, Height: 10}) }

// conduitAt is the conduit building on (x, y), 0 for none.
func conduitAt(g *Game, x, y int) entity.EntityID {
	return g.layerBldgAt(x, y, construction.LayerConduit)
}

func TestConduitIndexedOnItsLayer(t *testing.T) {
	t.Parallel()
	g := conduitGame()
	wall := buildNow(t, g, construction.Wall, item.Wood, construction.South, 3, 3)
	cond := buildNow(t, g, construction.PowerConduit, item.Steel, construction.South, 3, 3)
	if wall == cond {
		t.Fatal("wall and conduit are one building")
	}
	// the index answers on the conduit layer, with no scan fallback
	if id, scan := g.bldgIndex().at(3, 3, construction.LayerConduit); id != cond || scan {
		t.Fatalf("conduit index slot = %d (scan %v), want %d", id, scan, cond)
	}
	if id, scan := g.bldgIndex().at(3, 3, construction.LayerObject); id != wall || scan {
		t.Fatalf("object index slot = %d (scan %v), want the wall %d", id, scan, wall)
	}
	// a planned conduit is indexed the same way
	g.placeFrameStuff(construction.HiddenConduit, item.Steel, construction.South, 4, 4)
	fid := g.layerFrameAt(4, 4, construction.LayerConduit)
	if fid == 0 {
		t.Fatal("no hidden conduit frame")
	}
	if id, scan := g.frameIndex().at(4, 4, construction.LayerConduit); id != fid || scan {
		t.Fatalf("frame index slot = %d (scan %v), want %d", id, scan, fid)
	}
	// a removed conduit leaves the slot empty, the wall stays
	g.removeBuilding(cond)
	if conduitAt(g, 3, 3) != 0 || g.layerBldgAt(3, 3, construction.LayerObject) != wall {
		t.Fatal("removing the conduit must leave the wall")
	}
}

// TestConduitLayerConflictFallsBackToListOrder: two conduits on one cell,
// as no placement rule allows, are answered like the scan (the earlier one
// in the building list), and the later one takes over when it goes.
func TestConduitLayerConflictFallsBackToListOrder(t *testing.T) {
	t.Parallel()
	g := conduitGame()
	first := buildNow(t, g, construction.PowerConduit, item.Steel, construction.South, 3, 3)
	id := g.entities.Next()
	g.bldgIDs = append(g.bldgIDs, id)
	g.bldgs.Set(id, construction.Building{Kind: construction.HiddenConduit, Stuff: item.Steel, X: 3, Y: 3})
	g.noteBuilding(id)
	if !g.idx.bldgs.conflict[3*16+3] {
		t.Fatal("cell not marked conflicted")
	}
	if got := conduitAt(g, 3, 3); got != first {
		t.Fatalf("conduit at 3,3 = %d, want the first %d", got, first)
	}
	g.removeBuilding(first)
	if got := conduitAt(g, 3, 3); got != id {
		t.Fatalf("after removal conduit at 3,3 = %d, want %d", got, id)
	}
}

// TestConduitNeverBlocks: the blocker lookups skip the conduit layer (it
// can't block movement), which is only sound while no conduit blocks.
func TestConduitNeverBlocks(t *testing.T) {
	t.Parallel()
	for _, d := range construction.All() {
		if d.Layer == construction.LayerConduit && d.BlocksMove {
			t.Fatalf("%s is on the conduit layer and blocks movement", d.Name)
		}
	}
	g := conduitGame()
	buildNow(t, g, construction.PowerConduit, item.Steel, construction.South, 5, 5)
	if g.layerBlockerAt(5, 5) != 0 || g.blockingFrameAt(5, 5) != 0 || !g.grid.Walkable(5, 5) {
		t.Fatal("a conduit must not block the way")
	}
}

func TestConduitSharesTileWithWallDoorAndBench(t *testing.T) {
	t.Parallel()
	g := conduitGame()
	wall := buildNow(t, g, construction.Wall, item.Wood, construction.South, 3, 3)
	door := buildNow(t, g, construction.Door, item.Wood, construction.South, 4, 3)
	bench := buildNow(t, g, construction.Workbench, item.Wood, construction.South, 8, 3) // 2x1: (8,3) and (9,3)
	cells := [][2]int{{3, 3}, {4, 3}, {8, 3}, {9, 3}}
	for _, c := range cells {
		for _, k := range []construction.Kind{construction.PowerConduit, construction.HiddenConduit} {
			if ok, r := g.canPlaceStuffAt(k, item.Steel, construction.South, c[0], c[1]); !ok {
				t.Fatalf("%s under the piece at %v: %q", k, c, r)
			}
		}
		buildNow(t, g, construction.PowerConduit, item.Steel, construction.South, c[0], c[1])
	}
	// the standing pieces still answer on their own layer, and still block
	for _, c := range []struct {
		id   entity.EntityID
		x, y int
	}{{wall, 3, 3}, {door, 4, 3}, {bench, 8, 3}, {bench, 9, 3}} {
		if got := g.layerBldgAt(c.x, c.y, construction.LayerObject); got != c.id {
			t.Fatalf("object layer at %d,%d = %d, want %d", c.x, c.y, got, c.id)
		}
		if conduitAt(g, c.x, c.y) == 0 {
			t.Fatalf("no conduit at %d,%d", c.x, c.y)
		}
	}
	if g.layerBlockerAt(3, 3) != wall || g.grid.Walkable(3, 3) {
		t.Fatal("the wall still blocks with a conduit under it")
	}
	// one conduit per cell, either kind
	if ok, r := g.canPlaceStuffAt(construction.PowerConduit, item.Steel, construction.South, 3, 3); ok || r != ReasonSame {
		t.Fatalf("a conduit on the same conduit: %v %q", ok, r)
	}
	if ok, r := g.canPlaceStuffAt(construction.HiddenConduit, item.Steel, construction.South, 3, 3); ok || r != ReasonOccupied {
		t.Fatalf("a hidden conduit on a conduit: %v %q", ok, r)
	}
	// a planned conduit holds its cell too
	g.placeFrameStuff(construction.PowerConduit, item.Steel, construction.South, 6, 6)
	if ok, r := g.canPlaceStuffAt(construction.HiddenConduit, item.Steel, construction.South, 6, 6); ok || r != ReasonPlanned {
		t.Fatalf("conduit over a planned conduit: %v %q", ok, r)
	}
	// a wall may still go over a conduit-only cell
	if ok, r := g.canPlaceStuffAt(construction.Wall, item.Wood, construction.South, 6, 6); !ok {
		t.Fatalf("a wall beside the conduit plan: %q", r)
	}
}

func TestConduitRefusedOnRockWaterAndVents(t *testing.T) {
	t.Parallel()
	g := conduitGame()
	g.setTerrain(2, 2, terrain.Rock)
	g.setTerrain(3, 2, terrain.RockSmooth)
	fillTerrain(g, 5, 2, 5, 2, terrain.ShallowWater)
	fillTerrain(g, 6, 2, 6, 2, terrain.DeepWater)
	if id := g.placeGenerated(construction.Building{Kind: construction.SteamVent, X: 8, Y: 2}); id == 0 {
		t.Fatal("no steam vent")
	}
	for _, c := range []struct {
		x, y int
		want string
	}{
		{2, 2, ReasonRock}, {3, 2, ReasonRock},
		{5, 2, ReasonWater}, {6, 2, ReasonWater},
		{8, 2, ReasonNatural}, {9, 3, ReasonNatural},
	} {
		for _, k := range []construction.Kind{construction.PowerConduit, construction.HiddenConduit} {
			if ok, r := g.canPlaceStuffAt(k, item.Steel, construction.South, c.x, c.y); ok || r != c.want {
				t.Errorf("%s at %d,%d: %v %q, want %q", k, c.x, c.y, ok, r, c.want)
			}
		}
	}
	if ok, r := g.canPlaceStuffAt(construction.PowerConduit, item.Steel, construction.South, 12, 7); !ok {
		t.Fatalf("a conduit on open ground: %q", r)
	}
	if ok, r := g.canPlaceStuffAt(construction.PowerConduit, item.Steel, construction.South, 99, 99); ok || r != ReasonOutside {
		t.Fatalf("a conduit off the map: %v %q", ok, r)
	}
}

// TestConduitOnABridge: a deck holds a conduit (light), the deck can't come
// down under it, and a deck that is destroyed takes the conduit along.
func TestConduitOnABridge(t *testing.T) {
	t.Parallel()
	g := conduitGame()
	fillTerrain(g, 4, 4, 6, 4, terrain.ShallowWater)
	if ok, r := g.canPlaceStuffAt(construction.PowerConduit, item.Steel, construction.South, 5, 4); ok || r != ReasonWater {
		t.Fatalf("a conduit on bare water: %v %q", ok, r)
	}
	deck := buildNow(t, g, construction.Bridge, item.Wood, construction.South, 5, 4)
	if ok, r := g.canPlaceStuffAt(construction.PowerConduit, item.Steel, construction.South, 5, 4); !ok {
		t.Fatalf("a conduit on a bridge: %q", r)
	}
	cond := buildNow(t, g, construction.PowerConduit, item.Steel, construction.South, 5, 4)
	if !g.bridgeInUse(g.mustBuilding(deck)) {
		t.Fatal("a deck with a conduit on it is in use")
	}
	g.markDeconstruct(5, 4, 5, 4, true)
	if b, _ := g.bldgs.Get(deck); b.Deconstruct {
		t.Fatal("the bridge was marked for removal under its conduit")
	}
	if b, _ := g.bldgs.Get(cond); !b.Deconstruct {
		t.Fatal("the conduit on the bridge should take the mark")
	}
	// a destroyed deck takes the conduit into the water with it
	g.collapseBridge(g.mustBuilding(deck))
	g.removeBuildingAs(deck, false)
	if conduitAt(g, 5, 4) != 0 {
		t.Fatal("the conduit survived the bridge it stood on")
	}
}

func (g *Game) mustBuilding(id entity.EntityID) construction.Building {
	b, ok := g.bldgs.Get(id)
	if !ok {
		panic("no such building")
	}
	return b
}

// TestFrameUnderWallIsDeliveredAndBuilt: a colonist reaches a conduit frame
// under a standing wall from beside it, hauls the steel there and builds it;
// the wall is untouched and a left-over steel never lands inside the wall.
func TestFrameUnderWallIsDeliveredAndBuilt(t *testing.T) {
	t.Parallel()
	g := conduitGame()
	wall := buildNow(t, g, construction.Wall, item.Wood, construction.South, 8, 5)
	pid, err := g.SpawnNamedPawn(3, 3, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.SpawnStack(item.Steel, 5, 4, 4); err != nil {
		t.Fatal(err)
	}
	g.placeFrameStuff(construction.PowerConduit, item.Steel, construction.South, 8, 5)
	fid := g.layerFrameAt(8, 5, construction.LayerConduit)
	if fid == 0 {
		t.Fatal("no conduit frame under the wall")
	}
	if g.grid.Walkable(8, 5) {
		t.Fatal("the wall must block its cell")
	}
	sx, sy := g.workSpot(pid, 8, 5)
	if (sx == 8 && sy == 5) || !g.grid.Walkable(sx, sy) || !pathingAdjacent(sx, sy, 8, 5) || !g.reachable(pid, sx, sy) {
		t.Fatalf("work spot (%d,%d) must be a reachable walkable cell beside the wall", sx, sy)
	}
	if !stepChunks(g, 600, func() bool { return conduitAt(g, 8, 5) != 0 }) {
		fr, _ := g.frames.Get(fid)
		t.Fatalf("the conduit under the wall was never built; frame %+v", fr)
	}
	if g.layerBldgAt(8, 5, construction.LayerObject) != wall || g.grid.Walkable(8, 5) {
		t.Fatal("building the conduit disturbed the wall")
	}
	if it := g.itemAt(8, 5); it != 0 {
		t.Fatalf("an item %d lies inside the wall", it)
	}
}

// TestFrameAtAWallCrossingIsReachedDiagonally: a conduit under the middle
// of a plus of walls has no open side, only open corners; it is still
// delivered and built, and taken down again.
func TestFrameAtAWallCrossingIsReachedDiagonally(t *testing.T) {
	t.Parallel()
	g := conduitGame()
	for _, c := range [][2]int{{8, 4}, {8, 5}, {8, 6}, {7, 5}, {9, 5}} {
		buildNow(t, g, construction.Wall, item.Wood, construction.South, c[0], c[1])
	}
	pid, err := g.SpawnNamedPawn(3, 3, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.SpawnStack(item.Steel, 5, 4, 4); err != nil {
		t.Fatal(err)
	}
	g.placeFrameStuff(construction.HiddenConduit, item.Steel, construction.South, 8, 5)
	if !g.reachable(pid, 8, 5) {
		t.Fatal("the crossing must be reachable from a corner")
	}
	if !stepChunks(g, 800, func() bool { return conduitAt(g, 8, 5) != 0 }) {
		t.Fatal("the conduit under the crossing was never built")
	}
	id := conduitAt(g, 8, 5)
	b, _ := g.bldgs.Get(id)
	b.Deconstruct, b.WorkLeft = true, construction.DeconstructTicks(b.Kind)
	g.bldgs.Set(id, b)
	if !stepChunks(g, 800, func() bool { return conduitAt(g, 8, 5) == 0 }) {
		t.Fatal("the conduit under the crossing was never taken down")
	}
}

func pathingAdjacent(ax, ay, bx, by int) bool {
	dx, dy := ax-bx, ay-by
	return dx >= -1 && dx <= 1 && dy >= -1 && dy <= 1 && (dx != 0 || dy != 0)
}

// TestDeconstructConduitUnderWall: only the conduit comes down, a
// colonist works from beside the wall, and the refund (a hidden conduit
// gives one steel back; a plain one nothing) is not buried in it.
func TestDeconstructConduitUnderWall(t *testing.T) {
	t.Parallel()
	g := conduitGame()
	wall := buildNow(t, g, construction.Wall, item.Wood, construction.South, 8, 5)
	cond := buildNow(t, g, construction.HiddenConduit, item.Steel, construction.South, 8, 5)
	if _, err := g.SpawnNamedPawn(3, 3, "Ada"); err != nil {
		t.Fatal(err)
	}
	b, _ := g.bldgs.Get(cond)
	b.Deconstruct, b.WorkLeft = true, construction.DeconstructTicks(b.Kind)
	g.bldgs.Set(cond, b)
	if !stepChunks(g, 600, func() bool { return conduitAt(g, 8, 5) == 0 }) {
		t.Fatal("the conduit under the wall was never taken down")
	}
	if g.layerBldgAt(8, 5, construction.LayerObject) != wall || g.grid.Walkable(8, 5) {
		t.Fatal("deconstructing the conduit must leave the wall standing")
	}
	if it := g.itemAt(8, 5); it != 0 {
		t.Fatalf("the refund (item %d) was dropped inside the wall", it)
	}
	if !g.refundSomewhereNear(item.Steel, 8, 5) {
		t.Fatal("the conduit's steel refund is nowhere beside the wall")
	}
}

// refundSomewhereNear reports a loose stack of k within two cells of (x, y).
func (g *Game) refundSomewhereNear(k item.Kind, x, y int) bool {
	for _, id := range g.itemIDs {
		st, ok := g.stacks.Get(id)
		if ok && st.Kind == k && st.Carrier == 0 && abs(st.X-x) <= 2 && abs(st.Y-y) <= 2 {
			return true
		}
	}
	return false
}

// TestOrderMenuAtWallAndConduit: Prioritize deconstructing is offered for
// both pieces on a cell, each on its own layer, and the conduit's order
// targets the conduit.
func TestOrderMenuAtWallAndConduit(t *testing.T) {
	t.Parallel()
	g := conduitGame()
	wall := buildNow(t, g, construction.Wall, item.Wood, construction.South, 8, 5)
	cond := buildNow(t, g, construction.PowerConduit, item.Steel, construction.South, 8, 5)
	pid, err := g.SpawnNamedPawn(3, 3, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	g.markDeconstruct(8, 5, 8, 5, true)
	for _, id := range []entity.EntityID{wall, cond} {
		if b, _ := g.bldgs.Get(id); !b.Deconstruct {
			t.Fatalf("building %d not marked", id)
		}
	}
	opts := g.OrdersAt(pid, 8, 5)
	var wallOpt, condOpt *OrderOption
	for i := range opts {
		switch opts[i].Label {
		case "Prioritize deconstructing wall":
			wallOpt = &opts[i]
		case "Prioritize deconstructing power conduit":
			condOpt = &opts[i]
		}
	}
	if wallOpt == nil || condOpt == nil {
		t.Fatalf("both deconstruct orders expected, got %+v", opts)
	}
	if wallOpt.Disabled || condOpt.Disabled {
		t.Fatalf("both orders should be enabled: wall %q conduit %q", wallOpt.Reason, condOpt.Reason)
	}
	if condOpt.Cmd.Layer != uint8(construction.LayerConduit) || wallOpt.Cmd.Layer != uint8(construction.LayerObject) {
		t.Fatalf("layers: wall %d conduit %d", wallOpt.Cmd.Layer, condOpt.Cmd.Layer)
	}
	// ordering the conduit sends the colonist to take the conduit down
	rec, why := g.forcedJob(pid, command.OrderDeconstruct, 8, 5, construction.LayerConduit)
	if why != "" || rec.Target != cond {
		t.Fatalf("forced job target %d reason %q, want the conduit %d", rec.Target, why, cond)
	}
	if rec.TX == 8 && rec.TY == 5 {
		t.Fatal("the work spot is inside the wall")
	}
}

// TestOrderMenuBuildsConduitFrameUnderWall: a ready conduit frame under a
// wall is offered as Prioritize building, on the conduit layer.
func TestOrderMenuBuildsConduitFrameUnderWall(t *testing.T) {
	t.Parallel()
	g := conduitGame()
	buildNow(t, g, construction.Wall, item.Wood, construction.South, 8, 5)
	pid, err := g.SpawnNamedPawn(3, 3, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	g.placeFrameStuff(construction.PowerConduit, item.Steel, construction.South, 8, 5)
	fid := g.layerFrameAt(8, 5, construction.LayerConduit)
	fr, _ := g.frames.Get(fid)
	fr.Wood = fr.Need
	g.frames.Set(fid, fr)
	found := false
	for _, o := range g.OrdersAt(pid, 8, 5) {
		if o.Label == "Prioritize building power conduit" {
			found = true
			if o.Disabled || o.Cmd.Layer != uint8(construction.LayerConduit) {
				t.Fatalf("order %+v", o)
			}
		}
	}
	if !found {
		t.Fatal("no Prioritize building order for the conduit frame")
	}
}

// TestFireAndConduits: fire does not burn a steel conduit and adds no fuel,
// but the fire layers include the conduit layer (a wooden conduit, which no
// rule allows, would burn).
func TestFireAndConduits(t *testing.T) {
	t.Parallel()
	g := conduitGame()
	wall := buildNow(t, g, construction.Wall, item.GraniteBlocks, construction.South, 6, 5)
	cond := buildNow(t, g, construction.PowerConduit, item.Steel, construction.South, 6, 5)
	base := g.cellFuel(6, 5, nil)
	if fire.BuildingFlammability(construction.PowerConduit, item.Steel) != 0 {
		t.Fatal("a steel conduit must not be flammable")
	}
	hp := func(id entity.EntityID) int { b, _ := g.bldgs.Get(id); return b.HP() }
	wallHP, condHP := hp(wall), hp(cond)
	g.burnCell(fire.Fire{X: 6, Y: 5, Size: 200}, nil)
	if hp(cond) != condHP || hp(wall) != wallHP {
		t.Fatal("fire must not hurt a steel conduit or a stone wall")
	}
	if g.cellFuel(6, 5, nil) != base {
		t.Fatal("a steel conduit must add no fuel")
	}
	// a conduit made of wood (forced; placement forbids it) is fuel
	wood := buildNow(t, g, construction.HiddenConduit, item.Steel, construction.South, 9, 5)
	b, _ := g.bldgs.Get(wood)
	noFuel := g.cellFuel(9, 5, nil)
	b.Stuff = item.Wood
	g.bldgs.Set(wood, b)
	if g.cellFuel(9, 5, nil).Flammability <= noFuel.Flammability {
		t.Fatal("fireLayers must include the conduit layer")
	}
}
