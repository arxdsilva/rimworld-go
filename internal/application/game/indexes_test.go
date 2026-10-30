package game

import (
	"bytes"
	"reflect"
	"slices"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
)

// The index self-check (indexSelfCheck, on for every test here) proves
// each indexed lookup equals the original scan across the whole suite;
// these tests cover the paths ordinary play rarely takes.

func TestIndexConflictFallsBackToListOrder(t *testing.T) {
	g := New(Config{Seed: 1, Width: 10, Height: 10})
	if !g.BuildNow("wall", "wood", "south", 3, 3) {
		t.Fatal("first wall")
	}
	first := g.layerBldgAt(3, 3, construction.LayerObject)
	// a second object on the same cell, as no placement rule allows:
	// the cell must answer like the scan (the earlier building).
	id := g.entities.Next()
	g.bldgIDs = append(g.bldgIDs, id)
	g.bldgs.Set(id, construction.Building{Kind: construction.Wall, X: 3, Y: 3})
	g.noteBuilding(id)
	if !g.idx.bldgs.conflict[3*10+3] {
		t.Fatal("cell not marked conflicted")
	}
	if got := g.layerBldgAt(3, 3, construction.LayerObject); got != first {
		t.Fatalf("layerBldgAt = %d, want the first building %d", got, first)
	}
	if got := g.layerBlockerAt(3, 3); got != first {
		t.Fatalf("layerBlockerAt = %d, want %d", got, first)
	}
	g.removeBuilding(first)
	if got := g.layerBldgAt(3, 3, construction.LayerObject); got != id {
		t.Fatalf("after removal layerBldgAt = %d, want %d", got, id)
	}
}

func TestIndexOffMapAndUnknownLayer(t *testing.T) {
	g := New(Config{Seed: 1, Width: 6, Height: 6})
	g.BuildNow("wall", "wood", "south", 0, 0)
	for _, c := range [][2]int{{-1, 0}, {0, -1}, {6, 0}, {0, 6}} {
		if g.layerBldgAt(c[0], c[1], construction.LayerObject) != 0 || g.layerBlockerAt(c[0], c[1]) != 0 ||
			g.plantAt(c[0], c[1]) != 0 || g.layerFrameAt(c[0], c[1], construction.LayerObject) != 0 || g.blockingFrameAt(c[0], c[1]) != 0 {
			t.Fatalf("off-map %v found something", c)
		}
	}
	if g.layerBldgAt(0, 0, construction.Layer(9)) != 0 || g.layerFrameAt(0, 0, construction.Layer(9)) != 0 {
		t.Fatal("unknown layer found something")
	}
}

func TestIndexFollowsMovesAndLoad(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 12})
	if !g.BuildNow("table", "wood", "south", 2, 2) {
		t.Fatal("table")
	}
	bid := g.layerBldgAt(2, 2, construction.LayerObject)
	g.moveBuilding(bid, construction.South, 7, 7)
	fid := g.layerFrameAt(7, 7, construction.LayerObject)
	if fid == 0 {
		t.Fatal("reinstall frame not indexed")
	}
	fr, _ := g.frames.Get(fid)
	g.completeFrame(fid, fr)
	if g.layerBldgAt(2, 2, construction.LayerObject) != 0 || g.layerBldgAt(7, 7, construction.LayerObject) != bid {
		t.Fatal("moved building not found at its new place only")
	}
	var buf bytes.Buffer
	if err := g.Save(&buf, "idx"); err != nil {
		t.Fatal(err)
	}
	l, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if l.layerBldgAt(7, 7, construction.LayerObject) != bid {
		t.Fatal("index not rebuilt after Load")
	}
}

// sameSlice reports two views sharing one backing array (both empty
// counts as shared: there is nothing to tell apart).
func sameSlice[T any](a, b []T) bool {
	if len(a) == 0 || len(b) == 0 {
		return len(a) == len(b)
	}
	return &a[0] == &b[0]
}

// identityIffEqual fails unless each World() list of b keeps a's slice
// exactly when it reads the same (PERF-002: the renderer's identity memos).
func identityIffEqual(t *testing.T, when string, a, b WorldView) {
	t.Helper()
	check := func(name string, same, equal bool) {
		t.Helper()
		if same != equal {
			t.Errorf("%s: %s shared %v but equal %v", when, name, same, equal)
		}
	}
	check("Items", sameSlice(a.Items, b.Items), reflect.DeepEqual(a.Items, b.Items))
	check("Plants", sameSlice(a.Plants, b.Plants), reflect.DeepEqual(a.Plants, b.Plants))
	check("Buildings", sameSlice(a.Buildings, b.Buildings), reflect.DeepEqual(a.Buildings, b.Buildings))
	check("Frames", sameSlice(a.Frames, b.Frames), reflect.DeepEqual(a.Frames, b.Frames))
	check("Zones", sameSlice(a.Zones, b.Zones), reflect.DeepEqual(a.Zones, b.Zones))
}

func TestViewCache(t *testing.T) {
	g := NewColony(Config{Seed: 2, Width: 48, Height: 40})
	g.Step(5)
	plain := g.World()
	g.SetViewCache(true)
	a := g.World()
	b := g.World()
	if !reflect.DeepEqual(plain, a) {
		t.Fatal("cached World differs from the uncached one")
	}
	if &a.Items[0] != &b.Items[0] {
		t.Fatal("same-tick World not shared")
	}
	pa, pb := g.Snapshot(), g.Snapshot()
	if &pa[0] != &pb[0] {
		t.Fatal("same-tick Snapshot not shared")
	}
	g.Enqueue(command.Command{Kind: command.DesignateStockpile, X: 1, Y: 1, X2: 2, Y2: 2})
	g.ApplyPending()
	c := g.World()
	if len(c.Zones) == len(a.Zones) {
		t.Fatal("ApplyPending did not refresh the cached World")
	}
	// a tick refreshes the read model; a list keeps its slice exactly
	// when it reads the same
	for i := 0; i < 20; i++ {
		g.Step(1)
		d := g.World()
		if p := g.Snapshot(); &p[0] == &pa[0] {
			t.Fatal("Step did not refresh the Snapshot")
		}
		identityIffEqual(t, "after a tick", c, d)
		c = d
	}

	// forced changes move the list they touch
	if _, err := g.SpawnStack(item.Wood, 5, 20, 20); err != nil {
		t.Fatal(err)
	}
	d := g.World()
	if sameSlice(c.Items, d.Items) || len(d.Items) != len(c.Items)+1 {
		t.Fatal("a new stack kept the item list")
	}
	identityIffEqual(t, "after SpawnStack", c, d)
	c = d

	var wild PlantView
	for _, p := range c.Plants {
		if !p.Cut && p.Zone == "" && g.CanZoneAt("grow", p.X, p.Y) {
			wild = p
			break
		}
	}
	if wild.ID == 0 {
		t.Fatal("no zoneable wild plant on the map")
	}
	g.Enqueue(command.Command{Kind: command.DesignateCut, X: wild.X, Y: wild.Y, X2: wild.X, Y2: wild.Y})
	g.ApplyPending()
	d = g.World()
	if sameSlice(c.Plants, d.Plants) || !plantOf(d.Plants, wild.ID).Cut {
		t.Fatal("a cut mark kept the plant list")
	}
	identityIffEqual(t, "after DesignateCut", c, d)
	c = d

	g.Enqueue(command.Command{Kind: command.DesignateGrowZone, X: wild.X, Y: wild.Y, X2: wild.X, Y2: wild.Y, Build: uint8(plant.Potato)})
	g.ApplyPending()
	d = g.World()
	if sameSlice(c.Zones, d.Zones) || len(d.Zones) != len(c.Zones)+1 {
		t.Fatal("a new grow zone kept the zone list")
	}
	if sameSlice(c.Plants, d.Plants) || plantOf(d.Plants, wild.ID).Zone != "potato" {
		t.Fatalf("the plant under a new grow zone: %+v", plantOf(d.Plants, wild.ID))
	}
	identityIffEqual(t, "after DesignateGrowZone", c, d)

	g.SetViewCache(false)
	if c, d := g.World(), g.World(); &c.Items[0] == &d.Items[0] {
		t.Fatal("cache off but World shared")
	}
}

// plantOf is plant id's view in ps (zero when absent).
func plantOf(ps []PlantView, id entity.EntityID) PlantView {
	for _, p := range ps {
		if p.ID == id {
			return p
		}
	}
	return PlantView{}
}

func TestSharedLayersCopyOnWrite(t *testing.T) {
	g := NewColony(Config{Seed: 2, Width: 48, Height: 40})
	a := g.World()
	g.Step(1)
	b := g.World()
	if &a.Terrain[0] != &b.Terrain[0] || &a.RoomGrid[0] != &b.RoomGrid[0] || &a.Grass[0] != &b.Grass[0] || &a.Home[0] != &b.Home[0] {
		t.Fatal("unchanged layers were copied")
	}
	old := slices.Clone(a.Terrain)
	if !g.SetGround(0, 0, 3, 3, "rock", "granite", "") {
		t.Fatal("SetGround")
	}
	c := g.World()
	if &c.Terrain[0] == &a.Terrain[0] || !slices.Equal(a.Terrain, old) {
		t.Fatal("terrain change wrote into a slice already handed out")
	}
	if c.Terrain[0] == old[0] {
		t.Fatal("terrain view not updated")
	}
}

func TestShareLayer(t *testing.T) {
	var c layerCache
	if shareLayer(&c, []uint8(nil)) != nil {
		t.Fatal("empty layer not nil")
	}
	src := []uint8{1, 2, 3}
	a := shareLayer(&c, src)
	if b := shareLayer(&c, src); &a[0] != &b[0] {
		t.Fatal("unchanged layer copied")
	}
	src[1] = 9
	b := shareLayer(&c, src)
	if &a[0] == &b[0] || a[1] != 2 || b[1] != 9 {
		t.Fatal("changed layer not copied on write")
	}
	if d := shareLayer(&c, []uint8{1, 9}); len(d) != 2 {
		t.Fatal("resized layer")
	}
}

func TestTileSetBounds(t *testing.T) {
	s := newTileSet(70)
	s.add(-1)
	s.add(69)
	s.add(500)
	if !s.has(69) || s.has(-1) || s.has(500) || s.has(68) {
		t.Fatal("tileSet bounds")
	}
}

func TestDropIDsKeepsOrder(t *testing.T) {
	ids := []entity.EntityID{5, 1, 4, 1, 3}
	got := dropIDs(ids, func(id entity.EntityID) bool { return id == 1 })
	if !slices.Equal(got, []entity.EntityID{5, 4, 3}) {
		t.Fatalf("dropIDs = %v", got)
	}
}

// TestSpillPingPongEnds: two unmergeable same-kind stacks side by side
// (corpses of different species) used to bounce a third between them
// until the stack overflowed; it now comes to rest.
func TestSpillPingPongEnds(t *testing.T) {
	g := New(Config{Seed: 1, Width: 9, Height: 9})
	for y := 0; y < 9; y++ {
		for x := 0; x < 9; x++ {
			if !(x == 4 && (y == 4 || y == 5)) {
				g.grid.SetWalkable(x, y, false)
			}
		}
	}
	a, _ := g.SpawnStack(item.CorpseAnimal, 1, 4, 4)
	b, _ := g.SpawnStack(item.CorpseAnimal, 1, 4, 5)
	for i, id := range []entity.EntityID{a, b} {
		st, _ := g.stacks.Get(id)
		st.Animal = uint8(i)
		g.putStack(id, st)
	}
	c, _ := g.SpawnStack(item.CorpseAnimal, 1, 4, 4)
	st, _ := g.stacks.Get(c)
	st.Animal = 2
	g.mergeOrSet(c, st)
	if got, _ := g.stacks.Get(c); got.X != 4 || (got.Y != 4 && got.Y != 5) {
		t.Fatalf("stack ended at %d,%d", got.X, got.Y)
	}
}
