package env

import (
	"math"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/light"
	"rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	"rimworld-go/internal/application/game"
)

// Puddles (PLAY-022): one smooth wet veil over the ground, not per-tile
// stickers.
//
// The ground has a low-frequency relief (PuddleField, value noise seeded by
// the map's size), and rain raises a water level (PuddleLevel) with
// WorldView.Wetness: wherever the relief lies under the level, water
// stands. Patches therefore span several tiles, start in the same hollows
// every time it rains and grow and shrink smoothly with the wetness.
//
// Water lies only on open, flat soil-type ground (PuddleGroundCode: soil,
// rich soil, gravel; grass grows on all three) that is not a floor or a
// crop's furrows, and never under a roof, on a wall or rock. Ground that
// takes no water raises the relief near it, so a puddle's edge curves away
// from a road or a wall instead of stopping on the tile line.
//
// The veil is the wet image of the snow chunks (snow_cache.go): one
// premultiplied image per 32-tile chunk holding the wet darkening of open
// ground and the puddles on top of it, so the two are one layer with one
// mask. Its alpha covers the ground below; its grey channel, a share of
// the alpha, is the water's colour, which the draw tints with the sky
// (PuddleSky): wet ground darkens, water takes the reflected sky's slate
// blue. It is drawn through the lit shader like the ground
// (drawBakedColor, drawChunkFlat), with no normal map, after the furrows
// and under the grass tufts at every zoom tier (draw_art.go), at 8 texels
// a tile up close for crisp shores. A chunk is rebaked only when its
// wetness bucket, its snow, its ground, roofs or crops, or the zoom's
// detail change, within a cost budget a frame (PuddleBakeBudget).
// Snow fades the water out as it deepens; raindrops ripple only where the
// baked cover holds water (PuddleAt).

const (
	// WetBuckets is how finely the bake follows the wetness: a step moves
	// a puddle's edge by a sliver, so a rebake never pops.
	WetBuckets = 64
	// PuddleBakeBudget is the bake cost (bakeCost: a 4-texel chunk bake is
	// 1, an 8-texel one 4) a pass may spend on rebakes.
	PuddleBakeBudget = 8
	// wetDarken is the wet darkening of all open ground at full wetness
	// (the old flat wet veil), wetRefl its reflection share.
	wetDarken = 0.16
	wetRefl   = 0.13
	// A puddle covers the ground by puddleAlpha at its edge, up to
	// puddleAlpha+puddleAlphaDeep in its middle, with water whose colour
	// is puddleRefl of the sky it reflects: the ground moves toward that
	// one colour as the water deepens (darker on light soil, cooler and a
	// little lighter on dark), so the edge never rings.
	puddleAlpha     = 0.35
	puddleAlphaDeep = 0.2
	puddleRefl      = 0.62
	// puddleEdge is how far (tiles) the cover ramps in from the water's
	// edge; puddleDeep how far in the middle is reached.
	puddleEdge = 0.2
	puddleDeep = 0.6
	// puddleWall raises the relief by up to this within puddleWallReach
	// tiles of ground that takes no water, more than the deepest water, so
	// no puddle reaches its edge.
	puddleWall      = 0.5
	puddleWallReach = 0.45
	// puddleDip is how far the relief can dip inside a tile below its
	// lowest corner (measured 0.097), for the dry-tile fast path.
	puddleDip = 0.11
	// puddleSnowGone is the snow depth (0..1) that hides the water.
	puddleSnowGone = 0.3
	// PuddleRippleMin is the cover a raindrop must land in to ripple.
	PuddleRippleMin = 0.5
	// puddleGradMin bounds the relief's slope in the edge distance, so
	// the flat bottom of a hollow reads as deep water.
	puddleGradMin = 0.06
)

// PuddleGroundCode marks the terrains water lies on.
var PuddleGroundCode = func() (out [256]bool) {
	for i, n := range terrainNames {
		out[i] = n == "soil" || n == "rich_soil" || n == "gravel"
	}
	return out
}()

// WetBucket is the bake's step of a wetness (0..WetBuckets).
func WetBucket(wet float64) uint8 { return uint8(math.Round(gfx.Clamp01(wet) * WetBuckets)) }

// PuddleSalt seeds the map's relief: its size (WorldView carries no seed),
// so a map keeps its puddles across saves and loads.
func PuddleSalt(mw, mh int) uint64 {
	return art.Mix64(uint64(uint32(mw))<<32 | uint64(uint32(mh)) ^ 0x9dd1e5)
}

// PuddleField is the ground's relief at world point (fx, fy), 0..1: low
// in the hollows where water gathers first. Three octaves of value noise
// (puddleOctaves), each on a turned lattice so no shore runs along the
// tile grid.
func PuddleField(fx, fy float64, salt uint64) float64 {
	v := 0.0
	for _, o := range puddleOctaves {
		x, y := o.lattice(fx, fy)
		v += o.w * ValueNoise(x, y, salt^o.salt)
	}
	return v
}

// puddleOctave is one octave of the relief: value noise scale tiles
// across, its lattice turned by an angle (cos c, sin s), weighted w.
type puddleOctave struct {
	scale, w, c, s float64
	salt           uint64
}

func octave(scale, ang, w float64, salt uint64) puddleOctave {
	return puddleOctave{scale: scale, w: w, c: math.Cos(ang), s: math.Sin(ang), salt: salt}
}

// puddleOctaves: hollows about 4.5 tiles across, puddles about 1.9, and a
// fine wobble on their shores.
var puddleOctaves = [3]puddleOctave{octave(4.5, 0.52, 0.52, 0), octave(1.9, -0.95, 0.36, 0x2f17), octave(0.95, 1.3, 0.12, 0x51c3)}

// lattice is world point (fx, fy) in the octave's lattice units.
func (o *puddleOctave) lattice(fx, fy float64) (float64, float64) {
	return (fx*o.c - fy*o.s) / o.scale, (fx*o.s + fy*o.c) / o.scale
}

// puddleLevels is the water level by wetness: none until the ground is
// damp, then about 1.5% of open soil under water at 0.3, 5% at 0.5, 10%
// at 0.75 and 15% soaked (the relief's measured quantiles).
var puddleLevels = [...][2]float64{{0, 0}, {0.12, 0.12}, {0.3, 0.212}, {0.5, 0.273}, {0.75, 0.319}, {1, 0.352}}

// PuddleLevel is the water level at wetness wet (0..1): the relief under
// it holds water. It only rises with the wetness.
func PuddleLevel(wet float64) float64 {
	wet = gfx.Clamp01(wet)
	for i := 1; i < len(puddleLevels); i++ {
		a, b := puddleLevels[i-1], puddleLevels[i]
		if wet <= b[0] {
			return a[1] + (b[1]-a[1])*(wet-a[0])/(b[0]-a[0])
		}
	}
	return puddleLevels[len(puddleLevels)-1][1]
}

// PuddleShade is the wet veil at one point: the darkening alpha, the
// reflection weight (premultiplied, never over the alpha) and the puddle
// cover 0..1, for open ground darkened aw by the wetness, dist tiles in
// from the water's edge (negative: dry) and the snow's fade (1: none). It
// only deepens toward the middle, so a puddle has no rim.
func PuddleShade(aw, dist, fade float64) (a, refl, cover float64) {
	cover = sstep(dist/puddleEdge) * fade
	pa := puddleAlpha + puddleAlphaDeep*sstep(dist/puddleDeep)
	rw := aw * wetRefl
	return aw + cover*(pa-aw), rw + cover*(pa*puddleRefl-rw), cover
}

// PuddleSky is the wet veil's vertex colour: the sky the water reflects
// (the reflection weight in the bake's grey channel takes its hue), a
// cool grey-blue by day tinted like the sky ambient, dim at night. The
// alpha scale is 1: the darkening is baked.
func PuddleSky(l light.State) [4]float32 {
	day := gfx.Clamp01((float64(light.Lum3(l.SkyAmbient)) - 0.18) / (0.52 - 0.18))
	k := 0.35 + 0.65*day
	base := [3]float64{0.46, 0.62, 0.80} // the marsh puddles' slate blue, lit
	hue := [3]float64{1, 1, 1}
	if m := max(l.SkyAmbient[0], l.SkyAmbient[1], l.SkyAmbient[2]); m > 0 {
		for i := range hue {
			hue[i] = float64(l.SkyAmbient[i] / m)
		}
	}
	var out [4]float32
	for i := range 3 {
		out[i] = float32(base[i] * (0.5 + 0.5*hue[i]) * k)
	}
	out[3] = 1
	return out
}

// DrawPuddleVeil draws the rain puddles (PLAY-022): the wet veil over the
// ground and under the grass tufts, at every zoom tier: stamped through
// s, baked from c's chunk index.
func (e *Layer) DrawPuddleVeil(v *render.View, screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64, s Stamp, c Chunks) {
	sc := e.SnowC
	sc.WetDraws = 0
	if WetBucket(w.Wetness) == 0 {
		return
	}
	e.SetView(v, sw, sh, tile)
	e.SetWeather(w) // the bake reads its snow
	col := PuddleSky(e.Lit.Light)
	e.eachSnowChunk(v, w, tile, true, c, func(ch *SnowChunk, cx, cy, tx0, ty0, tx1, ty1 int) {
		if ch.anyWet && ch.wetImg != nil {
			drawChunkMask(v, screen, ch.wetImg, ch.WetPx, cx, cy, tx0, ty0, tx1, ty1, tile, col, s)
			sc.WetDraws++
		}
	})
}

// PuddleAt is the baked puddle cover (0..1) at world point (fx, fy): 0
// off the chunks baked so far.
func (sc *SnowCache) PuddleAt(fx, fy float64) float64 {
	if sc.Chunks == nil || fx < 0 || fy < 0 || fx >= float64(sc.w) || fy >= float64(sc.h) {
		return 0
	}
	cx, cy := int(fx)/SnowChunkTiles, int(fy)/SnowChunkTiles
	c := sc.Chunks[cy*((sc.w+SnowChunkTiles-1)/SnowChunkTiles)+cx]
	if c == nil || c.Pud == nil {
		return 0
	}
	tx := SnowPad + int((fx-float64(cx*SnowChunkTiles))*float64(c.WetPx))
	ty := SnowPad + int((fy-float64(cy*SnowChunkTiles))*float64(c.WetPx))
	return float64(c.Pud[ty*WetImgW(c.WetPx)+tx]) / 255
}

// --- the bake -----------------------------------------------------------------

// WetBake is one chunk bake's wet inputs over the chunk and its one-tile
// ring (ring tile (0, 0) is world tile (ox, oy)); snow_cache.go fills the
// wet image texel by texel from it.
type WetBake struct {
	aw     float64   // the wet darkening of open ground
	level  float64   // the water level
	pools  bool      // water may stand somewhere in the ring
	ox, oy int       // world tile of ring tile (0, 0)
	allow  []bool    // snowRing² tiles water lies on
	ban    []bool    // snowRing² floors and crops (scratch)
	dry    []bool    // snowRing² tiles no water can reach (fast path)
	field  []float64 // (snowRing+1)² relief at the tile corners
	snow   []float64 // (snowRing+1)² snow depth at the corners
	bufs   []int32
	lat    [len(puddleOctaves)]noiseLattice
}

// prepareWet fills wb for chunk (cx, cy) at wetness bucket wet, given
// the ring's open tiles (snowRing², ring tile (0, 0) at (cx·32−1, cy·32−1)).
func (e *Layer) prepareWet(wb *WetBake, w *game.WorldView, open []bool, mw, mh, cx, cy int, wet uint8, c Chunks) {
	n := snowRing * snowRing
	nc := (snowRing + 1) * (snowRing + 1)
	if cap(wb.allow) < n {
		wb.allow, wb.ban, wb.dry = make([]bool, n), make([]bool, n), make([]bool, n)
		wb.field, wb.snow = make([]float64, nc), make([]float64, nc)
	}
	wb.allow, wb.ban, wb.dry = wb.allow[:n], wb.ban[:n], wb.dry[:n]
	wf := float64(wet) / WetBuckets
	wb.aw = wetDarken * wf
	wb.level = PuddleLevel(wf)
	wb.pools = false
	if wb.level <= 0 {
		return
	}
	ox, oy := cx*SnowChunkTiles-1, cy*SnowChunkTiles-1
	wb.ox, wb.oy = ox, oy
	markPuddleBans(wb, w, ox, oy, mw, mh, c)
	any := false
	for ry := 0; ry < snowRing; ry++ {
		for rx := 0; rx < snowRing; rx++ {
			i := ry*snowRing + rx
			x, y := ox+rx, oy+ry
			ok := x < 0 || y < 0 || x >= mw || y >= mh // off the map walls nothing in
			if !ok && open[i] && !wb.ban[i] {
				code, known := TerrainCode(*w, x, y)
				ok = !known || PuddleGroundCode[code]
			}
			wb.allow[i] = ok
			any = any || ok && open[i]
		}
	}
	if !any {
		return
	}
	wb.FillLattices(float64(ox), float64(oy), float64(ox+snowRing), float64(oy+snowRing), PuddleSalt(mw, mh))
	for ry := 0; ry <= snowRing; ry++ {
		for rx := 0; rx <= snowRing; rx++ {
			x, y := ox+rx, oy+ry
			k := ry*(snowRing+1) + rx
			wb.field[k] = wb.Relief(float64(x), float64(y))
			wb.snow[k] = e.SnowCorner(x, y)
		}
	}
	// a tile whose relief stays over the level, or under deep snow at
	// every corner, holds no water: its texels take the fast path
	for ry := 0; ry < snowRing; ry++ {
		for rx := 0; rx < snowRing; rx++ {
			i := ry*snowRing + rx
			if !wb.allow[i] || !open[i] {
				wb.dry[i] = true
				continue
			}
			k := ry*(snowRing+1) + rx
			lo, loSnow := math.Inf(1), math.Inf(1)
			for _, j := range [4]int{k, k + 1, k + snowRing + 1, k + snowRing + 2} {
				lo = math.Min(lo, wb.field[j])
				loSnow = math.Min(loSnow, wb.snow[j])
			}
			wb.dry[i] = lo-puddleDip >= wb.level || loSnow >= puddleSnowGone
			wb.pools = wb.pools || !wb.dry[i]
		}
	}
}

// FillLattices caches each octave's lattice over world rectangle
// [x0, x1] × [y0, y1].
func (wb *WetBake) FillLattices(x0, y0, x1, y1 float64, salt uint64) {
	for i := range puddleOctaves {
		o := &puddleOctaves[i]
		lx0, ly0, lx1, ly1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
		for _, p := range [4][2]float64{{x0, y0}, {x1, y0}, {x0, y1}, {x1, y1}} {
			x, y := o.lattice(p[0], p[1])
			lx0, ly0, lx1, ly1 = math.Min(lx0, x), math.Min(ly0, y), math.Max(lx1, x), math.Max(ly1, y)
		}
		wb.lat[i].fill(lx0, ly0, lx1, ly1, salt^o.salt)
	}
}

// Relief is the Relief at world point (fx, fy), from the cached
// lattices (PuddleField's value).
func (wb *WetBake) Relief(fx, fy float64) float64 {
	f := 0.0
	for i := range puddleOctaves {
		o := &puddleOctaves[i]
		f += o.w * wb.lat[i].at(o.lattice(fx, fy))
	}
	return f
}

// wallDist is how far (tiles) point (u, v) of ring tile (rx, ry) lies
// from the nearest neighbouring tile that takes no water; +Inf when none
// is near. Neighbours past the ring count as open: they are farther than
// puddleWallReach from any texel the bake writes.
func (wb *WetBake) wallDist(rx, ry int, u, v float64) float64 {
	d := math.Inf(1)
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			nx, ny := rx+dx, ry+dy
			if dx == 0 && dy == 0 || nx < 0 || ny < 0 || nx >= snowRing || ny >= snowRing || wb.allow[ny*snowRing+nx] {
				continue
			}
			// the gap from the neighbour's square to the point, per axis
			var ex, ey float64
			switch dx {
			case -1:
				ex = u
			case 1:
				ex = 1 - u
			}
			switch dy {
			case -1:
				ey = v
			case 1:
				ey = 1 - v
			}
			d = math.Min(d, math.Hypot(ex, ey))
		}
	}
	return d
}

// wet reports ring tile (rx, ry) may hold water (it is not on the fast
// dry path).
func (wb *WetBake) wet(rx, ry int) bool { return wb.pools && !wb.dry[ry*snowRing+rx] }

// height is the relief at (u, v) of ring tile (rx, ry), raised toward
// ground that takes no water so a shore curves away from it.
func (wb *WetBake) height(rx, ry int, u, v float64) float64 {
	g := wb.Relief(float64(wb.ox+rx)+u, float64(wb.oy+ry)+v)
	if dw := wb.wallDist(rx, ry, u, v); dw < puddleWallReach {
		t := 1 - dw/puddleWallReach
		g += puddleWall * t * t * (3 - 2*t)
	}
	return g
}

// snowFade is how much of the water snow leaves at (u, v) of ring tile
// (rx, ry): 1 bare, 0 under puddleSnowGone.
func (wb *WetBake) snowFade(rx, ry int, u, v float64) float64 {
	k := ry*(snowRing+1) + rx
	s0 := wb.snow[k] + (wb.snow[k+1]-wb.snow[k])*u
	s1 := wb.snow[k+snowRing+1] + (wb.snow[k+snowRing+2]-wb.snow[k+snowRing+1])*u
	return 1 - sstep((s0+(s1-s0)*v)/puddleSnowGone)
}

// shade is the wet veil at a texel whose height is g, with the height's
// slope (per tile) measured across its neighbouring texels, so the
// distance to the shore follows the relief's bends at the texel's own
// scale.
func (wb *WetBake) shade(g, slope, fade float64) (a, refl, cover float64) {
	if slope < puddleGradMin {
		slope = puddleGradMin
	}
	return PuddleShade(wb.aw, (wb.level-g)/slope, fade)
}

// markPuddleBans marks the ring's floors and crops in wb.ban: from c's
// chunk read model when w is the simulation's current world, else by
// scanning w (tests hand worlds in directly).
func markPuddleBans(wb *WetBake, w *game.WorldView, ox, oy, mw, mh int, c Chunks) {
	clear(wb.ban)
	mark := func(x, y, bw, bh int) {
		for ty := max(y, oy); ty < min(y+max(bh, 1), oy+snowRing); ty++ {
			for tx := max(x, ox); tx < min(x+max(bw, 1), ox+snowRing); tx++ {
				wb.ban[(ty-oy)*snowRing+tx-ox] = true
			}
		}
	}
	x0, y0 := max(ox, 0), max(oy, 0)
	x1, y1 := min(ox+snowRing-1, mw-1), min(oy+snowRing-1, mh-1)
	if x1 < x0 || y1 < y0 {
		return
	}
	floor := func(b *game.BuildingView) {
		if b.Floor {
			mark(b.X, b.Y, b.W, b.H)
		}
	}
	if c.Current(w.Buildings) {
		cs := game.ChunkSize
		// a floor's anchor is its only tile, so the chunks under the ring hold them all
		for gy := y0 / cs; gy <= y1/cs; gy++ {
			for gx := x0 / cs; gx <= x1/cs; gx++ {
				for _, bi := range c.ChunkView(gx, gy).Buildings {
					if int(bi) < len(w.Buildings) {
						floor(&w.Buildings[bi])
					}
				}
			}
		}
	} else {
		for i := range w.Buildings {
			floor(&w.Buildings[i])
		}
	}
	wb.bufs = c.PlantsOn(w.Plants, x0, y0, x1, y1, wb.bufs[:0])
	for _, pi := range wb.bufs {
		if int(pi) < len(w.Plants) && IsCropPlant(&w.Plants[pi]) {
			mark(w.Plants[pi].X, w.Plants[pi].Y, 1, 1)
		}
	}
}

// puddleCropKey is the plant versions of the read-model chunks under
// snow chunk (cx, cy) and its ring: it moves when a crop (or any plant)
// there comes, goes or grows a stage. Memoised per pass.
func (sc *SnowCache) puddleCropKey(cx, cy, mw, mh int, c Chunks) uint64 {
	cs := game.ChunkSize
	gcw, gch := (mw+cs-1)/cs, (mh+cs-1)/cs
	if len(sc.cropVer) != gcw*gch {
		sc.cropVer, sc.cropPass = make([]uint64, gcw*gch), make([]uint32, gcw*gch)
	}
	x0, y0 := max(cx*SnowChunkTiles-1, 0)/cs, max(cy*SnowChunkTiles-1, 0)/cs
	x1, y1 := min(cx*SnowChunkTiles+SnowChunkTiles, mw-1)/cs, min(cy*SnowChunkTiles+SnowChunkTiles, mh-1)/cs
	h := uint64(0xc0f5)
	for gy := y0; gy <= y1; gy++ {
		for gx := x0; gx <= x1; gx++ {
			i := gy*gcw + gx
			if sc.cropPass[i] != sc.pass {
				sc.cropVer[i], sc.cropPass[i] = c.ChunkView(gx, gy).PlantVer, sc.pass
			}
			h = art.Mix64(h ^ sc.cropVer[i] ^ uint64(i)<<40)
		}
	}
	return h
}

// sstep is motion.Smoothstep01 for the bake's inner loops (math.Max and
// math.Min do not inline).
func sstep(x float64) float64 {
	switch {
	case x <= 0:
		return 0
	case x >= 1:
		return 1
	}
	return x * x * (3 - 2*x)
}

// noiseLattice caches ValueNoise's lattice values over a rectangle, so a
// bake evaluates the relief without hashing every corner.
type noiseLattice struct {
	ix0, iy0, nx, ny int
	v                []float64
}

// fill caches the lattice points around [x0, x1] × [y0, y1] (lattice units).
func (l *noiseLattice) fill(x0, y0, x1, y1 float64, salt uint64) {
	l.ix0, l.iy0 = int(math.Floor(x0)), int(math.Floor(y0))
	l.nx, l.ny = int(math.Floor(x1))-l.ix0+2, int(math.Floor(y1))-l.iy0+2
	if cap(l.v) < l.nx*l.ny {
		l.v = make([]float64, l.nx*l.ny)
	}
	l.v = l.v[:l.nx*l.ny]
	for j := 0; j < l.ny; j++ {
		for i := 0; i < l.nx; i++ {
			l.v[j*l.nx+i] = motion.Hash01(uint64(int64(l.ix0+i)), uint64(int64(l.iy0+j)), salt)
		}
	}
}

// at is ValueNoise(fx, fy, salt) from the cache (fx, fy inside the
// filled rectangle).
func (l *noiseLattice) at(fx, fy float64) float64 {
	ix, iy := math.Floor(fx), math.Floor(fy)
	u, v := sstep(fx-ix), sstep(fy-iy)
	i := (int(iy)-l.iy0)*l.nx + int(ix) - l.ix0
	a, b, c, d := l.v[i], l.v[i+1], l.v[i+l.nx], l.v[i+l.nx+1]
	top := a + (b-a)*u
	bot := c + (d-c)*u
	return top + (bot-top)*v
}
