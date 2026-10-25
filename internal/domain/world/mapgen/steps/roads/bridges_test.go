package roads

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/terrain"
)

// riverBand paints a river across the map: rows y0..y1, shallow on the
// outer rows and deep inside, with its centreline.
func riverBand(c *mapgen.Ctx, y0, y1 int) {
	for y := y0; y <= y1; y++ {
		for x := 0; x < c.W; x++ {
			w := mapgen.Deep
			if y == y0 || y == y1 {
				w = mapgen.Shallow
			}
			setWater(c, x, y, w)
			c.F.River[c.Idx(x, y)] = true
		}
	}
	mid := float64(y0+y1+1) / 2
	for x := 0; x <= c.W; x += 10 {
		c.F.RiverLine = append(c.F.RiverLine, mapgen.PointF{X: float64(x), Y: mid})
	}
}

// strips groups the bridge cells into crossings (8-connected) and checks
// each is a straight, 2-wide, contiguous strip with Track beyond both ends
// of each lane. It returns the number of crossings.
func strips(t *testing.T, c *mapgen.Ctx) int {
	t.Helper()
	br := make([]bool, c.W*c.H)
	for _, p := range c.R.Bridges {
		i := c.Idx(p.X, p.Y)
		if br[i] {
			t.Fatalf("bridge %v listed twice", p)
		}
		br[i] = true
		tt := c.R.Terrain.At(p.X, p.Y)
		if !terrain.Bridgeable(tt) {
			t.Fatalf("bridge %v on %v", p, tt)
		}
		if c.R.Ocean[i] {
			t.Fatalf("bridge %v on the open sea", p)
		}
		if !c.F.Road[i] || !c.F.Blocked[i] {
			t.Fatalf("bridge %v not a blocked road cell", p)
		}
	}
	label, sizes := mapgen.Components(c.W, c.H, br, true)
	for k := range sizes {
		var cells []mapgen.Point
		for i, l := range label {
			if int(l) == k {
				x, y := c.XY(i)
				cells = append(cells, mapgen.Point{X: x, Y: y})
			}
		}
		xs, ys := map[int]bool{}, map[int]bool{}
		for _, p := range cells {
			xs[p.X], ys[p.Y] = true, true
		}
		var vertical bool
		switch {
		case len(xs) <= 2 && len(ys) > 2:
			vertical = true
		case len(ys) <= 2 && len(xs) > 2:
		case len(xs) <= 2 && len(ys) <= 2:
			vertical = len(ys) >= len(xs)
		default:
			t.Fatalf("crossing %v is not a straight 2-wide strip", cells)
		}
		lanes := map[int][]int{}
		for _, p := range cells {
			if vertical {
				lanes[p.X] = append(lanes[p.X], p.Y)
			} else {
				lanes[p.Y] = append(lanes[p.Y], p.X)
			}
		}
		if len(lanes) != 2 {
			t.Fatalf("crossing %v has %d lanes, want 2", cells, len(lanes))
		}
		for lane, ss := range lanes {
			slices.Sort(ss)
			if ss[len(ss)-1]-ss[0] != len(ss)-1 {
				t.Fatalf("lane %d of crossing %v has a gap: %v", lane, cells, ss)
			}
			for _, s := range []int{ss[0] - 1, ss[len(ss)-1] + 1} {
				x, y := lane, s
				if !vertical {
					x, y = s, lane
				}
				if !c.In(x, y) || c.R.Terrain.At(x, y) != terrain.Track {
					t.Fatalf("lane %d of crossing %v does not end on Track at %d,%d", lane, cells, x, y)
				}
			}
		}
	}
	// no fords: every road cell on water is a bridge
	for i := range c.F.Road {
		if c.F.Road[i] && terrain.IsWater(c.R.Terrain.Cells[i]) && !br[i] {
			x, y := c.XY(i)
			t.Fatalf("road over water without a bridge at %d,%d", x, y)
		}
	}
	return len(sizes)
}

func TestRoadBridgesRiverStraight(t *testing.T) {
	c := fixture(21, 100, 100, dirs(0))
	riverBand(c, 30, 34)
	run(t, c)
	if n := strips(t, c); n != 1 {
		t.Fatalf("%d crossings, want 1", n)
	}
	if len(c.R.Bridges) != 2*5 {
		t.Fatalf("%d bridge cells, want 2 lanes × 5 rows: %v", len(c.R.Bridges), c.R.Bridges)
	}
	for y := 30; y <= 34; y++ {
		for x := 0; x < 100; x++ {
			want := terrain.DeepWater
			if y == 30 || y == 34 {
				want = terrain.ShallowWater
			}
			if c.R.Terrain.At(x, y) != want {
				t.Fatalf("water at %d,%d changed to %v", x, y, c.R.Terrain.At(x, y))
			}
		}
	}
	if len(c.F.RoadEnds) != 1 || c.F.RoadEnds[0][0].Y != 0 {
		t.Fatalf("road ends %v", c.F.RoadEnds)
	}
	e, j := c.F.RoadEnds[0][0], c.F.RoadEnds[0][1]
	if !reach(c, c.Idx(e.X, e.Y))[c.Idx(j.X, j.Y)] {
		t.Fatal("edge and junction not joined over the bridge")
	}
}

func TestRoadBridgeOverDiagonalRiverIsStraight(t *testing.T) {
	c := fixture(22, 100, 100, dirs(0))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			if d := x + y - 70; d >= -3 && d <= 3 {
				setWater(c, x, y, mapgen.Deep)
				c.F.River[c.Idx(x, y)] = true
			}
		}
	}
	c.F.RiverLine = []mapgen.PointF{{X: 0, Y: 70}, {X: 35, Y: 35}, {X: 70, Y: 0}}
	run(t, c)
	if n := strips(t, c); n != 1 {
		t.Fatalf("%d crossings, want 1", n)
	}
	e, j := c.F.RoadEnds[0][0], c.F.RoadEnds[0][1]
	if !reach(c, c.Idx(e.X, e.Y))[c.Idx(j.X, j.Y)] {
		t.Fatal("edge and junction not joined over the bridge")
	}
}

func TestRoadCrossesLakeOnBridge(t *testing.T) {
	c := fixture(23, 100, 100, dirs(0))
	// a lake the road would walk far around: water x 2..97, y 25..28
	for y := 25; y <= 28; y++ {
		for x := 2; x <= 97; x++ {
			setWater(c, x, y, mapgen.Deep)
			c.F.Lake[c.Idx(x, y)] = true
		}
	}
	run(t, c)
	if n := strips(t, c); n != 1 {
		t.Fatalf("%d crossings, want 1", n)
	}
}

func TestRoadNeverBridgesOcean(t *testing.T) {
	sea := func(c *mapgen.Ctx, gap bool) {
		for y := 30; y <= 35; y++ {
			for x := 0; x < c.W; x++ {
				if gap && x >= 80 && x <= 86 {
					continue
				}
				setWater(c, x, y, mapgen.Deep)
				c.F.Coast[c.Idx(x, y)] = true
				c.R.Ocean[c.Idx(x, y)] = true
			}
		}
	}
	c := fixture(24, 100, 100, dirs(0))
	sea(c, true)
	run(t, c)
	if len(c.R.Bridges) != 0 {
		t.Fatalf("bridges %v over the sea", c.R.Bridges)
	}
	for i := range c.F.Road {
		if c.F.Road[i] && c.R.Ocean[i] {
			t.Fatal("road on the open sea")
		}
	}
	if len(c.R.Roads) != 1 {
		t.Fatal("road not routed through the land gap")
	}
	// no gap: the road cannot reach the junction; logged, nothing laid
	c = fixture(24, 100, 100, dirs(0))
	sea(c, false)
	run(t, c)
	if len(c.R.Roads) != 0 || len(c.R.Bridges) != 0 || len(c.F.RoadEnds) != 0 {
		t.Fatal("road laid across the open sea")
	}
	if len(c.R.Metrics.Repairs) != 1 || !strings.Contains(c.R.Metrics.Repairs[0], "road 1") {
		t.Fatalf("failure not logged: %v", c.R.Metrics.Repairs)
	}
}

func TestRouteRetriesThroughThickRock(t *testing.T) {
	c := fixture(25, 100, 100, dirs(0))
	for y := 20; y < 50; y++ {
		for x := 0; x < 100; x++ {
			setRock(c, x, y)
		}
	}
	c.F.RockDepth = mapgen.ChebDepth(100, 100, c.F.Rock)
	run(t, c)
	if len(c.R.Roads) != 1 || len(c.R.Metrics.Repairs) != 0 {
		t.Fatalf("roads %d repairs %v", len(c.R.Roads), c.R.Metrics.Repairs)
	}
	e, j := c.F.RoadEnds[0][0], c.F.RoadEnds[0][1]
	if !reach(c, c.Idx(e.X, e.Y))[c.Idx(j.X, j.Y)] {
		t.Fatal("road not cut through the mountain")
	}
}

func TestRouteAndLayExported(t *testing.T) {
	mk := func() *mapgen.Ctx {
		c := fixture(26, 80, 80, 0)
		riverBand(c, 40, 43)
		return c
	}
	a, b := mk(), mk()
	from, to := mapgen.Point{X: 20, Y: 5}, mapgen.Point{X: 50, Y: 70}
	pa, pb := Route(a, from, to), Route(b, from, to)
	if len(pa) == 0 || pa[0] != from || pa[len(pa)-1] != to {
		t.Fatalf("path %v", pa)
	}
	for k := 1; k < len(pa); k++ {
		if dx, dy := pa[k].X-pa[k-1].X, pa[k].Y-pa[k-1].Y; max(abs(dx), abs(dy)) != 1 {
			t.Fatalf("path jumps at %d: %v -> %v", k, pa[k-1], pa[k])
		}
	}
	if !reflect.DeepEqual(pa, pb) {
		t.Fatal("Route is not deterministic")
	}
	Lay(a, pa)
	Lay(b, pb)
	if !reflect.DeepEqual(a.R.Terrain, b.R.Terrain) || !reflect.DeepEqual(a.R.Bridges, b.R.Bridges) || !reflect.DeepEqual(a.F.Road, b.F.Road) {
		t.Fatal("Lay is not deterministic")
	}
	if n := strips(t, a); n != 1 {
		t.Fatalf("%d crossings, want 1", n)
	}
	if !reach(a, a.Idx(from.X, from.Y))[a.Idx(to.X, to.Y)] {
		t.Fatal("laid road not connected")
	}
	// laying it again adds no second bridge
	nb := len(a.R.Bridges)
	Lay(a, pa)
	if len(a.R.Bridges) != nb || strips(t, a) != 1 {
		t.Fatal("relaying the same road bridged again")
	}
	// the sea is impassable: no route
	c := fixture(26, 40, 40, 0)
	for x := range 40 {
		setWater(c, x, 20, mapgen.Deep)
		c.R.Ocean[c.Idx(x, 20)] = true
	}
	if p := Route(c, mapgen.Point{X: 5, Y: 5}, mapgen.Point{X: 5, Y: 35}); p != nil {
		t.Fatalf("route across the sea: %v", p)
	}
	Lay(c, nil) // nothing to lay
	if Route(c, mapgen.Point{X: -1, Y: 5}, mapgen.Point{X: 5, Y: 5}) != nil {
		t.Fatal("route from off the map")
	}
}

func TestLayFallsBackWhereNoStraightStripFits(t *testing.T) {
	// a 1-cell gap in rock banks: no 2-wide strip has dry banks, so the
	// raw crossing is bridged as routed
	c := fixture(27, 40, 40, 0)
	for y := 15; y <= 18; y++ {
		for x := range 40 {
			setWater(c, x, y, mapgen.Deep)
		}
	}
	for _, y := range []int{14, 19} {
		for x := range 40 {
			if x != 20 {
				setRock(c, x, y)
			}
		}
	}
	var path []mapgen.Point
	for y := 5; y <= 30; y++ {
		path = append(path, mapgen.Point{X: 20, Y: y})
	}
	Lay(c, path)
	for y := 15; y <= 18; y++ {
		i := c.Idx(20, y)
		if !c.F.Road[i] || !slices.Contains(c.R.Bridges, mapgen.Point{X: 20, Y: y}) {
			t.Fatalf("raw crossing cell 20,%d not bridged", y)
		}
	}
	if !reach(c, c.Idx(20, 5))[c.Idx(20, 30)] {
		t.Fatal("raw crossing not connected")
	}
	for _, p := range c.R.Bridges {
		if !terrain.Bridgeable(c.R.Terrain.At(p.X, p.Y)) {
			t.Fatalf("bridge on %v", c.R.Terrain.At(p.X, p.Y))
		}
	}
}

func TestSecondRoadReusesBridge(t *testing.T) {
	c := fixture(28, 120, 120, dirs(0, 1))
	riverBand(c, 30, 34)
	run(t, c)
	if len(c.R.Roads) != 2 {
		t.Fatalf("roads %d", len(c.R.Roads))
	}
	strips(t, c)
	for k, ends := range c.F.RoadEnds {
		if !reach(c, c.Idx(ends[0].X, ends[0].Y))[c.Idx(ends[1].X, ends[1].Y)] {
			t.Fatalf("road %d not connected", k)
		}
	}
}

func TestMarshWithoutWaterStaysTrack(t *testing.T) {
	c := fixture(29, 100, 100, dirs(0))
	for y := 20; y < 30; y++ {
		for x := 0; x < 100; x++ {
			c.R.Terrain.Set(x, y, terrain.Marsh)
		}
	}
	run(t, c)
	if len(c.R.Bridges) != 0 {
		t.Fatal("bridge over dry marsh")
	}
	crossed := false
	for x := 0; x < 100; x++ {
		crossed = crossed || (c.F.Road[c.Idx(x, 25)] && c.R.Terrain.At(x, 25) == terrain.Track)
	}
	if !crossed {
		t.Fatal("marsh not crossed on Track")
	}
}
