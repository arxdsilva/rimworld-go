package game

import (
	"strings"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/terrain"
)

// riverGame is a 20x10 soil map with a north-south river: shallow water
// in column 5, deep water in columns 6..8, shallow again in 9, then a
// marsh column (11) and a mud column (12).
func riverGame(t *testing.T) *Game {
	t.Helper()
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	fillTerrain(g, 5, 0, 5, 9, terrain.ShallowWater)
	fillTerrain(g, 6, 0, 8, 9, terrain.DeepWater)
	fillTerrain(g, 9, 0, 9, 9, terrain.ShallowWater)
	fillTerrain(g, 11, 0, 11, 9, terrain.Marsh)
	fillTerrain(g, 12, 0, 12, 9, terrain.Mud)
	return g
}

// bridgeNow lays a finished wooden bridge at (x, y).
func bridgeNow(t *testing.T, g *Game, x, y int) {
	t.Helper()
	buildNow(t, g, construction.Bridge, item.Wood, construction.South, x, y)
}

func TestBridgePlacementRules(t *testing.T) {
	t.Parallel()
	g := riverGame(t)
	// the sea: deep water in column 15 (marked as ocean)
	fillTerrain(g, 15, 0, 15, 9, terrain.DeepWater)
	mask := make([]bool, 20*10)
	for y := 0; y < 10; y++ {
		mask[y*20+15] = true
	}
	g.setOcean(mask)
	for _, c := range []struct {
		name string
		x, y int
		ok   bool
		why  string
	}{
		{"shallow river", 5, 3, true, ""},
		{"deep river", 7, 3, true, ""},
		{"marsh", 11, 3, true, ""},
		{"mud", 12, 3, true, ""},
		{"soil", 2, 3, false, ReasonNotBridgeable},
		{"open sea", 15, 3, false, ReasonOpenSea},
	} {
		ok, why := g.CanPlaceAt("bridge", "wood", "south", c.x, c.y)
		if ok != c.ok || why != c.why {
			t.Errorf("bridge on %s: %v %q, want %v %q", c.name, ok, why, c.ok, c.why)
		}
	}
	// wood only
	if ok, why := g.CanPlaceAt("bridge", "granite blocks", "south", 7, 3); ok || why != ReasonStuff {
		t.Errorf("stone bridge: %v %q", ok, why)
	}
	// nothing but a bridge goes on plain water
	if ok, why := g.CanPlaceAt("wall", "wood", "south", 7, 3); ok || why != ReasonWater {
		t.Errorf("wall on open water: %v %q", ok, why)
	}
	// a bridge already there, or one planned: no second one
	bridgeNow(t, g, 7, 3)
	if ok, why := g.CanPlaceAt("bridge", "wood", "south", 7, 3); ok || why != ReasonSame {
		t.Errorf("bridge on a bridge: %v %q", ok, why) // the same piece: skipped silently
	}
	if ok, why := g.CanPlace("bridge", 7, 3); ok || why != ReasonOccupied {
		t.Errorf("bridge on a bridge (no material): %v %q", ok, why)
	}
	g.placeFrameStuff(construction.Bridge, item.Wood, construction.South, 7, 4)
	if ok, why := g.CanPlaceAt("bridge", "wood", "south", 7, 4); ok || why != ReasonPlanned {
		t.Errorf("bridge on a planned bridge: %v %q", ok, why)
	}
	// a planned bridge is not footing yet
	if ok, why := g.CanPlaceAt("wall", "wood", "south", 7, 4); ok || why != ReasonWater {
		t.Errorf("wall on a planned bridge: %v %q", ok, why)
	}
}

func TestLegacyMapBridgesDeepWater(t *testing.T) {
	t.Parallel()
	g := riverGame(t) // no ocean mask: a legacy map
	for x := 5; x <= 9; x++ {
		if ok, why := g.CanPlaceAt("bridge", "wood", "south", x, 2); !ok {
			t.Errorf("legacy river bridge at x=%d refused: %q", x, why)
		}
	}
	// a mask with no sea is no sea
	g.setOcean(make([]bool, 200))
	if g.br.ocean != nil {
		t.Fatal("an empty sea mask kept")
	}
	g.setOcean([]bool{true}) // wrong size
	if g.br.ocean != nil {
		t.Fatal("a mis-sized sea mask kept")
	}
}

func TestBridgeMakesDeepWaterWalkableAndPathUsesIt(t *testing.T) {
	t.Parallel()
	g := riverGame(t)
	id, err := g.SpawnNamedPawn(2, 4, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	if g.reachable(id, 15, 4) {
		t.Fatal("the far bank reachable across deep water")
	}
	for x := 6; x <= 8; x++ {
		bridgeNow(t, g, x, 4)
		if !g.grid.Walkable(x, 4) || !g.cellWalkable(x, 4) {
			t.Fatalf("bridge at %d,4 not walkable", x)
		}
	}
	if !g.reachable(id, 15, 4) {
		t.Fatal("the far bank not reachable over the bridge")
	}
	path, ok := g.pathTo(id, 15, 4)
	if !ok {
		t.Fatal("no path over the bridge")
	}
	onBridge := false
	for _, p := range path {
		if p.X == 7 && p.Y == 4 {
			onBridge = true
		}
		if g.terr.At(p.X, p.Y) == terrain.DeepWater && !g.bridged(p.X, p.Y) {
			t.Fatalf("path swims at %v", p)
		}
	}
	if !onBridge {
		t.Fatalf("path %v skips the bridge", path)
	}
	// the bridge is dry footing: drops, spawns and stockpiles
	if !g.dryWalkable(7, 4) || g.isWaterAt(7, 4) {
		t.Fatal("a bridged cell is still water")
	}
	if g.dryWalkable(7, 5) || !g.isWaterAt(7, 5) {
		t.Fatal("open water beside the bridge became dry")
	}
	// a terrain rewrite keeps the deck walkable
	g.setTerrain(7, 4, terrain.DeepWater)
	if !g.grid.Walkable(7, 4) {
		t.Fatal("setTerrain dropped the bridge's footing")
	}
}

func TestBridgeMovesAtFullSpeedAndPrefersDeckToWading(t *testing.T) {
	t.Parallel()
	g := riverGame(t)
	bridgeNow(t, g, 5, 4)
	bridgeNow(t, g, 11, 4)
	g.ensureRooms()
	for _, c := range [][2]int{{5, 4}, {11, 4}} {
		if got := g.weatherSlow(c[0], c[1], 100); got != 100 {
			t.Errorf("step on bridge %v = %d, want 100", c, got)
		}
		if g.groundExtra(c[0], c[1]) != 0 {
			t.Errorf("bridge %v costs extra", c)
		}
	}
	if g.weatherSlow(5, 5, 100) <= 100 || g.groundExtra(5, 5) == 0 {
		t.Fatal("plain shallow water lost its slowness")
	}
}

func TestNoSoakedOnBridge(t *testing.T) {
	t.Parallel()
	g := riverGame(t)
	id, _ := g.SpawnNamedPawn(4, 4, "Ada")
	bridgeNow(t, g, 5, 4)
	g.setCarried(id, 400)
	g.dirtStep(id, Position{X: 5, Y: 4})
	if g.hasMemory(id, mood.Soaked) {
		t.Fatal("soaked on a bridge")
	}
	g.dirtStep(id, Position{X: 5, Y: 5}) // plain shallow water still soaks
	if !g.hasMemory(id, mood.Soaked) {
		t.Fatal("wading no longer soaks")
	}
}

func TestBridgeSupportRules(t *testing.T) {
	t.Parallel()
	g := riverGame(t)
	bridgeNow(t, g, 7, 3)
	bridgeNow(t, g, 11, 3) // over marsh: the ground holds anything
	for _, c := range []struct {
		kind, stuff string
		ok          bool
	}{
		{"wall", "wood", true},
		{"door", "wood", true},
		{"fence", "wood", true},
		{"bed", "wood", true},
		{"wall", "granite blocks", false},
		{"wall", "steel", false},
		{"floor", "wood", false},
		{"floor", "granite blocks", false},
		{"grave", "wood", false},
	} {
		ok, why := g.CanPlaceAt(c.kind, c.stuff, "south", 7, 3)
		if c.kind == "bed" { // 1x2: both cells on the deck
			bridgeNow(t, g, 7, 4)
			ok, why = g.CanPlaceAt(c.kind, c.stuff, "south", 7, 3)
		}
		if ok != c.ok || (!ok && why != ReasonNoSupport) {
			t.Errorf("%s %s on a bridge: %v %q, want %v", c.stuff, c.kind, ok, why, c.ok)
		}
	}
	// CanPlace (no material) takes the default: a wooden wall
	if ok, why := g.CanPlace("wall", 7, 3); !ok {
		t.Errorf("default wall on a bridge: %q", why)
	}
	if ok, why := g.CanPlaceStuff("wall", "granite blocks", 7, 3); ok || why != ReasonNoSupport {
		t.Errorf("CanPlaceStuff stone wall: %v %q", ok, why)
	}
	if ok, why := g.CanPlaceAt("wall", "granite blocks", "south", 11, 3); !ok {
		t.Errorf("stone wall on a bridged marsh: %q", why)
	}
	// a wooden wall on the deck can't be swapped to stone
	buildNow(t, g, construction.Wall, item.Wood, construction.South, 7, 3)
	if ok, why := g.CanPlaceAt("wall", "granite blocks", "south", 7, 3); ok || why != ReasonNoSupport {
		t.Errorf("stone swap on a bridge: %v %q", ok, why)
	}
	if g.grid.Walkable(7, 3) {
		t.Fatal("a wall on a bridge does not block")
	}
}

func TestBridgeOnBlockedOrTreeCellRefused(t *testing.T) {
	t.Parallel()
	g := riverGame(t)
	buildNow(t, g, construction.Wall, item.Wood, construction.South, 11, 2) // a wall standing in the marsh
	if ok, why := g.CanPlaceAt("bridge", "wood", "south", 11, 2); ok || why != ReasonBlocked {
		t.Errorf("bridge under a wall: %v %q", ok, why)
	}
}

func TestBridgeDestroyedTakesItsLoadDown(t *testing.T) {
	t.Parallel()
	g := riverGame(t)
	for _, c := range [][2]int{{7, 2}, {7, 3}, {7, 4}, {5, 3}} {
		bridgeNow(t, g, c[0], c[1])
	}
	wall := buildNow(t, g, construction.Wall, item.Wood, construction.South, 7, 2)
	if _, err := g.SpawnStack(item.Steel, 10, 7, 3); err != nil { // on deep water: sinks
		t.Fatal(err)
	}
	if _, err := g.SpawnStack(item.Cloth, 5, 5, 3); err != nil { // on shallow water: stays
		t.Fatal(err)
	}
	pid, err := g.SpawnNamedPawn(7, 4, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	aid, err := g.spawnAnimal(animal.Species(0), 4, 4)
	if err != nil {
		t.Fatal(err)
	}
	g.positions.Set(aid, Position{X: 7, Y: 3}) // the animal wandered onto the deck
	g.placeFrameStuff(construction.Door, item.Wood, construction.South, 7, 4)
	woodBefore := g.World().Wood
	// the deck under the steel goes first: one message for what sank
	g.damageBuilding(g.bridgeAt(7, 3), 1_000_000)
	sank := 0
	for _, m := range g.messages {
		if strings.Contains(m.Text, "sank") {
			sank++
			if m.Text != "The steel sank." {
				t.Errorf("sank message %q", m.Text)
			}
		}
	}
	if sank != 1 {
		t.Fatalf("%d sank messages, want 1: %+v", sank, g.messages)
	}
	for _, c := range [][2]int{{7, 2}, {7, 4}, {5, 3}} {
		g.damageBuilding(g.bridgeAt(c[0], c[1]), 1_000_000)
	}
	if g.bldgs.Has(wall) {
		t.Fatal("the wall stands on nothing")
	}
	if g.World().Wood > woodBefore+4*construction.Refund(construction.Bridge) {
		t.Fatalf("the wall was refunded: wood %d -> %d", woodBefore, g.World().Wood)
	}
	if k, _ := stackAt(g, 7, 3); k == item.Steel {
		t.Fatal("steel floats on deep water")
	}
	for _, id := range g.itemIDs {
		if st, _ := g.stacks.Get(id); st.Kind == item.Steel {
			t.Fatalf("the steel survived at %d,%d", st.X, st.Y)
		}
	}
	if k, n := stackAt(g, 5, 3); k != item.Cloth || n != 5 {
		t.Fatalf("cloth on shallow water: %v x%d", k, n)
	}
	for _, id := range []struct {
		who string
		pos Position
	}{{"pawn", posOf(g, pid)}, {"animal", posOf(g, aid)}} {
		if !g.grid.Walkable(id.pos.X, id.pos.Y) {
			t.Fatalf("%s left in deep water at %v", id.who, id.pos)
		}
	}
	for _, c := range [][2]int{{7, 2}, {7, 3}, {7, 4}} {
		if g.grid.Walkable(c[0], c[1]) || g.cellWalkable(c[0], c[1]) {
			t.Fatalf("deep water at %v still walkable", c)
		}
	}
	if !g.grid.Walkable(5, 3) {
		t.Fatal("shallow water became unwalkable")
	}
	// the door planned on the deck is cancelled, and nothing stands planned
	// on open water but the bridges' rebuild blueprints
	for _, fid := range g.frameIDs {
		if fr, _ := g.frames.Get(fid); fr.Kind != construction.Bridge {
			t.Fatalf("a %s is still planned on open water at %d,%d", fr.Kind, fr.X, fr.Y)
		}
	}
}

func TestBridgeDeconstructRefusedWhileInUse(t *testing.T) {
	t.Parallel()
	g := riverGame(t)
	bridgeNow(t, g, 7, 3)
	bridgeNow(t, g, 7, 4)
	wall := buildNow(t, g, construction.Wall, item.Wood, construction.South, 7, 3)
	g.markDeconstruct(7, 3, 7, 4, true)
	if b, _ := g.bldgs.Get(g.bridgeAt(7, 3)); b.Deconstruct {
		t.Fatal("a bridge under a wall marked for deconstruction")
	}
	refused := false
	for _, m := range g.messages {
		refused = refused || strings.Contains(m.Text, ReasonBridgeInUse)
	}
	if !refused {
		t.Fatal("no refusal message")
	}
	if b, _ := g.bldgs.Get(g.bridgeAt(7, 4)); !b.Deconstruct {
		t.Fatal("a free bridge not marked")
	}
	// a wall raised after the mark: the deconstruct job gives up
	g.markDeconstruct(7, 3, 7, 3, false)
	g.removeBuilding(wall)
	g.markDeconstruct(7, 3, 7, 3, true)
	bid := g.bridgeAt(7, 3)
	buildNow(t, g, construction.Door, item.Wood, construction.South, 7, 3)
	pid, _ := g.SpawnNamedPawn(4, 3, "Ada")
	j := job.Record{Kind: job.Deconstruct, Target: bid, State: job.Do}
	g.doDeconstruct(pid, j)
	if j2, _ := g.jobs.Get(pid); j2.State != job.Fail {
		t.Fatalf("deconstructing a bridge in use: %+v", j2)
	}
	if b, _ := g.bldgs.Get(bid); !g.bldgs.Has(bid) || b.Deconstruct {
		t.Fatal("the bridge in use lost its deck or kept its mark")
	}
	// the free one comes down with a refund of 3 wood, which floats ashore
	before := g.World().Wood
	g.removeBuilding(g.bridgeAt(7, 4))
	if got := g.World().Wood - before; got != construction.Refund(construction.Bridge) || got != 3 {
		t.Fatalf("bridge refund %d", got)
	}
	if g.grid.Walkable(7, 4) {
		t.Fatal("deep water walkable after its bridge came down")
	}
}

func TestBridgeResearchLock(t *testing.T) {
	t.Parallel()
	g := riverGame(t)
	g.research = research.Progress{}
	if ok, why := g.CanPlaceAt("bridge", "wood", "south", 7, 3); ok || why != "needs research: Bridges" {
		t.Fatalf("locked bridge: %v %q", ok, why)
	}
	g.Apply(command.Command{Kind: command.DesignateBuild, Build: uint8(construction.Bridge), X: 7, Y: 3})
	g.ApplyPending()
	if g.layerFrameAt(7, 3, construction.LayerFoundation) != 0 {
		t.Fatal("a locked bridge was planned")
	}
	g.research = research.Complete()
	g.Apply(command.Command{Kind: command.DesignateBuild, Build: uint8(construction.Bridge), X: 7, Y: 3})
	g.ApplyPending()
	if g.layerFrameAt(7, 3, construction.LayerFoundation) == 0 {
		t.Fatal("an unlocked bridge was not planned")
	}
}

func TestBridgeZones(t *testing.T) {
	t.Parallel()
	g := riverGame(t)
	bridgeNow(t, g, 7, 3)
	bridgeNow(t, g, 11, 3)
	if !g.CanZoneAt("stockpile", 7, 3) || !g.CanZoneAt("stockpile", 11, 3) {
		t.Fatal("no stockpile on a bridge")
	}
	if g.CanZoneAt("stockpile", 7, 4) {
		t.Fatal("stockpile on open water")
	}
	if g.ZoneRefusal(ZoneGrow, 11, 3) != ZoneRefuseBridge || g.zoneable(zone.KindGrow, 7, 3) {
		t.Fatal("a growing zone on a bridge")
	}
	if !g.zoneable(zone.KindGrow, 11, 4) {
		t.Fatal("plain marsh lost its growing")
	}
}

func TestBuildersSpanDeepWaterFromTheShore(t *testing.T) {
	g := riverGame(t)
	if _, err := g.SpawnNamedPawn(2, 4, "Ada"); err != nil {
		t.Fatal(err)
	}
	if _, err := g.SpawnStack(item.Wood, 40, 3, 4); err != nil {
		t.Fatal(err)
	}
	for x := 5; x <= 9; x++ {
		build(g, construction.Bridge, construction.South, x, 4)
	}
	g.Step(TicksPerCell * 1500)
	for x := 5; x <= 9; x++ {
		if !g.bridged(x, 4) {
			t.Fatalf("bridge at %d,4 not built; frames %+v", x, g.World().Frames)
		}
	}
	if !g.grid.Walkable(8, 4) {
		t.Fatal("the span is not walkable")
	}
}

func TestBridgeSaveRoundTrip(t *testing.T) {
	t.Parallel()
	g := riverGame(t)
	fillTerrain(g, 15, 0, 15, 9, terrain.DeepWater)
	mask := make([]bool, 20*10)
	for y := 0; y < 10; y++ {
		mask[y*20+15] = true
	}
	g.setOcean(mask)
	bridgeNow(t, g, 7, 3)
	g.Step(60) // rooms and tread are derived on the first tick
	h := saveLoad(t, g)
	if g.StateHash() != h.StateHash() {
		t.Fatal("a bridged colony changes in a save round trip")
	}
	if !h.bridged(7, 3) || !h.grid.Walkable(7, 3) {
		t.Fatal("the bridge is lost in a save")
	}
	if ok, why := h.CanPlaceAt("bridge", "wood", "south", 15, 3); ok || why != ReasonOpenSea {
		t.Fatalf("the sea is lost in a save: %v %q", ok, why)
	}
	// an older save has no sea: every deep water is river
	h.importOcean(nil)
	if ok, _ := h.CanPlaceAt("bridge", "wood", "south", 15, 3); !ok {
		t.Fatal("an older save refuses a deep-water bridge")
	}
	h.importOcean([]uint64{1}) // mis-sized
	if h.br.ocean != nil {
		t.Fatal("a mis-sized sea loaded")
	}
}

func TestGeneratedBridgesApplied(t *testing.T) {
	t.Parallel()
	g := siteGame(t, "broken_hull", func(r *mapgen.Result) {
		r.Ocean = make([]bool, 100*100)
		for y := 0; y < 100; y++ {
			r.Terrain.Set(30, y, terrain.DeepWater)
			r.Terrain.Set(31, y, terrain.ShallowWater)
			r.Terrain.Set(90, y, terrain.DeepWater)
			r.Ocean[y*100+90] = true
		}
		r.Bridges = []mapgen.Point{{X: 30, Y: 50}, {X: 31, Y: 50}, {X: 30, Y: 51}, {X: 31, Y: 51}, {X: 5, Y: 5}}
	})
	for _, c := range [][2]int{{30, 50}, {31, 50}, {30, 51}, {31, 51}} {
		b, ok := bldgAt(g, construction.Bridge, c[0], c[1])
		if !ok || b.Stuff != item.Wood {
			t.Fatalf("no wooden bridge at %v", c)
		}
		if !g.grid.Walkable(c[0], c[1]) {
			t.Fatalf("generated bridge at %v not walkable", c)
		}
	}
	if _, ok := bldgAt(g, construction.Bridge, 5, 5); ok {
		t.Fatal("a bridge laid on dry soil")
	}
	if !g.isOcean(90, 40) || g.isOcean(30, 40) {
		t.Fatal("the sea mask is not kept")
	}
	if g.grid.Walkable(30, 52) {
		t.Fatal("deep water beside the bridge walkable")
	}
}

func TestFilledGraveStaysBlocked(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 10, Height: 10})
	bid := buildNow(t, g, construction.Grave, item.Wood, construction.South, 3, 3)
	if !g.cellWalkable(3, 3) {
		t.Fatal("an open grave blocks")
	}
	g.setGrave(bid, graveRec{Name: "Bo"})
	if g.cellWalkable(3, 3) || g.cellWalkable(3, 4) {
		t.Fatal("a filled grave is walkable")
	}
	g.setTerrain(3, 3, terrain.Soil)
	if g.grid.Walkable(3, 3) {
		t.Fatal("a terrain rewrite opened a filled grave")
	}
}

func TestBridgeTakenDownFloatsItsLoadAshore(t *testing.T) {
	t.Parallel()
	g := riverGame(t)
	bridgeNow(t, g, 7, 3)
	if _, err := g.SpawnStack(item.Steel, 10, 7, 3); err != nil {
		t.Fatal(err)
	}
	g.removeBuilding(g.bridgeAt(7, 3)) // deconstructed: nothing is destroyed
	found := false
	for _, id := range g.itemIDs {
		if st, _ := g.stacks.Get(id); st.Kind == item.Steel {
			found = st.Qty == 10 && g.dryWalkable(st.X, st.Y)
		}
	}
	if !found {
		t.Fatal("the steel on a dismantled deck did not float ashore")
	}
	// out on a wide lake with no shore in reach, it sinks
	h := New(Config{Seed: 1, Width: 20, Height: 10})
	fillTerrain(h, 1, 0, 19, 9, terrain.DeepWater)
	bridgeNow(t, h, 12, 5)
	if _, err := h.SpawnStack(item.Steel, 10, 12, 5); err != nil {
		t.Fatal(err)
	}
	h.removeBuilding(h.bridgeAt(12, 5))
	for _, id := range h.itemIDs {
		if st, _ := h.stacks.Get(id); st.Kind == item.Steel {
			t.Fatalf("steel kept at %d,%d on open water", st.X, st.Y)
		}
	}
	if len(h.messages) == 0 || !strings.Contains(h.messages[len(h.messages)-1].Text, "sank") {
		t.Fatalf("no sank message: %+v", h.messages)
	}
}
