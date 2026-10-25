package landform

import (
	"math"
	"testing"

	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/site"
)

// fillRect marks [x0, x1) × [y0, y1) in m.
func fillRect(c *mapgen.Ctx, m []bool, x0, y0, x1, y1 int, v bool) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			m[c.Idx(x, y)] = v
		}
	}
}

func caveCount(c *mapgen.Ctx) int {
	n := 0
	for _, v := range c.F.Cave {
		if v {
			n++
		}
	}
	return n
}

// bigRock is a 200×150 map with one 120×90 block of rock (10800 cells)
// and one 12×12 block (144 cells, under the threshold).
func bigRock(seed uint64, caves bool) *mapgen.Ctx {
	c := newCtx(200, 150, seed, site.Site{Hills: site.Mountainous, Caves: caves})
	fillRect(c, c.F.Rock, 40, 30, 160, 120, true)
	fillRect(c, c.F.Rock, 5, 5, 17, 17, true)
	return c
}

func TestCavesOnlyWithSiteCaves(t *testing.T) {
	c := bigRock(1, false)
	runSteps(t, c, Caves{})
	if caveCount(c) != 0 {
		t.Fatal("caves without Site.Caves")
	}
	for seed := uint64(0); seed < 3; seed++ {
		c := newCtx(150, 150, seed, site.Site{Hills: site.Mountainous})
		runSteps(t, c, Elevation{}, RockMask{}, Caves{})
		if caveCount(c) != 0 {
			t.Fatal("generated map has caves without Site.Caves")
		}
	}
	ice := bigRock(1, true)
	ice.Cfg.Biome = biome.DefOf(biome.SeaIce)
	runSteps(t, ice, Caves{})
	if caveCount(ice) != 0 {
		t.Fatal("caves on sea ice")
	}
}

// Caves stay inside qualifying rock, leave Rock set, never reach the map
// edge, and an open tunnel reaches open ground.
func TestCavesInsideLargeRock(t *testing.T) {
	for seed := uint64(0); seed < 8; seed++ {
		c := bigRock(seed, true)
		before := append([]bool(nil), c.F.Rock...)
		runSteps(t, c, Caves{})
		if caveCount(c) < 40 {
			t.Fatalf("seed %d: only %d cave cells", seed, caveCount(c))
		}
		mouth := false
		for i, cave := range c.F.Cave {
			if c.F.Rock[i] != before[i] {
				t.Fatalf("seed %d: caves changed the rock mask", seed)
			}
			if !cave {
				continue
			}
			x, y := c.XY(i)
			if x < 40 || x >= 160 || y < 30 || y >= 120 {
				t.Fatalf("seed %d: cave cell (%d, %d) outside its rock body", seed, x, y)
			}
			for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				if j := c.Idx(x+d[0], y+d[1]); !c.F.Rock[j] {
					mouth = true
				}
			}
		}
		if !mouth {
			t.Fatalf("seed %d: no cave cell touches open ground", seed)
		}
	}
}

// The cave threshold grows with the map's area.
func TestCavesThresholdScales(t *testing.T) {
	c := newCtx(500, 500, 2, site.Site{Caves: true})
	fillRect(c, c.F.Rock, 100, 100, 135, 135, true) // 1225 cells: enough at 250², not at 500²
	runSteps(t, c, Caves{})
	if caveCount(c) != 0 {
		t.Fatal("caves in a body under the scaled threshold")
	}
	small := newCtx(250, 250, 2, site.Site{Caves: true})
	fillRect(small, small.F.Rock, 100, 100, 135, 135, true)
	runSteps(t, small, Caves{})
	if caveCount(small) == 0 {
		t.Fatal("no caves in a 1225-cell body at 250²")
	}
}

// A map that is all rock gets closed tunnels only, away from the edge.
func TestCavesAllRockClosedAwayFromEdge(t *testing.T) {
	c := newCtx(120, 120, 5, site.Site{Caves: true})
	fillRect(c, c.F.Rock, 0, 0, 120, 120, true)
	runSteps(t, c, Caves{})
	if caveCount(c) == 0 {
		t.Fatal("no closed tunnel in solid rock")
	}
	for i, v := range c.F.Cave {
		x, y := c.XY(i)
		if v && (x < edgeMargin || y < edgeMargin || x >= c.W-edgeMargin || y >= c.H-edgeMargin) {
			t.Fatalf("cave cell (%d, %d) at the map edge", x, y)
		}
	}
}

// Tunnels start 3.5–5.5 wide, taper to no less than 1.3, bend smoothly,
// branch at most to level 2, and closed ones start deep.
func TestTunnelShape(t *testing.T) {
	branched := false
	for seed := uint64(0); seed < 10; seed++ {
		c := bigRock(seed, true)
		depth := mapgen.ChebDepth(c.W, c.H, c.F.Rock)
		tr := carve(c, c.Rand("caves"))
		if len(tr) == 0 {
			t.Fatalf("seed %d: no tunnels", seed)
		}
		open := 0
		for _, tu := range tr {
			if tu.level > maxLevel {
				t.Fatalf("branch level %d", tu.level)
			}
			if tu.level > 0 {
				branched = true
			}
			p0 := tu.pts[0]
			if tu.level == 0 && (p0.w < widthLo || p0.w > widthHi) {
				t.Fatalf("seed %d: start width %.2f", seed, p0.w)
			}
			if tu.level == 0 && tu.open {
				open++
			}
			if !tu.open && tu.level == 0 && depth[c.Idx(int(p0.x), int(p0.y))] < 6 {
				t.Fatalf("seed %d: closed tunnel starts at depth %d", seed, depth[c.Idx(int(p0.x), int(p0.y))])
			}
			for k, p := range tu.pts {
				if p.w < minWidth {
					t.Fatalf("width %.2f under the minimum", p.w)
				}
				if k > 0 {
					if p.w >= tu.pts[k-1].w {
						t.Fatalf("width does not taper: %.3f after %.3f", p.w, tu.pts[k-1].w)
					}
					if turn := math.Abs(angleDiff(p.head, tu.pts[k-1].head)); turn > maxTurn+1e-9 {
						t.Fatalf("turn %.3f rad in one step", turn)
					}
				}
			}
		}
		if open == 0 {
			t.Fatalf("seed %d: no open tunnel", seed)
		}
	}
	if !branched {
		t.Fatal("no tunnel ever branched over 10 seeds")
	}
}

// Carved cells near a tunnel's start outnumber those near its end. A
// tunnel that ends where another one passes measures the junction, not
// its own width, so those are left out.
func TestTunnelCarvedWidthTapers(t *testing.T) {
	wider := 0
	total := 0
	for seed := uint64(0); seed < 10; seed++ {
		c := bigRock(seed, true)
		trs := carve(c, c.Rand("caves"))
		for k, tu := range trs {
			if tu.level != 0 || len(tu.pts) < 25 || endsNearOther(trs, k, 8) {
				continue
			}
			near := func(p tracePt) int {
				n := 0
				for dy := -4; dy <= 4; dy++ {
					for dx := -4; dx <= 4; dx++ {
						x, y := int(p.x)+dx, int(p.y)+dy
						if c.In(x, y) && c.F.Cave[c.Idx(x, y)] {
							n++
						}
					}
				}
				return n
			}
			// Sample inside the tunnel, a few steps from both ends.
			a, b := near(tu.pts[6]), near(tu.pts[len(tu.pts)-3])
			total++
			if a > b {
				wider++
			}
		}
	}
	if total == 0 || float64(wider) < 0.75*float64(total) {
		t.Fatalf("start wider than end in %d of %d tunnels", wider, total)
	}
}

// endsNearOther reports whether tunnel k's last point lies within r of
// another tunnel's path.
func endsNearOther(trs []trace, k int, r float64) bool {
	e := trs[k].pts[len(trs[k].pts)-1]
	for j, o := range trs {
		if j == k {
			continue
		}
		for _, p := range o.pts {
			if math.Hypot(p.x-e.x, p.y-e.y) <= r {
				return true
			}
		}
	}
	return false
}

func TestCavesDeterministic(t *testing.T) {
	a, b := bigRock(4, true), bigRock(4, true)
	runSteps(t, a, Caves{})
	runSteps(t, b, Caves{})
	o := bigRock(5, true)
	runSteps(t, o, Caves{})
	diff := false
	for i := range a.F.Cave {
		if a.F.Cave[i] != b.F.Cave[i] {
			t.Fatal("same seed, different caves")
		}
		if a.F.Cave[i] != o.F.Cave[i] {
			diff = true
		}
	}
	if !diff {
		t.Fatal("other seed, same caves")
	}
}

// On generated mountainous maps caves appear and stay in big bodies.
func TestCavesOnGeneratedMaps(t *testing.T) {
	for seed := uint64(0); seed < 4; seed++ {
		c := newCtx(200, 200, seed, site.Site{Hills: site.Mountainous, Caves: true})
		runSteps(t, c, Elevation{}, RockMask{})
		label, sizes := mapgen.Components(c.W, c.H, c.F.Rock, false)
		runSteps(t, c, Caves{})
		if caveCount(c) == 0 {
			t.Fatalf("seed %d: no caves on a mountainous map", seed)
		}
		min := caveMinCells(c)
		for i, v := range c.F.Cave {
			if v && (label[i] < 0 || sizes[label[i]] < min) {
				t.Fatalf("seed %d: cave outside a qualifying rock body", seed)
			}
		}
	}
}

func TestAngleDiff(t *testing.T) {
	if d := angleDiff(0.1, 2*math.Pi-0.1); math.Abs(d-0.2) > 1e-9 {
		t.Fatalf("angleDiff = %v", d)
	}
}
