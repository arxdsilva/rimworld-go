package roads

import (
	"math"
	"reflect"
	"testing"

	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/site"
	"rimworld-go/internal/domain/world/terrain"
)

func dirs(ds ...site.Dir8) site.Dir8Set {
	var s site.Dir8Set
	for _, d := range ds {
		s |= 1 << d
	}
	return s
}

func fixture(seed uint64, w, h int, road site.Dir8Set) *mapgen.Ctx {
	c := mapgen.NewCtx(mapgen.Config{Seed: seed, W: w, H: h, Site: site.Site{Road: road}})
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c.R.Terrain.Set(x, y, terrain.Soil)
		}
	}
	return c
}

func setRock(c *mapgen.Ctx, x, y int) {
	i := c.Idx(x, y)
	c.F.Rock[i] = true
	c.R.Terrain.Set(x, y, terrain.Rock)
	c.R.Geology.Stone[i] = geology.Granite
	c.R.Roof[i] = roof.Natural
}

func setWater(c *mapgen.Ctx, x, y int, w mapgen.Water) {
	c.F.Water[c.Idx(x, y)] = w
	if w == mapgen.Deep {
		c.R.Terrain.Set(x, y, terrain.DeepWater)
	} else {
		c.R.Terrain.Set(x, y, terrain.ShallowWater)
	}
}

func run(t *testing.T, c *mapgen.Ctx) {
	t.Helper()
	if err := (Roads{}).Run(c); err != nil {
		t.Fatal(err)
	}
}

// reach flood-fills 4-connected road cells (Track or ford) from the first
// Track cell on the given cell.
func reach(c *mapgen.Ctx, from int) map[int]bool {
	seen := map[int]bool{}
	if !c.F.Road[from] {
		return seen
	}
	st := []int{from}
	seen[from] = true
	for len(st) > 0 {
		i := st[len(st)-1]
		st = st[:len(st)-1]
		x, y := i%c.W, i/c.W
		for _, o := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			nx, ny := x+o[0], y+o[1]
			if !c.In(nx, ny) {
				continue
			}
			j := c.Idx(nx, ny)
			if c.F.Road[j] && !seen[j] {
				seen[j] = true
				st = append(st, j)
			}
		}
	}
	return seen
}

func edgeRoadCells(c *mapgen.Ctx, edge func(x, y int) bool) []int {
	var out []int
	for i := range c.F.Road {
		if c.F.Road[i] && edge(i%c.W, i/c.W) {
			out = append(out, i)
		}
	}
	return out
}

func TestNoRoad(t *testing.T) {
	c := fixture(1, 60, 60, 0)
	run(t, c)
	if len(c.R.Roads) != 0 {
		t.Fatal("roads drawn without Site.Road")
	}
	for i := range c.F.Road {
		if c.F.Road[i] || c.F.Blocked[i] {
			t.Fatal("cell marked")
		}
	}
}

func TestName(t *testing.T) {
	if (Roads{}).Name() != "roads" {
		t.Fatal("name")
	}
}

func TestDeterminism(t *testing.T) {
	a, b, d := fixture(5, 100, 100, dirs(0, 3)), fixture(5, 100, 100, dirs(0, 3)), fixture(6, 100, 100, dirs(0, 3))
	run(t, a)
	run(t, b)
	run(t, d)
	if !reflect.DeepEqual(a.R.Terrain, b.R.Terrain) || !reflect.DeepEqual(a.R.Roads, b.R.Roads) {
		t.Fatal("same seed differs")
	}
	if reflect.DeepEqual(a.R.Roads, d.R.Roads) {
		t.Fatal("other seed identical")
	}
}

func TestSingleRoadConnectsEdgeToJunction(t *testing.T) {
	c := fixture(2, 120, 120, dirs(0))
	run(t, c)
	top := edgeRoadCells(c, func(x, y int) bool { return y == 0 })
	if len(top) == 0 {
		t.Fatal("no road cell on the north edge")
	}
	got := reach(c, top[0])
	mid := 0
	for i := range got {
		x, y := i%c.W, i/c.W
		if math.Hypot(float64(x-60), float64(y-60)) < 30 {
			mid++
		}
	}
	if mid == 0 {
		t.Fatal("road does not reach the junction")
	}
	for i := range got {
		if c.F.Road[i] && c.R.Terrain.At(i%c.W, i/c.W) != terrain.Track {
			t.Fatal("road cell not Track")
		}
		if !c.F.Blocked[i] {
			t.Fatal("road cell not Blocked")
		}
		if c.R.Geology.Stone[i] != geology.None {
			t.Fatal("track keeps stone")
		}
	}
	if len(c.R.Roads) != 1 {
		t.Fatalf("roads = %d", len(c.R.Roads))
	}
}

func TestTwoRoadsMeet(t *testing.T) {
	c := fixture(3, 150, 150, dirs(0, 4, 2))
	run(t, c)
	if len(c.R.Roads) != 3 {
		t.Fatalf("roads = %d", len(c.R.Roads))
	}
	top := edgeRoadCells(c, func(x, y int) bool { return y == 0 })
	bot := edgeRoadCells(c, func(x, y int) bool { return y == 149 })
	right := edgeRoadCells(c, func(x, y int) bool { return x == 149 })
	if len(top) == 0 || len(bot) == 0 || len(right) == 0 {
		t.Fatal("missing edge ends")
	}
	got := reach(c, top[0])
	if !got[bot[0]] || !got[right[0]] {
		t.Fatal("roads do not meet")
	}
}

func TestRiverWithFordIsBridged(t *testing.T) {
	c := fixture(4, 100, 100, dirs(0))
	// A river across the map at y 30..33 with a shallow ford at x 70..72:
	// the road crosses on a bridge, never wading.
	for y := 30; y <= 33; y++ {
		for x := 0; x < 100; x++ {
			if x >= 70 && x <= 72 {
				setWater(c, x, y, mapgen.Shallow)
			} else {
				setWater(c, x, y, mapgen.Deep)
			}
		}
	}
	run(t, c)
	if strips(t, c) != 1 {
		t.Fatal("want one bridge")
	}
	for y := 30; y <= 33; y++ {
		for x := 0; x < 100; x++ {
			if c.F.Water[c.Idx(x, y)] == mapgen.Deep && c.R.Terrain.At(x, y) != terrain.DeepWater {
				t.Fatal("deep water overwritten")
			}
			if c.F.Water[c.Idx(x, y)] == mapgen.Shallow && c.R.Terrain.At(x, y) != terrain.ShallowWater {
				t.Fatal("ford terrain overwritten")
			}
		}
	}
	top := edgeRoadCells(c, func(x, y int) bool { return y == 0 })
	if len(top) == 0 {
		t.Fatal("no road at the edge")
	}
	got := reach(c, top[0])
	found := false
	for i := range got {
		if math.Hypot(float64(i%100-50), float64(i/100-50)) < 30 {
			found = true
		}
	}
	if !found {
		t.Fatal("not connected over the bridge")
	}
}

func TestEdgeOnDeepWaterShifts(t *testing.T) {
	c := fixture(4, 80, 80, dirs(0))
	for x := 30; x < 50; x++ {
		setWater(c, x, 0, mapgen.Deep)
	}
	run(t, c)
	if len(edgeRoadCells(c, func(x, y int) bool { return y == 0 })) == 0 {
		t.Fatal("no endpoint found")
	}
}

func TestAllDeepEdgeSkipsRoad(t *testing.T) {
	c := fixture(4, 40, 40, dirs(0))
	for x := 0; x < 40; x++ {
		setWater(c, x, 0, mapgen.Deep)
	}
	run(t, c)
	if len(c.R.Roads) != 0 {
		t.Fatal("road without an endpoint")
	}
}

func wall(c *mapgen.Ctx, y, gapFrom, gapTo int) {
	for x := 0; x < c.W; x++ {
		if gapFrom >= 0 && x >= gapFrom && x <= gapTo {
			continue
		}
		for dy := 0; dy < 3; dy++ {
			setRock(c, x, y+dy)
		}
	}
}

func TestRockWallWithGapRoutesThroughGap(t *testing.T) {
	c := fixture(7, 100, 100, dirs(0))
	wall(c, 20, 60, 66)
	run(t, c)
	for i := range c.F.Road {
		if c.F.Road[i] && c.F.Rock[i] {
			t.Fatal("rock cell still rock under road")
		}
	}
	cutRock := 0
	for x := 0; x < 100; x++ {
		for y := 20; y < 23; y++ {
			if (x < 60 || x > 66) && c.R.Terrain.At(x, y) != terrain.Rock {
				cutRock++
			}
		}
	}
	if cutRock != 0 {
		t.Fatalf("wall cut although a gap exists (%d cells)", cutRock)
	}
	gapRoad := false
	for x := 60; x <= 66; x++ {
		gapRoad = gapRoad || c.F.Road[c.Idx(x, 21)]
	}
	if !gapRoad {
		t.Fatal("road skipped the gap")
	}
}

func TestFullWallIsCut(t *testing.T) {
	c := fixture(7, 100, 100, dirs(0))
	wall(c, 20, -1, -1)
	run(t, c)
	top := edgeRoadCells(c, func(x, y int) bool { return y == 0 })
	if len(top) == 0 {
		t.Fatal("no road")
	}
	got := reach(c, top[0])
	crossed := 0
	for i := range got {
		if i/100 >= 20 && i/100 < 23 {
			crossed++
			if c.F.Rock[i] || c.R.Terrain.At(i%100, i/100) != terrain.Track {
				t.Fatal("cut cell not Track / still F.Rock")
			}
			if c.R.Roof[i] != 0 || c.R.Geology.Stone[i] != geology.None || c.R.Geology.OreAt(i%100, i/100) != geology.NoOre {
				t.Fatal("invariants broken on cut rock")
			}
		}
	}
	if crossed < 3 {
		t.Fatal("wall not crossed")
	}
	// Rock elsewhere is untouched and still consistent (I1).
	for i := range c.F.Rock {
		if c.F.Rock[i] != terrain.IsRockWall(c.R.Terrain.At(i%100, i/100)) {
			t.Fatal("I1 broken")
		}
	}
}

func TestStraightDiagonalRoadIsSmooth(t *testing.T) {
	c := fixture(8, 200, 200, dirs(3))
	run(t, c)
	if len(c.R.Roads) != 1 {
		t.Fatal("no road")
	}
	line := c.R.Roads[0]
	// Direction changes (> 20 degrees) per 10 cells of road length.
	changes, length := 0, 0.0
	for i := 1; i+1 < len(line); i++ {
		a1 := math.Atan2(line[i].Y-line[i-1].Y, line[i].X-line[i-1].X)
		a2 := math.Atan2(line[i+1].Y-line[i].Y, line[i+1].X-line[i].X)
		d := math.Abs(a2 - a1)
		if d > math.Pi {
			d = 2*math.Pi - d
		}
		if d > 20*math.Pi/180 {
			changes++
		}
	}
	for i := 1; i < len(line); i++ {
		length += math.Hypot(line[i].X-line[i-1].X, line[i].Y-line[i-1].Y)
	}
	if rate := float64(changes) / (length / 10); rate > 2 {
		t.Fatalf("%d sharp turns over %.0f cells", changes, length)
	}
	// No 90-degree stair steps in the raster: track run lengths on rows.
}

func TestEdgesSoft(t *testing.T) {
	c := fixture(9, 200, 200, dirs(0, 4))
	run(t, c)
	gravel, track := 0, 0
	for i := range c.F.Road {
		switch c.R.Terrain.At(i%200, i/200) {
		case terrain.Gravel:
			gravel++
			if c.F.Road[i] {
				t.Fatal("gravel edge marked as road")
			}
			if !c.F.Blocked[i] {
				t.Fatal("gravel edge not blocked")
			}
		case terrain.Track:
			track++
		}
	}
	if gravel == 0 || track == 0 {
		t.Fatalf("gravel %d track %d", gravel, track)
	}
}

func TestChunksUnderRoadRemoved(t *testing.T) {
	c := fixture(10, 100, 100, dirs(0))
	c.R.Chunks = []mapgen.ChunkSpawn{{X: 5, Y: 90, Stone: geology.Granite}}
	run(t, c)
	var onRoad mapgen.ChunkSpawn
	for i := range c.F.Road {
		if c.F.Road[i] && i/100 > 10 && i/100 < 40 {
			onRoad = mapgen.ChunkSpawn{X: i % 100, Y: i / 100, Stone: geology.Granite}
			break
		}
	}
	c2 := fixture(10, 100, 100, dirs(0))
	c2.R.Chunks = []mapgen.ChunkSpawn{{X: 5, Y: 90, Stone: geology.Granite}, onRoad}
	c2.F.Blocked[c2.Idx(onRoad.X, onRoad.Y)] = true
	run(t, c2)
	if len(c2.R.Chunks) != 1 || c2.R.Chunks[0].X != 5 {
		t.Fatalf("chunks = %v", c2.R.Chunks)
	}
	if !c2.F.Blocked[c2.Idx(onRoad.X, onRoad.Y)] {
		t.Fatal("road cell not blocked")
	}
}

func TestTinyMaps(t *testing.T) {
	for _, sz := range [][2]int{{24, 24}, {2, 2}, {1, 1}, {30, 24}} {
		c := fixture(11, sz[0], sz[1], dirs(0, 1, 2, 3, 4, 5, 6, 7))
		run(t, c)
		if sz[0] == 24 && len(c.R.Roads) != 8 {
			t.Fatalf("24x24 roads = %d", len(c.R.Roads))
		}
	}
}

func TestMarshIsCostly(t *testing.T) {
	c := fixture(12, 100, 100, dirs(0))
	// A marsh band the road can walk around on one side only.
	for y := 20; y < 30; y++ {
		for x := 0; x < 100; x++ {
			c.R.Terrain.Set(x, y, terrain.Marsh)
		}
	}
	for y := 20; y < 30; y++ {
		c.R.Terrain.Set(80, y, terrain.Gravel)
	}
	c.F.Blocked[c.Idx(50, 10)] = true
	run(t, c)
	if len(c.R.Roads) != 1 {
		t.Fatal("no road")
	}
}

func TestJunctionPrefersOpenSoil(t *testing.T) {
	c := fixture(13, 100, 100, dirs(0))
	for y := 46; y < 54; y++ {
		for x := 46; x < 54; x++ {
			c.R.Terrain.Set(x, y, terrain.Marsh)
		}
	}
	rt := &router{c: c, w: 100, h: 100}
	j := rt.junction()
	if c.R.Terrain.At(j%100, j/100) != terrain.Soil {
		t.Fatal("junction on marsh")
	}
	// All-rock map falls back to the centre.
	for i := 0; i < 100*100; i++ {
		setRock(c, i%100, i/100)
	}
	if j := rt.junction(); j != c.Idx(50, 50) {
		t.Fatal("fallback")
	}
}

func TestDegenerateRoad(t *testing.T) {
	// A single-cell path (start == junction) must not panic.
	c := fixture(14, 3, 3, dirs(0))
	run(t, c)
}

func BenchmarkFourRoads500(b *testing.B) {
	for n := 0; n < b.N; n++ {
		b.StopTimer()
		c := fixture(15, 500, 500, dirs(0, 2, 4, 6))
		for y := 100; y < 130; y++ {
			for x := 0; x < 500; x++ {
				if x%90 > 12 {
					setRock(c, x, y)
				}
			}
		}
		b.StartTimer()
		if err := (Roads{}).Run(c); err != nil {
			b.Fatal(err)
		}
	}
}
