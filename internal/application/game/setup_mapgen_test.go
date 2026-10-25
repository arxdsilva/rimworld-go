package game

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/mapgen/pipeline"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/terrain"
)

// siteFixture is a hand-made generator Result on an n x n soil map: a
// granite rock block (60..79)^2 under a natural roof, thick in its middle
// (65..74)^2; a chunk at (30,30); a wooden shed ruin by (40,20); a steam
// vent at (70,30); a berry bush at (10,10); grass only at (11,11); the
// landing spot at (20,20).
func siteFixture(n int) mapgen.Result {
	res := mapgen.Result{W: n, H: n, Terrain: terrain.NewMap(n, n), Geology: geology.NewMap(n, n),
		Roof: make([]roof.Cell, n*n), Grass: make([]bool, n*n), Start: mapgen.Point{X: 20, Y: 20}}
	for y := 60; y < 80; y++ {
		for x := 60; x < 80; x++ {
			i := y*n + x
			res.Terrain.Set(x, y, terrain.Rock)
			res.Geology.Set(x, y, geology.Granite)
			res.Roof[i] = roof.Natural
			if x >= 65 && x < 75 && y >= 65 && y < 75 {
				res.Roof[i] = roof.Natural | roof.NaturalThick
			}
		}
	}
	res.Chunks = []mapgen.ChunkSpawn{{X: 30, Y: 30, Stone: geology.Granite}}
	res.Ruins = []mapgen.Ruin{{Kind: mapgen.Shed, X0: 40, Y0: 20, X1: 42, Y1: 21,
		Walls:  []mapgen.RuinWall{{X: 40, Y: 20, Stuff: item.Wood, DamagePct: 50}, {X: 41, Y: 20, Stuff: item.GraniteBlocks, DamagePct: 80}},
		Floors: []mapgen.RuinFloor{{X: 40, Y: 21, Stuff: item.Wood}},
		Loot:   []mapgen.Loot{{X: 41, Y: 21, Kind: item.Steel, Qty: 20}},
		Rubble: []mapgen.ChunkSpawn{{X: 42, Y: 21, Stone: geology.Granite}},
	}}
	res.Vents = []mapgen.Point{{X: 70, Y: 30}}
	res.Plants = []mapgen.PlantSpawn{{X: 10, Y: 10, Species: plant.BerryBush, Growth: 500}}
	res.Grass[11*n+11] = true
	return res
}

// siteGame builds scenario key's Setup colony on a 100x100 map from the
// fixture (edit adjusts it) the way the builder does after generation.
func siteGame(t testing.TB, key string, edit func(*mapgen.Result)) *Game {
	t.Helper()
	s := setupFor(t, key, func(s *Setup) { s.MapSize = 100 })
	g := newSetupGame(Config{Setup: s}, s)
	res := siteFixture(100)
	if edit != nil {
		edit(&res)
	}
	g.applySite(res)
	g.finishSetup()
	return g
}

// bldgAt is the building of kind k with its origin at (x, y).
func bldgAt(g *Game, k construction.Kind, x, y int) (construction.Building, bool) {
	for _, id := range g.bldgIDs {
		if b, ok := g.bldgs.Get(id); ok && b.Kind == k && b.X == x && b.Y == y {
			return b, true
		}
	}
	return construction.Building{}, false
}

func bldgIDAt(g *Game, k construction.Kind, x, y int) entity.EntityID {
	for _, bid := range g.bldgIDs {
		if b, ok := g.bldgs.Get(bid); ok && b.Kind == k && b.X == x && b.Y == y {
			return bid
		}
	}
	return 0
}

func stackAt(g *Game, x, y int) (item.Kind, int) {
	if id := g.itemAt(x, y); id != 0 {
		st, _ := g.stacks.Get(id)
		return st.Kind, st.Qty
	}
	return 0, 0
}

// hasMessage: wildlife_test.go.

func TestSetupBuilderUsesMapgen(t *testing.T) {
	t.Parallel()
	s := setupFor(t, "broken_hull", func(s *Setup) { s.MapSize = 64 })
	g := buildSetup(t, s)
	if g.ng.setup.GenVersion != mapgen.Version {
		t.Fatalf("GenVersion %d, want %d", g.ng.setup.GenVersion, mapgen.Version)
	}
	if s.GenVersion != 0 {
		t.Fatal("the wizard's Setup was changed: the game keeps its own copy")
	}
	res, err := pipeline.Generate(mapgen.Config{Seed: s.Seed, W: 64, H: 64, Site: s.Site, Biome: biome.DefOf(s.Site.Biome)})
	if err != nil {
		t.Fatal(err)
	}
	if x, y := g.landingSpot(); !g.ng.hasLanding || x != res.Start.X || y != res.Start.Y {
		t.Fatalf("landing (%d,%d), generator start %v", x, y, res.Start)
	}
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			if g.terr.At(x, y) != res.Terrain.At(x, y) && !g.hasBuildingCell(x, y) {
				t.Fatalf("terrain at (%d,%d) is %v, generated %v", x, y, g.terr.At(x, y), res.Terrain.At(x, y))
			}
		}
	}
}

// hasBuildingCell: a building covers (x, y) (a pod shell may have been
// placed there, but terrain is never changed by one).
func (g *Game) hasBuildingCell(x, y int) bool {
	return g.layerBldgAt(x, y, construction.LayerObject) != 0 || g.layerBldgAt(x, y, construction.LayerFloor) != 0
}

func TestApplySiteLaysTheFixture(t *testing.T) {
	t.Parallel()
	g := siteGame(t, "broken_hull", nil)
	if x, y := g.landingSpot(); x != 20 || y != 20 {
		t.Fatalf("landing (%d,%d), want (20,20)", x, y)
	}
	if l := g.World().Landing; l != [2]int{20, 20} {
		t.Fatalf("view landing %v", l)
	}
	// terrain and geology
	if g.terr.At(70, 70) != terrain.Rock || g.geo.At(70, 70) != geology.Granite || g.terr.At(5, 5) != terrain.Soil {
		t.Fatal("terrain or geology not laid")
	}
	// roofs: thin over the rock rim, thick in the middle, none outside
	if c := g.roofAt(61, 61); !c.Has(roof.Natural) || c.Thick() {
		t.Fatalf("rim roof %b", c)
	}
	if c := g.roofAt(70, 70); !c.Thick() {
		t.Fatalf("deep roof %b", c)
	}
	if g.roofAt(5, 5).Roofed() {
		t.Fatal("open ground is roofed")
	}
	// a chunk
	if k, q := stackAt(g, 30, 30); k != item.GraniteChunk || q != 1 {
		t.Fatalf("chunk: %v x%d", k, q)
	}
	// the ruin: damaged walls, an old floor, loot and rubble
	w1, ok := bldgAt(g, construction.Wall, 40, 20)
	if !ok || !w1.Ruin || w1.Stuff != item.Wood || w1.Damage != construction.MaxHP(construction.Wall, item.Wood)*50/100 {
		t.Fatalf("ruin wall %+v %v", w1, ok)
	}
	w2, ok := bldgAt(g, construction.Wall, 41, 20)
	if !ok || !w2.Ruin || w2.Damage != construction.MaxHP(construction.Wall, item.GraniteBlocks)*80/100 || w2.HP() <= 0 {
		t.Fatalf("stone ruin wall %+v", w2)
	}
	if g.grid.Walkable(40, 20) {
		t.Fatal("a ruin wall still blocks")
	}
	if f, ok := bldgAt(g, construction.Floor, 40, 21); !ok || !f.Ruin || f.Stuff != item.Wood {
		t.Fatalf("ruin floor %+v %v", f, ok)
	}
	if k, q := stackAt(g, 41, 21); k != item.Steel || q != 20 {
		t.Fatalf("loot %v x%d", k, q)
	}
	if k, _ := stackAt(g, 42, 21); k != item.GraniteChunk {
		t.Fatalf("rubble %v", k)
	}
	ruinView := false
	for _, bv := range g.World().Buildings {
		if bv.X == 40 && bv.Y == 20 && bv.Kind == "wall" {
			ruinView = bv.Ruin && bv.HPPct == 50
		}
	}
	if !ruinView {
		t.Fatal("the view does not show the ruin")
	}
	// the vent, walkable
	if _, ok := bldgAt(g, construction.SteamVent, 70, 30); !ok || !g.grid.Walkable(71, 31) {
		t.Fatal("no walkable steam vent")
	}
	// plants and grass
	if pid := g.plantAt(10, 10); pid == 0 {
		t.Fatal("no bush")
	} else if p, _ := g.plants.Get(pid); p.Species != plant.BerryBush || p.Growth != 500 {
		t.Fatalf("bush %+v", p)
	}
	if !g.hasGrass(11, 11) || g.hasGrass(12, 12) || g.hasGrass(10, 10) {
		t.Fatal("grass is not the generator's mask")
	}
}

func TestApplySiteSkipsBadPieces(t *testing.T) {
	t.Parallel()
	g := siteGame(t, "broken_hull", func(r *mapgen.Result) {
		r.Ruins[0].Walls = append(r.Ruins[0].Walls, mapgen.RuinWall{X: 70, Y: 70, Stuff: item.Wood}) // in rock
		r.Chunks = append(r.Chunks, mapgen.ChunkSpawn{X: 61, Y: 61, Stone: geology.Granite}, mapgen.ChunkSpawn{X: 5, Y: 5})
		r.Roof[3] = roof.NaturalThick // thick alone still means natural
	})
	if _, ok := bldgAt(g, construction.Wall, 70, 70); ok {
		t.Fatal("a ruin wall inside rock")
	}
	if k, _ := stackAt(g, 61, 61); k != 0 {
		t.Fatal("a chunk inside rock")
	}
	if k, _ := stackAt(g, 5, 5); k != 0 {
		t.Fatal("a chunk without stone")
	}
	if c := g.roofAt(3, 0); !c.Has(roof.Natural) || !c.Thick() {
		t.Fatalf("thick-only roof %b", c)
	}
}

func TestApplySiteWrongSizeKeepsPlainMap(t *testing.T) {
	t.Parallel()
	s := setupFor(t, "broken_hull", func(s *Setup) { s.MapSize = 50 })
	g := newSetupGame(Config{Setup: s}, s)
	g.applySite(mapgen.Result{})
	if g.ng.hasLanding {
		t.Fatal("a failed generator has no landing spot")
	}
	if x, y := g.landingSpot(); x != 25 || y != 25 {
		t.Fatalf("landing (%d,%d), want the centre", x, y)
	}
	if g.geo.W != 50 || len(g.geo.Stone) != 50*50 {
		t.Fatal("no geology")
	}
}

func TestPodsLandAroundLandingSpot(t *testing.T) {
	t.Parallel()
	g := siteGame(t, "broken_hull", nil)
	if len(g.ng.pods) < 4 {
		t.Fatalf("%d pods", len(g.ng.pods))
	}
	land := Position{20, 20}
	for _, p := range g.ng.pods {
		if chebyshev(Position{p.X, p.Y}, land) > podRadius {
			t.Fatalf("pod at (%d,%d), not around the landing spot", p.X, p.Y)
		}
	}
	stepUntilLanded(t, g)
	for _, id := range g.pawnIDs {
		p, _ := g.positions.Get(id)
		if chebyshev(p, land) > podRadius+3+8 {
			t.Fatalf("colonist at %v, far from the landing spot", p)
		}
	}
	if m := g.messages; len(m) == 0 || m[len(m)-1].X != 20 || m[len(m)-1].Y != 20 {
		t.Fatalf("intro letter not at the landing spot: %+v", m)
	}
}

func TestPodsNeverUnderThickRoof(t *testing.T) {
	t.Parallel()
	g := siteGame(t, "broken_hull", func(r *mapgen.Result) {
		// open ground left of and on the landing spot is under thick rock
		for y := 10; y <= 30; y++ {
			for x := 10; x <= 20; x++ {
				r.Roof[y*100+x] = roof.Natural | roof.NaturalThick
			}
		}
	})
	if len(g.ng.pods) == 0 {
		t.Fatal("no pods")
	}
	for _, p := range g.ng.pods {
		if g.roofAt(p.X, p.Y).Thick() {
			t.Fatalf("pod at (%d,%d) under thick rock", p.X, p.Y)
		}
	}
}

func TestStandingArrivalAroundLandingSpot(t *testing.T) {
	t.Parallel()
	g := siteGame(t, "ember_clan", nil)
	if len(g.pawnIDs) == 0 {
		t.Fatal("nobody arrived")
	}
	for _, id := range g.pawnIDs {
		p, _ := g.positions.Get(id)
		if d := chebyshev(p, Position{20, 20}); d < 2 || d > 6 {
			t.Fatalf("clansperson at %v, not in the ring round the landing spot", p)
		}
	}
}

func TestStartHomeAroundLandingSpot(t *testing.T) {
	t.Parallel()
	g := siteGame(t, "broken_hull", nil)
	w := g.Width()
	if !g.inHome(20*w+20) || !g.inHome((20+startHomeHalf)*w+20) {
		t.Fatal("no Home at the landing spot")
	}
	if g.inHome(50*w + 50) {
		t.Fatal("Home at the map centre")
	}
	if g.inHome(20*w + 40) {
		t.Fatal("Home round the ruin")
	}
}

func TestThickRoofCannotBeRemoved(t *testing.T) {
	t.Parallel()
	g := siteGame(t, "broken_hull", nil)
	g.designateRoof(64, 64, 70, 70, false)
	if c := g.roofAt(70, 70); c.Has(roof.PlanRemove) || !c.Thick() {
		t.Fatalf("thick roof %b", c)
	}
	if !hasMessage(g, ReasonThickRoof) {
		t.Fatal("no thick-rock notice")
	}
	n := len(g.messages)
	g.designateRoof(60, 60, 62, 62, false) // thin natural: refused quietly, as before
	if len(g.messages) != n {
		t.Fatal("a thin natural roof posted the thick notice")
	}
	// mining keeps the flags; thick rock never collapses
	g.roofOnMined(70, 70)
	if !g.roofAt(70, 70).Thick() {
		t.Fatal("mining dropped the thick roof")
	}
	g.roofx.unsupported = true
	g.checkRoofSupport()
	if !g.roofAt(70, 70).Thick() || !g.roofAt(61, 61).Has(roof.Natural) {
		t.Fatal("a natural roof collapsed")
	}
	// saves keep the flag
	h := saveLoad(t, g)
	if !h.roofAt(70, 70).Thick() || h.roofAt(61, 61).Thick() {
		t.Fatal("thick roofs lost in a save")
	}
}

func TestVentWarmsTheAirAroundIt(t *testing.T) {
	t.Parallel()
	g := siteGame(t, "broken_hull", nil)
	g.ensureRooms()
	ctrl := g.temperatureAt(90, 10)
	for _, c := range [][2]int{{70, 30}, {71, 31}, {68, 28}, {73, 33}} {
		if d := g.temperatureAt(c[0], c[1]) - ctrl; d != ventOutdoorHeat {
			t.Fatalf("(%d,%d) is %d warmer, want %d", c[0], c[1], d, ventOutdoorHeat)
		}
	}
	for _, c := range [][2]int{{67, 30}, {74, 30}, {70, 27}, {70, 34}} {
		if d := g.temperatureAt(c[0], c[1]) - ctrl; d != 0 {
			t.Fatalf("(%d,%d) is %d warmer, beyond the vent's reach", c[0], c[1], d)
		}
	}
}

// walledRoom rings the interior (x0..x1, y0..y1) with wooden walls.
func walledRoom(g *Game, x0, y0, x1, y1 int) {
	for x := x0 - 1; x <= x1+1; x++ {
		g.placeGenerated(construction.Building{Kind: construction.Wall, Stuff: item.Wood, X: x, Y: y0 - 1})
		g.placeGenerated(construction.Building{Kind: construction.Wall, Stuff: item.Wood, X: x, Y: y1 + 1})
	}
	for y := y0; y <= y1; y++ {
		g.placeGenerated(construction.Building{Kind: construction.Wall, Stuff: item.Wood, X: x0 - 1, Y: y})
		g.placeGenerated(construction.Building{Kind: construction.Wall, Stuff: item.Wood, X: x1 + 1, Y: y})
	}
}

func TestVentWarmsItsRoom(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 3, Width: 40, Height: 40})
	walledRoom(g, 2, 2, 6, 6)
	walledRoom(g, 22, 2, 26, 6)
	g.placeGenerated(construction.Building{Kind: construction.SteamVent, Stuff: item.GraniteBlocks, X: 3, Y: 3})
	g.RoofRooms()
	g.ensureRooms()
	a, b := g.roomAt(4, 4), g.roomAt(24, 4)
	if a < 0 || b < 0 || a == b {
		t.Fatalf("rooms %d %d", a, b)
	}
	cold := g.climate.Outdoor(g.clock.Now()) - 300
	g.roomTemps[a], g.roomTemps[b] = cold, cold
	for range 400 {
		g.heatRooms()
	}
	if d := g.roomTemps[a] - g.roomTemps[b]; d < ventRoomHeat-3 || d > ventRoomHeat+3 {
		t.Fatalf("the vent's room is %d warmer than its twin, want about %d", d, ventRoomHeat)
	}
	if d := g.temperatureAt(4, 4) - g.temperatureAt(24, 4); d != g.roomTemps[a]-g.roomTemps[b] {
		t.Fatal("indoors the outdoor warming must not apply")
	}
}

func TestVentIsANaturalFeature(t *testing.T) {
	t.Parallel()
	g := siteGame(t, "broken_hull", nil)
	vid := bldgIDAt(g, construction.SteamVent, 70, 30)
	if vid == 0 {
		t.Fatal("no vent")
	}
	g.markDeconstruct(68, 28, 73, 33, true)
	if b, _ := bldgAt(g, construction.SteamVent, 70, 30); b.Deconstruct {
		t.Fatal("a vent marked for deconstruction")
	}
	if !hasMessage(g, "natural feature") {
		t.Fatal("no refusal notice")
	}
	if ok, why := g.CanMoveTo(vid, "south", 80, 30); ok || why != ReasonNatural {
		t.Fatalf("move: %v %q", ok, why)
	}
	for _, k := range []construction.Kind{construction.Wall, construction.Floor, construction.Bed} {
		if ok, why := g.canPlaceAt(k, construction.South, 71, 31); ok || why != ReasonNatural {
			t.Fatalf("%v over the vent: %v %q", k, ok, why)
		}
	}
	if ok, why := g.canPlaceStuffAt(construction.SteamVent, item.Wood, construction.South, 70, 30); ok || why != ReasonNatural {
		t.Fatalf("vent swap: %v %q", ok, why)
	}
}

func TestRuinsAreNotTheColonys(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 3, Width: 40, Height: 40})
	w0 := g.Wealth()
	rb := g.placeGenerated(construction.Building{Kind: construction.Bed, Stuff: item.Wood, X: 5, Y: 5, Ruin: true, Damage: 10})
	vent := g.placeGenerated(construction.Building{Kind: construction.SteamVent, Stuff: item.GraniteBlocks, X: 20, Y: 20})
	if rb == 0 || vent == 0 {
		t.Fatal("not placed")
	}
	if g.Wealth() != w0 {
		t.Fatalf("a ruin adds wealth: %d -> %d", w0, g.Wealth())
	}
	nb := g.placeGenerated(construction.Building{Kind: construction.Bed, Stuff: item.Wood, X: 10, Y: 5})
	if g.Wealth() <= w0 {
		t.Fatal("a colony bed adds no wealth")
	}
	// no Home round ruins or vents
	g.importHome(nil, 0)
	if w := g.Width(); g.inHome(5*w+5) || g.inHome(20*w+20) || !g.inHome(5*w+10) {
		t.Fatal("Home anchoring: ruins and vents must not anchor, colony pieces must")
	}
	// nobody mends a ruin
	pid, err := g.SpawnNamedPawn(8, 8, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	if g.assignRepair(pid) {
		t.Fatal("a builder went to repair a ruin")
	}
	// half the refund: a bed gives back 2 wood, a ruined one 1
	full := construction.Refund(construction.Bed)
	g.removeBuilding(rb)
	if k, q := stackAt(g, 5, 5); k != item.Wood || q != full/2 {
		t.Fatalf("ruin refund %v x%d, want wood x%d", k, q, full/2)
	}
	g.removeBuilding(nb)
	if k, q := stackAt(g, 10, 5); k != item.Wood || q != full {
		t.Fatalf("refund %v x%d, want wood x%d", k, q, full)
	}
}

func TestRebuiltRuinIsTheColonys(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 3, Width: 40, Height: 40})
	id := g.placeGenerated(construction.Building{Kind: construction.Bed, Stuff: item.Wood, X: 5, Y: 5, Ruin: true, Damage: 10})
	b, _ := g.bldgs.Get(id)
	g.completeSwap(0, construction.Frame{Kind: construction.Bed, Stuff: item.Steel, X: 5, Y: 5, Swaps: id}, b)
	nb, _ := g.bldgs.Get(id)
	if nb.Ruin || nb.Damage != 0 || nb.Stuff != item.Steel {
		t.Fatalf("swapped ruin %+v", nb)
	}
	if k, q := stackAt(g, 5, 5); k != item.Wood || q != construction.Refund(construction.Bed)/2 {
		t.Fatalf("old material back %v x%d", k, q)
	}
}

func TestLandingSaveRoundTrip(t *testing.T) {
	t.Parallel()
	g := siteGame(t, "broken_hull", nil)
	g.Step(60) // rooms and support are derived on the first tick
	h := saveLoad(t, g)
	if g.StateHash() != h.StateHash() {
		t.Fatal("a site colony changes in a save round trip")
	}
	if x, y := h.landingSpot(); !h.ng.hasLanding || x != 20 || y != 20 {
		t.Fatalf("loaded landing (%d,%d)", x, y)
	}
	if b, ok := bldgAt(h, construction.Wall, 40, 20); !ok || !b.Ruin {
		t.Fatal("the ruin flag is lost in a save")
	}
	h.ensureRooms()
	if h.temperatureAt(70, 30)-h.temperatureAt(90, 10) != ventOutdoorHeat {
		t.Fatal("a loaded vent does not warm")
	}
	if h.ng.setup.GenVersion != g.ng.setup.GenVersion {
		t.Fatal("GenVersion lost")
	}
	// an older save has no landing spot: the map centre
	h.importLanding(nil)
	if x, y := h.landingSpot(); h.ng.hasLanding || x != 50 || y != 50 {
		t.Fatalf("older save landing (%d,%d)", x, y)
	}
	h.importLanding([]int{500, 3})
	if h.ng.hasLanding {
		t.Fatal("an off-map landing spot accepted")
	}
	// a legacy colony saves none and keeps the centre
	l := NewColony(Config{Seed: 5, Width: 60, Height: 60})
	if l.exportLanding() != nil {
		t.Fatal("a legacy colony saved a landing spot")
	}
	if x, y := saveLoad(t, l).landingSpot(); x != 30 || y != 30 {
		t.Fatalf("legacy landing (%d,%d)", x, y)
	}
}

// A ruined piece salvages half its normal refund, rounded up, so a broken
// wall still gives something back.
func TestRuinRefundHalfRoundedUp(t *testing.T) {
	for _, k := range []construction.Kind{construction.Wall, construction.Floor, construction.Table} {
		full := construction.Refund(k)
		got := refundOf(construction.Building{Kind: k, Ruin: true})
		if want := (full + 1) / 2; got != want || (full > 0 && got == 0) {
			t.Fatalf("%v: ruin refund %d of %d, want %d", k, got, full, want)
		}
		if refundOf(construction.Building{Kind: k}) != full {
			t.Fatalf("%v: an ordinary piece refunds in full", k)
		}
	}
}
