package start

import (
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/mapgen/steps/roads"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/site"
	"rimworld-go/internal/domain/world/terrain"
)

// roadFixture is an all-soil map with one road from the north edge to the
// junction near the middle, the start beside the junction.
func roadFixture(t *testing.T, seed uint64, w, h int) *mapgen.Ctx {
	t.Helper()
	c := fixture(seed, w, h)
	c.Cfg.Site.Road = site.Dir8Set(1 << 0)
	if err := (roads.Roads{}).Run(c); err != nil {
		t.Fatal(err)
	}
	if len(c.F.RoadEnds) != 1 {
		t.Fatalf("fixture road ends %v", c.F.RoadEnds)
	}
	j := c.F.RoadEnds[0][1]
	c.R.Start = mapgen.Point{X: j.X, Y: min(h-1, j.Y+8)}
	return c
}

// roadCellAtRow is a Track cell of the road on row y.
func roadCellAtRow(t *testing.T, c *mapgen.Ctx, y int) mapgen.Point {
	t.Helper()
	for x := range c.W {
		if c.F.Road[c.Idx(x, y)] && c.R.Terrain.At(x, y) == terrain.Track {
			return mapgen.Point{X: x, Y: y}
		}
	}
	t.Fatalf("no road on row %d", y)
	return mapgen.Point{}
}

func repairWith(c *mapgen.Ctx, sub string) bool {
	return slices.ContainsFunc(c.R.Metrics.Repairs, func(s string) bool { return strings.Contains(s, sub) })
}

func TestValidateLeavesConnectedRoadsAlone(t *testing.T) {
	c := roadFixture(t, 31, 100, 100)
	before := slices.Clone(c.R.Terrain.Cells)
	validate(t, c)
	if repairWith(c, "road") {
		t.Fatalf("connected road repaired: %v", c.R.Metrics.Repairs)
	}
	if !slices.Equal(before, c.R.Terrain.Cells) {
		t.Fatal("terrain changed under a connected road")
	}
	if b := BrokenRoads(c); len(b) != 0 {
		t.Fatalf("broken roads %v", b)
	}
}

func TestValidateReroutesRoadBrokenByRock(t *testing.T) {
	c := roadFixture(t, 32, 100, 100)
	p := roadCellAtRow(t, c, 25)
	within(c, p, 4, func(x, y, _ int) { setRock(c, x, y) })
	c.F.RockDepth = mapgen.ChebDepth(c.W, c.H, c.F.Rock)
	if b := BrokenRoads(c); !slices.Equal(b, []int{0}) {
		t.Fatalf("break not seen: %v", b)
	}
	validate(t, c)
	if !hasRepair(c, "road 1 re-routed") {
		t.Fatalf("repairs %v", c.R.Metrics.Repairs)
	}
	if b := BrokenRoads(c); len(b) != 0 {
		t.Fatalf("still broken: %v", b)
	}
	if v := Violations(c); len(v) > 0 {
		t.Fatalf("violations %v", v)
	}
	for i := range c.F.Road {
		if c.F.Road[i] && terrain.IsRockWall(c.R.Terrain.Cells[i]) {
			t.Fatal("road cell left on rock")
		}
	}
}

func TestValidateReroutesRoadAcrossWaterGap(t *testing.T) {
	c := roadFixture(t, 33, 100, 100)
	// a river now cuts the road: deep water across the whole map
	for x := range c.W {
		for y := 24; y <= 27; y++ {
			setWater(c, x, y, mapgen.Deep)
			c.F.River[c.Idx(x, y)] = true
		}
	}
	validate(t, c)
	if !hasRepair(c, "road 1 re-routed") {
		t.Fatalf("repairs %v", c.R.Metrics.Repairs)
	}
	if b := BrokenRoads(c); len(b) != 0 {
		t.Fatalf("still broken: %v", b)
	}
	if len(c.R.Bridges) == 0 {
		t.Fatal("the river was not bridged")
	}
	for _, b := range c.R.Bridges {
		if !terrain.Bridgeable(c.R.Terrain.At(b.X, b.Y)) || !c.F.Road[c.Idx(b.X, b.Y)] {
			t.Fatalf("bridge %v on %v", b, c.R.Terrain.At(b.X, b.Y))
		}
	}
	if v := Violations(c); len(v) > 0 {
		t.Fatalf("violations %v", v)
	}
}

func TestValidateRevertsUnrepairableRoad(t *testing.T) {
	c := roadFixture(t, 34, 100, 100)
	// the open sea now cuts the road off from its edge
	for x := range c.W {
		for y := 20; y <= 24; y++ {
			setWater(c, x, y, mapgen.Deep)
			c.F.Coast[c.Idx(x, y)] = true
			c.R.Ocean[c.Idx(x, y)] = true
		}
	}
	validate(t, c)
	if !slices.ContainsFunc(c.R.Metrics.Repairs, func(s string) bool {
		return strings.HasPrefix(s, "road 1") && strings.Contains(s, "reverted")
	}) {
		t.Fatalf("repairs %v", c.R.Metrics.Repairs)
	}
	for i := range c.F.Road {
		if x, y := c.XY(i); y < 20 && (c.F.Road[i] || c.R.Terrain.Cells[i] == terrain.Track) {
			t.Fatalf("dangling road left at %d,%d", x, y)
		}
	}
	if len(c.F.RoadEnds) != 0 || len(c.R.Roads) != 0 {
		t.Fatalf("reverted road still listed: %v", c.F.RoadEnds)
	}
	if b := BrokenRoads(c); len(b) != 0 {
		t.Fatalf("broken %v", b)
	}
	if v := Violations(c); len(v) > 0 {
		t.Fatalf("violations %v", v)
	}
}

func TestValidateBridgesAreWalkable(t *testing.T) {
	c := fixture(35, 100, 100)
	for x := range c.W {
		for y := 46; y <= 53; y++ {
			setWater(c, x, y, mapgen.Deep)
		}
	}
	for y := 45; y <= 54; y++ {
		for x := 50; x <= 51; x++ {
			i := c.Idx(x, y)
			c.F.Road[i], c.F.Blocked[i] = true, true
			if y == 45 || y == 54 {
				c.R.Terrain.Cells[i] = terrain.Track
			} else {
				c.R.Bridges = append(c.R.Bridges, mapgen.Point{X: x, Y: y})
			}
		}
	}
	c.R.Start = mapgen.Point{X: 50, Y: 75}
	if v := Violations(c); len(v) > 0 {
		t.Fatalf("water under a bridge reported: %v", v)
	}
	validate(t, c)
	if hasRepair(c, "reach") || c.R.Metrics.StartReach < 0.99 {
		t.Fatalf("bridge not walked: reach %v repairs %v", c.R.Metrics.StartReach, c.R.Metrics.Repairs)
	}
	if len(c.R.Bridges) != 16 {
		t.Fatalf("bridges %d", len(c.R.Bridges))
	}
}

func TestValidateBridgeAndOceanInvariants(t *testing.T) {
	c := fixture(36, 40, 40)
	c.R.Start = mapgen.Point{X: 20, Y: 20}
	setWater(c, 5, 5, mapgen.Deep)
	c.R.Ocean[c.Idx(5, 5)] = true
	setWater(c, 6, 6, mapgen.Shallow)
	c.R.Bridges = []mapgen.Point{
		{X: 6, Y: 6},  // fine
		{X: 6, Y: 6},  // listed twice
		{X: 8, Y: 8},  // on soil
		{X: 5, Y: 5},  // on the open sea
		{X: -1, Y: 3}, // off the map
	}
	c.F.Road[c.Idx(6, 6)], c.F.Blocked[c.Idx(6, 6)] = true, true
	c.R.Ocean[c.Idx(9, 9)] = true // the sea on dry land
	v := Violations(c)
	if len(v) != 1 || !strings.HasPrefix(v[0], "I6: 5 broken") {
		t.Fatalf("violations %v", v)
	}
	validate(t, c)
	if v := Violations(c); len(v) > 0 {
		t.Fatalf("after validate %v", v)
	}
	if !slices.Equal(c.R.Bridges, []mapgen.Point{{X: 6, Y: 6}}) || c.R.Ocean[c.Idx(9, 9)] || !c.R.Ocean[c.Idx(5, 5)] {
		t.Fatalf("bridges %v", c.R.Bridges)
	}
}

// vaultFixture puts a 9x9 vault inside a rock block.
func vaultFixture() (*mapgen.Ctx, mapgen.Vault) {
	c := fixture(37, 60, 60)
	for y := 10; y <= 40; y++ {
		for x := 10; x <= 40; x++ {
			setRock(c, x, y)
		}
	}
	v := mapgen.Vault{ID: 1, X0: 20, Y0: 20, X1: 28, Y1: 28, WallStuff: item.Steel}
	for y := v.Y0; y <= v.Y1; y++ {
		for x := v.X0; x <= v.X1; x++ {
			i := c.Idx(x, y)
			c.R.Terrain.Cells[i] = terrain.StoneSmooth
			c.F.Rock[i], c.F.Blocked[i] = false, true
			if v.Inside(x, y) {
				v.Floors = append(v.Floors, mapgen.RuinFloor{X: x, Y: y, Stuff: item.Steel})
			} else {
				v.Walls = append(v.Walls, mapgen.Point{X: x, Y: y})
			}
		}
	}
	v.Caskets = []mapgen.Casket{{X: 22, Y: 22}}
	v.Loot = []mapgen.Loot{{X: 25, Y: 25, Kind: item.Steel, Qty: 5}}
	v.DefenderSpots = []mapgen.Point{{X: 24, Y: 26}}
	c.R.Vaults = []mapgen.Vault{v}
	c.F.RockDepth = mapgen.ChebDepth(c.W, c.H, c.F.Rock)
	c.R.Start = mapgen.Point{X: 50, Y: 50}
	return c, v
}

func TestValidateKeepsVaults(t *testing.T) {
	c, v := vaultFixture()
	if got := Violations(c); len(got) > 0 {
		t.Fatalf("sound vault reported: %v", got)
	}
	c.R.Plants = []mapgen.PlantSpawn{{X: 24, Y: 24, Species: plant.Tree}}
	c.R.Chunks = []mapgen.ChunkSpawn{{X: 20, Y: 21, Stone: geology.Granite}}
	validate(t, c)
	if got := Violations(c); len(got) > 0 {
		t.Fatalf("violations %v", got)
	}
	if len(c.R.Vaults) != 1 || !slices.Equal(c.R.Vaults[0].Walls, v.Walls) || !slices.Equal(c.R.Vaults[0].Floors, v.Floors) ||
		!slices.Equal(c.R.Vaults[0].Caskets, v.Caskets) || !slices.Equal(c.R.Vaults[0].Loot, v.Loot) ||
		!slices.Equal(c.R.Vaults[0].DefenderSpots, v.DefenderSpots) {
		t.Fatalf("vault changed: %+v", c.R.Vaults)
	}
	if len(c.R.Plants) != 0 || len(c.R.Chunks) != 0 {
		t.Fatal("plant or chunk kept on the vault")
	}
	for y := v.Y0; y <= v.Y1; y++ {
		for x := v.X0; x <= v.X1; x++ {
			i := c.Idx(x, y)
			if c.R.Terrain.Cells[i] != terrain.StoneSmooth || c.R.Geology.Stone[i] != geology.Granite ||
				c.R.Roof[i] != roof.Natural || !c.F.Blocked[i] {
				t.Fatalf("vault cell %d,%d repaired away", x, y)
			}
		}
	}
	// the sealed interior is not land the start must reach: nothing carved
	if hasRepair(c, "reach") {
		t.Fatalf("carved to the vault: %v", c.R.Metrics.Repairs)
	}
}

func TestValidateDropsBrokenVaultPieces(t *testing.T) {
	c, _ := vaultFixture()
	v := &c.R.Vaults[0]
	v.Loot = append(v.Loot, mapgen.Loot{X: 5, Y: 5, Kind: item.Steel, Qty: 1}) // outside
	v.Caskets = append(v.Caskets, mapgen.Casket{X: 27, Y: 27})                 // its foot is the wall
	v.DefenderSpots = append(v.DefenderSpots, mapgen.Point{X: 40, Y: 40})
	// a second vault on water is dropped whole
	setWater(c, 45, 45, mapgen.Shallow)
	c.R.Vaults = append(c.R.Vaults, mapgen.Vault{ID: 2, X0: 44, Y0: 44, X1: 46, Y1: 46})
	if got := Violations(c); len(got) != 1 || !strings.HasPrefix(got[0], "I5") {
		t.Fatalf("violations %v", got)
	}
	validate(t, c)
	if got := Violations(c); len(got) > 0 {
		t.Fatalf("violations %v", got)
	}
	if len(c.R.Vaults) != 1 || len(c.R.Vaults[0].Loot) != 1 || len(c.R.Vaults[0].Caskets) != 1 || len(c.R.Vaults[0].DefenderSpots) != 1 {
		t.Fatalf("vaults %+v", c.R.Vaults)
	}
}

func TestRoadsOverNoiseMapsStayConnected(t *testing.T) {
	for seed := uint64(1); seed <= 6; seed++ {
		c := noiseFixture(seed, 150, 150)
		c.Cfg.Site.Road = site.Dir8Set(1<<0 | 1<<2 | 1<<5)
		if err := (roads.Roads{}).Run(c); err != nil {
			t.Fatal(err)
		}
		run(t, c)
		validate(t, c)
		if b := BrokenRoads(c); len(b) > 0 {
			t.Fatalf("seed %d: broken roads %v (%v)", seed, b, c.R.Metrics.Repairs)
		}
		if v := Violations(c); len(v) > 0 {
			t.Fatalf("seed %d: %v", seed, v)
		}
	}
}
