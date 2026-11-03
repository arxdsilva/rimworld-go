package globe

import (
	"math"

	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/planet"
	"rimworld-go/internal/domain/world/site"
	"rimworld-go/pkg/randx"
)

// Relief (plan §1.2): every hill and mountain stands up off the sphere.
// Heights are computed once per planet in object space; Build only rotates
// and projects them. Water stays flat at radius 1, so coasts meet at sea
// level, and a corner is shared by index, so neighbouring tiles can never
// open a crack between them.

const (
	reliefScale  = 0.022 // radius gained per reliefRefM metres of height; original tuning
	reliefRefM   = 6000  // original tuning
	reliefCapM   = 8250  // the highest planet elevation (6000 m) plus the largest jittered lift (1.25 x 1800 m)
	liftSmall    = 300   // small hills: centre lift, m; original tuning
	liftLarge    = 900   // large hills; original tuning
	liftMountain = 1800  // mountainous; original tuning
	liftJitter   = 0.25  // +-25 % per tile, from a stable tile hash; original tuning
	snowBaseM    = 2600  // snowline at 0 °C mean, m; original tuning
	snowPerC     = 120   // snowline rise per °C of tile mean temperature; original tuning
	snowBandM    = 300   // soft band above the snowline; original tuning
	rockBelowM   = 1100  // bare rock starts this far below the snowline; original tuning
	rockFadeM    = 700   // and reaches rockMix over this height; original tuning
	rockMix      = 0.7   // original tuning
	overlayLift  = 0.0012
	// shadeRelief exaggerates the hill lift for the face normals only: at
	// the drawn height a 1 km slope across a tile leans a few degrees, too
	// little for the light to show; the geometry keeps the true scale.
	// Original tuning.
	shadeRelief = 3
	// cornerLiftShare is how much of the sharing tiles' hill lift a corner
	// takes; original tuning.
	cornerLiftShare = 0.8
)

// ReliefMax is the largest radius a relief vertex reaches (the sphere is 1).
// The halo bake and the silhouette test use it.
const ReliefMax = 1 + reliefScale*reliefCapM/reliefRefM

// Faceted lighting (plan §1.2): ambient plus Lambert on the face normal.
const (
	lightAmbient = 0.38
	lightDiffuse = 0.62
)

// reliefR is the radius of a point h metres above sea level.
func reliefR(h float32) float32 {
	return 1 + reliefScale*clampF32(h, 0, reliefCapM)/reliefRefM
}

// planetGeom is the once-per-planet geometry: deduplicated corners and
// their relief, the face normals, the river, road, coast and grid lines,
// and the per-vertex colours of the current mode.
type planetGeom struct {
	cornerID    []uint32     // per corner copy: its unique corner
	cornerPos   [][3]float32 // unique corner on the unit sphere
	cornerTiles [][3]int32   // the tiles sharing a unique corner (-1: none)
	cornerH     []float32    // unique corner height, m
	cornerR     []float32    // its radius
	cornerDisp  [][3]float32 // unique corner on the relief
	centreH     []float32    // tile centre peak height, m
	snowAt      []float32    // tile snowline, m
	centreDisp  [][3]float32 // tile centre on the relief
	baseDisp    [][3]float32 // tile centre at its land height without the hill lift (rivers, roads)
	faceN       [][3]float32 // per corner copy k of tile i: normal of the triangle centre, k, k+1
	ccol        []rgb        // centre vertex colour, current mode
	kcol        []rgb        // corner vertex colour per corner copy, current mode
	Rivers      []globeSeg
	Roads       []globeSeg
	Coast       []globeSeg   // land/ocean and land/lake edges
	edges       [][2]uint32  // every tile edge once, as unique corners (the grid)
	feats       []featLabel  // named features, largest first
	labels      []Label      // this frame's placed names (at most maxLabels)
	markWH      [][2]float32 // settlement name sizes, logical
	measuredAt  float64      // the UI scale the label sizes were measured at
	grid        bool         // draw the tile grid when zoomed in
}

func isLand(p *planet.Planet, i int) bool {
	return p.Water[i] == planet.Land && p.Biome[i] != biome.SeaIce
}

// hillLift is tile i's centre lift in metres, jittered per tile.
func hillLift(p *planet.Planet, i int) float32 {
	var l float32
	switch p.Hill[i] {
	case site.SmallHills:
		l = liftSmall
	case site.LargeHills:
		l = liftLarge
	case site.Mountainous:
		l = liftMountain
	default:
		return 0
	}
	j := float32(hashUnit(randx.Mix64(uint64(i)*0x9E3779B97F4A7C15 + 0x5EED)))
	return l * (1 - liftJitter + 2*liftJitter*j)
}

func landH(p *planet.Planet, i int) float32 {
	if p.Water[i] == planet.Ocean {
		return 0
	}
	return float32(max(0, int(p.Elev[i])))
}

func scale3(v [3]float32, k float32) [3]float32 { return [3]float32{v[0] * k, v[1] * k, v[2] * k} }

// newGeom builds the planet's relief and lines.
func newGeom(p *planet.Planet) *planetGeom {
	n := p.TileCount()
	geo := &planetGeom{}
	geo.cornerID, geo.cornerPos = DedupeCorners(p.Corners)
	nc := len(geo.cornerPos)
	geo.cornerTiles = make([][3]int32, nc)
	for c := range geo.cornerTiles {
		geo.cornerTiles[c] = [3]int32{-1, -1, -1}
	}
	for i := 0; i < n; i++ {
		for o := p.CornerOff[i]; o < p.CornerOff[i+1]; o++ {
			ct := &geo.cornerTiles[geo.cornerID[o]]
			for s := range ct {
				if ct[s] < 0 {
					ct[s] = int32(i)
					break
				}
			}
		}
	}
	// Corner heights: the mean land elevation of the sharing tiles, plus a
	// share of the mean of their two largest hill lifts. A corner between two
	// mountains then sits near their peaks, so neighbouring mountains join
	// into a ridge instead of standing as separate pyramids, while a corner
	// at the foot of a lone peak stays low. A corner touching the ocean sits
	// at sea level.
	geo.cornerH = make([]float32, nc)
	cornerLift := make([]float32, nc)
	geo.cornerR = make([]float32, nc)
	geo.cornerDisp = make([][3]float32, nc)
	for c := 0; c < nc; c++ {
		var sum, l1, l2 float32 // l1 >= l2: the two largest lifts
		k, sea := 0, false
		for _, t := range geo.cornerTiles[c] {
			if t < 0 {
				continue
			}
			if p.Water[t] == planet.Ocean {
				sea = true
			}
			sum += landH(p, int(t))
			if isLand(p, int(t)) {
				if l := hillLift(p, int(t)); l > l1 {
					l1, l2 = l, l1
				} else if l > l2 {
					l2 = l
				}
			}
			k++
		}
		if !sea && k > 0 {
			cornerLift[c] = cornerLiftShare * (l1 + l2) / 2
			geo.cornerH[c] = sum/float32(k) + cornerLift[c]
		}
		geo.cornerR[c] = reliefR(geo.cornerH[c])
		geo.cornerDisp[c] = scale3(geo.cornerPos[c], geo.cornerR[c])
	}
	geo.centreH = make([]float32, n)
	geo.centreDisp = make([][3]float32, n)
	geo.baseDisp = make([][3]float32, n)
	for i := 0; i < n; i++ {
		h := landH(p, i)
		geo.baseDisp[i] = scale3(p.Center[i], reliefR(h))
		if isLand(p, i) {
			h += hillLift(p, i)
		}
		geo.centreH[i] = h
		geo.centreDisp[i] = scale3(p.Center[i], reliefR(h))
	}
	geo.snowAt = make([]float32, n)
	for i := 0; i < n; i++ {
		geo.snowAt[i] = snowline(p, i)
	}
	geo.faceN = make([][3]float32, len(p.Corners))
	geo.faceNormals(p, cornerLift)
	geo.ccol = make([]rgb, n)
	geo.kcol = make([]rgb, len(p.Corners))
	geo.buildLines(p)
	geo.buildLabels(p)
	return geo
}

// facetWeight is how much of a face's own lean its light shows, by
// hilliness; the rest is the tile's lean (the plane of its corners, which
// carries the slope of the land around it). Mountains are fully faceted,
// so peaks and ridges read; low land keeps the broad slope without the
// per-tile ripple of its height noise. Original tuning.
var facetWeight = [...]float32{site.HillsDefault: 0.2, site.Flat: 0.2, site.SmallHills: 0.2, site.LargeHills: 0.35, site.Mountainous: 1}

// faceNormals fills the shading normal of every fan face, on a relief
// whose hill lift (not the land elevation) is exaggerated by shadeRelief:
// hills and mountains read, flat land shades as the plain sphere did.
func (geo *planetGeom) faceNormals(p *planet.Planet, cornerLift []float32) {
	// The exaggeration grows with the lift, so mountains stand out and
	// rolling hills stay calm.
	shadeAt := func(unit [3]float32, h, lift float32) [3]float32 {
		extra := (shadeRelief - 1) * lift * min(lift/liftMountain, 1)
		return scale3(unit, 1+reliefScale*(h+extra)/reliefRefM)
	}
	var ring [6][3]float32
	for i := 0; i < p.TileCount(); i++ {
		o0, o1 := p.CornerOff[i], p.CornerOff[i+1]
		m := o1 - o0
		for k := uint32(0); k < m; k++ {
			a := geo.cornerID[o0+k]
			ring[k] = shadeAt(geo.cornerPos[a], geo.cornerH[a], cornerLift[a])
		}
		tile := newell(ring[:m], p.Center[i])
		w := float32(0.3)
		if isLand(p, i) && int(p.Hill[i]) < len(facetWeight) {
			w = facetWeight[p.Hill[i]]
		}
		var lift float32
		if isLand(p, i) {
			lift = hillLift(p, i)
		}
		c := shadeAt(p.Center[i], geo.centreH[i], lift)
		for k := uint32(0); k < m; k++ {
			f := faceNormal(c, ring[k], ring[(k+1)%m])
			geo.faceN[o0+k] = norm3([3]float32{tile[0] + w*(f[0]-tile[0]), tile[1] + w*(f[1]-tile[1]), tile[2] + w*(f[2]-tile[2])})
		}
	}
}

// newell is the outward unit normal of a polygon (Newell's method).
func newell(ring [][3]float32, out [3]float32) [3]float32 {
	var nx, ny, nz float64
	for k := range ring {
		a, b := ring[k], ring[(k+1)%len(ring)]
		nx += float64((a[1] - b[1]) * (a[2] + b[2]))
		ny += float64((a[2] - b[2]) * (a[0] + b[0]))
		nz += float64((a[0] - b[0]) * (a[1] + b[1]))
	}
	l := math.Sqrt(nx*nx + ny*ny + nz*nz)
	if l == 0 {
		return norm3(out)
	}
	if nx*float64(out[0])+ny*float64(out[1])+nz*float64(out[2]) < 0 {
		l = -l
	}
	return [3]float32{float32(nx / l), float32(ny / l), float32(nz / l)}
}

// faceNormal is the outward unit normal of triangle c, a, b (in float64,
// the edges are short).
func faceNormal(c, a, b [3]float32) [3]float32 {
	ux, uy, uz := float64(a[0]-c[0]), float64(a[1]-c[1]), float64(a[2]-c[2])
	vx, vy, vz := float64(b[0]-c[0]), float64(b[1]-c[1]), float64(b[2]-c[2])
	nx, ny, nz := uy*vz-uz*vy, uz*vx-ux*vz, ux*vy-uy*vx
	l := math.Sqrt(nx*nx + ny*ny + nz*nz)
	if l == 0 {
		return norm3(c)
	}
	if nx*float64(c[0])+ny*float64(c[1])+nz*float64(c[2]) < 0 {
		l = -l
	}
	return [3]float32{float32(nx / l), float32(ny / l), float32(nz / l)}
}

// across is the tile on the other side of tile i's edge from unique corner
// a to unique corner b, -1 when none.
func (geo *planetGeom) across(i int32, a, b uint32) int32 {
	for _, t := range geo.cornerTiles[a] {
		if t < 0 || t == i {
			continue
		}
		for _, u := range geo.cornerTiles[b] {
			if u == t {
				return t
			}
		}
	}
	return -1
}

// buildLines turns rivers, roads, coasts and tile edges into surface lines.
func (geo *planetGeom) buildLines(p *planet.Planet) {
	lift := float32(1 + overlayLift)
	n := p.TileCount()
	for i := 0; i < n; i++ {
		o0, o1 := p.CornerOff[i], p.CornerOff[i+1]
		m := o1 - o0
		for k := uint32(0); k < m; k++ {
			a, b := geo.cornerID[o0+k], geo.cornerID[o0+(k+1)%m]
			j := geo.across(int32(i), a, b)
			if j < 0 || int32(i) < j {
				geo.edges = append(geo.edges, [2]uint32{a, b})
			}
			if j >= 0 && isLand(p, i) && p.Water[j] != planet.Land {
				geo.Coast = append(geo.Coast, globeSeg{scale3(geo.cornerDisp[a], lift), scale3(geo.cornerDisp[b], lift), 1, 0, 0})
			}
		}
	}
	// Rivers run centre, edge midpoint, centre; the midpoint takes the lower
	// of the two shared corners, so a river lies in the valley between tiles.
	// A river that reaches the sea or a lake ends at the shore (the edge
	// midpoint, on the coast stroke), and a source tile tapers its first
	// half from a thin start.
	fed := make([]bool, n)
	for i := range p.River {
		if p.River[i] != site.NoRiver && p.RiverTo[i] >= 0 {
			fed[p.RiverTo[i]] = true
		}
	}
	for i := range p.River {
		to := p.RiverTo[i]
		if p.River[i] == site.NoRiver || to < 0 || p.Water[i] != planet.Land {
			continue
		}
		c0, c1 := p.Center[i], p.Center[to]
		dir := [3]float32{c0[0] + c1[0], c0[1] + c1[1], c0[2] + c1[2]}
		mr := min(reliefR(landH(p, i)), reliefR(landH(p, int(to))))
		if ca, cb, ok := geo.sharedEdge(p, i, to); ok {
			mr = min(geo.cornerR[ca], geo.cornerR[cb])
			pa, pb := geo.cornerPos[ca], geo.cornerPos[cb]
			dir = [3]float32{pa[0] + pb[0], pa[1] + pb[1], pa[2] + pb[2]}
		}
		mid := scale3(norm3(dir), mr*lift)
		w := uint8(p.River[i])
		full := riverWidth[min(int(w), len(riverWidth)-1)]
		start := full
		if !fed[i] {
			start = full * riverSourceTaper
		}
		geo.Rivers = append(geo.Rivers, globeSeg{scale3(geo.baseDisp[i], lift), mid, w, start, full})
		if p.Water[to] == planet.Land {
			geo.Rivers = append(geo.Rivers, globeSeg{mid, scale3(geo.baseDisp[to], lift), w, full, full})
		}
	}
	for k, r := range p.Roads {
		cl := uint8(p.RoadClassOf(k))
		geo.Roads = append(geo.Roads, globeSeg{scale3(geo.baseDisp[r[0]], lift), scale3(geo.baseDisp[r[1]], lift), cl, 0, 0})
	}
}

// sharedEdge is the unique corner pair on the edge between tiles i and j.
func (geo *planetGeom) sharedEdge(p *planet.Planet, i int, j int32) (a, b uint32, ok bool) {
	o0, o1 := p.CornerOff[i], p.CornerOff[i+1]
	m := o1 - o0
	for k := uint32(0); k < m; k++ {
		a, b = geo.cornerID[o0+k], geo.cornerID[o0+(k+1)%m]
		if geo.across(int32(i), a, b) == j {
			return a, b, true
		}
	}
	return 0, 0, false
}

// waterClass separates land, ocean and lake for the soft borders: corner
// colours blend only within a class, so coasts stay crisp.
func waterClass(p *planet.Planet, i int) uint8 {
	if p.Water[i] == planet.Land && p.Biome[i] == biome.SeaIce {
		return planet.Ocean
	}
	return p.Water[i]
}

// snowline is tile i's snow height from its mean temperature brought down
// to sea level: the stored mean already falls with height, and using it as
// is would count the height twice and bury every mountain in snow.
func snowline(p *planet.Planet, i int) float32 {
	lat := float64(p.Lat[i]) / 10
	lapse := planet.TempAt(lat, 0, p.Params.Temp) - planet.TempAt(lat, int(p.Elev[i]), p.Params.Temp)
	t := float32(p.Temp[i])/10 + float32(lapse)
	return snowBaseM + snowPerC*t
}

// snowed colours a vertex h metres high on tile i: bare rock in a band
// below the snowline (the tree line), then snow above it.
func (geo *planetGeom) snowed(p *planet.Planet, i int, c rgb, h float32) rgb {
	if p.Biome[i] == biome.IceSheet && p.Water[i] == planet.Land {
		return PalRGB("snow")
	}
	if !isLand(p, i) {
		return c
	}
	if r := (h - geo.snowAt[i] + rockBelowM) / rockFadeM; r > 0 {
		c = MixRGB(c, PalRGB("rock"), rockMix*min(r, 1))
	}
	t := (h - geo.snowAt[i]) / snowBandM
	if t <= 0 {
		return c
	}
	if t >= 1 {
		return PalRGB("snow")
	}
	return MixRGB(c, PalRGB("snow"), t)
}

// vertexColours fills the centre and corner colours from base for the
// mode: corners average the sharing tiles of the same water class, the
// centre keeps the tile's own colour; biome mode adds snow.
func (geo *planetGeom) vertexColours(p *planet.Planet, base []rgb, snow bool) {
	for i := range base {
		c := base[i]
		if snow {
			c = geo.snowed(p, i, c, geo.centreH[i])
		}
		geo.ccol[i] = c
		cls := waterClass(p, i)
		for o := p.CornerOff[i]; o < p.CornerOff[i+1]; o++ {
			id := geo.cornerID[o]
			var sum rgb
			var k float32
			for _, t := range geo.cornerTiles[id] {
				if t < 0 || waterClass(p, int(t)) != cls {
					continue
				}
				b := base[t]
				sum[0], sum[1], sum[2] = sum[0]+b[0], sum[1]+b[1], sum[2]+b[2]
				k++
			}
			kc := base[i]
			if k > 0 {
				kc = scaleRGB(sum, 1/k)
			}
			if snow {
				kc = geo.snowed(p, i, kc, geo.cornerH[id])
			}
			geo.kcol[o] = kc
		}
	}
}

// coastalShallow reports whether ocean tile i borders land or a lake and is
// shallower than 200 m (the shallow band).
func coastalShallow(p *planet.Planet, i int) bool {
	if p.Water[i] != planet.Ocean || p.Elev[i] <= -200 {
		return false
	}
	for _, nb := range p.Neighbours(i) {
		if p.Water[nb] != planet.Ocean {
			return true
		}
	}
	return false
}
