package generation

// GenerateSite builds the local map of a landing site (NG-1 §4.3): the
// biome's ground under the site's hills, then caves, coast, river, lakes,
// wet ground, a road track, a reachability safety pass and wild plants.
// The zero Site is exactly GenerateBiome (the legacy map).

import (
	"math"

	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/site"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/pkg/noise"
	"rimworld-go/pkg/randx"
)

// Per-pass RNG salts: each pass draws from its own stream, so turning one
// feature on never moves another. The terrain noise and the plants keep the
// legacy stream (mapSalt), which is what makes a featureless site the
// legacy map.
const (
	mapSalt   uint64 = 0x6d61706765
	ridgeSalt uint64 = 0x7269646765
	caveSalt  uint64 = 0x6361766573
	coastSalt uint64 = 0x636f617374
	riverSalt uint64 = 0x7269766572
	lakeSalt  uint64 = 0x6c616b6573
	wetSalt   uint64 = 0x776574
)

// Site generation numbers (NG-1 §4.3; original).
const (
	// WaterGuard is the Chebyshev radius around the centre kept free of water.
	WaterGuard = StartRadius + 2
	// caveGuard keeps cave tunnels away from the start.
	caveGuard = StartRadius + 4
	// riverPush: river control points within this radius are moved out.
	riverPush = StartRadius + 10
	// ridgeGap keeps the mountain ridge band this far from the centre.
	ridgeGap = StartRadius + 8
	// coastDeepFrac of the map size is deep water off a coast edge.
	coastDeepFrac = 0.18
	coastJitter   = 6 // ± tiles of shoreline noise
	coastShallow  = 5 // shallow tiles after the deep band
	// Fords cross a river every fordEvery±fordJitter tiles, fordWidth wide.
	fordEvery  = 45
	fordJitter = 10
	fordWidth  = 3
	// ReachShare is the permille of walkable land the centre must reach.
	ReachShare = 700
	// safetyTries carve 3-wide crossings; later tries carve 5-wide.
	safetyTries    = 8
	safetyMaxTries = 32
)

// hillsParams are a hills level's rock and stone noise thresholds and the
// weight that pushes rock toward the map edges.
type hillsParams struct {
	rock, stone, edge float64
	ridge             bool // a mountain ridge band across the map
}

var hillsTable = [...]hillsParams{
	site.HillsDefault: {0.80, 0.71, 0.28, false},
	site.Flat:         {0.88, 0.80, 0.15, false},
	site.SmallHills:   {0.82, 0.73, 0.28, false},
	site.LargeHills:   {0.75, 0.66, 0.34, false},
	site.Mountainous:  {0.66, 0.58, 0.40, true},
}

func hillsOf(h site.Hills) hillsParams {
	if int(h) >= len(hillsTable) {
		return hillsTable[site.HillsDefault]
	}
	return hillsTable[h]
}

// River widths (tiles) and deep cores per size.
var riverWidths = [...]struct{ width, core float64 }{
	site.Creek:      {3, 0},
	site.River:      {9, 3},
	site.LargeRiver: {15, 7},
}

// GenerateSite builds the map for biome b at landing site s.
func GenerateSite(seed uint64, w, h int, b biome.Def, s site.Site) Result {
	res, _ := generateSite(seed, w, h, b, s)
	return res
}

// siteGen is one site generation in progress.
type siteGen struct {
	seed   uint64
	w, h   int
	cx, cy int
	b      biome.Def
	s      site.Site
	m      terrain.Map
	fresh  []bool // river and lake water: wet ground and silt grow by it
	cave   []bool
	river  riverInfo
	lakes  []lakeInfo
	carved int // safety crossings carved
}

// riverInfo is the generated river's centre line (for tests and callers).
type riverInfo struct {
	path    [][2]float64
	arc     []float64 // arc length at each path point
	fords   []float64 // arc positions of the ford centres
	in, out [2]float64
	half    float64
	core    float64
}

type lakeInfo struct {
	x, y, r int
	ring    terrain.Type
}

func generateSite(seed uint64, w, h int, b biome.Def, s site.Site) (Result, *siteGen) {
	if s.Zero() {
		return GenerateBiome(seed, w, h, b), nil
	}
	r := randx.New(seed ^ mapSalt)
	res := newResult(w, h)
	hp := hillsOf(s.Hills)
	var ridge func(x, y int) float64
	if hp.ridge {
		ridge = ridgeField(seed, w, h)
	}
	baseTerrain(r, &res, b, hp, ridge)
	g := &siteGen{seed: seed, w: w, h: h, cx: w / 2, cy: h / 2, b: b, s: s, m: res.Terrain}
	if w > 0 && h > 0 {
		g.fresh = make([]bool, w*h)
		if s.Caves {
			g.caves()
		}
		g.coast()
		g.riverPass()
		g.lakePass()
		g.wetGround()
		g.road()
		g.safety()
	}
	if g.cave != nil {
		res.Cave = g.cave
	}
	spawnPlants(r, &res, b)
	return res, g
}

func newResult(w, h int) Result {
	cx, cy := w/2, h/2
	return Result{
		Terrain: terrain.NewMap(w, h),
		ClearX0: cx - StartRadius, ClearY0: cy - StartRadius,
		ClearX1: cx + StartRadius, ClearY1: cy + StartRadius,
	}
}

func (res *Result) inClear(x, y int) bool {
	return x >= res.ClearX0 && x <= res.ClearX1 && y >= res.ClearY0 && y <= res.ClearY1
}

// baseTerrain is the rock, stone and soft ground of Generate (legacy
// biome) or GenerateBiome (any other), with hp's thresholds and an
// optional ridge boost on the rock noise. It draws exactly the legacy
// noise from r.
func baseTerrain(r *randx.Rand, res *Result, b biome.Def, hp hillsParams, ridge func(x, y int) float64) {
	m := res.Terrain
	w, h := m.W, m.H
	rock := noise.New(r, w, h, 11)
	rock2 := noise.New(r, w, h, 4)
	fert := noise.New(r, w, h, 7)
	cx, cy := w/2, h/2
	noRock := b.Ground.Ice >= 1000
	var soft []int32
	var fv []float64
	if !b.Legacy {
		soft = make([]int32, 0, w*h)
		fv = make([]float64, w*h)
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx := math.Abs(float64(x-cx)) / float64(w/2+1)
			dy := math.Abs(float64(y-cy)) / float64(h/2+1)
			v := 0.75*rock.At(x, y) + 0.25*rock2.At(x, y) + hp.edge*math.Max(dx, dy)
			if ridge != nil {
				v += ridge(x, y)
			}
			i := y*w + x
			switch {
			case noRock:
			case v > hp.rock && !res.inClear(x, y):
				m.Cells[i] = terrain.Rock
				continue
			case v > hp.stone:
				m.Cells[i] = terrain.Stone
				continue
			}
			f := fert.At(x, y)
			if b.Legacy {
				t := terrain.Soil
				switch {
				case f > 0.66:
					t = terrain.RichSoil
				case f < 0.22:
					t = terrain.Gravel
				}
				m.Cells[i] = t
				continue
			}
			fv[i] = f
			soft = append(soft, int32(i))
		}
	}
	if !b.Legacy {
		rankGround(m, soft, fv, b.Ground)
	}
}

// ridgeField is the mountain's central mass: a ridge band across the map
// at a seeded angle, its near edge ridgeGap tiles from the centre.
func ridgeField(seed uint64, w, h int) func(x, y int) float64 {
	rr := randx.New(seed ^ ridgeSalt)
	theta := float64(rr.IntN(3600)) / 3600 * 2 * math.Pi
	nx, ny := math.Cos(theta), math.Sin(theta)
	hw := math.Max(4, float64(min(w, h))/8)
	off := float64(ridgeGap) + hw
	if rr.IntN(2) == 0 {
		off = -off
	}
	cx, cy := float64(w/2), float64(h/2)
	return func(x, y int) float64 {
		d := (float64(x)-cx)*nx + (float64(y)-cy)*ny
		t := 1 - math.Abs(d-off)/hw
		if t <= 0 {
			return 0
		}
		return 0.35 * t * t * (3 - 2*t)
	}
}

// spawnPlants rolls b's wild plants on every fertile tile outside the
// clear start (water, track and rock have no fertility).
func spawnPlants(r *randx.Rand, res *Result, b biome.Def) {
	m := res.Terrain
	for y := 0; y < m.H; y++ {
		for x := 0; x < m.W; x++ {
			if res.inClear(x, y) || m.Cells[y*m.W+x].Fertility() == 0 {
				continue
			}
			roll, acc := r.IntN(1000), 0
			for _, pd := range b.Plants {
				if acc += pd.Permil; roll < acc {
					res.Plants = append(res.Plants, PlantSpawn{X: x, Y: y, Species: pd.Species,
						Growth: startGrowth(r, pd.Species)})
					break
				}
			}
		}
	}
}

// ---- helpers ----

func (g *siteGen) in(x, y int) bool { return x >= 0 && y >= 0 && x < g.w && y < g.h }

// startDist is the Chebyshev distance from the map centre.
func (g *siteGen) startDist(x, y int) int {
	return max(abs(x-g.cx), abs(y-g.cy))
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// setWater writes water t at i, or fallback inside the start guard.
func (g *siteGen) setWater(i int, t, fallback terrain.Type) bool {
	if g.startDist(i%g.w, i/g.w) <= WaterGuard {
		g.m.Cells[i] = fallback
		return false
	}
	g.m.Cells[i] = t
	return true
}

// soft reports open natural ground that wet-ground passes may change.
func soft(t terrain.Type) bool {
	return t == terrain.Soil || t == terrain.RichSoil || t == terrain.Gravel || t == terrain.Sand
}

// uniform is a draw in [-1, 1].
func uniform(r *randx.Rand) float64 { return float64(r.IntN(2001)-1000) / 1000 }

// ---- caves ----

// caves carves 3–6 worm tunnels through rock (rock becomes rough stone
// ground under a natural roof); nothing is carved near the start.
func (g *siteGen) caves() {
	r := randx.New(g.seed ^ caveSalt)
	n := 3 + r.IntN(4)
	for range n {
		sx, sy, ok := 0, 0, false
		for range 200 {
			x, y := r.IntN(g.w), r.IntN(g.h)
			if g.m.Cells[y*g.w+x] == terrain.Rock && g.startDist(x, y) > caveGuard {
				sx, sy, ok = x, y, true
				break
			}
		}
		if !ok {
			continue
		}
		length := 40 + r.IntN(81)
		rad := 1 + r.IntN(2)
		ang := float64(r.IntN(3600)) / 3600 * 2 * math.Pi
		turn := 0.0
		px, py := float64(sx), float64(sy)
		for step := 0; step < length; step++ {
			g.carveCave(int(math.Round(px)), int(math.Round(py)), rad)
			turn = 0.7*turn + 0.3*uniform(r)*0.6
			ang += turn
			px += math.Cos(ang)
			py += math.Sin(ang)
			if px < 0 || py < 0 || px > float64(g.w-1) || py > float64(g.h-1) {
				break
			}
		}
	}
}

func (g *siteGen) carveCave(x, y, rad int) {
	for dy := -rad; dy <= rad; dy++ {
		for dx := -rad; dx <= rad; dx++ {
			nx, ny := x+dx, y+dy
			if dx*dx+dy*dy > rad*rad+rad || !g.in(nx, ny) || g.startDist(nx, ny) <= caveGuard {
				continue
			}
			if i := ny*g.w + nx; g.m.Cells[i] == terrain.Rock {
				g.m.Cells[i] = terrain.Stone
				if g.cave == nil {
					g.cave = make([]bool, g.w*g.h)
				}
				g.cave[i] = true
			}
		}
	}
}

// ---- coast ----

// coastFrame is an edge's distance and along-edge coordinate per cell, and
// the map size across it.
func (g *siteGen) coastFrame(d site.Dir8, x, y int) (dist float64, along int, size float64) {
	w, h := g.w, g.h
	fx, fy := float64(x), float64(y)
	ex, ey := float64(w-1)-fx, float64(h-1)-fy // distances to E and S
	sz := float64(min(w, h))
	switch d {
	case 0:
		return fy, x, float64(h)
	case 2:
		return ex, y, float64(w)
	case 4:
		return ey, x, float64(h)
	case 6:
		return fx, y, float64(w)
	case 1: // NE
		return (fy + ex) / 2, x - y + h, sz
	case 3: // SE
		return (ey + ex) / 2, x + y, sz
	case 5: // SW
		return (ey + fx) / 2, x - y + h, sz
	default: // NW
		return (fx + fy) / 2, x + y, sz
	}
}

// coast floods each coast edge: deep water out to 0.18 of the map size
// (± noise along the shore), coastShallow tiles of shallow water, then a
// 3–5 tile sand beach. Water replaces rock.
func (g *siteGen) coast() {
	if g.s.Coast == 0 {
		return
	}
	r := randx.New(g.seed ^ coastSalt)
	n := g.w + g.h + 1
	class := make([]uint8, g.w*g.h) // 0 land, 1 beach, 2 shallow, 3 deep
	for d := site.Dir8(0); d < 8; d++ {
		if !g.s.Coast.Has(d) {
			continue
		}
		shore := noise.New(r, n, 1, 24)
		beach := noise.New(r, n, 1, 16)
		for y := 0; y < g.h; y++ {
			for x := 0; x < g.w; x++ {
				dist, along, size := g.coastFrame(d, x, y)
				deep := coastDeepFrac*size + (shore.At(along, 0)-0.5)*2*coastJitter
				bw := 3 + min(2, int(beach.At(along, 0)*3))
				c := uint8(0)
				switch {
				case dist < deep:
					c = 3
				case dist < deep+coastShallow:
					c = 2
				case dist < deep+coastShallow+float64(bw):
					c = 1
				}
				if i := y*g.w + x; c > class[i] {
					class[i] = c
				}
			}
		}
	}
	for i, c := range class {
		switch c {
		case 3:
			g.setWater(i, terrain.DeepWater, terrain.Sand)
		case 2:
			g.setWater(i, terrain.ShallowWater, terrain.Sand)
		case 1:
			g.m.Cells[i] = terrain.Sand
		}
	}
}

// ---- river ----

// edgePoint is the point at parameter p (0..1) along edge d; a corner
// direction runs along the last quarter of one edge and the first of the
// next, through the corner.
func (g *siteGen) edgePoint(d site.Dir8, p float64) [2]float64 {
	W, H := float64(g.w-1), float64(g.h-1)
	switch d {
	case 0:
		return [2]float64{p * W, 0}
	case 2:
		return [2]float64{W, p * H}
	case 4:
		return [2]float64{p * W, H}
	case 6:
		return [2]float64{0, p * H}
	}
	e := 0.25 * float64(min(g.w, g.h))
	var k, u1, u2 [2]float64
	switch d {
	case 1:
		k, u1, u2 = [2]float64{W, 0}, [2]float64{-1, 0}, [2]float64{0, 1}
	case 3:
		k, u1, u2 = [2]float64{W, H}, [2]float64{0, -1}, [2]float64{-1, 0}
	case 5:
		k, u1, u2 = [2]float64{0, H}, [2]float64{1, 0}, [2]float64{0, -1}
	default:
		k, u1, u2 = [2]float64{0, 0}, [2]float64{0, 1}, [2]float64{1, 0}
	}
	q := (p - 0.5) * 2
	if q < 0 {
		return [2]float64{k[0] - q*e*u1[0], k[1] - q*e*u1[1]}
	}
	return [2]float64{k[0] + q*e*u2[0], k[1] + q*e*u2[1]}
}

// inward is the unit vector from edge d into the map.
func inward(d site.Dir8) [2]float64 {
	v := [8][2]float64{{0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}, {1, 0}, {1, 1}}[d&7]
	l := math.Hypot(v[0], v[1])
	return [2]float64{v[0] / l, v[1] / l}
}

func (g *siteGen) clampPt(p [2]float64) [2]float64 {
	return [2]float64{math.Max(0, math.Min(float64(g.w-1), p[0])), math.Max(0, math.Min(float64(g.h-1), p[1]))}
}

func pathLen(pts [][2]float64) float64 {
	l := 0.0
	for i := 1; i < len(pts); i++ {
		l += math.Hypot(pts[i][0]-pts[i-1][0], pts[i][1]-pts[i-1][1])
	}
	return l
}

// riverPath is the river's dense centre line from its In edge to its Out
// edge: a midpoint-displaced polyline that bends around the start.
func (g *siteGen) riverPath(r *randx.Rand, half float64) [][2]float64 {
	in, out := g.s.River.In&7, g.s.River.Out&7
	frac := func(lo, hi float64) float64 { return lo + (hi-lo)*float64(r.IntN(1001))/1000 }
	var ctl [][2]float64
	if in == out {
		a := g.edgePoint(in, frac(0.10, 0.35))
		b := g.edgePoint(in, frac(0.65, 0.90))
		size := float64(min(g.w, g.h))
		if in == 0 || in == 4 {
			size = float64(g.h)
		} else if in == 2 || in == 6 {
			size = float64(g.w)
		}
		n := inward(in)
		mid := g.clampPt([2]float64{(a[0]+b[0])/2 + n[0]*0.4*size, (a[1]+b[1])/2 + n[1]*0.4*size})
		ctl = [][2]float64{a, mid, b}
	} else {
		ctl = [][2]float64{g.edgePoint(in, frac(0.25, 0.75)), g.edgePoint(out, frac(0.25, 0.75))}
	}
	amp := 0.12 * pathLen(ctl)
	for range 3 {
		next := [][2]float64{ctl[0]}
		for i := 1; i < len(ctl); i++ {
			p, q := ctl[i-1], ctl[i]
			dx, dy := q[0]-p[0], q[1]-p[1]
			l := math.Hypot(dx, dy)
			mid := [2]float64{(p[0] + q[0]) / 2, (p[1] + q[1]) / 2}
			if l > 0 {
				u := uniform(r) * amp
				mid[0] += -dy / l * u
				mid[1] += dx / l * u
			}
			next = append(next, g.clampPt(mid), q)
		}
		ctl = next
		amp /= 2
	}
	// control points too near the start move outward
	c := [2]float64{float64(g.cx), float64(g.cy)}
	for i := 1; i < len(ctl)-1; i++ {
		ctl[i] = g.clampPt(pushOut(ctl[i], c, riverPush))
	}
	dense := densify(ctl)
	return g.detour(dense, c, math.Max(riverPush, float64(StartRadius)+5+half))
}

// pushOut moves p radially to distance rad from c when nearer.
func pushOut(p, c [2]float64, rad float64) [2]float64 {
	dx, dy := p[0]-c[0], p[1]-c[1]
	d := math.Hypot(dx, dy)
	if d >= rad {
		return p
	}
	if d == 0 {
		dx, dy, d = 1, 0, 1
	}
	return [2]float64{c[0] + dx/d*rad, c[1] + dy/d*rad}
}

// densify resamples a polyline at steps of at most one tile.
func densify(ctl [][2]float64) [][2]float64 {
	out := [][2]float64{ctl[0]}
	for i := 1; i < len(ctl); i++ {
		p, q := ctl[i-1], ctl[i]
		n := int(math.Ceil(math.Hypot(q[0]-p[0], q[1]-p[1])))
		for k := 1; k <= n; k++ {
			t := float64(k) / float64(n)
			out = append(out, [2]float64{p[0] + (q[0]-p[0])*t, p[1] + (q[1]-p[1])*t})
		}
	}
	return out
}

// detour replaces every run of the path inside radius rad of c with an arc
// around the circle, the short way from where it entered to where it left.
func (g *siteGen) detour(path [][2]float64, c [2]float64, rad float64) [][2]float64 {
	inside := func(p [2]float64) bool { return math.Hypot(p[0]-c[0], p[1]-c[1]) < rad }
	out := make([][2]float64, 0, len(path))
	for i := 0; i < len(path); {
		if !inside(path[i]) {
			out = append(out, path[i])
			i++
			continue
		}
		j := i
		for j < len(path) && inside(path[j]) {
			j++
		}
		if i == 0 || j == len(path) {
			for k := i; k < j; k++ { // the run touches an end: push it out
				out = append(out, g.clampPt(pushOut(path[k], c, rad)))
			}
			i = j
			continue
		}
		a0 := math.Atan2(path[i-1][1]-c[1], path[i-1][0]-c[0])
		a1 := math.Atan2(path[j][1]-c[1], path[j][0]-c[0])
		da := math.Remainder(a1-a0, 2*math.Pi)
		n := max(1, int(math.Ceil(math.Abs(da)*rad)))
		for k := 0; k <= n; k++ {
			a := a0 + da*float64(k)/float64(n)
			out = append(out, g.clampPt([2]float64{c[0] + rad*math.Cos(a), c[1] + rad*math.Sin(a)}))
		}
		i = j
	}
	return densify(out)
}

// segDist is the distance from (px, py) to segment a-b and the fraction
// along it of the nearest point.
func segDist(px, py float64, a, b [2]float64) (float64, float64) {
	dx, dy := b[0]-a[0], b[1]-a[1]
	l2 := dx*dx + dy*dy
	t := 0.0
	if l2 > 0 {
		t = math.Max(0, math.Min(1, ((px-a[0])*dx+(py-a[1])*dy)/l2))
	}
	return math.Hypot(px-a[0]-t*dx, py-a[1]-t*dy), t
}

// riverPass lays the river: shallow water out to half its width, a deep
// core, 3-wide shallow fords every 45±10 tiles and 1–2 tiles of mud bank.
// It never overwrites coast water.
func (g *siteGen) riverPass() {
	size := g.s.River.Size
	if size == site.NoRiver || int(size) >= len(riverWidths) {
		return
	}
	r := randx.New(g.seed ^ riverSalt)
	half, core := riverWidths[size].width/2, riverWidths[size].core/2
	path := g.riverPath(r, half)
	arc := make([]float64, len(path))
	for i := 1; i < len(path); i++ {
		arc[i] = arc[i-1] + math.Hypot(path[i][0]-path[i-1][0], path[i][1]-path[i-1][1])
	}
	total := arc[len(arc)-1]
	var fords []float64
	for f := float64(fordEvery - fordJitter + r.IntN(2*fordJitter+1)); f < total; f += float64(fordEvery - fordJitter + r.IntN(2*fordJitter+1)) {
		fords = append(fords, f)
	}
	g.river = riverInfo{path: path, arc: arc, fords: fords, in: path[0], out: path[len(path)-1], half: half, core: core}
	bank := noise.New(r, g.w, g.h, 5)
	reach := half + 2
	dist := make([]float32, g.w*g.h)
	at := make([]float32, g.w*g.h)
	for i := range dist {
		dist[i] = math.MaxFloat32
	}
	for s := 1; s < len(path); s++ {
		a, b := path[s-1], path[s]
		x0, x1 := int(math.Floor(math.Min(a[0], b[0])-reach)), int(math.Ceil(math.Max(a[0], b[0])+reach))
		y0, y1 := int(math.Floor(math.Min(a[1], b[1])-reach)), int(math.Ceil(math.Max(a[1], b[1])+reach))
		for y := max(0, y0); y <= min(g.h-1, y1); y++ {
			for x := max(0, x0); x <= min(g.w-1, x1); x++ {
				d, t := segDist(float64(x), float64(y), a, b)
				if i := y*g.w + x; float32(d) < dist[i] {
					dist[i] = float32(d)
					at[i] = float32(arc[s-1] + t*(arc[s]-arc[s-1]))
				}
			}
		}
	}
	inFord := func(s float64) bool {
		for _, f := range fords {
			if math.Abs(s-f) <= fordWidth/2.0 {
				return true
			}
		}
		return false
	}
	for i, d32 := range dist {
		d := float64(d32)
		if d > reach {
			continue
		}
		t := g.m.Cells[i]
		if terrain.IsWater(t) {
			continue // the sea keeps its own water
		}
		if d <= half {
			wt := terrain.ShallowWater
			if d <= core && !inFord(float64(at[i])) {
				wt = terrain.DeepWater
			}
			g.fresh[i] = g.setWater(i, wt, terrain.Mud)
			continue
		}
		bw := 1.0
		if bank.At(i%g.w, i/g.w) > 0.5 {
			bw = 2
		}
		if d <= half+bw && !terrain.IsRockWall(t) {
			g.m.Cells[i] = terrain.Mud
		}
	}
}

// ---- lakes ----

// lakePass places the site's lakes: noisy blobs of radius 6–14 outside
// the start, deep in the middle, ringed by marsh (wet biomes) or mud.
func (g *siteGen) lakePass() {
	if g.s.Lakes == 0 {
		return
	}
	r := randx.New(g.seed ^ lakeSalt)
	blob := noise.New(r, g.w, g.h, 6)
	ring := terrain.Mud
	if g.b.Wetness >= 60 {
		ring = terrain.Marsh
	}
	for range min(int(g.s.Lakes), 3) {
		rad := 6 + r.IntN(9)
		rw := 2 + r.IntN(3)
		reach := int(math.Ceil(1.25*float64(rad))) + rw + 1
		lx, ly, ok := 0, 0, false
		for range 40 {
			x, y := r.IntN(g.w), r.IntN(g.h)
			if g.startDist(x, y) > WaterGuard+reach {
				lx, ly, ok = x, y, true
				break
			}
		}
		if !ok {
			continue
		}
		g.lakes = append(g.lakes, lakeInfo{x: lx, y: ly, r: rad, ring: ring})
		for y := max(0, ly-reach); y <= min(g.h-1, ly+reach); y++ {
			for x := max(0, lx-reach); x <= min(g.w-1, lx+reach); x++ {
				i := y*g.w + x
				t := g.m.Cells[i]
				if terrain.IsWater(t) {
					continue
				}
				re := float64(rad) * (0.75 + 0.5*blob.At(x, y))
				d := math.Hypot(float64(x-lx), float64(y-ly))
				switch {
				case d < re-4:
					g.fresh[i] = g.setWater(i, terrain.DeepWater, ring)
				case d < re:
					g.fresh[i] = g.setWater(i, terrain.ShallowWater, ring)
				case d < re+float64(rw) && t != terrain.Rock:
					g.m.Cells[i] = ring
				}
			}
		}
	}
}

// ---- wet ground ----

// wetGround: soft ground within 4 tiles of fresh water may turn to marsh
// ((5-d)·12 %) in biomes with marsh; otherwise soil within 3 tiles becomes
// rich soil 40 % of the time (river silt).
func (g *siteGen) wetGround() {
	dist := make([]int8, g.w*g.h)
	var queue []int32
	for i, f := range g.fresh {
		if f {
			dist[i] = 0
			queue = append(queue, int32(i))
		} else {
			dist[i] = -1
		}
	}
	if len(queue) == 0 {
		return
	}
	for head := 0; head < len(queue); head++ {
		i := int(queue[head])
		if dist[i] >= 4 {
			continue
		}
		x, y := i%g.w, i/g.w
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				nx, ny := x+dx, y+dy
				if !g.in(nx, ny) {
					continue
				}
				if n := ny*g.w + nx; dist[n] < 0 {
					dist[n] = dist[i] + 1
					queue = append(queue, int32(n))
				}
			}
		}
	}
	r := randx.New(g.seed ^ wetSalt)
	marshy := g.b.Ground.Marsh > 0
	for i, d := range dist {
		if d < 1 || !soft(g.m.Cells[i]) {
			continue
		}
		roll := r.IntN(100)
		switch {
		case marshy && roll < (5-int(d))*12:
			g.m.Cells[i] = terrain.Marsh
		case !marshy && d <= 3 && g.m.Cells[i] == terrain.Soil && roll < 40:
			g.m.Cells[i] = terrain.RichSoil
		}
	}
}

// ---- road ----

// road lays a 2-wide packed dirt track from each road edge toward the
// centre, stopping at StartRadius. It cuts through rock but not water.
func (g *siteGen) road() {
	if g.s.Road == 0 {
		return
	}
	W, H := g.w-1, g.h-1
	starts := [8][2]int{{g.cx, 0}, {W, 0}, {W, g.cy}, {W, H}, {g.cx, H}, {0, H}, {0, g.cy}, {0, 0}}
	for d := site.Dir8(0); d < 8; d++ {
		if !g.s.Road.Has(d) {
			continue
		}
		sx, sy := starts[d][0], starts[d][1]
		dx, dy := float64(g.cx-sx), float64(g.cy-sy)
		n := max(abs(g.cx-sx), abs(g.cy-sy))
		horiz := math.Abs(dx) >= math.Abs(dy)
		for k := 0; k <= n; k++ {
			x := sx + int(math.Round(dx*float64(k)/float64(max(1, n))))
			y := sy + int(math.Round(dy*float64(k)/float64(max(1, n))))
			if g.startDist(x, y) <= StartRadius {
				break
			}
			g.track(x, y)
			if horiz {
				g.track(x, y+1)
			} else {
				g.track(x+1, y)
			}
		}
	}
}

func (g *siteGen) track(x, y int) {
	if !g.in(x, y) {
		return
	}
	if i := y*g.w + x; !terrain.IsWater(g.m.Cells[i]) {
		g.m.Cells[i] = terrain.Track
	}
}

// ---- safety ----

// safety makes the start usable: no water within WaterGuard of the centre,
// and the centre reaches a map edge and at least ReachShare‰ of the
// walkable land. Until it does, it carves a crossing (deep water becomes
// shallow, rock becomes rough stone) along the cheapest way to the largest
// unreached land (or to the edge): 3 wide for safetyTries tries, then 5.
func (g *siteGen) safety() {
	for y := g.cy - WaterGuard; y <= g.cy+WaterGuard; y++ {
		for x := g.cx - WaterGuard; x <= g.cx+WaterGuard; x++ {
			if g.in(x, y) && terrain.IsWater(g.m.Cells[y*g.w+x]) {
				g.m.Cells[y*g.w+x] = terrain.Sand
			}
		}
	}
	for try := 0; try < safetyMaxTries; try++ {
		reached, edge, ok := g.reachable()
		if ok {
			return
		}
		rad := 1
		if try >= safetyTries {
			rad = 2
		}
		if !g.carve(reached, edge, rad) {
			return
		}
		g.carved++
	}
}

// land is walkable ground that is not water.
func land(t terrain.Type) bool { return t.Walkable() && !terrain.IsWater(t) }

// reachable floods (4-connected) from the centre over walkable tiles. ok
// is a reached map edge plus ReachShare‰ of the land.
func (g *siteGen) reachable() (reached []bool, edge, ok bool) {
	reached = make([]bool, g.w*g.h)
	start := g.cy*g.w + g.cx
	if !g.m.Cells[start].Walkable() {
		return reached, false, false
	}
	queue := []int32{int32(start)}
	reached[start] = true
	got := 0
	for head := 0; head < len(queue); head++ {
		i := int(queue[head])
		x, y := i%g.w, i/g.w
		if land(g.m.Cells[i]) {
			got++
		}
		if x == 0 || y == 0 || x == g.w-1 || y == g.h-1 {
			edge = true
		}
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			nx, ny := x+d[0], y+d[1]
			if !g.in(nx, ny) {
				continue
			}
			if n := ny*g.w + nx; !reached[n] && g.m.Cells[n].Walkable() {
				reached[n] = true
				queue = append(queue, int32(n))
			}
		}
	}
	total := 0
	for _, t := range g.m.Cells {
		if land(t) {
			total++
		}
	}
	return reached, edge, edge && got*1000 >= ReachShare*total
}

// carve opens the cheapest crossing (fewest blocked tiles, 0-1 BFS) from
// the reached area to the edge (when no edge is reached) or to the largest
// unreached patch of land, rad tiles to either side of the way.
func (g *siteGen) carve(reached []bool, edge bool, rad int) bool {
	n := g.w * g.h
	target := make([]bool, n)
	if !edge {
		for x := 0; x < g.w; x++ {
			target[x], target[(g.h-1)*g.w+x] = true, true
		}
		for y := 0; y < g.h; y++ {
			target[y*g.w], target[y*g.w+g.w-1] = true, true
		}
	} else {
		comp := make([]int32, n)
		best, bestSize, id := int32(0), 0, int32(0)
		for i := range n {
			if comp[i] != 0 || reached[i] || !land(g.m.Cells[i]) {
				continue
			}
			id++
			size := 0
			stack := []int32{int32(i)}
			comp[i] = id
			for len(stack) > 0 {
				c := int(stack[len(stack)-1])
				stack = stack[:len(stack)-1]
				size++
				x, y := c%g.w, c/g.w
				for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
					nx, ny := x+d[0], y+d[1]
					if !g.in(nx, ny) {
						continue
					}
					if m := ny*g.w + nx; comp[m] == 0 && !reached[m] && g.m.Cells[m].Walkable() {
						comp[m] = id
						stack = append(stack, int32(m))
					}
				}
			}
			if size > bestSize {
				best, bestSize = id, size
			}
		}
		if best == 0 {
			return false
		}
		for i, c := range comp {
			target[i] = c == best
		}
	}
	// 0-1 BFS: stepping onto a blocked tile costs 1
	cost := make([]int32, n)
	parent := make([]int32, n)
	for i := range cost {
		cost[i] = math.MaxInt32
		parent[i] = -1
	}
	dq := make([]int32, 0, n)
	front := make([]int32, 0, n)
	for i, r := range reached {
		if r {
			cost[i] = 0
			front = append(front, int32(i))
		}
	}
	end := int32(-1)
	for len(front) > 0 || len(dq) > 0 {
		var i int32
		if len(front) > 0 {
			i = front[len(front)-1]
			front = front[:len(front)-1]
		} else {
			i = dq[0]
			dq = dq[1:]
		}
		if target[i] && !reached[i] {
			end = i
			break
		}
		x, y := int(i)%g.w, int(i)/g.w
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			nx, ny := x+d[0], y+d[1]
			if !g.in(nx, ny) {
				continue
			}
			m := int32(ny*g.w + nx)
			step := int32(0)
			if !g.m.Cells[m].Walkable() {
				step = 1
			}
			if c := cost[i] + step; c < cost[m] {
				cost[m], parent[m] = c, i
				if step == 0 {
					front = append(front, m)
				} else {
					dq = append(dq, m)
				}
			}
		}
	}
	if end < 0 {
		return false
	}
	for c := end; c >= 0 && !reached[c]; c = parent[c] {
		x, y := int(c)%g.w, int(c)/g.w
		for dy := -rad; dy <= rad; dy++ {
			for dx := -rad; dx <= rad; dx++ {
				if g.in(x+dx, y+dy) {
					g.open((y+dy)*g.w + x + dx)
				}
			}
		}
	}
	return true
}

// open makes a blocked tile crossable: deep water wades, rock is cut back
// to rough stone.
func (g *siteGen) open(i int) {
	switch t := g.m.Cells[i]; {
	case t == terrain.DeepWater:
		g.m.Cells[i] = terrain.ShallowWater
	case terrain.IsRockWall(t):
		g.m.Cells[i] = terrain.Stone
		if g.cave != nil {
			g.cave[i] = false
		}
	}
}
