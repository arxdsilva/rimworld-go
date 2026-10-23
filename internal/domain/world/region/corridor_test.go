package region

import (
	"testing"

	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/pkg/randx"
)

// colonyMap is a random big map: rock blobs, a river with fords, walled
// buildings with doors.
func colonyMap(r *randx.Rand, w, h int) *world {
	wd := &world{w: w, h: h, blocked: make([]bool, w*h), flags: make([]Flags, w*h), m: New(w, h)}
	set := func(x, y int, b bool, f Flags) {
		if x >= 0 && y >= 0 && x < w && y < h {
			wd.set(y*w+x, b, f)
		}
	}
	for k := w * h / 900; k > 0; k-- { // rock blobs
		cx, cy, rad := r.IntN(w), r.IntN(h), 2+r.IntN(9)
		for y := cy - rad; y <= cy+rad; y++ {
			for x := cx - rad; x <= cx+rad; x++ {
				if (x-cx)*(x-cx)+(y-cy)*(y-cy) <= rad*rad+r.IntN(rad+1) {
					set(x, y, true, 0)
				}
			}
		}
	}
	rx := w/3 + r.IntN(w/3) // a river across, with a few fords
	for y := 0; y < h; y++ {
		x := rx + (y/7)%3
		for d := 0; d < 3; d++ {
			set(x+d, y, r.IntN(40) != 0 || y%37 > 3, 0)
		}
		if y%53 < 3 {
			for d := 0; d < 3; d++ {
				set(x+d, y, false, 0)
			}
		}
	}
	for k := w * h / 4000; k > 0; k-- { // walled buildings
		x0, y0 := r.IntN(w), r.IntN(h)
		x1, y1 := x0+4+r.IntN(14), y0+4+r.IntN(14)
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				switch {
				case x == x0 || x == x1 || y == y0 || y == y1:
					set(x, y, true, 0)
				default:
					set(x, y, false, 0)
				}
			}
		}
		set(x0, y0+2, false, Door)
		set(x1, y1-1, false, Door)
	}
	return wd
}

// TestSetCellsMatchesEachCell: marking a region's cells a chunk row at a
// time sets exactly its cells, on widths whose chunk rows straddle words.
func TestSetCellsMatchesEachCell(t *testing.T) {
	for seed := uint64(1); seed <= 4; seed++ {
		r := randx.New(seed)
		w, h := 97+r.IntN(90), 61+r.IntN(70)
		wd := colonyMap(r, w, h)
		wd.m.Label(Colonist, 0, 0) // brings the regions up to date
		m := wd.m
		m.graph()
		words := (w*h + 63) / 64
		for reg := int32(0); reg < int32(m.p.n); reg++ {
			got, want := make([]uint64, words), make([]uint64, words)
			m.setCells(reg, got)
			m.eachCell(reg, func(i int) { want[i>>6] |= 1 << (uint(i) & 63) })
			for k := range got {
				if got[k] != want[k] {
					t.Fatalf("seed %d %dx%d region %d: word %d = %#x, want %#x", seed, w, h, reg, k, got[k], want[k])
				}
			}
		}
	}
}

func pathCost(sx, sy int, p []pathing.Point) int {
	c := 0
	for _, s := range p {
		c += pathing.Octile(sx, sy, s.X, s.Y)
		sx, sy = s.X, s.Y
	}
	return c
}

// TestCorridorPathValid: on big random maps, a long path searched inside
// the region corridor is legal (walkable, 8-connected, no corner cutting:
// FindIn walks the layer's own query) and costs at most 10% more than the
// optimal one; across the sample it is close to optimal on average.
func TestCorridorPathValid(t *testing.T) {
	worst, sum, n := 1.0, 0.0, 0
	for seed := uint64(1); seed <= 12; seed++ {
		r := randx.New(seed)
		w, h := 180+r.IntN(120), 160+r.IntN(120)
		wd := colonyMap(r, w, h)
		for l := Layer(0); l < Layers; l++ {
			q := layerQuery{w: w, h: h, blocked: wd.blocked, flags: wd.flags, l: l}
			for k := 0; k < 25; k++ {
				sx, sy, tx, ty := r.IntN(w), r.IntN(h), r.IntN(w), r.IntN(h)
				if !q.Walkable(sx, sy) || !q.Walkable(tx, ty) || pathing.Octile(sx, sy, tx, ty) <= 960 {
					continue
				}
				best, ok := pathing.Find(q, sx, sy, tx, ty)
				allow, cok := wd.m.Corridor(l, sx, sy, tx, ty)
				if ok != cok {
					t.Fatalf("seed %d layer %d: corridor found=%v, A* %v", seed, l, cok, ok)
				}
				if !ok {
					continue
				}
				p, pok := pathing.FindIn(q, allow, sx, sy, tx, ty)
				if !pok {
					t.Fatalf("seed %d layer %d (%d,%d)->(%d,%d): no path inside the corridor", seed, l, sx, sy, tx, ty)
				}
				px, py := sx, sy
				for _, s := range p {
					if !q.Walkable(s.X, s.Y) || !pathing.Adjacent(px, py, s.X, s.Y) {
						t.Fatalf("seed %d: illegal step (%d,%d)->(%d,%d)", seed, px, py, s.X, s.Y)
					}
					if s.X != px && s.Y != py && (!q.Walkable(s.X, py) || !q.Walkable(px, s.Y)) {
						t.Fatalf("seed %d: corner cut at (%d,%d)", seed, s.X, s.Y)
					}
					px, py = s.X, s.Y
				}
				if px != tx || py != ty {
					t.Fatalf("seed %d: path ends at (%d,%d), want (%d,%d)", seed, px, py, tx, ty)
				}
				ratio := float64(pathCost(sx, sy, p)) / float64(pathCost(sx, sy, best))
				if ratio > 1.10 {
					t.Fatalf("seed %d layer %d (%d,%d)->(%d,%d): corridor path costs %.3f× optimal", seed, l, sx, sy, tx, ty, ratio)
				}
				worst, sum, n = max(worst, ratio), sum+ratio, n+1
			}
		}
	}
	if n < 100 {
		t.Fatalf("only %d long pairs sampled", n)
	}
	t.Logf("%d long paths: mean %.4f×, worst %.4f× optimal", n, sum/float64(n), worst)
}
