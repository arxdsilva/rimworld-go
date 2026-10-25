package start

import (
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/terrain"
)

func run(t testing.TB, c *mapgen.Ctx) mapgen.Point {
	t.Helper()
	if err := (Start{}).Run(c); err != nil {
		t.Fatal(err)
	}
	return c.R.Start
}

func hasRepair(c *mapgen.Ctx, prefix string) bool {
	return slices.ContainsFunc(c.R.Metrics.Repairs, func(s string) bool { return strings.HasPrefix(s, prefix) })
}

// within calls f for every in-map cell within Euclidean radius rad of p.
func within(c *mapgen.Ctx, p mapgen.Point, rad int, f func(x, y, i int)) {
	for y := p.Y - rad; y <= p.Y+rad; y++ {
		for x := p.X - rad; x <= p.X+rad; x++ {
			if c.In(x, y) && (x-p.X)*(x-p.X)+(y-p.Y)*(y-p.Y) <= rad*rad {
				f(x, y, c.Idx(x, y))
			}
		}
	}
}

func TestStartName(t *testing.T) {
	if (Start{}).Name() != "start" || (Validate{}).Name() != "validate" {
		t.Fatal("step names are part of the contract")
	}
}

func TestStartPicksMeadowOverRockyCorner(t *testing.T) {
	c := fixture(1, 100, 100)
	for y := range 100 {
		for x := range 100 {
			if !(x >= 55 && y >= 55) && (x/2+y/2)%2 == 0 {
				setRock(c, x, y)
			}
		}
	}
	p := run(t, c)
	if p.X < 59 || p.Y < 59 || p.X > 79 || p.Y > 79 {
		t.Fatalf("start %v: want the open meadow (59..79), clear of rock and the edge margin", p)
	}
	if hasRepair(c, "start") {
		t.Fatalf("a passing spot needs no repair: %v", c.R.Metrics.Repairs)
	}
	if s := c.F.StartScore[c.Idx(p.X, p.Y)]; s < 0.5 || s > 1 {
		t.Fatalf("passing start scores %v, want [0.5, 1]", s)
	}
	for i, s := range c.F.StartScore {
		if s < 0 || s > 1 {
			t.Fatalf("StartScore[%d] = %v outside [0, 1]", i, s)
		}
		if s > c.F.StartScore[c.Idx(p.X, p.Y)] {
			t.Fatalf("cell %d scores %v above the chosen start", i, s)
		}
	}
}

func TestStartRespectsEdgeMargin(t *testing.T) {
	// only a strip along the west edge is open: the start still keeps the
	// margin and the repair opens a pad.
	c := fixture(2, 100, 100)
	for y := range 100 {
		for x := 15; x < 100; x++ {
			setRock(c, x, y)
		}
	}
	p := run(t, c)
	if p.X < 20 || p.Y < 20 || p.X > 79 || p.Y > 79 {
		t.Fatalf("start %v inside the 20-cell edge margin", p)
	}
	for _, sz := range [][2]int{{24, 24}, {60, 40}, {1, 1}, {3, 50}, {500, 60}} {
		c := fixture(3, sz[0], sz[1])
		p := run(t, c)
		mx, my := min(25, sz[0]/5), min(25, sz[1]/5)
		if p.X < mx || p.Y < my || sz[0]-1-p.X < mx || sz[1]-1-p.Y < my {
			t.Fatalf("%v: start %v breaks margin %d,%d", sz, p, mx, my)
		}
	}
}

func TestStartRepairOpensRockRingedSpot(t *testing.T) {
	c := fixture(4, 60, 60)
	for y := range 60 {
		for x := range 60 {
			if x < 27 || x > 33 || y < 27 || y > 33 {
				setRock(c, x, y)
				c.R.Geology.Ore[c.Idx(x, y)] = geology.Steel
			}
		}
	}
	p := run(t, c)
	if p.X < 27 || p.X > 33 || p.Y < 27 || p.Y > 33 {
		t.Fatalf("start %v left the open pocket", p)
	}
	within(c, p, 6, func(x, y, i int) {
		if c.F.Rock[i] || terrain.IsRockWall(c.R.Terrain.Cells[i]) {
			t.Fatalf("rock left at %d,%d within 6 of the start", x, y)
		}
		if c.R.Terrain.Cells[i] == terrain.Stone &&
			(c.R.Geology.Stone[i] == geology.None || c.R.Geology.Ore[i] != geology.NoOre || c.R.Roof[i] != 0) {
			t.Fatalf("opened cell %d,%d: stone %v ore %v roof %v", x, y, c.R.Geology.Stone[i], c.R.Geology.Ore[i], c.R.Roof[i])
		}
	})
	if c.R.Terrain.At(p.X+6, p.Y) != terrain.Stone {
		t.Fatalf("rock 6 cells out should be rough stone, got %v", c.R.Terrain.At(p.X+6, p.Y))
	}
	if !hasRepair(c, "start") {
		t.Fatalf("repair not logged: %v", c.R.Metrics.Repairs)
	}
}

func TestStartDriesShallowWater(t *testing.T) {
	c := fixture(5, 100, 100)
	for y := range 100 {
		for x := range 100 {
			if x%8 == 0 || y%8 == 0 {
				setWater(c, x, y, mapgen.Shallow)
			}
		}
	}
	setWater(c, 52, 52, mapgen.Deep)
	p := run(t, c)
	within(c, p, 4, func(x, y, i int) {
		if terrain.IsWater(c.R.Terrain.Cells[i]) || c.F.Water[i].Wet() {
			t.Fatalf("water left at %d,%d within 4 of the start %v", x, y, p)
		}
	})
	if !slices.Contains(c.R.Terrain.Cells, terrain.Mud) || !hasRepair(c, "start") {
		t.Fatal("dried water becomes mud and is logged")
	}
}

func TestStartClearsChunksNearStart(t *testing.T) {
	c := fixture(6, 100, 100)
	for _, q := range [][2]int{{50, 56}, {44, 50}, {50, 66}, {30, 30}} {
		c.R.Chunks = append(c.R.Chunks, mapgen.ChunkSpawn{X: q[0], Y: q[1], Stone: geology.Granite})
		c.F.Blocked[c.Idx(q[0], q[1])] = true
	}
	p := run(t, c)
	if len(c.R.Chunks) != 2 {
		t.Fatalf("start %v: %d chunks left, want the 2 far ones", p, len(c.R.Chunks))
	}
	for _, ch := range c.R.Chunks {
		if dx, dy := ch.X-p.X, ch.Y-p.Y; dx*dx+dy*dy <= 144 {
			t.Fatalf("chunk %v within 12 of start %v", ch, p)
		}
	}
	if c.F.Blocked[c.Idx(50, 56)] || c.F.Blocked[c.Idx(44, 50)] || !c.F.Blocked[c.Idx(30, 30)] {
		t.Fatal("removed chunks free their cells; kept ones stay blocked")
	}
}

func TestStartDeterministicTieBreak(t *testing.T) {
	centre := []mapgen.Point{{X: 49, Y: 49}, {X: 50, Y: 49}, {X: 49, Y: 50}, {X: 50, Y: 50}}
	seen := map[mapgen.Point]bool{}
	for seed := uint64(1); seed <= 32; seed++ {
		a, b := fixture(seed, 100, 100), fixture(seed, 100, 100)
		pa, pb := run(t, a), run(t, b)
		if pa != pb || !slices.Equal(a.F.StartScore, b.F.StartScore) {
			t.Fatalf("seed %d: %v vs %v", seed, pa, pb)
		}
		if !slices.Contains(centre, pa) {
			t.Fatalf("seed %d: open map start %v, want a centre cell", seed, pa)
		}
		seen[pa] = true
	}
	if len(seen) < 2 {
		t.Fatalf("seeded tie-breaks never vary: %v", seen)
	}
}

func TestStartLeansTowardRoad(t *testing.T) {
	c := fixture(7, 150, 150)
	for y := range 150 {
		i := c.Idx(120, y)
		c.R.Terrain.Cells[i] = terrain.Track
		c.F.Road[i], c.F.Blocked[i] = true, true
	}
	p := run(t, c)
	if p.X < 88 || p.X > 100 {
		t.Fatalf("start %v: want within ~25 of the road at x=120, near the centre", p)
	}
}

func TestStartPrefersFertileAndStone(t *testing.T) {
	// the east half is fertile and has stone; the west is barren.
	c := fixture(8, 200, 100)
	for i := range c.F.Fertility {
		if x, _ := c.XY(i); x < 100 {
			c.F.Fertility[i] = 0
		} else {
			c.F.Fertility[i] = 1
		}
	}
	for y := 40; y < 60; y++ {
		setRock(c, 190, y)
	}
	if p := run(t, c); p.X <= 100 {
		t.Fatalf("start %v: fertile, stony east half should win", p)
	}
}

func TestStartNoPanicOnFeatureless(t *testing.T) {
	// all deep water: nothing passes, the start still lands and dries a pad.
	c := fixture(9, 40, 40)
	for i := range c.R.Terrain.Cells {
		x, y := c.XY(i)
		setWater(c, x, y, mapgen.Deep)
	}
	p := run(t, c)
	if terrain.IsWater(c.R.Terrain.At(p.X, p.Y)) {
		t.Fatalf("start %v on water", p)
	}
}
