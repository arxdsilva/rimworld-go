// Package water holds the MAPGEN-2 water passes (.cursor/PLAY013_PLAN.md
// §2.3): the coast bands, the river with its banks, floodplain, canyon and
// fords, the lakes, and the wetness field that finishes after them. All
// numbers are our own.
package water

import (
	"math"

	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/site"
)

// chamfer turns d (0 or a known distance at sources, mapgen.Far elsewhere)
// into the distance to the nearest source along 8-connected steps of 1 and
// √2 (within 8% of the straight-line distance), in two raster passes.
func chamfer(w, h int, d []float32) {
	const diag = float32(math.Sqrt2)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*w + x
			v := d[i]
			if x > 0 {
				v = min(v, d[i-1]+1)
			}
			if y > 0 {
				j := i - w
				v = min(v, d[j]+1)
				if x > 0 {
					v = min(v, d[j-1]+diag)
				}
				if x < w-1 {
					v = min(v, d[j+1]+diag)
				}
			}
			d[i] = v
		}
	}
	for y := h - 1; y >= 0; y-- {
		for x := w - 1; x >= 0; x-- {
			i := y*w + x
			v := d[i]
			if x < w-1 {
				v = min(v, d[i+1]+1)
			}
			if y < h-1 {
				j := i + w
				v = min(v, d[j]+1)
				if x < w-1 {
					v = min(v, d[j+1]+diag)
				}
				if x > 0 {
					v = min(v, d[j-1]+diag)
				}
			}
			d[i] = v
		}
	}
}

// distFrom is the chamfer distance to the nearest src cell (mapgen.Far
// everywhere when there is none).
func distFrom(w, h int, src []bool) []float32 {
	d := make([]float32, w*h)
	found := false
	for i := range d {
		if src[i] {
			found = true
		} else {
			d[i] = mapgen.Far
		}
	}
	if found {
		chamfer(w, h, d)
	}
	return d
}

// table is a piecewise-linear curve through (xs[k], ys[k]), flat beyond.
func table(xs, ys []float64, x float64) float64 {
	if x <= xs[0] {
		return ys[0]
	}
	for k := 1; k < len(xs); k++ {
		if x <= xs[k] {
			t := (x - xs[k-1]) / (xs[k] - xs[k-1])
			return ys[k-1] + (ys[k]-ys[k-1])*t
		}
	}
	return ys[len(ys)-1]
}

// spread maps noise (≈0.5 ± 0.25 most of the time) onto the full [0, 1].
func spread(v float64) float64 { return mapgen.Clamp01((v - 0.25) * 2) }

// inward is the unit vector from map edge d into the map (y grows south).
func inward(d site.Dir8) mapgen.PointF {
	v := [8][2]float64{{0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}, {1, 0}, {1, 1}}[d&7]
	l := math.Hypot(v[0], v[1])
	return mapgen.PointF{X: v[0] / l, Y: v[1] / l}
}

// edgePoint is the border point at share p (0..1) along edge d, in
// continuous map units ([0, W]×[0, H]). A corner direction runs along the
// ends of its two edges through the corner.
func edgePoint(w, h int, d site.Dir8, p float64) mapgen.PointF {
	W, H := float64(w), float64(h)
	switch d & 7 {
	case 0:
		return mapgen.PointF{X: p * W, Y: 0}
	case 2:
		return mapgen.PointF{X: W, Y: p * H}
	case 4:
		return mapgen.PointF{X: p * W, Y: H}
	case 6:
		return mapgen.PointF{X: 0, Y: p * H}
	}
	e := 0.25 * math.Min(W, H)
	var k, u1, u2 mapgen.PointF
	switch d & 7 {
	case 1:
		k, u1, u2 = mapgen.PointF{X: W}, mapgen.PointF{X: -1}, mapgen.PointF{Y: 1}
	case 3:
		k, u1, u2 = mapgen.PointF{X: W, Y: H}, mapgen.PointF{Y: -1}, mapgen.PointF{X: -1}
	case 5:
		k, u1, u2 = mapgen.PointF{Y: H}, mapgen.PointF{X: 1}, mapgen.PointF{Y: -1}
	default:
		k, u1, u2 = mapgen.PointF{}, mapgen.PointF{Y: 1}, mapgen.PointF{X: 1}
	}
	q := (p - 0.5) * 2
	if q < 0 {
		return mapgen.PointF{X: k.X - q*e*u1.X, Y: k.Y - q*e*u1.Y}
	}
	return mapgen.PointF{X: k.X + q*e*u2.X, Y: k.Y + q*e*u2.Y}
}

// resample is the polyline pts re-spaced at step units of arc length
// (first and last points kept).
func resample(pts []mapgen.PointF, step float64) []mapgen.PointF {
	if len(pts) < 2 {
		return pts
	}
	out := []mapgen.PointF{pts[0]}
	carry := 0.0 // arc length since the last emitted point
	for k := 1; k < len(pts); k++ {
		a, b := pts[k-1], pts[k]
		l := math.Hypot(b.X-a.X, b.Y-a.Y)
		pos := step - carry
		for ; pos <= l; pos += step {
			t := pos / l
			out = append(out, mapgen.PointF{X: a.X + (b.X-a.X)*t, Y: a.Y + (b.Y-a.Y)*t})
		}
		carry = l - (pos - step)
	}
	if last := pts[len(pts)-1]; out[len(out)-1] != last {
		if carry < step*0.3 && len(out) > 1 {
			out[len(out)-1] = last
		} else {
			out = append(out, last)
		}
	}
	return out
}
