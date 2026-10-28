package env

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"slices"
	"strconv"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/light"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
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
//	24–48 px  at most GrassMidCap copies a tile, live sway;
//	16–24 px  GrassMidCap copies baked into the chunk images (no sway);
//	          the bake's key folds in the chunk's grass (GrassChunkSig);
//	< 16 px   a soft grass tint baked into the map image (GrassMapSig).

const (
	// GrassCellJitter is how far a copy may sit from its grid cell's
	// centre, in tiles.
	GrassCellJitter = 0.18
	// GrassEdgeSpill is how far past the tile edge an edge copy may sit.
	GrassEdgeSpill = 0.15
	// grassLeanMax is the grass lean limit (ART §27.3), radians.
	grassLeanMax = 0.35
	// GrassLiveMin is the tile size (px) from which tufts are drawn live;
	// below it the chunk tier bakes them.
	GrassLiveMin = 24.0
	// GrassMidCap is the copies a tile draws below LiveMin.
	GrassMidCap = 4
	// tuftRefH is a short tuft's height at scale 1, in tiles: the atlas
	// sizes every tuft sprite by the short set's median height, so the
	// composition holds whatever canvas the art is rendered at.
	tuftRefH = 0.55
	// grassTallNoise is the ValueNoise level above which a soil tile lies
	// in a tall-grass patch.
	grassTallNoise = 0.64
	// grassCoverReach is how far up and left of a tile a building or frame
	// anchor can lie and still cover it.
	grassCoverReach = VisPad
	// grassSigBudget is how many map-tier chunk signatures a frame may
	// recompute after the grass changed (the rest show a frame later).
	grassSigBudget = 64
)

// GrassGroundCode marks the terrains the simulation seeds grass on: any
// fertile ground (plant.GrassGround outside the legacy map; pinned by
// TestGrassMatchesRendererRule).
var GrassGroundCode = func() (out [256]bool) {
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

// TuftCopies is how many tuft copies a grass tile of this growth draws.
func TuftCopies(growth uint8) int {
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

// TuftCopy is one tuft copy of a tile: its foot at (dx, dy) tiles from
// the tile's top-left, its scale, its share of the tile's sway, the
// variant it draws, mirrored or not, from the tall set or not.
type TuftCopy struct {
	Dx, Dy, Scale, sway float64
	Variant             uint32
	Mirror, Tall        bool
}

// GrassCopies appends tile (x, y)'s tuft copies at this growth, at most
// limit of them, to out: a pure function of the tile. tallTile marks a
// tile in a tall patch or on rich soil (GrassTallAt): two copies in three
// draw tall there.
func GrassCopies(x, y int, growth uint8, limit int, tallTile bool, out []TuftCopy) []TuftCopy {
	n := min(TuftCopies(growth), max(limit, 1))
	order := tuftOrder[n]
	if order == nil {
		order = tuftOrder[9][:n]
	}
	th := sprite.TileHash(x, y)
	sym := art.Mix64(th^0x67726173) & 7
	size := tuftGrowthSize(TuftCopies(growth))
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
		h := art.Mix64(th ^ uint64(k+1)*0x9E3779B97F4A7C15 ^ 0x7475667473)
		u := func(shift uint) float64 { return float64(h>>shift&0xffff) / 65535 }
		c := TuftCopy{
			Dx:      tuftAxis(cx, u(0)),
			Dy:      tuftAxis(cy, u(16)),
			sway:    0.8 + 0.4*u(32),
			Variant: uint32(h >> 49),
			Mirror:  h>>48&1 != 0,
			Tall:    tallTile && (h>>60)%3 != 0,
		}
		s := u(40)
		if c.Tall {
			c.Scale = (0.7 + 0.2*s) * size
		} else {
			c.Scale = (0.45 + 0.2*s) * size
		}
		out = append(out, c)
	}
	return out
}

// tuftAxis is a copy's position along one axis: its cell's centre ±
// GrassCellJitter, an edge cell reaching up to GrassEdgeSpill past the
// tile edge; u in [0, 1].
func tuftAxis(cell int, u float64) float64 {
	c := (float64(cell) + 0.5) / 3
	lo, hi := c-GrassCellJitter, c+GrassCellJitter
	if cell == 0 {
		lo = -GrassEdgeSpill
	}
	if cell == 2 {
		hi = 1 + GrassEdgeSpill
	}
	return lo + (hi-lo)*u
}

// GrassTallAt reports a tile whose grass grows tall: rich soil, or a
// smooth ValueNoise patch (so tall grass clusters, never checkerboards).
func GrassTallAt(code uint8, x, y int) bool {
	if int(code) < len(terrainNames) && terrainNames[code] == "rich_soil" {
		return true
	}
	return ValueNoise(float64(x)/6, float64(y)/6, 0x7a11) > grassTallNoise
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
// the frame's mask (GrassFrameMask); with no simulation grass data, the
// old hashed rule.
func (gb *GrassBatch) grassAt(w *game.WorldView, code uint8, x, y int) bool {
	if !GrassGroundCode[code] {
		return false
	}
	if len(w.Grass) == 0 {
		return (sprite.TileHash(x, y)>>8)%GrassEvery == 0
	}
	i := y*w.TerrainW + x
	return i >= 0 && i < len(w.Grass) && i < len(gb.mask) && w.Grass[i] == 1 && gb.mask[i]&GrassCovered == 0
}

// --- the tuft atlas -----------------------------------------------------------

// The tuft sprites (PLAY-010).
var (
	GrassShortKeys   = vKeys("plant/grass_short_v", 6)
	GrassTallKeys    = vKeys("plant/grass_tall_v", 4)
	GrassDormantKeys = vKeys("plant/grass_short_dormant_v", 3)
)

func vKeys(prefix string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = prefix + strconv.Itoa(i)
	}
	return out
}

// grassDryCS dries the short tufts into straw when no dormant art exists.
var grassDryCS = sprite.ColorScale{1.12, 1.0, 0.62, 1}

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
	art                  *art.Catalog
	img                  *ebiten.Image
	short, tall, dormant []tuftSprite
	dry                  bool // dormant is the short set, dried by grassDryCS
}

// grassAtlasPad is the transparent margin around each packed tuft (the
// shader's bilinear taps reach half a pixel out).
const grassAtlasPad = 2

// buildGrassAtlas packs c's tuft sprites (alpha-trimmed) into one image.
// No tuft art: an atlas without sprites (nothing is drawn).
func buildGrassAtlas(c *art.Catalog) *grassAtlas {
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
	sets := [3][]crop{measure(pick(GrassShortKeys, nil)), measure(pick(GrassTallKeys, nil)), measure(pick(GrassDormantKeys, nil))}
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
		hs[i] = float64(cr.r.Dy()) * cr.k / art.TilePx
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
				bx: float32(cr.r.Dx()) / 2, k: float32(cr.k / art.TilePx * norm)}
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

// GrassBatch is the renderer's grass state: the atlas, the frame's grass
// mask and reused vertex buffers, the map-tier signature memo, and
// counters the tests read.
type GrassBatch struct {
	atlas  *grassAtlas
	mask   []uint8 // GrassMask bits per tile (grass_draw.go)
	Verts  []ebiten.Vertex
	idx    []uint32
	copies []TuftCopy
	sigs   map[int]grassMemo
	budget int // map-tier signature recomputes left this frame
	frame  uint64
	// counters (tests): Quads in the last batch, live batches drawn,
	// tuft bakes into chunk images and grass-tint bakes into the map.
	Quads, Flushes, Baked, Tinted int
	// OnFlush, when set (tests), runs as a live batch is drawn.
	OnFlush func()
	// Level is the grass detail (Options > Graphics, PLAY-019): the empty
	// string or config.GrassFull draws every tuft, GrassReduced and
	// GrassSparse thin them per tile (GrassKeep).
	Level string
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

// GrassKeep is how many of n tufts tile (x, y) keeps at detail level: n
// times the level's density, the fraction rounded up or down by the tile's
// own hash. It is a pure function of the tile, so a tile thins the same
// way every frame and in every bake, and a sparser level only ever keeps
// fewer tufts than a denser one.
func GrassKeep(level string, x, y, n int) int {
	d := grassDensity(level)
	if d >= 1 || n <= 0 {
		return n
	}
	f := float64(n) * d
	k := int(f)
	if u := float64(art.Mix64(sprite.TileHash(x, y)^0x7468696e)>>11) / (1 << 53); u < f-float64(k) {
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

// atlasFor is the tuft atlas of art c (built on first use).
func (gb *GrassBatch) atlasFor(c *art.Catalog) *grassAtlas {
	if c == nil {
		return nil
	}
	if gb.atlas == nil || gb.atlas.art != c {
		gb.atlas = buildGrassAtlas(c)
	}
	if gb.atlas.img == nil {
		return nil
	}
	return gb.atlas
}

// DrawGrassBatch draws the tufts reaching into tiles x0..x1 × y0..y1 as
// one triangle batch (the frame's mask filled first): at most limit
// copies a tile, swayed and lit live, or still and unlit inside a bake
// (baking). c is the chunk index the mask reads.
func (e *Layer) DrawGrassBatch(v *render.View, dst *ebiten.Image, w *game.WorldView, x0, y0, x1, y1 int, tile float64, sw, sh, limit int, baking bool, c Chunks) {
	x0, y0, x1, y1 = e.GrassReach(v, x0, y0, x1, y1)
	e.GrassFrameMask(w, x0, y0, x1, y1, c)
	e.GrassBatchOf(v, dst, w, x0, y0, x1, y1, tile, sw, sh, limit, baking)
}

// GrassReach pads a tile range by the tiles whose tufts reach into it (a
// column each side, a row north and south), clamped to the map.
func (e *Layer) GrassReach(v *render.View, x0, y0, x1, y1 int) (int, int, int, int) {
	return max(x0-1, 0), max(y0-1, 0), min(x1+1, v.MapW-1), min(y1+1, v.MapH-1)
}

// GrassBatchOf is DrawGrassBatch over an already padded range with the
// mask in place.
func (e *Layer) GrassBatchOf(v *render.View, dst *ebiten.Image, w *game.WorldView, x0, y0, x1, y1 int, tile float64, sw, sh, limit int, baking bool) {
	gb := e.Grass
	gb.Quads = 0
	gb.Verts, gb.idx = gb.Verts[:0], gb.idx[:0]
	a := gb.atlasFor(v.Art)
	if a == nil || w.TerrainW <= 0 || x1 < x0 || y1 < y0 {
		return
	}
	dormant := grassDormant(w)
	tint := sprite.GrassTint(w.BiomeKey)
	if dormant && a.dry {
		tint = sprite.MulCS(tint, grassDryCS)
	}
	snowQ := grassSnowQ(e.Snow)
	tl := float32(tile)
	fsw, fsh := float32(sw), float32(sh)
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			code, ok := TerrainCode(*w, x, y)
			if !ok || !gb.grassAt(w, code, x, y) {
				continue
			}
			growth := grassGrowthAt(w, y*w.TerrainW+x)
			keep := GrassKeep(gb.Level, x, y, min(TuftCopies(growth), max(limit, 1)))
			if keep == 0 { // thinned away (grass detail)
				continue
			}
			lean, sy, alpha := 0.0, 1.0, 1.0
			sky, lamp := float32(1), float32(0)
			if baking {
				sy, alpha = e.tuftSnow(x, y, snowQ)
			} else {
				ps := e.PlantPose(v, x, y, KindGrass, 0.05, 1)
				lean, sy = -ps.Skew, ps.Sy
				if ps.Cs != (sprite.ColorScale{}) {
					alpha = float64(ps.Cs[3])
				}
				sky, lamp = e.Lit.LightTile(v, x, y)
			}
			if alpha < 1.0/255 {
				continue
			}
			a32 := float32(alpha)
			cr, cg, cb := tint[0]*tint[3]*a32, tint[1]*tint[3]*a32, tint[2]*tint[3]*a32
			if !baking && e.Lit.Off { // lighting off: the tile's flat light rides in the colour
				f := light.LitFactor(&e.Lit.Light, [3]float32{0, 0, 1}, sky, lamp)
				cr, cg, cb = cr*f[0], cg*f[1], cb*f[2]
			}
			gb.copies = GrassCopies(x, y, growth, keep, GrassTallAt(code, x, y), gb.copies[:0])
			sortCopies(gb.copies)
			tx, ty := v.WorldToScreenF(float64(x), float64(y))
			for i := range gb.copies {
				c := &gb.copies[i]
				set := a.short
				switch {
				case dormant:
					set = a.dormant
				case c.Tall:
					set = a.tall
				}
				sp := &set[c.Variant%uint32(len(set))]
				k := float32(c.Scale) * tl * sp.k // screen px per atlas px
				fx, fy := tx+float32(c.Dx)*tl, ty+float32(c.Dy)*tl
				wq, hq := sp.w*k, sp.h*k*float32(sy)
				left := fx - sp.bx*k
				if c.Mirror {
					left = fx - (sp.w-sp.bx)*k
				}
				top := fy - hq
				l := min(max(lean*c.sway, -grassLeanMax), grassLeanMax)
				shift := float32(l+l*l*l/3) * hq // tan(l), |l| ≤ 0.35
				if fy < 0 || top > fsh || min(left, left+shift)+wq < 0 || max(left, left+shift) > fsw {
					continue
				}
				u0, u1 := sp.sx, sp.sx+sp.w
				if c.Mirror {
					u0, u1 = u1, u0
				}
				n := uint32(len(gb.Verts))
				gb.Verts = append(gb.Verts,
					ebiten.Vertex{DstX: left + shift, DstY: top, SrcX: u0, SrcY: sp.sy, ColorR: cr, ColorG: cg, ColorB: cb, ColorA: a32, Custom0: sky, Custom1: lamp},
					ebiten.Vertex{DstX: left + shift + wq, DstY: top, SrcX: u1, SrcY: sp.sy, ColorR: cr, ColorG: cg, ColorB: cb, ColorA: a32, Custom0: sky, Custom1: lamp},
					ebiten.Vertex{DstX: left, DstY: fy, SrcX: u0, SrcY: sp.sy + sp.h, ColorR: cr, ColorG: cg, ColorB: cb, ColorA: a32, Custom0: sky, Custom1: lamp},
					ebiten.Vertex{DstX: left + wq, DstY: fy, SrcX: u1, SrcY: sp.sy + sp.h, ColorR: cr, ColorG: cg, ColorB: cb, ColorA: a32, Custom0: sky, Custom1: lamp})
				gb.idx = append(gb.idx, n, n+1, n+2, n+1, n+3, n+2)
			}
		}
	}
	gb.Quads = len(gb.Verts) / 4
	e.flushGrass(dst, a.img, baking)
}

// sortCopies orders a tile's copies north to south (insertion sort: at
// most nine), so a nearer tuft overlaps the one behind it.
func sortCopies(cs []TuftCopy) {
	for i := 1; i < len(cs); i++ {
		for j := i; j > 0 && cs[j].Dy < cs[j-1].Dy; j-- {
			cs[j], cs[j-1] = cs[j-1], cs[j]
		}
	}
}

// flushGrass draws the batch: lit through lit.kage on screen, plain
// inside a bake (unlit albedo, lit when the bake is drawn).
func (e *Layer) flushGrass(dst, src *ebiten.Image, baking bool) {
	gb := e.Grass
	if len(gb.idx) == 0 {
		return
	}
	if baking {
		gb.Baked++
		dst.DrawTriangles32(gb.Verts, gb.idx, src, &ebiten.DrawTrianglesOptions{Filter: ebiten.FilterLinear})
		return
	}
	gb.Flushes++
	if gb.OnFlush != nil {
		gb.OnFlush()
	}
	e.Lit.Calls++
	if e.Lit.Off { // lighting off: the vertex colours already carry the light
		dst.DrawTriangles32(gb.Verts, gb.idx, src, &ebiten.DrawTrianglesOptions{Filter: ebiten.FilterLinear})
		return
	}
	sh := e.Lit.LitShader()
	if sh == nil { // no shader: unlit
		dst.DrawTriangles32(gb.Verts, gb.idx, src, &ebiten.DrawTrianglesOptions{Filter: ebiten.FilterLinear})
		return
	}
	e.Lit.SyncUniforms()
	e.Lit.Opts.Images[0] = src
	dst.DrawTrianglesShader32(gb.Verts, gb.idx, sh, &e.Lit.Opts)
}

// grassSnowQ is the snow depth bakes use: quantised, so a slow snowfall
// rebakes a handful of times, not every frame.
func grassSnowQ(snow float64) float64 { return math.Round(gfx.Clamp01(snow)*20) / 20 }

// tuftSnow is a still tuft's height factor and alpha under snow depth
// snow (PlantPose's snow fade without the sway).
func (e *Sky) tuftSnow(x, y int, snow float64) (sy, alpha float64) {
	if snow <= 0 || !e.outdoors(x, y) {
		return 1, 1
	}
	thr := 0.15 + 0.6*float64(sprite.TileHash(x, y)>>8&0xffff)/65536
	return 1 - 0.5*snow, 1 - gfx.Clamp01((snow-thr)/0.2)
}

// DrawGrassCutMarks draws the cut mark over each visible grass tile
// marked for cutting in x0..x1 × y0..y1 (mask in place), in sprite pass ps.
func (e *Layer) DrawGrassCutMarks(v *render.View, ps sprite.Pass, dst *ebiten.Image, w *game.WorldView, x0, y0, x1, y1 int, tile float64, sw, sh int) {
	gb := e.Grass
	for _, c := range w.GrassCut {
		x, y := c[0], c[1]
		if x < x0 || x > x1 || y < y0 || y > y1 {
			continue
		}
		code, ok := TerrainCode(*w, x, y)
		if !ok || !gb.grassAt(w, code, x, y) {
			continue
		}
		if i := y*w.TerrainW + x; i < len(gb.mask) && gb.mask[i]&GrassMarked != 0 {
			e.P.Draw(v, ps, dst, "overlay/designation_cut", color.NRGBA{}, float64(x), float64(y), tile, sw, sh)
		}
	}
}

// --- baked tiers ------------------------------------------------------------------

// GrassChunkSig is the grass a chunk image of chunk (cx, cy) shows with
// its tufts baked: each grass tile of the chunk and the ring around it
// (tufts reach across), its copy bucket and tall look, plus the frame's
// tint, dormancy and snow; 0 when no grass shows. Computed every frame
// for the chunks drawn (a few dozen at this zoom), so it is exact.
func (e *Layer) GrassChunkSig(v *render.View, w *game.WorldView, cx, cy int, c Chunks) uint64 {
	cs := game.ChunkSize
	x0, y0, x1, y1 := e.GrassReach(v, cx*cs, cy*cs, cx*cs+cs-1, cy*cs+cs-1)
	e.GrassFrameMask(w, x0, y0, x1, y1, c)
	gb := e.Grass
	var h uint64
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			code, ok := TerrainCode(*w, x, y)
			if !ok || !gb.grassAt(w, code, x, y) {
				continue
			}
			n := uint64(TuftCopies(grassGrowthAt(w, y*w.TerrainW+x)))
			if GrassTallAt(code, x, y) {
				n |= 16
			}
			h = art.Mix64(h ^ uint64(y*w.TerrainW+x)<<5 ^ n)
		}
	}
	if h == 0 {
		return 0
	}
	return e.GrassLookSig(w, h)
}

// GrassLookSig folds the frame-wide grass look (biome tint, dormancy,
// quantised snow) into h.
func (e *Layer) GrassLookSig(w *game.WorldView, h uint64) uint64 {
	t := sprite.GrassTint(w.BiomeKey)
	h = art.Mix64(h ^ uint64(math.Float32bits(t[0]))<<32 ^ uint64(math.Float32bits(t[1])))
	h = art.Mix64(h ^ uint64(math.Float32bits(t[2])))
	if grassDormant(w) {
		h = art.Mix64(h ^ 0xd0)
	}
	if d := grassDensity(e.Grass.Level); d < 1 { // the detail level thins the baked tufts
		h = art.Mix64(h ^ uint64(d*100))
	}
	return art.Mix64(h ^ uint64(grassSnowQ(e.Snow)*20))
}

// GrassBakeKey is chunk version ver with sig folded in (sig 0: no grass,
// the plain version).
func GrassBakeKey(ver, sig uint64) uint64 {
	if sig == 0 {
		return ver
	}
	return ver ^ art.Mix64(sig^0x9e3779b97f4a7c15)
}

// GrassMapSig is chunk (cx, cy)'s grass tint signature for the map bake:
// grass tiles of the chunk and its ring by copy bucket (cover does not
// matter: buildings and plant marks draw over the tint). It is memoised
// on the chunk's ground version and the grass layers, and at most
// grassSigBudget chunks a frame (the chunk cache's frame counter) recompute
// a stale memo.
func (e *Layer) GrassMapSig(v *render.View, w *game.WorldView, cx, cy int, ver, frame uint64) uint64 {
	gb := e.Grass
	if gb.sigs == nil {
		gb.sigs = map[int]grassMemo{}
	}
	if gb.frame != frame {
		gb.frame, gb.budget = frame, grassSigBudget
	}
	c := cy*((v.MapW+game.ChunkSize-1)/game.ChunkSize) + cx
	key := grassMemo{ver: ver, grassVer: w.GrassVer, grass: light.First(w.Grass), growth: light.First(w.GrassGrowth)}
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
	x0, y0, x1, y1 := e.GrassReach(v, cx*cs, cy*cs, cx*cs+cs-1, cy*cs+cs-1)
	var h uint64
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if d := grassTintBucket(w, x, y); d != 0 {
				h = art.Mix64(h ^ uint64(y*w.TerrainW+x)<<5 ^ uint64(d))
			}
		}
	}
	if h != 0 {
		t := sprite.GrassTint(w.BiomeKey)
		h = art.Mix64(h ^ uint64(math.Float32bits(t[0]))<<32 ^ uint64(math.Float32bits(t[1])))
		h = art.Mix64(h ^ uint64(math.Float32bits(t[2])))
		if grassDormant(w) {
			h = art.Mix64(h ^ 0xd0)
		}
	}
	key.sig = h
	gb.sigs[c] = key
	return h
}

// grassTintBucket is tile (x, y)'s grass for the map tint: its copy
// count (1..9) on grass ground with grass, else 0.
func grassTintBucket(w *game.WorldView, x, y int) int {
	code, ok := TerrainCode(*w, x, y)
	if !ok || !GrassGroundCode[code] {
		return 0
	}
	i := y*w.TerrainW + x
	if len(w.Grass) == 0 {
		if (sprite.TileHash(x, y)>>8)%GrassEvery != 0 {
			return 0
		}
	} else if i >= len(w.Grass) || w.Grass[i] != 1 {
		return 0
	}
	return TuftCopies(grassGrowthAt(w, i))
}

// grassTintRGB is the map tint's base colour (grass green, ART §9.1
// #708052, a touch darker so it reads as cover, not paint).
var grassTintRGB = [3]float32{0.40, 0.47, 0.29}

// BakeGrassTint draws the soft grass tint of tiles x0..x1-1 × y0..y1-1
// into the map bake (inside withBake, after the ground): one quad a tile,
// each corner's alpha the mean of the four tiles meeting there, so the
// tint fades across tile edges. white is the bake's white source image.
func (e *Layer) BakeGrassTint(v *render.View, dst *ebiten.Image, w *game.WorldView, x0, y0, x1, y1 int, tile float64, white *ebiten.Image) {
	gb := e.Grass
	gb.Verts, gb.idx = gb.Verts[:0], gb.idx[:0]
	dens := func(x, y int) float32 {
		if n := grassTintBucket(w, x, y); n > 0 {
			return 0.15 + 0.035*float32(n)
		}
		return 0
	}
	t := sprite.GrassTint(w.BiomeKey)
	if grassDormant(w) {
		t = sprite.MulCS(t, grassDryCS)
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
			sx, sy := v.WorldToScreenF(float64(x), float64(y))
			n := uint32(len(gb.Verts))
			for k := 0; k < 4; k++ {
				dx, dy := sx, sy
				if k&1 != 0 {
					dx += float32(tile)
				}
				if k&2 != 0 {
					dy += float32(tile)
				}
				gb.Verts = append(gb.Verts, ebiten.Vertex{DstX: dx, DstY: dy, SrcX: 0.5, SrcY: 0.5,
					ColorR: cr * a[k], ColorG: cg * a[k], ColorB: cb * a[k], ColorA: a[k]})
			}
			gb.idx = append(gb.idx, n, n+1, n+2, n+1, n+3, n+2)
		}
	}
	if len(gb.idx) == 0 {
		return
	}
	gb.Tinted++
	dst.DrawTriangles32(gb.Verts, gb.idx, white, &ebiten.DrawTrianglesOptions{})
}
