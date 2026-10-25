package water

import (
	"math"
	"testing"

	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/site"
)

func riverSite(size site.RiverSize, in, out site.Dir8) site.Site {
	return site.Site{Hills: site.Flat, River: site.RiverPath{Size: size, In: in, Out: out}}
}

func channel(c *mapgen.Ctx, i int) bool { return c.F.River[i] && c.F.Water[i].Wet() }

func TestRiverDeterministic(t *testing.T) {
	s := riverSite(site.River, 0, 4)
	a, b, o := fixture(5, 150, 150, s), fixture(5, 150, 150, s), fixture(6, 150, 150, s)
	run(t, a, River{})
	run(t, b, River{})
	run(t, o, River{})
	if !sameFields(a.F, b.F) {
		t.Fatal("same seed gave different rivers")
	}
	if sameFields(a.F, o.F) {
		t.Fatal("other seed gave the same river")
	}
}

func TestRiverNoneLeavesNoWater(t *testing.T) {
	c := fixture(1, 90, 70, site.Site{Hills: site.Flat})
	run(t, c, River{})
	if len(c.F.RiverLine) != 0 {
		t.Fatal("river line without a river")
	}
	for i := range c.F.Water {
		if c.F.Water[i] != mapgen.Dry || c.F.River[i] || c.F.RiverDist[i] != mapgen.Far {
			t.Fatalf("featureless map has river at %d", i)
		}
	}
}

// The channel is one 4-connected body of water touching both its edges.
func TestRiverTouchesBothEdgesContinuously(t *testing.T) {
	paths := [][2]site.Dir8{{0, 4}, {6, 2}, {0, 2}, {4, 4}, {6, 6}, {1, 5}, {7, 4}, {2, 3}}
	for _, sz := range []site.RiverSize{site.Creek, site.River, site.LargeRiver} {
		for _, p := range paths {
			for seed := uint64(1); seed <= 3; seed++ {
				for _, dim := range [][2]int{{200, 160}, {60, 60}, {24, 24}} {
					c := fixture(seed, dim[0], dim[1], riverSite(sz, p[0], p[1]))
					run(t, c, River{})
					wet := make([]bool, c.W*c.H)
					for i := range wet {
						wet[i] = channel(c, i)
					}
					label, sizes := mapgen.Components(c.W, c.H, wet, false)
					if len(sizes) != 1 {
						t.Fatalf("size %d path %v seed %d %v: %d channel pieces", sz, p, seed, dim, len(sizes))
					}
					in, out := false, false
					for i, l := range label {
						if l == 0 {
							in = in || onBorder(c, i, p[0])
							out = out || onBorder(c, i, p[1])
						}
					}
					if !in || !out {
						t.Fatalf("size %d path %v seed %d %v: touches in %v out %v", sz, p, seed, dim, in, out)
					}
					if len(c.F.RiverLine) < 2 {
						t.Fatal("no river line")
					}
				}
			}
		}
	}
}

func channelWidth(c *mapgen.Ctx) float64 {
	return float64(count(c, func(i int) bool { return channel(c, i) })) / lineLen(c.F.RiverLine)
}

func lineLen(l []mapgen.PointF) float64 {
	s := 0.0
	for i := 1; i < len(l); i++ {
		s += math.Hypot(l[i].X-l[i-1].X, l[i].Y-l[i-1].Y)
	}
	return s
}

func TestRiverWidthsAndCore(t *testing.T) {
	for seed := uint64(1); seed <= 4; seed++ {
		prev := 0.0
		for _, sz := range []site.RiverSize{site.Creek, site.River, site.LargeRiver} {
			c := fixture(seed, 250, 250, riverSite(sz, 6, 2))
			run(t, c, River{})
			w := channelWidth(c)
			if w <= prev*1.3 {
				t.Fatalf("seed %d: size %d width %.2f not clearly above %.2f", seed, sz, w, prev)
			}
			prev = w
			deep := count(c, func(i int) bool { return c.F.River[i] && c.F.Water[i] == mapgen.Deep })
			if (sz == site.Creek) != (deep == 0) {
				t.Fatalf("seed %d size %d: %d deep cells", seed, sz, deep)
			}
		}
	}
}

// Banks hug the channel and the floodplain hugs the banks; both sides have
// a bank.
func TestRiverBandsBesideChannel(t *testing.T) {
	for seed := uint64(1); seed <= 4; seed++ {
		c := fixture(seed, 220, 220, riverSite(site.LargeRiver, 0, 4))
		run(t, c, River{})
		near := func(i int, ok func(j int) bool) bool {
			x, y := c.XY(i)
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if c.In(x+dx, y+dy) && ok(c.Idx(x+dx, y+dy)) {
						return true
					}
				}
			}
			return false
		}
		banks, plain := 0, 0
		for i, w := range c.F.Water {
			switch w {
			case mapgen.Bank:
				banks++
				if !c.F.River[i] || !near(i, func(j int) bool {
					return channel(c, j) || c.F.Water[j] == mapgen.Bank && c.F.RiverDist[j] < c.F.RiverDist[i]
				}) {
					t.Fatalf("seed %d: bank cell %d away from the channel", seed, i)
				}
			case mapgen.Floodplain:
				plain++
				if !c.F.River[i] || !near(i, func(j int) bool {
					return c.F.Water[j] == mapgen.Bank || c.F.Water[j] == mapgen.Floodplain && c.F.RiverDist[j] < c.F.RiverDist[i]
				}) {
					t.Fatalf("seed %d: floodplain cell %d away from the bank", seed, i)
				}
			}
		}
		if banks == 0 || plain == 0 {
			t.Fatalf("seed %d: %d bank, %d floodplain cells", seed, banks, plain)
		}
		// every dry cell beside the channel is bank
		for i := range c.F.Water {
			if !channel(c, i) {
				continue
			}
			x, y := c.XY(i)
			for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				if j := c.Idx(x+d[0], y+d[1]); c.In(x+d[0], y+d[1]) && !channel(c, j) && c.F.Water[j] != mapgen.Bank {
					t.Fatalf("seed %d: cell (%d,%d) beside the channel is %v", seed, x+d[0], y+d[1], c.F.Water[j])
				}
			}
		}
	}
}

// A forced bend: the floodplain is wider on the inside of the curve.
func TestRiverFloodplainWiderInsideBend(t *testing.T) {
	const W, H, R = 200, 200, 45.0
	cx, cy := 100.0, 100.0
	var line []mapgen.PointF
	for y := float64(H + 10); y > cy; y -= 0.5 {
		line = append(line, mapgen.PointF{X: cx + R, Y: y})
	}
	for a := 0.0; a <= math.Pi; a += 0.5 / R {
		line = append(line, mapgen.PointF{X: cx + R*math.Cos(a), Y: cy - R*math.Sin(a)})
	}
	for y := cy; y < H+10; y += 0.5 {
		line = append(line, mapgen.PointF{X: cx - R, Y: y})
	}
	for seed := uint64(1); seed <= 3; seed++ {
		c := fixture(seed, W, H, riverSite(site.River, 4, 4))
		layRiver(c, c.Rand("river"), line, site.River)
		in, out := 0, 0
		for i, w := range c.F.Water {
			x, y := c.XY(i)
			if w != mapgen.Floodplain || float64(y) > cy-10 {
				continue
			}
			if math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy) < R {
				in++
			} else {
				out++
			}
		}
		if in <= out {
			t.Fatalf("seed %d: inside floodplain %d cells, outside %d", seed, in, out)
		}
	}
}

// Through solid rock the river leaves a walkable dry strip of at least two
// cells on both sides of its whole length, and rock stays beyond.
func TestRiverCanyon(t *testing.T) {
	for _, sz := range []site.RiverSize{site.Creek, site.River, site.LargeRiver} {
		for seed := uint64(1); seed <= 3; seed++ {
			s := riverSite(sz, 0, 4)
			s.Hills = site.Mountainous
			c := fixture(seed, 200, 200, s)
			for i := range c.F.Rock {
				c.F.Rock[i] = true
			}
			run(t, c, River{})
			l := c.F.RiverLine
			for k := 6; k < len(l)-6; k += 3 {
				tx, ty := l[k+1].X-l[k-1].X, l[k+1].Y-l[k-1].Y
				n := math.Hypot(tx, ty)
				nx, ny := -ty/n, tx/n
				for _, sd := range []float64{1, -1} {
					cell := func(d float64) (int, bool) {
						x, y := int(math.Floor(l[k].X+sd*nx*d)), int(math.Floor(l[k].Y+sd*ny*d))
						return c.Idx(x, y), c.In(x, y)
					}
					d := 0.0
					for ; d < 40; d += 0.25 {
						i, ok := cell(d)
						if !ok || !c.F.Water[i].Wet() {
							break
						}
					}
					for _, e := range []float64{0, 0.5, 1, 1.5} {
						i, ok := cell(d + e)
						if !ok {
							break
						}
						if c.F.Rock[i] || c.F.Water[i].Wet() {
							t.Fatalf("size %d seed %d: no dry walk beside the channel at %v (+%.1f)", sz, seed, l[k], d+e)
						}
					}
				}
			}
			rock := count(c, func(i int) bool { return c.F.Rock[i] })
			if rock < c.W*c.H/2 {
				t.Fatalf("size %d seed %d: the canyon cleared %d of %d rock cells", sz, seed, c.W*c.H-rock, c.W*c.H)
			}
		}
	}
}

// crossings are the arc positions along the river line where a walker can
// get from one side to the other through a short strip across the river.
func crossings(c *mapgen.Ctx) (pos []float64, total float64) {
	l := c.F.RiverLine
	arc := 0.0
	for k := 1; k < len(l)-1; k++ {
		arc += math.Hypot(l[k].X-l[k-1].X, l[k].Y-l[k-1].Y)
		tx, ty := l[k+1].X-l[k-1].X, l[k+1].Y-l[k-1].Y
		n := math.Hypot(tx, ty)
		tx, ty = tx/n, ty/n
		nx, ny := -ty, tx
		const R, half = 16.0, 3.0
		x0, x1 := int(l[k].X-R-1), int(l[k].X+R+1)
		y0, y1 := int(l[k].Y-R-1), int(l[k].Y+R+1)
		in := func(x, y int) (float64, bool) {
			px, py := float64(x)+0.5-l[k].X, float64(y)+0.5-l[k].Y
			a, b := px*tx+py*ty, px*nx+py*ny
			ok := c.In(x, y) && math.Abs(a) <= half && math.Abs(b) <= R
			if ok {
				i := c.Idx(x, y)
				ok = !c.F.Rock[i] && c.F.Water[i] != mapgen.Deep
			}
			return b, ok
		}
		// a local grid around the strip keeps the flood fill small
		x0, y0 = max(0, x0), max(0, y0)
		x1, y1 = min(c.W-1, x1), min(c.H-1, y1)
		lw, lh := x1-x0+1, y1-y0+1
		if lw <= 0 || lh <= 0 {
			continue
		}
		src := make([]bool, lw*lh)
		pass := make([]bool, lw*lh)
		var dst []int
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				b, ok := in(x, y)
				if !ok {
					continue
				}
				i, li := c.Idx(x, y), (y-y0)*lw+(x-x0)
				pass[li] = true
				if !c.F.Water[i].Wet() {
					if b > 0 {
						src[li] = true
					} else {
						dst = append(dst, li)
					}
				}
			}
		}
		d := mapgen.BFSDist(lw, lh, src, pass, false)
		for _, i := range dst {
			if d[i] >= 0 {
				pos = append(pos, arc)
				break
			}
		}
	}
	return pos, arc
}

func TestRiverCrossingGuarantee(t *testing.T) {
	for _, sz := range []site.RiverSize{site.Creek, site.River, site.LargeRiver} {
		for seed := uint64(1); seed <= 5; seed++ {
			for _, p := range [][2]site.Dir8{{0, 4}, {6, 2}, {2, 2}, {1, 6}} {
				c := fixture(seed, 300, 260, riverSite(sz, p[0], p[1]))
				run(t, c, River{})
				pos, total := crossings(c)
				if len(pos) == 0 {
					t.Fatalf("size %d seed %d path %v: no crossing at all", sz, seed, p)
				}
				prev := 0.0
				for _, s := range append(pos, total) {
					if s-prev > 90 {
						t.Fatalf("size %d seed %d path %v: %.0f cells without a crossing (from %.0f, length %.0f)", sz, seed, p, s-prev, prev, total)
					}
					prev = s
				}
			}
		}
	}
}

func TestRiverDistAndLine(t *testing.T) {
	c := fixture(3, 180, 140, riverSite(site.River, 6, 2))
	run(t, c, River{})
	for _, p := range c.F.RiverLine {
		if p.X < 0 || p.Y < 0 || p.X > float64(c.W) || p.Y > float64(c.H) {
			t.Fatalf("line point %v off the map", p)
		}
		x, y := min(c.W-1, int(p.X)), min(c.H-1, int(p.Y))
		if d := c.F.RiverDist[c.Idx(x, y)]; d > 1 {
			t.Fatalf("centreline cell (%d,%d) is %v from the river", x, y, d)
		}
	}
	for i, d := range c.F.RiverDist {
		if d == mapgen.Far || d < 0 {
			t.Fatalf("cell %d has no river distance", i)
		}
		if channel(c, i) && d > 12 {
			t.Fatalf("channel cell %d is %v from the centreline", i, d)
		}
	}
}

func TestRiverKeepsTheSea(t *testing.T) {
	for seed := uint64(1); seed <= 3; seed++ {
		c := fixture(seed, 200, 200, site.Site{Hills: site.Flat, Coast: dirs(4), CoastNear: 1,
			River: site.RiverPath{Size: site.LargeRiver, In: 0, Out: 4}})
		run(t, c, Coast{})
		sea := append([]mapgen.Water(nil), c.F.Water...)
		run(t, c, River{})
		for i, w := range sea {
			if w.Wet() && c.F.Water[i] != w {
				t.Fatalf("seed %d: river overwrote sea cell %d (%v -> %v)", seed, i, w, c.F.Water[i])
			}
			if w.Wet() && c.F.River[i] {
				t.Fatalf("seed %d: sea cell %d marked as river", seed, i)
			}
		}
		if count(c, func(i int) bool { return channel(c, i) }) == 0 {
			t.Fatal("no river")
		}
	}
}
