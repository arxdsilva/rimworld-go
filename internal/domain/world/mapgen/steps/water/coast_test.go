package water

import (
	"testing"

	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/site"
)

func TestCoastDeterministic(t *testing.T) {
	s := site.Site{Hills: site.Flat, Coast: dirs(0, 2), CoastNear: 2}
	a, b, o := fixture(7, 120, 100, s), fixture(7, 120, 100, s), fixture(8, 120, 100, s)
	run(t, a, Coast{})
	run(t, b, Coast{})
	run(t, o, Coast{})
	if !sameFields(a.F, b.F) {
		t.Fatal("same seed gave different coasts")
	}
	if sameFields(a.F, o.F) {
		t.Fatal("other seed gave the same coast")
	}
}

// Walking inland from every coast edge, the bands come deep, then shallow,
// then beach, then dry land, never out of order.
func TestCoastBandsOrdered(t *testing.T) {
	type walk struct {
		d              site.Dir8
		x0, y0, dx, dy int // start and step; the lateral offset moves along the edge
		lx, ly         int
	}
	const W, H = 200, 200
	walks := []walk{
		{0, 0, 0, 0, 1, 1, 0},
		{2, W - 1, 0, -1, 0, 0, 1},
		{4, 0, H - 1, 0, -1, 1, 0},
		{6, 0, 0, 1, 0, 0, 1},
		{1, W - 1, 0, -1, 1, -1, -1}, // NE corner: diagonal walks inward, lateral offsets slide along it
	}
	for _, wk := range walks {
		for seed := uint64(1); seed <= 4; seed++ {
			c := fixture(seed, W, H, site.Site{Hills: site.Flat, Coast: dirs(wk.d), CoastNear: 2})
			run(t, c, Coast{})
			sawShallow, sawBeach := false, false
			for k := -20; k <= 20; k += 4 {
				x, y := wk.x0, wk.y0
				if wk.d%2 == 0 {
					x, y = x+wk.lx*(W/2+k), y+wk.ly*(H/2+k)
				} else {
					// shift the diagonal start along the corner's edges
					if k < 0 {
						x += k
					} else {
						y += k
					}
				}
				prev := 3
				first := true
				for c.In(x, y) {
					rk := rank(c.F.Water[c.Idx(x, y)])
					if first && rk < 2 {
						t.Fatalf("dir %d seed %d: edge cell (%d,%d) is not water", wk.d, seed, x, y)
					}
					first = false
					if rk > prev {
						t.Fatalf("dir %d seed %d: band order broken at (%d,%d): %d after %d", wk.d, seed, x, y, rk, prev)
					}
					sawShallow = sawShallow || rk == 2
					sawBeach = sawBeach || rk == 1
					prev = rk
					x, y = x+wk.dx, y+wk.dy
				}
			}
			if !sawShallow || !sawBeach {
				t.Fatalf("dir %d seed %d: shallow %v beach %v", wk.d, seed, sawShallow, sawBeach)
			}
		}
	}
}

func oceanCells(c *mapgen.Ctx) int {
	return count(c, func(i int) bool { return c.F.Coast[i] && c.F.Water[i].Wet() })
}

func TestCoastWidthGrowsAsOceanNears(t *testing.T) {
	for seed := uint64(1); seed <= 5; seed++ {
		prev := -1
		for _, near := range []uint8{6, 4, 2, 1} {
			c := fixture(seed, 160, 160, site.Site{Hills: site.Flat, Coast: dirs(6), CoastNear: near})
			run(t, c, Coast{})
			n := oceanCells(c)
			if n <= prev {
				t.Fatalf("seed %d: CoastNear %d gave %d ocean cells, not more than %d", seed, near, n, prev)
			}
			prev = n
		}
	}
	// a bigger map has a proportionally wider coast
	small := fixture(3, 150, 150, site.Site{Hills: site.Flat, Coast: dirs(0), CoastNear: 1})
	big := fixture(3, 300, 300, site.Site{Hills: site.Flat, Coast: dirs(0), CoastNear: 1})
	run(t, small, Coast{})
	run(t, big, Coast{})
	if float64(oceanCells(big))/300 < 1.6*float64(oceanCells(small))/150 {
		t.Fatal("coast width does not scale with the map")
	}
}

func TestCoastOceanConnectedToItsEdge(t *testing.T) {
	for seed := uint64(1); seed <= 6; seed++ {
		c := fixture(seed, 140, 110, site.Site{Hills: site.Flat, Coast: dirs(0, 3, 6), CoastNear: 1})
		run(t, c, Coast{})
		wet := make([]bool, c.W*c.H)
		for i := range wet {
			wet[i] = c.F.Coast[i] && c.F.Water[i].Wet()
		}
		label, sizes := mapgen.Components(c.W, c.H, wet, false)
		touches := make([]bool, len(sizes))
		for i, l := range label {
			if l < 0 {
				continue
			}
			for _, d := range []site.Dir8{0, 3, 6} {
				if onBorder(c, i, d) {
					touches[l] = true
				}
			}
		}
		for k, ok := range touches {
			if !ok {
				t.Fatalf("seed %d: ocean piece %d (%d cells) is cut off from its edge", seed, k, sizes[k])
			}
		}
		// every beach cell lies close to the sea (a band, not specks): at most
		// the widest beach away, √2 times that where a corner's band meets
		// the map border
		for i, w := range c.F.Water {
			if w == mapgen.Beach && c.F.CoastDist[i] > 8*1.415+1 {
				t.Fatalf("seed %d: beach cell %d is %v from the sea", seed, i, c.F.CoastDist[i])
			}
		}
	}
}

func TestCoastDistAndMasks(t *testing.T) {
	c := fixture(4, 120, 120, site.Site{Hills: site.Flat, Coast: dirs(4), CoastNear: 3})
	for i := range c.F.Rock {
		c.F.Rock[i] = true // the sea cuts through rock
	}
	run(t, c, Coast{})
	for i, w := range c.F.Water {
		switch {
		case w.Wet():
			if c.F.CoastDist[i] != 0 || !c.F.Coast[i] || c.F.Rock[i] {
				t.Fatalf("sea cell %d: dist %v coast %v rock %v", i, c.F.CoastDist[i], c.F.Coast[i], c.F.Rock[i])
			}
		case c.F.Coast[i]:
			if w != mapgen.Beach {
				t.Fatalf("coast cell %d is %v", i, w)
			}
		}
		if c.F.CoastDist[i] == mapgen.Far || c.F.CoastDist[i] < 0 {
			t.Fatalf("cell %d has no coast distance", i)
		}
	}
	// distance grows inland (north of a south coast)
	x := c.W / 2
	if !(c.F.CoastDist[c.Idx(x, 0)] > c.F.CoastDist[c.Idx(x, c.H/2)]) {
		t.Fatal("coast distance does not grow inland")
	}
}

func TestCoastNoneLeavesNoSea(t *testing.T) {
	c := fixture(1, 80, 60, site.Site{Hills: site.Flat})
	run(t, c, Coast{})
	for i := range c.F.Water {
		if c.F.Water[i] != mapgen.Dry || c.F.Coast[i] || c.F.CoastDist[i] != mapgen.Far {
			t.Fatalf("featureless map has coast at %d", i)
		}
	}
}

func TestCoastTinyMaps(t *testing.T) {
	for _, sz := range [][2]int{{1, 1}, {3, 2}, {24, 24}, {50, 30}} {
		c := fixture(9, sz[0], sz[1], site.Site{Hills: site.Flat, Coast: 0xff, CoastNear: 1})
		run(t, c, Coast{})
		if sz[0] >= 24 && count(c, func(i int) bool { return c.F.Water[i] == mapgen.Dry }) == 0 {
			t.Fatalf("%v: the sea drowned the whole map", sz)
		}
	}
}
