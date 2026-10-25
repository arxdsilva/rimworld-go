package water

import (
	"testing"

	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/site"
)

func lakeWater(c *mapgen.Ctx, i int) bool { return c.F.Lake[i] && c.F.Water[i].Wet() }

func TestLakesDeterministic(t *testing.T) {
	s := site.Site{Hills: site.Flat, Lakes: 2}
	a, b, o := fixture(2, 160, 160, s), fixture(2, 160, 160, s), fixture(3, 160, 160, s)
	run(t, a, Lakes{})
	run(t, b, Lakes{})
	run(t, o, Lakes{})
	if !sameFields(a.F, b.F) {
		t.Fatal("same seed gave different lakes")
	}
	if sameFields(a.F, o.F) {
		t.Fatal("other seed gave the same lakes")
	}
}

func TestLakesNone(t *testing.T) {
	c := fixture(1, 100, 100, site.Site{Hills: site.Flat})
	run(t, c, Lakes{})
	if count(c, func(i int) bool { return c.F.Lake[i] || c.F.Water[i] != mapgen.Dry }) != 0 {
		t.Fatal("lakes on a site without lakes")
	}
}

// Lakes keep their count and are ringed by shore: no lake water touches
// anything but more lake water or shore.
func TestLakesCountAndRing(t *testing.T) {
	for seed := uint64(1); seed <= 6; seed++ {
		for n := uint8(1); n <= 3; n++ {
			c := fixture(seed, 250, 250, site.Site{Hills: site.Flat, Lakes: n})
			for i := range c.F.Rock {
				x, y := c.XY(i)
				c.F.Rock[i] = (x/25+y/25)%2 == 0 && x < 60 // some rock to cut
			}
			run(t, c, Lakes{})
			wet := make([]bool, c.W*c.H)
			for i := range wet {
				wet[i] = lakeWater(c, i)
			}
			_, sizes := mapgen.Components(c.W, c.H, wet, false)
			if len(sizes) != int(n) {
				t.Fatalf("seed %d: %d lakes asked, %d drawn", seed, n, len(sizes))
			}
			deep := count(c, func(i int) bool { return c.F.Lake[i] && c.F.Water[i] == mapgen.Deep })
			if deep == 0 {
				t.Fatalf("seed %d: lakes without a deep middle", seed)
			}
			for i := range wet {
				if !c.F.Lake[i] {
					continue
				}
				if c.F.Rock[i] {
					t.Fatalf("seed %d: rock left in lake cell %d", seed, i)
				}
				if w := c.F.Water[i]; w != mapgen.Deep && w != mapgen.Shallow && w != mapgen.Shore {
					t.Fatalf("seed %d: lake cell %d is %v", seed, i, w)
				}
				if !wet[i] {
					continue
				}
				x, y := c.XY(i)
				for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
					if j := c.Idx(x+d[0], y+d[1]); c.In(x+d[0], y+d[1]) && !wet[j] && c.F.Water[j] != mapgen.Shore {
						t.Fatalf("seed %d: lake water %d touches %v", seed, i, c.F.Water[j])
					}
				}
			}
			// with a free map the lakes keep off the centre third
			for i := range wet {
				x, y := c.XY(i)
				if c.F.Lake[i] && x > c.W/3 && x < 2*c.W/3 && y > c.H/3 && y < 2*c.H/3 {
					t.Fatalf("seed %d: lake cell (%d,%d) in the centre third", seed, x, y)
				}
			}
		}
	}
}

func TestLakesAwayFromRiverAndCoast(t *testing.T) {
	for seed := uint64(1); seed <= 6; seed++ {
		c := fixture(seed, 250, 250, site.Site{Hills: site.Flat, Lakes: 3, Coast: dirs(6), CoastNear: 2,
			River: site.RiverPath{Size: site.River, In: 0, Out: 4}})
		run(t, c, Coast{}, River{})
		// the outline of the river and coast cells (the nearest one to any
		// outside cell is on it)
		var other []int
		taken := func(x, y int) bool { i := c.Idx(x, y); return c.F.River[i] || c.F.Coast[i] }
		for i := range c.F.River {
			x, y := c.XY(i)
			if !taken(x, y) {
				continue
			}
			for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				if c.In(x+d[0], y+d[1]) && !taken(x+d[0], y+d[1]) {
					other = append(other, i)
					break
				}
			}
		}
		run(t, c, Lakes{})
		lakes := 0
		for i := range c.F.Lake {
			if !c.F.Lake[i] {
				continue
			}
			lakes++
			if c.F.River[i] || c.F.Coast[i] {
				t.Fatalf("seed %d: lake cell %d on the river or coast", seed, i)
			}
			for _, j := range other {
				if dist2(c, i, j) < 12 {
					t.Fatalf("seed %d: lake cell %d is %.1f from river/coast cell %d", seed, i, dist2(c, i, j), j)
				}
			}
		}
		if lakes == 0 {
			t.Fatalf("seed %d: no room found for any lake", seed)
		}
	}
}

func TestLakesTinyMaps(t *testing.T) {
	for _, sz := range [][2]int{{1, 1}, {5, 3}, {24, 24}, {50, 50}} {
		c := fixture(4, sz[0], sz[1], site.Site{Hills: site.Flat, Lakes: 3, River: site.RiverPath{Size: site.Creek, In: 0, Out: 4}})
		all(t, c)
	}
}
