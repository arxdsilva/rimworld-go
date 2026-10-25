package water

import (
	"math"
	"sort"

	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/site"
	"rimworld-go/pkg/noise2"
	"rimworld-go/pkg/randx"
)

// River is the "river" pass (MAPGEN-2 §2.3): a meandering channel from
// its In edge to its Out edge (a U-bend when they match), with a deep
// core, banks, a floodplain wider on the inside of bends, a canyon through
// rock and fords. Writes River, RiverDist, RiverLine, Water; cuts Rock.
// It never touches the sea.
type River struct{}

// Name is the step's stream and label key.
func (River) Name() string { return "river" }

// River tuning (our own numbers).
const (
	riverMinSide      = 4    // smaller maps get no river
	meanderMin        = 0.16 // meander amplitude, share of the river's length
	meanderMax        = 0.24
	meanderEnds       = 0.15 // share of the length at each end that eases out of the meander
	widthVary         = 0.20 // smooth width variation along the river
	widthCell         = 50.0 // lattice of the width variation
	bankCell          = 15.0 // lattice of the bank width
	plainCell         = 25.0 // lattice of the floodplain width
	bankMin           = 1.2  // bank width range (cells); above 1 so every cell beside the channel is bank
	bankMax           = 3.0
	canyonBank        = 3.0 // walkable cut beside the channel through rock
	plainMin          = 2.0 // floodplain width range (cells)
	plainSpan         = 2.0 // noise adds up to this
	plainBend         = 2.0 // the inside of a tight bend adds up to this
	bendRadius        = 40.0
	curveHalfSpan     = 6 // points either side for the curvature estimate
	fordFirstMin      = 20.0
	fordFirstMax      = 40.0
	fordGapMin        = 60.0
	fordGapMax        = 86.0
	fordLenMin        = 3.0
	fordLenMax        = 5.0
	uBendDepthMin     = 0.45 // U-bend handle length, share of the map across
	uBendDepthMax     = 0.60
	edgeSpreadMin     = 0.25 // where on its edge the river enters or leaves
	edgeSpreadMax     = 0.75
	uEdgeNear         = 0.15 // U-bend ends: the first in [near, near+0.2], the second mirrored
	plainMaxCells     = plainMin + plainSpan + plainBend
	fordFixes         = 6    // rounds of the crossing check
	fordCheckHalf     = 3.0  // half length of the strip the check walks across
	meanderFixes      = 30   // rounds of local meander damping
	meanderSmallFloor = 0.2  // the big meander is damped only once the wiggles are this small
	meanderDamp       = 0.85 // displacement kept per round where a meander misbehaves
	meanderDampSpan   = 5    // cells over which a damping fades out
	halfWidthFloor    = 1.0  // keeps every channel 4-connected
)

// riverWidth is the channel width at Scale 1 (×√Scale), riverCore the
// deep core's share of it, by size.
var (
	riverWidth = [...]float64{site.Creek: 3, site.River: 7, site.LargeRiver: 12}
	riverCore  = [...]float64{site.Creek: 0, site.River: 0.30, site.LargeRiver: 0.45}
)

// Run is the pass.
func (River) Run(c *mapgen.Ctx) error {
	rp := c.Cfg.Site.River
	if rp.Size == site.NoRiver || int(rp.Size) >= len(riverWidth) || c.W < riverMinSide || c.H < riverMinSide {
		return nil
	}
	r := c.Rand("river")
	layRiver(c, r, riverCentre(c, r, rp), rp.Size)
	return nil
}

// across is the map's size across from edge d.
func across(c *mapgen.Ctx, d site.Dir8) float64 {
	switch d & 7 {
	case 0, 4:
		return float64(c.H)
	case 2, 6:
		return float64(c.W)
	}
	return float64(min(c.W, c.H))
}

func clampMap(c *mapgen.Ctx, p mapgen.PointF) mapgen.PointF {
	return mapgen.PointF{X: math.Max(0, math.Min(float64(c.W), p.X)), Y: math.Max(0, math.Min(float64(c.H), p.Y))}
}

// riverCentre is the dense centreline (one point per cell of length),
// running on past both edges so the channel reaches the map border.
func riverCentre(c *mapgen.Ctx, r *randx.Rand, rp site.RiverPath) []mapgen.PointF {
	frac := func(lo, hi float64) float64 { return lo + (hi-lo)*r.Float64() }
	in, out := rp.In&7, rp.Out&7
	var a, b mapgen.PointF
	var ha, hb float64
	if in == out {
		a = edgePoint(c.W, c.H, in, frac(uEdgeNear, uEdgeNear+0.2))
		b = edgePoint(c.W, c.H, in, frac(1-uEdgeNear-0.2, 1-uEdgeNear))
		ha = frac(uBendDepthMin, uBendDepthMax) * across(c, in)
		hb = ha
	} else {
		a = edgePoint(c.W, c.H, in, frac(edgeSpreadMin, edgeSpreadMax))
		b = edgePoint(c.W, c.H, out, frac(edgeSpreadMin, edgeSpreadMax))
		ha = math.Hypot(b.X-a.X, b.Y-a.Y) / 3
		hb = ha
	}
	na, nb := inward(in), inward(out)
	p1 := clampMap(c, mapgen.PointF{X: a.X + na.X*ha, Y: a.Y + na.Y*ha})
	p2 := clampMap(c, mapgen.PointF{X: b.X + nb.X*hb, Y: b.Y + nb.Y*hb})

	// the base curve: a cubic leaving and entering square to the edges
	hull := math.Hypot(p1.X-a.X, p1.Y-a.Y) + math.Hypot(p2.X-p1.X, p2.Y-p1.Y) + math.Hypot(b.X-p2.X, b.Y-p2.Y)
	m := int(2*hull) + 8
	pts := make([]mapgen.PointF, m+1)
	for k := range pts {
		t := float64(k) / float64(m)
		u := 1 - t
		w0, w1, w2, w3 := u*u*u, 3*u*u*t, 3*u*t*t, t*t*t
		pts[k] = mapgen.PointF{X: w0*a.X + w1*p1.X + w2*p2.X + w3*b.X, Y: w0*a.Y + w1*p1.Y + w2*p2.Y + w3*b.Y}
	}
	base := resample(pts, 1)

	// meanders: smooth sideways displacement along the length
	L := float64(len(base) - 1)
	amp := frac(meanderMin, meanderMax) * L
	cell1, cell2 := math.Max(1, L/3), math.Max(1, L/7)
	s1, s2 := c.NoiseSeed("river/meander"), c.NoiseSeed("river/wiggle")
	y1, y2 := (float64(r.IntN(1000))+0.37)*cell1, (float64(r.IntN(1000))+0.61)*cell2
	ease := math.Max(1, meanderEnds*L)
	big, small := make([]float64, len(base)), make([]float64, len(base))
	nx, ny := make([]float64, len(base)), make([]float64, len(base))
	for k := range base {
		lo, hi := base[max(0, k-1)], base[min(len(base)-1, k+1)]
		tx, ty := hi.X-lo.X, hi.Y-lo.Y
		if tl := math.Hypot(tx, ty); tl > 0 {
			nx[k], ny[k] = -ty/tl, tx/tl
		}
		s := float64(k)
		env := mapgen.Smoothstep(0, ease, s) * mapgen.Smoothstep(0, ease, L-s)
		big[k] = env * amp * (2*noise2.Gradient(s1, s, y1, cell1) - 1)
		small[k] = env * amp / 2 * (2*noise2.Gradient(s2, s, y2, cell2) - 1)
	}
	bent := tameMeanders(c, base, nx, ny, big, small, riverWidth[rp.Size]*math.Sqrt(c.Scale())*(1+widthVary)/2)
	line := resample(bent, 1)

	// run on past both edges
	ext := int(math.Ceil(riverWidth[rp.Size]*math.Sqrt(c.Scale())*(1+widthVary)/2+canyonBank+plainMaxCells)) + 3
	full := make([]mapgen.PointF, 0, len(line)+2*ext)
	start, end := line[0], line[len(line)-1]
	for k := ext; k >= 1; k-- {
		full = append(full, mapgen.PointF{X: start.X - na.X*float64(k), Y: start.Y - na.Y*float64(k)})
	}
	full = append(full, line...)
	for k := 1; k <= ext; k++ {
		full = append(full, mapgen.PointF{X: end.X - nb.X*float64(k), Y: end.Y - nb.Y*float64(k)})
	}
	return full
}

// tameMeanders displaces base by big+small along its normals, then damps
// the displacement locally (smoothly; the small wiggles first, the big
// meander only once they are nearly gone) wherever the result bends tighter than
// minRadius, comes back within minSep of a non-adjacent part of itself, or
// pushes the river toward the map border inside its outer bands.
func tameMeanders(c *mapgen.Ctx, base []mapgen.PointF, nx, ny, big, small []float64, hw float64) []mapgen.PointF {
	n := len(base)
	minRadius := hw + canyonBank + 2
	minSep := 4*hw + 2
	margin := hw + canyonBank + plainMaxCells + 1
	border := func(p mapgen.PointF) float64 {
		return math.Min(math.Min(p.X, float64(c.W)-p.X), math.Min(p.Y, float64(c.H)-p.Y))
	}
	fb, fs := make([]float64, n), make([]float64, n)
	for k := range fb {
		fb[k], fs[k] = 1, 1
	}
	bent := make([]mapgen.PointF, n)
	arc := make([]float64, n)
	theta := make([]float64, n)
	bad := make([]bool, n)
	for range meanderFixes {
		for k, p := range base {
			d := big[k]*fb[k] + small[k]*fs[k]
			bent[k] = clampMap(c, mapgen.PointF{X: p.X + nx[k]*d, Y: p.Y + ny[k]*d})
		}
		for k := range bent {
			if k > 0 {
				arc[k] = arc[k-1] + math.Hypot(bent[k].X-bent[k-1].X, bent[k].Y-bent[k-1].Y)
			}
			lo, hi := bent[max(0, k-1)], bent[min(n-1, k+1)]
			theta[k] = math.Atan2(hi.Y-lo.Y, hi.X-lo.X)
		}
		found := false
		for k := range bent {
			bad[k] = border(bent[k]) < math.Min(margin, border(base[k]))-1e-9
			lo, hi := max(0, k-curveHalfSpan), min(n-1, k+curveHalfSpan)
			if ds := arc[hi] - arc[lo]; ds > 0 && math.Abs(math.Remainder(theta[hi]-theta[lo], 2*math.Pi))*minRadius > ds {
				bad[k] = true
			}
			found = found || bad[k]
		}
		// non-adjacent parts too close (every second point is enough)
		for i := 0; i < n; i += 2 {
			for j := i + 2; j < n; j += 2 {
				if arc[j]-arc[i] > 2*minSep && math.Hypot(bent[i].X-bent[j].X, bent[i].Y-bent[j].Y) < minSep {
					bad[i], bad[j], found = true, true, true
				}
			}
		}
		if !found {
			break
		}
		for k := range fb {
			switch {
			case !bad[k]:
			case fs[k] > meanderSmallFloor:
				fs[k] *= meanderDamp
			default:
				fb[k] *= meanderDamp
			}
		}
		fb, fs = smoothMin(fb, meanderDampSpan), smoothMin(fs, meanderDampSpan)
	}
	return bent
}

// smoothMin is a moving minimum over ±h followed by a moving average over
// ±h: never above v anywhere, and smooth.
func smoothMin(v []float64, h int) []float64 {
	n := len(v)
	m := make([]float64, n)
	for k := range v {
		m[k] = v[k]
		for j := max(0, k-h); j <= min(n-1, k+h); j++ {
			m[k] = math.Min(m[k], v[j])
		}
	}
	out := make([]float64, n)
	for k := range m {
		s, cnt := 0.0, 0
		for j := max(0, k-h); j <= min(n-1, k+h); j++ {
			s += m[j]
			cnt++
		}
		out[k] = s / float64(cnt)
	}
	return out
}

// riverProfile is the river's shape sampled at every centreline point.
type riverProfile struct {
	arc, half, bank, plain, curve []float64
	tx, ty                        []float64 // unit tangent
}

func profile(c *mapgen.Ctx, line []mapgen.PointF, size site.RiverSize) riverProfile {
	n := len(line)
	p := riverProfile{arc: make([]float64, n), half: make([]float64, n), bank: make([]float64, n),
		plain: make([]float64, n), curve: make([]float64, n), tx: make([]float64, n), ty: make([]float64, n)}
	for k := 1; k < n; k++ {
		p.arc[k] = p.arc[k-1] + math.Hypot(line[k].X-line[k-1].X, line[k].Y-line[k-1].Y)
	}
	theta := make([]float64, n)
	for k := range line {
		lo, hi := line[max(0, k-1)], line[min(n-1, k+1)]
		tx, ty := hi.X-lo.X, hi.Y-lo.Y
		if l := math.Hypot(tx, ty); l > 0 {
			p.tx[k], p.ty[k] = tx/l, ty/l
		} else if k > 0 {
			p.tx[k], p.ty[k] = p.tx[k-1], p.ty[k-1]
		} else {
			p.tx[k] = 1
		}
		theta[k] = math.Atan2(p.ty[k], p.tx[k])
	}
	wide := riverWidth[size] * math.Sqrt(c.Scale()) / 2
	sw, sb, sp := c.NoiseSeed("river/width"), c.NoiseSeed("river/bank"), c.NoiseSeed("river/plain")
	for k := range line {
		lo, hi := max(0, k-curveHalfSpan), min(n-1, k+curveHalfSpan)
		if ds := p.arc[hi] - p.arc[lo]; ds > 0 {
			p.curve[k] = math.Remainder(theta[hi]-theta[lo], 2*math.Pi) / ds
		}
		s := p.arc[k]
		v := math.Max(-1, math.Min(1, 2.5*(2*noise2.Gradient(sw, s, 0.37*widthCell, widthCell)-1)))
		p.half[k] = math.Max(halfWidthFloor, wide*(1+widthVary*v))
		p.bank[k] = bankMin + (bankMax-bankMin)*spread(noise2.Gradient(sb, s, 0.41*bankCell, bankCell))
		p.plain[k] = plainMin + plainSpan*spread(noise2.Gradient(sp, s, 0.29*plainCell, plainCell))
	}
	return p
}

// segDist is the distance from (px, py) to segment a-b and the share along
// it of the nearest point.
func segDist(px, py float64, a, b mapgen.PointF) (float64, float64) {
	dx, dy := b.X-a.X, b.Y-a.Y
	l2 := dx*dx + dy*dy
	t := 0.0
	if l2 > 0 {
		t = mapgen.Clamp01(((px-a.X)*dx + (py-a.Y)*dy) / l2)
	}
	return math.Hypot(px-a.X-t*dx, py-a.Y-t*dy), t
}

// layRiver carves the river along a dense centreline (points about one
// cell apart; parts off the map only shape the ends).
func layRiver(c *mapgen.Ctx, r *randx.Rand, line []mapgen.PointF, size site.RiverSize) {
	n := len(line)
	if n < 2 {
		return
	}
	inMap := func(p mapgen.PointF) bool {
		return p.X >= 0 && p.Y >= 0 && p.X <= float64(c.W) && p.Y <= float64(c.H)
	}
	k0, k1 := -1, -1
	for k, p := range line {
		if inMap(p) {
			if k0 < 0 {
				k0 = k
			}
			k1 = k
		}
	}
	if k0 < 0 {
		return
	}
	p := profile(c, line, size)
	hwMax := 0.0
	for _, h := range p.half {
		hwMax = math.Max(hwMax, h)
	}
	reach := hwMax + canyonBank + plainMaxCells + 1

	// nearest centreline segment of every cell within reach
	N := c.W * c.H
	best := make([]float32, N)
	for i := range best {
		best[i] = mapgen.Far
	}
	seg := make([]int32, N)
	share := make([]float32, N)
	for k := 0; k < n-1; k++ {
		a, b := line[k], line[k+1]
		x0, x1 := int(math.Floor(math.Min(a.X, b.X)-reach)), int(math.Ceil(math.Max(a.X, b.X)+reach))
		y0, y1 := int(math.Floor(math.Min(a.Y, b.Y)-reach)), int(math.Ceil(math.Max(a.Y, b.Y)+reach))
		for y := max(0, y0); y <= min(c.H-1, y1); y++ {
			for x := max(0, x0); x <= min(c.W-1, x1); x++ {
				d, t := segDist(float64(x)+0.5, float64(y)+0.5, a, b)
				if i := y*c.W + x; float32(d) < best[i] {
					best[i], seg[i], share[i] = float32(d), int32(k), float32(t)
				}
			}
		}
	}

	f := c.F
	core := riverCore[size]
	for i, d32 := range best {
		if float64(d32) > reach || (f.Coast[i] && f.Water[i].Wet()) {
			continue // too far, or the sea
		}
		k, t := int(seg[i]), float64(share[i])
		lerp := func(v []float64) float64 { return v[k] + (v[k+1]-v[k])*t }
		hw, bw, fw, kap := lerp(p.half), lerp(p.bank), lerp(p.plain), lerp(p.curve)
		x, y := c.XY(i)
		a := line[k]
		side := p.tx[k]*(float64(y)+0.5-a.Y) - p.ty[k]*(float64(x)+0.5-a.X)
		bend := mapgen.Clamp01(math.Abs(kap) * bendRadius)
		if side*kap > 0 { // the inside of the bend
			fw += plainBend * bend
		} else {
			fw = math.Max(plainMin, fw-bend)
		}
		d := float64(d32)
		switch {
		case d <= hw:
			w := mapgen.Shallow
			if d <= core*hw {
				w = mapgen.Deep
			}
			f.Water[i], f.River[i], f.Rock[i], f.Coast[i] = w, true, false, false
		case f.Coast[i]: // a beach keeps its sand beside the mouth
		case d <= hw+bw || (f.Rock[i] && d <= hw+math.Max(bw, canyonBank)):
			f.Water[i], f.River[i], f.Rock[i] = mapgen.Bank, true, false
		case d <= hw+bw+fw && !f.Rock[i]:
			f.Water[i], f.River[i] = mapgen.Floodplain, true
		}
	}

	if core > 0 {
		fords(c, r, line, p, k0, k1, hwMax)
	}
	for i, d := range best {
		if float64(d) <= reach {
			f.RiverDist[i] = d
		}
	}
	chamfer(c.W, c.H, f.RiverDist)
	f.RiverLine = append([]mapgen.PointF(nil), line[k0:k1+1]...)
}

// fords turn strips of the deep core into shallow water across the river:
// the first within fordFirstMax of where it enters, then every fordGapMin
// to fordGapMax cells of length, and at least one.
func fords(c *mapgen.Ctx, r *randx.Rand, line []mapgen.PointF, p riverProfile, k0, k1 int, hwMax float64) {
	s0, s1 := p.arc[k0], p.arc[k1]
	var at []float64
	for s := s0 + fordFirstMin + (fordFirstMax-fordFirstMin)*r.Float64(); s < s1; s += fordGapMin + (fordGapMax-fordGapMin)*r.Float64() {
		at = append(at, s)
	}
	if len(at) == 0 {
		at = append(at, (s0+s1)/2)
	}
	for _, s := range at {
		k := min(k1, max(k0, sort.SearchFloat64s(p.arc, s)))
		half := (fordLenMin + (fordLenMax-fordLenMin)*r.Float64()) / 2
		ford(c, line, p, k, half, p.half[k]+2, hwMax+3)
	}
	// check: walk the river and add a wider ford in the middle of every
	// stretch still longer than fordGapMax without a crossing
	reach := hwMax + canyonBank + 3
	for range fordFixes {
		prev, fixed := s0, false
		gap := func(s float64) {
			if s-prev > fordGapMax {
				m := min(k1, max(k0, sort.SearchFloat64s(p.arc, (s+prev)/2)))
				ford(c, line, p, m, fordLenMax/2+1, hwMax+3, hwMax+4)
				fixed = true
			}
			prev = s
		}
		for k := k0; k <= k1; k += 2 {
			if crosses(c, line, p, k, fordCheckHalf, reach) {
				gap(p.arc[k])
			}
		}
		gap(s1)
		if !fixed {
			return
		}
	}
}

// ford turns the river's deep cells in a strip across centreline point k
// (half its length along the river, lat to each side) shallow.
func ford(c *mapgen.Ctx, line []mapgen.PointF, p riverProfile, k int, half, lat, rad float64) {
	f := c.F
	ctr, tx, ty := line[k], p.tx[k], p.ty[k]
	for y := max(0, int(ctr.Y-rad)); y <= min(c.H-1, int(ctr.Y+rad)); y++ {
		for x := max(0, int(ctr.X-rad)); x <= min(c.W-1, int(ctr.X+rad)); x++ {
			i := c.Idx(x, y)
			if !f.River[i] || f.Water[i] != mapgen.Deep {
				continue
			}
			dx, dy := float64(x)+0.5-ctr.X, float64(y)+0.5-ctr.Y
			if math.Abs(dx*tx+dy*ty) <= half && math.Abs(dy*tx-dx*ty) <= lat {
				f.Water[i] = mapgen.Shallow
			}
		}
	}
}

// crosses reports a walk from dry ground on one side of the river to dry
// ground on the other inside the strip across point k (half along the
// river, reach to each side), never through deep water or rock.
func crosses(c *mapgen.Ctx, line []mapgen.PointF, p riverProfile, k int, half, reach float64) bool {
	f := c.F
	ctr, tx, ty := line[k], p.tx[k], p.ty[k]
	x0, x1 := max(0, int(ctr.X-reach)-1), min(c.W-1, int(ctr.X+reach)+1)
	y0, y1 := max(0, int(ctr.Y-reach)-1), min(c.H-1, int(ctr.Y+reach)+1)
	lw, lh := x1-x0+1, y1-y0+1
	if lw <= 0 || lh <= 0 {
		return false
	}
	src, pass := make([]bool, lw*lh), make([]bool, lw*lh)
	var dst []int
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			dx, dy := float64(x)+0.5-ctr.X, float64(y)+0.5-ctr.Y
			b := dy*tx - dx*ty
			i, li := c.Idx(x, y), (y-y0)*lw+(x-x0)
			if math.Abs(dx*tx+dy*ty) > half || math.Abs(b) > reach || f.Rock[i] || f.Water[i] == mapgen.Deep {
				continue
			}
			pass[li] = true
			if !f.Water[i].Wet() {
				if b > 0 {
					src[li] = true
				} else {
					dst = append(dst, li)
				}
			}
		}
	}
	if len(dst) == 0 {
		return false
	}
	d := mapgen.BFSDist(lw, lh, src, pass, false)
	for _, i := range dst {
		if d[i] >= 0 {
			return true
		}
	}
	return false
}
