package ebiten

import (
	"image"
	"image/draw"
	"math"
	"slices"
	"strconv"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/platform/config"
)

// Grass as composed tufts (PLAY-010 §2.11, ART §9.1). A grassy tile is
// never a carpet: it is up to nine small tuft copies in a jittered 3×3
// grid, the edge copies spilling a little past the tile so neighbours
// interleave and the field reads continuous while soil shows between
// tufts. Every copy in view is one triangle batch from a small runtime
// atlas of the tuft sprites, lit like a sprite (lit.kage: per-vertex sky
// and lamp light), coloured by the biome's grassTint and the snow fade,
// its top vertices swayed by the wind field.
//
// Level of detail by on-screen tile size:
//
//	≥ 48 px   every copy, live sway;
//	24–48 px  at most grassMidCap copies a tile, live sway;
//	16–24 px  grassMidCap copies baked into the chunk images (no sway);
//	          the bake's key folds in the chunk's grass (grassChunkSig);
//	< 16 px   a soft grass tint baked into the map image (grassMapSig).

const (
	// grassCellJitter is how far a copy may sit from its grid cell's
	// centre, in tiles.
	grassCellJitter = 0.18
	// grassEdgeSpill is how far past the tile edge an edge copy may sit.
	grassEdgeSpill = 0.15
	// grassLeanMax is the grass lean limit (ART §27.3), radians.
	grassLeanMax = 0.35
	// grassLiveMin is the tile size (px) from which tufts are drawn live;
	// below it the chunk tier bakes them.
	grassLiveMin = 24.0
	// grassMidCap is the copies a tile draws below lodLiveMin.
	grassMidCap = 4
	// tuftRefH is a short tuft's height at scale 1, in tiles: the atlas
	// sizes every tuft sprite by the short set's median height, so the
	// composition holds whatever canvas the art is rendered at.
	tuftRefH = 0.55
	// grassTallNoise is the valueNoise level above which a soil tile lies
	// in a tall-grass patch.
	grassTallNoise = 0.64
	// grassCoverReach is how far up and left of a tile a building or frame
	// anchor can lie and still cover it.
	grassCoverReach = visPad
	// grassSigBudget is how many map-tier chunk signatures a frame may
	// recompute after the grass changed (the rest show a frame later).
	grassSigBudget = 64
)

// grassGroundCode marks the terrains the simulation seeds grass on: any
// fertile ground (plant.GrassGround outside the legacy map; pinned by
// TestGrassMatchesRendererRule).
var grassGroundCode = func() (out [256]bool) {
	for i, n := range terrainNames {
		out[i] = grassGroundName(n)
	}
	return out
}()

// grassGroundName reports a terrain that carries grass.
func grassGroundName(t string) bool {
	switch t {
	case "soil", "rich_soil", "gravel", "marsh", "sand", "mud":
		return true
	}
	return false
}

// tuftCopies is how many tuft copies a grass tile of this growth draws.
func tuftCopies(growth uint8) int {
	switch {
	case growth < 25:
		return 1
	case growth < 50:
		return 2
	case growth < 75:
		return 4
	case growth < 90:
		return 6
	}
	return 9
}

// tuftGrowthSize is a copy's size factor by copy count: shoots are a
// little smaller than a full sward (by bucket, so bakes key on it).
func tuftGrowthSize(n int) float64 {
	switch n {
	case 1:
		return 0.78
	case 2:
		return 0.84
	case 4:
		return 0.9
	case 6:
		return 0.96
	}
	return 1
}

// tuftOrder is which 3×3 cells (row-major) n copies take, well spread;
// each tile turns it by one of the grid's eight symmetries.
var tuftOrder = [10][]uint8{
	1: {4},
	2: {0, 8},
	4: {0, 2, 6, 8},
	6: {0, 2, 4, 6, 8, 7},
	9: {4, 0, 2, 6, 8, 1, 3, 5, 7},
}

// tuftCopy is one tuft copy of a tile: its foot at (dx, dy) tiles from
// the tile's top-left, its scale, its share of the tile's sway, the
// variant it draws, mirrored or not, from the tall set or not.
type tuftCopy struct {
	dx, dy, scale, sway float64
	variant             uint32
	mirror, tall        bool
}

// grassCopies appends tile (x, y)'s tuft copies at this growth, at most
// limit of them, to out: a pure function of the tile. tallTile marks a
// tile in a tall patch or on rich soil (grassTallAt): two copies in three
// draw tall there.
func grassCopies(x, y int, growth uint8, limit int, tallTile bool, out []tuftCopy) []tuftCopy {
	n := min(tuftCopies(growth), max(limit, 1))
	order := tuftOrder[n]
	if order == nil {
		order = tuftOrder[9][:n]
	}
	th := tileHash(x, y)
	sym := mix64(th^0x67726173) & 7
	size := tuftGrowthSize(tuftCopies(growth))
	for k, cell := range order {
		cx, cy := int(cell%3), int(cell/3)
		if sym&1 != 0 {
			cx = 2 - cx
		}
		if sym&2 != 0 {
			cy = 2 - cy
		}
		if sym&4 != 0 {
			cx, cy = cy, cx
		}
		h := mix64(th ^ uint64(k+1)*0x9E3779B97F4A7C15 ^ 0x7475667473)
		u := func(shift uint) float64 { return float64(h>>shift&0xffff) / 65535 }
		c := tuftCopy{
			dx:      tuftAxis(cx, u(0)),
			dy:      tuftAxis(cy, u(16)),
			sway:    0.8 + 0.4*u(32),
			variant: uint32(h >> 49),
			mirror:  h>>48&1 != 0,
			tall:    tallTile && (h>>60)%3 != 0,
		}
		s := u(40)
		if c.tall {
			c.scale = (0.7 + 0.2*s) * size
		} else {
			c.scale = (0.45 + 0.2*s) * size
		}
		out = append(out, c)
	}
	return out
}

// tuftAxis is a copy's position along one axis: its cell's centre ±
// grassCellJitter, an edge cell reaching up to grassEdgeSpill past the
// tile edge; u in [0, 1].
func tuftAxis(cell int, u float64) float64 {
	c := (float64(cell) + 0.5) / 3
	lo, hi := c-grassCellJitter, c+grassCellJitter
	if cell == 0 {
		lo = -grassEdgeSpill
	}
	if cell == 2 {
		hi = 1 + grassEdgeSpill
	}
	return lo + (hi-lo)*u
}

// grassTallAt reports a tile whose grass grows tall: rich soil, or a
// smooth valueNoise patch (so tall grass clusters, never checkerboards).
func grassTallAt(code uint8, x, y int) bool {
	if int(code) < len(terrainNames) && terrainNames[code] == "rich_soil" {
		return true
	}
	return valueNoise(float64(x)/6, float64(y)/6, 0x7a11) > grassTallNoise
}

// grassDormant reports the winter look: tufts draw their dormant (straw)
// sprites.
func grassDormant(w *game.WorldView) bool { return w.Season == "Winter" && w.OutdoorTemp < 50 }

// grassGrowthAt is grass tile i's growth (an empty GrassGrowth: full).
func grassGrowthAt(w *game.WorldView, i int) uint8 {
	if i < 0 || i >= len(w.GrassGrowth) {
		return 100
	}
	return w.GrassGrowth[i]
}

// grassAt reports whether tile (x, y), terrain code, shows grass, from
// the frame's mask (grassFrameMask); with no simulation grass data, the
// old hashed rule.
func (gb *grassBatch) grassAt(w *game.WorldView, code uint8, x, y int) bool {
	if !grassGroundCode[code] {
		return false
	}
	if len(w.Grass) == 0 {
		return (tileHash(x, y)>>8)%grassEvery == 0
	}
	i := y*w.TerrainW + x
	return i >= 0 && i < len(w.Grass) && i < len(gb.mask) && w.Grass[i] == 1 && gb.mask[i]&grassCovered == 0
}

// --- the tuft atlas -----------------------------------------------------------

// The tuft sprites (PLAY-010).
var (
	grassShortKeys   = vKeys("plant/grass_short_v", 6)
	grassTallKeys    = vKeys("plant/grass_tall_v", 4)
	grassDormantKeys = vKeys("plant/grass_short_dormant_v", 3)
)

func vKeys(prefix string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = prefix + strconv.Itoa(i)
	}
	return out
}

// grassDryCS dries the short tufts into straw when no dormant art exists.
var grassDryCS = colorScale{1.12, 1.0, 0.62, 1}

// tuftSprite is one tuft in the atlas: its source rectangle (px), its
// foot's x within it, and tiles per atlas pixel at scale 1.
type tuftSprite struct {
	sx, sy, w, h float32
	bx           float32
	k            float32
}

// grassAtlas packs the tuft sprites' alpha boxes into one image, so a
// whole field is one draw.
type grassAtlas struct {
	art                  *artCatalog
	img                  *ebiten.Image
	short, tall, dormant []tuftSprite
	dry                  bool // dormant is the short set, dried by grassDryCS
}

// grassAtlasPad is the transparent margin around each packed tuft (the
// shader's bilinear taps reach half a pixel out).
const grassAtlasPad = 2

// buildGrassAtlas packs c's tuft sprites (alpha-trimmed) into one image.
// No tuft art: an atlas without sprites (nothing is drawn).
func buildGrassAtlas(c *artCatalog) *grassAtlas {
	a := &grassAtlas{art: c}
	pick := func(keys, fallback []string) []string {
		var out []string
		for _, set := range [][]string{keys, fallback} {
			for _, k := range set {
				if c.Has(k) {
					out = append(out, k)
				}
			}
			if len(out) > 0 {
				break
			}
		}
		return out
	}
	type crop struct {
		pix  *image.NRGBA
		r    image.Rectangle
		k    float64 // canvas px per pix px
		x, y int     // packed position
	}
	measure := func(keys []string) []crop {
		var out []crop
		for _, key := range keys {
			sp := c.Sprite(key)
			if sp == nil || sp.Pix == nil {
				continue
			}
			r, ok := alphaBox(sp.Pix)
			if !ok {
				continue
			}
			k := 1.0
			if b := sp.Pix.Bounds(); b.Dx() > 0 && sp.Meta.Canvas[0] > 0 {
				k = float64(sp.Meta.Canvas[0]) / float64(b.Dx())
			}
			out = append(out, crop{pix: sp.Pix, r: r, k: k})
		}
		return out
	}
	sets := [3][]crop{measure(pick(grassShortKeys, nil)), measure(pick(grassTallKeys, nil)), measure(pick(grassDormantKeys, nil))}
	if len(sets[0]) == 0 {
		return a
	}
	if len(sets[1]) == 0 {
		sets[1] = sets[0]
	}
	if len(sets[2]) == 0 {
		sets[2], a.dry = sets[0], true
	}
	// size: the short set's median height is tuftRefH tiles at scale 1
	hs := make([]float64, len(sets[0]))
	for i, cr := range sets[0] {
		hs[i] = float64(cr.r.Dy()) * cr.k / artTilePx
	}
	slices.Sort(hs)
	norm := tuftRefH / max(hs[len(hs)/2], 1e-6)
	// shelf packing, rows up to 1024 px wide
	const rowW = 1024
	x, y, rowH, width := grassAtlasPad, grassAtlasPad, 0, 0
	for s := range sets {
		for i := range sets[s] {
			cr := &sets[s][i]
			if s > 0 && len(sets[0]) > 0 && &sets[s][0] == &sets[0][0] {
				break // a fallback set shares the short set's crops
			}
			if x+cr.r.Dx()+grassAtlasPad > rowW && x > grassAtlasPad {
				x, y, rowH = grassAtlasPad, y+rowH+grassAtlasPad, 0
			}
			cr.x, cr.y = x, y
			x += cr.r.Dx() + grassAtlasPad
			rowH = max(rowH, cr.r.Dy())
			width = max(width, x)
		}
	}
	pack := image.NewNRGBA(image.Rect(0, 0, width, y+rowH+grassAtlasPad))
	for s := range sets {
		for _, cr := range sets[s] {
			draw.Draw(pack, image.Rect(cr.x, cr.y, cr.x+cr.r.Dx(), cr.y+cr.r.Dy()), cr.pix, cr.r.Min, draw.Src)
		}
	}
	a.img = ebiten.NewImageFromImage(pack)
	out := func(set []crop) []tuftSprite {
		ts := make([]tuftSprite, len(set))
		for i, cr := range set {
			ts[i] = tuftSprite{sx: float32(cr.x), sy: float32(cr.y), w: float32(cr.r.Dx()), h: float32(cr.r.Dy()),
				bx: float32(cr.r.Dx()) / 2, k: float32(cr.k / artTilePx * norm)}
		}
		return ts
	}
	a.short, a.tall, a.dormant = out(sets[0]), out(sets[1]), out(sets[2])
	return a
}

// alphaBox is the bounds of pix's visible pixels (alpha ≥ 8).
func alphaBox(pix *image.NRGBA) (image.Rectangle, bool) {
	b := pix.Bounds()
	x0, y0, x1, y1 := b.Max.X, b.Max.Y, b.Min.X-1, b.Min.Y-1
	for y := b.Min.Y; y < b.Max.Y; y++ {
		row := pix.Pix[pix.PixOffset(b.Min.X, y):]
		for x := 0; x < b.Dx(); x++ {
			if row[4*x+3] >= 8 {
				x0, x1 = min(x0, b.Min.X+x), max(x1, b.Min.X+x)
				y0, y1 = min(y0, y), max(y1, y)
			}
		}
	}
	if x1 < x0 {
		return image.Rectangle{}, false
	}
	return image.Rect(x0, y0, x1+1, y1+1), true
}

// --- the batch ----------------------------------------------------------------

// grassBatch is the renderer's grass state: the atlas, the frame's grass
// mask and reused vertex buffers, the map-tier signature memo, and
// counters the tests read.
type grassBatch struct {
	atlas  *grassAtlas
	mask   []uint8 // grassMask bits per tile (grass_draw.go)
	verts  []ebiten.Vertex
	idx    []uint32
	copies []tuftCopy
	sigs   map[int]grassMemo
	budget int // map-tier signature recomputes left this frame
	frame  uint64
	// counters (tests): quads in the last batch, live batches drawn,
	// tuft bakes into chunk images and grass-tint bakes into the map.
	quads, flushes, baked, tinted int
	// onFlush, when set (tests), runs as a live batch is drawn.
	onFlush func()
	// level is the grass detail (Options > Graphics, PLAY-019): the empty
	// string or config.GrassFull draws every tuft, GrassReduced and
	// GrassSparse thin them per tile (grassKeep).
	level string
}

// grassDensity is the share of a tile's tufts a detail level keeps.
func grassDensity(level string) float64 {
	switch level {
	case config.GrassReduced:
		return 0.6
	case config.GrassSparse:
		return 0.3
	}
	return 1
}

// grassKeep is how many of n tufts tile (x, y) keeps at detail level: n
// times the level's density, the fraction rounded up or down by the tile's
// own hash. It is a pure function of the tile, so a tile thins the same
// way every frame and in every bake, and a sparser level only ever keeps
// fewer tufts than a denser one.
func grassKeep(level string, x, y, n int) int {
	d := grassDensity(level)
	if d >= 1 || n <= 0 {
		return n
	}
	f := float64(n) * d
	k := int(f)
	if u := float64(mix64(tileHash(x, y)^0x7468696e)>>11) / (1 << 53); u < f-float64(k) {
		k++
	}
	return k
}

// grassMemo is a chunk's map-tier grass signature and the inputs it was
// computed from.
type grassMemo struct {
	ver, grassVer uint64
	grass, growth *uint8
	sig           uint64
}

// grassAtlas is the tuft atlas of the current art (built on first use).
func (r *Renderer) grassAtlas() *grassAtlas {
	gb := &r.lod.grass
	if r.art == nil {
		return nil
	}
	if gb.atlas == nil || gb.atlas.art != r.art {
		gb.atlas = buildGrassAtlas(r.art)
	}
	if gb.atlas.img == nil {
		return nil
	}
	return gb.atlas
}

// drawGrassBatch draws the tufts reaching into tiles x0..x1 × y0..y1 as
// one triangle batch (the frame's mask filled first): at most limit
// copies a tile, swayed and lit live, or still and unlit inside a bake.
func (r *Renderer) drawGrassBatch(dst *ebiten.Image, w *game.WorldView, x0, y0, x1, y1 int, tile float64, sw, sh, limit int) {
	x0, y0, x1, y1 = r.grassReach(x0, y0, x1, y1)
	r.grassFrameMask(w, x0, y0, x1, y1)
	r.grassBatchOf(dst, w, x0, y0, x1, y1, tile, sw, sh, limit)
}

// grassReach pads a tile range by the tiles whose tufts reach into it (a
// column each side, a row north and south), clamped to the map.
func (r *Renderer) grassReach(x0, y0, x1, y1 int) (int, int, int, int) {
	return max(x0-1, 0), max(y0-1, 0), min(x1+1, r.game.Width()-1), min(y1+1, r.game.Height()-1)
}

// grassBatchOf is drawGrassBatch over an already padded range with the
// mask in place.
func (r *Renderer) grassBatchOf(dst *ebiten.Image, w *game.WorldView, x0, y0, x1, y1 int, tile float64, sw, sh, limit int) {
	gb := &r.lod.grass
	gb.quads = 0
	gb.verts, gb.idx = gb.verts[:0], gb.idx[:0]
	a := r.grassAtlas()
	if a == nil || w.TerrainW <= 0 || x1 < x0 || y1 < y0 {
		return
	}
	baking := r.lod.baking
	dormant := grassDormant(w)
	tint := grassTint(w.BiomeKey)
	if dormant && a.dry {
		tint = mulCS(tint, grassDryCS)
	}
	snowQ := grassSnowQ(r.env.snow)
	tl := float32(tile)
	fsw, fsh := float32(sw), float32(sh)
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			code, ok := terrainCode(*w, x, y)
			if !ok || !gb.grassAt(w, code, x, y) {
				continue
			}
			growth := grassGrowthAt(w, y*w.TerrainW+x)
			keep := grassKeep(gb.level, x, y, min(tuftCopies(growth), max(limit, 1)))
			if keep == 0 { // thinned away (grass detail)
				continue
			}
			lean, sy, alpha := 0.0, 1.0, 1.0
			sky, lamp := float32(1), float32(0)
			if baking {
				sy, alpha = r.tuftSnow(x, y, snowQ)
			} else {
				ps := r.plantPose(x, y, kindGrass, 0.05, 1)
				lean, sy = -ps.skew, ps.sy
				if ps.cs != (colorScale{}) {
					alpha = float64(ps.cs[3])
				}
				sky, lamp = r.lightTile(x, y)
			}
			if alpha < 1.0/255 {
				continue
			}
			a32 := float32(alpha)
			cr, cg, cb := tint[0]*tint[3]*a32, tint[1]*tint[3]*a32, tint[2]*tint[3]*a32
			if !baking && r.lit.off { // lighting off: the tile's flat light rides in the colour
				f := litFactor(&r.lit.light, [3]float32{0, 0, 1}, sky, lamp)
				cr, cg, cb = cr*f[0], cg*f[1], cb*f[2]
			}
			gb.copies = grassCopies(x, y, growth, keep, grassTallAt(code, x, y), gb.copies[:0])
			sortCopies(gb.copies)
			tx, ty := r.worldToScreenF(float64(x), float64(y))
			for i := range gb.copies {
				c := &gb.copies[i]
				set := a.short
				switch {
				case dormant:
					set = a.dormant
				case c.tall:
					set = a.tall
				}
				sp := &set[c.variant%uint32(len(set))]
				k := float32(c.scale) * tl * sp.k // screen px per atlas px
				fx, fy := tx+float32(c.dx)*tl, ty+float32(c.dy)*tl
				wq, hq := sp.w*k, sp.h*k*float32(sy)
				left := fx - sp.bx*k
				if c.mirror {
					left = fx - (sp.w-sp.bx)*k
				}
				top := fy - hq
				l := min(max(lean*c.sway, -grassLeanMax), grassLeanMax)
				shift := float32(l+l*l*l/3) * hq // tan(l), |l| ≤ 0.35
				if fy < 0 || top > fsh || min(left, left+shift)+wq < 0 || max(left, left+shift) > fsw {
					continue
				}
				u0, u1 := sp.sx, sp.sx+sp.w
				if c.mirror {
					u0, u1 = u1, u0
				}
				n := uint32(len(gb.verts))
				gb.verts = append(gb.verts,
					ebiten.Vertex{DstX: left + shift, DstY: top, SrcX: u0, SrcY: sp.sy, ColorR: cr, ColorG: cg, ColorB: cb, ColorA: a32, Custom0: sky, Custom1: lamp},
					ebiten.Vertex{DstX: left + shift + wq, DstY: top, SrcX: u1, SrcY: sp.sy, ColorR: cr, ColorG: cg, ColorB: cb, ColorA: a32, Custom0: sky, Custom1: lamp},
					ebiten.Vertex{DstX: left, DstY: fy, SrcX: u0, SrcY: sp.sy + sp.h, ColorR: cr, ColorG: cg, ColorB: cb, ColorA: a32, Custom0: sky, Custom1: lamp},
					ebiten.Vertex{DstX: left + wq, DstY: fy, SrcX: u1, SrcY: sp.sy + sp.h, ColorR: cr, ColorG: cg, ColorB: cb, ColorA: a32, Custom0: sky, Custom1: lamp})
				gb.idx = append(gb.idx, n, n+1, n+2, n+1, n+3, n+2)
			}
		}
	}
	gb.quads = len(gb.verts) / 4
	r.flushGrass(dst, a.img)
}

// sortCopies orders a tile's copies north to south (insertion sort: at
// most nine), so a nearer tuft overlaps the one behind it.
func sortCopies(cs []tuftCopy) {
	for i := 1; i < len(cs); i++ {
		for j := i; j > 0 && cs[j].dy < cs[j-1].dy; j-- {
			cs[j], cs[j-1] = cs[j-1], cs[j]
		}
	}
}

// flushGrass draws the batch: lit through lit.kage on screen, plain
// inside a bake (unlit albedo, lit when the bake is drawn).
func (r *Renderer) flushGrass(dst, src *ebiten.Image) {
	gb := &r.lod.grass
	if len(gb.idx) == 0 {
		return
	}
	if r.lod.baking {
		gb.baked++
		dst.DrawTriangles32(gb.verts, gb.idx, src, &ebiten.DrawTrianglesOptions{Filter: ebiten.FilterLinear})
		return
	}
	gb.flushes++
	if gb.onFlush != nil {
		gb.onFlush()
	}
	r.lit.calls++
	if r.lit.off { // lighting off: the vertex colours already carry the light
		dst.DrawTriangles32(gb.verts, gb.idx, src, &ebiten.DrawTrianglesOptions{Filter: ebiten.FilterLinear})
		return
	}
	sh := r.litShader()
	if sh == nil { // no shader: unlit
		dst.DrawTriangles32(gb.verts, gb.idx, src, &ebiten.DrawTrianglesOptions{Filter: ebiten.FilterLinear})
		return
	}
	r.lit.syncUniforms()
	r.lit.opts.Images[0] = src
	dst.DrawTrianglesShader32(gb.verts, gb.idx, sh, &r.lit.opts)
}

// grassSnowQ is the snow depth bakes use: quantised, so a slow snowfall
// rebakes a handful of times, not every frame.
func grassSnowQ(snow float64) float64 { return math.Round(clamp01(snow)*20) / 20 }

// tuftSnow is a still tuft's height factor and alpha under snow depth
// snow (plantPose's snow fade without the sway).
func (r *Renderer) tuftSnow(x, y int, snow float64) (sy, alpha float64) {
	if snow <= 0 || !r.env.outdoors(x, y) {
		return 1, 1
	}
	thr := 0.15 + 0.6*float64(tileHash(x, y)>>8&0xffff)/65536
	return 1 - 0.5*snow, 1 - clamp01((snow-thr)/0.2)
}

// drawGrassCutMarks draws the cut mark over each visible grass tile
// marked for cutting in x0..x1 × y0..y1 (mask in place).
func (r *Renderer) drawGrassCutMarks(dst *ebiten.Image, w *game.WorldView, x0, y0, x1, y1 int, tile float64, sw, sh int) {
	gb := &r.lod.grass
	for _, c := range w.GrassCut {
		x, y := c[0], c[1]
		if x < x0 || x > x1 || y < y0 || y > y1 {
			continue
		}
		code, ok := terrainCode(*w, x, y)
		if !ok || !gb.grassAt(w, code, x, y) {
			continue
		}
		if i := y*w.TerrainW + x; i < len(gb.mask) && gb.mask[i]&grassMarked != 0 {
			r.drawSprite(dst, "overlay/designation_cut", noTint, float64(x), float64(y), tile, sw, sh)
		}
	}
}

// --- baked tiers ------------------------------------------------------------------

// grassBakesTufts reports the tier whose chunk images hold the tufts.
func (r *Renderer) grassBakesTufts() bool {
	t := r.tilePx()
	return r.lod.tier == lodChunk && t < grassLiveMin
}

// grassChunkSig is the grass a chunk image of chunk (cx, cy) shows with
// its tufts baked: each grass tile of the chunk and the ring around it
// (tufts reach across), its copy bucket and tall look, plus the frame's
// tint, dormancy and snow; 0 when no grass shows. Computed every frame
// for the chunks drawn (a few dozen at this zoom), so it is exact.
func (r *Renderer) grassChunkSig(w *game.WorldView, cx, cy int) uint64 {
	cs := game.ChunkSize
	x0, y0, x1, y1 := r.grassReach(cx*cs, cy*cs, cx*cs+cs-1, cy*cs+cs-1)
	r.grassFrameMask(w, x0, y0, x1, y1)
	gb := &r.lod.grass
	var h uint64
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			code, ok := terrainCode(*w, x, y)
			if !ok || !gb.grassAt(w, code, x, y) {
				continue
			}
			n := uint64(tuftCopies(grassGrowthAt(w, y*w.TerrainW+x)))
			if grassTallAt(code, x, y) {
				n |= 16
			}
			h = mix64(h ^ uint64(y*w.TerrainW+x)<<5 ^ n)
		}
	}
	if h == 0 {
		return 0
	}
	return r.grassLookSig(w, h)
}

// grassLookSig folds the frame-wide grass look (biome tint, dormancy,
// quantised snow) into h.
func (r *Renderer) grassLookSig(w *game.WorldView, h uint64) uint64 {
	t := grassTint(w.BiomeKey)
	h = mix64(h ^ uint64(math.Float32bits(t[0]))<<32 ^ uint64(math.Float32bits(t[1])))
	h = mix64(h ^ uint64(math.Float32bits(t[2])))
	if grassDormant(w) {
		h = mix64(h ^ 0xd0)
	}
	if d := grassDensity(r.lod.grass.level); d < 1 { // the detail level thins the baked tufts
		h = mix64(h ^ uint64(d*100))
	}
	return mix64(h ^ uint64(grassSnowQ(r.env.snow)*20))
}

// grassBakeKey is chunk version ver with sig folded in (sig 0: no grass,
// the plain version).
func grassBakeKey(ver, sig uint64) uint64 {
	if sig == 0 {
		return ver
	}
	return ver ^ mix64(sig^0x9e3779b97f4a7c15)
}

// bakeTufts draws the tufts reaching into tiles x0..x1-1 × y0..y1-1 into
// dst, a chunk bake at px per tile with tile (x0, y0) at (ox, oy): still,
// unlit, at most grassMidCap copies a tile.
func (r *Renderer) bakeTufts(dst *ebiten.Image, w *game.WorldView, x0, y0, x1, y1 int, px, ox, oy float64) {
	r.withBake(dst, px, x0, y0, ox, oy, func(sw, sh int, tile float64) {
		r.drawGrassBatch(dst, w, x0, y0, x1-1, y1-1, tile, sw, sh, grassMidCap)
	})
}

// grassMapSig is chunk (cx, cy)'s grass tint signature for the map bake:
// grass tiles of the chunk and its ring by copy bucket (cover does not
// matter: buildings and plant marks draw over the tint). It is memoised
// on the chunk's ground version and the grass layers, and at most
// grassSigBudget chunks a frame recompute a stale memo.
func (r *Renderer) grassMapSig(w *game.WorldView, cx, cy int, ver uint64) uint64 {
	gb := &r.lod.grass
	if gb.sigs == nil {
		gb.sigs = map[int]grassMemo{}
	}
	if f := r.lod.chunks.frame; gb.frame != f {
		gb.frame, gb.budget = f, grassSigBudget
	}
	c := cy*((r.game.Width()+game.ChunkSize-1)/game.ChunkSize) + cx
	key := grassMemo{ver: ver, grassVer: w.GrassVer, grass: litFirst(w.Grass), growth: litFirst(w.GrassGrowth)}
	m, ok := gb.sigs[c]
	if ok {
		sig := m.sig
		m.sig = 0
		if m == key || gb.budget <= 0 {
			return sig
		}
		gb.budget--
	}
	cs := game.ChunkSize
	x0, y0, x1, y1 := r.grassReach(cx*cs, cy*cs, cx*cs+cs-1, cy*cs+cs-1)
	var h uint64
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if d := grassTintBucket(w, x, y); d != 0 {
				h = mix64(h ^ uint64(y*w.TerrainW+x)<<5 ^ uint64(d))
			}
		}
	}
	if h != 0 {
		t := grassTint(w.BiomeKey)
		h = mix64(h ^ uint64(math.Float32bits(t[0]))<<32 ^ uint64(math.Float32bits(t[1])))
		h = mix64(h ^ uint64(math.Float32bits(t[2])))
		if grassDormant(w) {
			h = mix64(h ^ 0xd0)
		}
	}
	key.sig = h
	gb.sigs[c] = key
	return h
}

// grassTintBucket is tile (x, y)'s grass for the map tint: its copy
// count (1..9) on grass ground with grass, else 0.
func grassTintBucket(w *game.WorldView, x, y int) int {
	code, ok := terrainCode(*w, x, y)
	if !ok || !grassGroundCode[code] {
		return 0
	}
	i := y*w.TerrainW + x
	if len(w.Grass) == 0 {
		if (tileHash(x, y)>>8)%grassEvery != 0 {
			return 0
		}
	} else if i >= len(w.Grass) || w.Grass[i] != 1 {
		return 0
	}
	return tuftCopies(grassGrowthAt(w, i))
}

// grassTintRGB is the map tint's base colour (grass green, ART §9.1
// #708052, a touch darker so it reads as cover, not paint).
var grassTintRGB = [3]float32{0.40, 0.47, 0.29}

// bakeGrassTint draws the soft grass tint of tiles x0..x1-1 × y0..y1-1
// into the map bake (inside withBake, after the ground): one quad a tile,
// each corner's alpha the mean of the four tiles meeting there, so the
// tint fades across tile edges.
func (r *Renderer) bakeGrassTint(dst *ebiten.Image, w *game.WorldView, x0, y0, x1, y1 int, tile float64) {
	gb := &r.lod.grass
	gb.verts, gb.idx = gb.verts[:0], gb.idx[:0]
	dens := func(x, y int) float32 {
		if n := grassTintBucket(w, x, y); n > 0 {
			return 0.15 + 0.035*float32(n)
		}
		return 0
	}
	t := grassTint(w.BiomeKey)
	if grassDormant(w) {
		t = mulCS(t, grassDryCS)
	}
	cr, cg, cb := grassTintRGB[0]*t[0], grassTintRGB[1]*t[1], grassTintRGB[2]*t[2]
	corner := func(x, y int) float32 { // the corner at the top-left of tile (x, y)
		return (dens(x-1, y-1) + dens(x, y-1) + dens(x-1, y) + dens(x, y)) / 4
	}
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			a := [4]float32{corner(x, y), corner(x+1, y), corner(x, y+1), corner(x+1, y+1)}
			if a == ([4]float32{}) {
				continue
			}
			sx, sy := r.worldToScreenF(float64(x), float64(y))
			n := uint32(len(gb.verts))
			for k := 0; k < 4; k++ {
				dx, dy := sx, sy
				if k&1 != 0 {
					dx += float32(tile)
				}
				if k&2 != 0 {
					dy += float32(tile)
				}
				gb.verts = append(gb.verts, ebiten.Vertex{DstX: dx, DstY: dy, SrcX: 0.5, SrcY: 0.5,
					ColorR: cr * a[k], ColorG: cg * a[k], ColorB: cb * a[k], ColorA: a[k]})
			}
			gb.idx = append(gb.idx, n, n+1, n+2, n+1, n+3, n+2)
		}
	}
	if len(gb.idx) == 0 {
		return
	}
	gb.tinted++
	dst.DrawTriangles32(gb.verts, gb.idx, r.lod.whiteImg(), &ebiten.DrawTrianglesOptions{})
}
