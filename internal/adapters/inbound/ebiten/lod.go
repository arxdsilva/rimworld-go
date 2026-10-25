package ebiten

import (
	"image/color"
	"math"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// Level of detail by on-screen tile size (NG-1 §6.7 E5):
//
//	≥ 48 px   live: every tile and object drawn as today;
//	16–48 px  chunk: each chunk's ground (terrain, edges, water, smoothed
//	          stone, rock, floors, carpets) is a baked 16 px/tile image
//	          (chunk_cache.go), drawn lit per tile; objects stay live;
//	< 16 px   map: one whole-map bake at 4 px/tile in ≤ 4096-px pages,
//	          ground, a soft grass tint, buildings and plants as marks; no
//	          shadows, tufts, decals or item sprites; creatures keep a
//	          minimum size and selection rings stay.
//
// Grass tufts (grass_batch.go) are live from 24 px (at most four a tile
// below 48 px) and baked into the chunk images from 16 to 24 px; the
// bake keys then fold in the chunk's grass (grassChunkSig, grassMapSig).
//
// Bakes are unlit albedo; drawBaked lights them per tile through lit.kage
// (flat, as the ground is lit live), so day, night, roofs and lamps read
// the same at every zoom. A bake is redone when its chunk's version
// (game.ChunkView.Ver, plus PlantVer for the map bake) moves, a few per
// frame (bakeBudget); a chunk not baked yet is drawn live meanwhile.

type lodTier uint8

const (
	lodLive lodTier = iota
	lodChunk
	lodMap
)

const (
	lodLiveMin  = 48.0 // px per tile: live from here up
	lodChunkMin = 16.0 // px per tile: chunk bakes from here up, the map bake below
	// chunkBakePx and mapBakePx are the bakes' pixels per tile.
	chunkBakePx = 16
	mapBakePx   = 4
	// bakePad is the margin baked around a chunk image, so bilinear taps
	// at its edge read the neighbouring ground.
	bakePad = 1
	// decalMinTile: ground-cover decals (snow, puddles) need at least this
	// tile size; below it the smooth veil alone shows.
	decalMinTile = 32.0
	// creatureMinTile is the smallest tile size creatures are drawn at.
	creatureMinTile = 16.0
)

// lodFor is the tier for an on-screen tile size.
func lodFor(tile float64) lodTier {
	switch {
	case tile >= lodLiveMin:
		return lodLive
	case tile >= lodChunkMin:
		return lodChunk
	}
	return lodMap
}

// lodState is the renderer's LOD and per-frame chunk state.
type lodState struct {
	tier lodTier
	// vis is this frame's visible tile range (inclusive) and chunks the
	// chunk views over it padded by visPad tiles (cx0..cx1 × cy0..cy1).
	vx0, vy0, vx1, vy1 int
	viewSet            bool // a frame has set vx0..vy1 (inView)
	cx0, cy0, cx1, cy1 int
	views              []game.ChunkView
	chunks             chunkCache
	pages              mapBake
	// reused per-frame buffers (draw_art.go)
	occupied, creatures map[[2]int]bool
	planned             map[[2]int]linkFlag
	cloth               []string // colonyCloths, once a frame
	bakeDraws           []artDraw
	verts               []ebiten.Vertex
	idx                 []uint32
	white               *ebiten.Image
	rectOp              ebiten.DrawImageOptions // solidRect's, reused
	baking              bool                    // drawLit draws unlit albedo (a bake)
	bakeCast            shadowCast
	grass               grassBatch // grass_batch.go: tuft atlas, mask, batch buffers
	// budget is how many chunk bakes this frame may still do.
	budget int
}

// visPad is how far past the visible tiles the per-frame chunk lists
// reach: tall sprites (trees, walls) south of the view and the longest
// shadows (shadowMaxLen) from any side.
const visPad = 5

// bakeBudget is how many chunk bakes a frame may do (ground of 256 tiles
// each); the rest wait for the next frames.
const bakeBudget = 24

// syncFrameChunks fetches this frame's chunk views over the visible tiles
// (padded); the object, floor, grass and shadow passes iterate only them.
func (r *Renderer) syncFrameChunks() {
	l := &r.lod
	x0, y0, x1, y1 := r.visibleTiles()
	l.vx0, l.vy0, l.vx1, l.vy1, l.viewSet = x0, y0, x1, y1, true
	w, h := r.game.Width(), r.game.Height()
	l.views = l.views[:0]
	if w <= 0 || h <= 0 || x1 < x0 || y1 < y0 {
		l.cx0, l.cy0, l.cx1, l.cy1 = 0, 0, -1, -1
		return
	}
	cs := game.ChunkSize
	l.cx0, l.cy0 = max(x0-visPad, 0)/cs, max(y0-visPad, 0)/cs
	l.cx1, l.cy1 = min(x1+visPad, w-1)/cs, min(y1+visPad, h-1)/cs
	for cy := l.cy0; cy <= l.cy1; cy++ {
		for cx := l.cx0; cx <= l.cx1; cx++ {
			l.views = append(l.views, r.game.ChunkLayer(cx, cy))
		}
	}
}

// plantsOn appends the indexes into w.Plants of the plants on tiles
// x0..x1 × y0..y1: from the simulation's chunk read model when w is its
// current world (the same shared list), else by scanning w (a world
// handed in directly, as tests do).
func (r *Renderer) plantsOn(w *game.WorldView, x0, y0, x1, y1 int, buf []int32) []int32 {
	if r.game != nil {
		if cur := r.game.World(); litFirst(cur.Plants) == litFirst(w.Plants) && len(cur.Plants) == len(w.Plants) {
			return r.game.PlantsIn(x0, y0, x1, y1, buf)
		}
	}
	for i := range w.Plants {
		if p := &w.Plants[i]; p.X >= x0 && p.X <= x1 && p.Y >= y0 && p.Y <= y1 {
			buf = append(buf, int32(i))
		}
	}
	return buf
}

// near reports whether tile (x, y) is within pad tiles of the view (south
// by padS, the other sides by pad).
func (l *lodState) near(x, y, pad, padS int) bool {
	return x >= l.vx0-pad && x <= l.vx1+pad && y >= l.vy0-pad && y <= l.vy1+padS
}

// inView is near once a frame has set the view (syncFrameChunks); before
// that (a pass drawn on its own, as tests do) every tile counts.
func (l *lodState) inView(x, y, pad, padS int) bool {
	return !l.viewSet || l.near(x, y, pad, padS)
}

// withBake runs fn with the camera, culling and lighting set up to draw
// the world into a bake: px pixels per tile with tile (x0, y0) at pixel
// (ox, oy) of dst's coordinate space, unlit, the water as its baked
// frame. sw and sh are dst's extent for culling.
func (r *Renderer) withBake(dst *ebiten.Image, px float64, x0, y0 int, ox, oy float64, fn func(sw, sh int, tile float64)) {
	cam, view, cast := r.cam, r.env.view, r.cast
	r.cam = camera{zoom: px / TileSize, offsetX: float64(x0)*px - ox, offsetY: float64(y0)*px - oy, placed: true}
	b := dst.Bounds()
	r.setView(b.Max.X, b.Max.Y, px)
	r.lod.baking = true
	r.cast = &r.lod.bakeCast // water_draw.go draws its baked frame, not the shader
	defer func() {
		r.cam, r.env.view, r.cast = cam, view, cast
		r.lod.baking = false
	}()
	fn(b.Max.X, b.Max.Y, px)
}

// drawUnlit is drawLit while baking: the plain sprite, colour-scaled.
func (r *Renderer) drawUnlit(dst, img *ebiten.Image, geo ebiten.GeoM, cs colorScale) {
	op := &ebiten.DrawImageOptions{GeoM: geo, Filter: ebiten.FilterLinear}
	if cs != noCS && cs != (colorScale{}) {
		op.ColorScale.Scale(cs[0]*cs[3], cs[1]*cs[3], cs[2]*cs[3], cs[3])
	}
	dst.DrawImage(img, op)
}

// bakeGround draws the ground of tiles x0..x1-1 × y0..y1-1 (and the ring
// around them that dst's clip lets show) into dst at px per tile, tile
// (x0, y0) at (ox, oy): terrain, edges and water, floors, carpets and
// sleeping spots, then natural rock in row order; with objects (the map
// bake) also buildings, frames' floors and plants as marks.
func (r *Renderer) bakeGround(dst *ebiten.Image, w *game.WorldView, x0, y0, x1, y1 int, px, ox, oy float64, objects bool) {
	r.withBake(dst, px, x0, y0, ox, oy, func(sw, sh int, tile float64) {
		mw, mh := r.game.Width(), r.game.Height()
		gx0, gy0 := max(x0-1, 0), max(y0-1, 0)
		gx1, gy1 := min(x1, mw-1), min(y1, mh-1)
		for y := gy0; y <= gy1; y++ {
			for x := gx0; x <= gx1; x++ {
				r.drawGroundTile(dst, w, x, y, tile, sw, sh)
			}
		}
		if objects { // the map bake: grass as a soft tint (grass_batch.go)
			r.bakeGrassTint(dst, w, gx0, gy0, gx1+1, gy1+1, tile)
		}
		// floors and floor markings from the chunks around the area
		cs := game.ChunkSize
		cx0, cy0 := max(x0-3, 0)/cs, max(y0-3, 0)/cs
		cx1, cy1 := min(x1+3, mw-1)/cs, min(y1+3, mh-1)/cs
		in := func(x, y, fw, fh int) bool {
			return x+max(fw, 1) > x0-1 && x <= x1 && y+max(fh, 1) > y0-1 && y <= y1
		}
		none := color.NRGBA{}
		for cy := cy0; cy <= cy1; cy++ {
			for cx := cx0; cx <= cx1; cx++ {
				for _, i := range r.game.ChunkLayer(cx, cy).Buildings {
					if int(i) >= len(w.Buildings) {
						continue
					}
					b := &w.Buildings[i]
					if !in(b.X, b.Y, b.W, b.H) {
						continue
					}
					switch {
					case b.Floor:
						r.drawTile(dst, r.buildKey("floor", b.Stuff, "", 0, false, false), float64(b.X), float64(b.Y), tile, sw, sh, tintCS(r.stuffTint(b.Stuff)))
					case b.Kind == bridgeKind:
						r.drawBridgeCell(dst, *w, r.bridges.sync(*w), b, tile, sw, sh)
					case b.Kind == "sleeping spot":
						r.drawSprite(dst, buildSprite(b.Kind, b.Facing, 0, false, false), none, float64(b.X), float64(b.Y), tile, sw, sh)
					case b.Kind == "carpet":
						r.drawSprite(dst, buildSpriteStuff(b.Kind, b.Stuff, "", 0, false, false), none, float64(b.X), float64(b.Y), tile, sw, sh)
					}
				}
			}
		}
		// rock (and, for the map, buildings and plant marks) in row order;
		// tall sprites from up to three rows south reach in
		draws := r.lod.bakeDraws[:0]
		ry0, ry1 := max(y0-1, 0), min(y1+2, mh-1)
		for y := ry0; y <= ry1; y++ {
			for x := gx0; x <= gx1; x++ {
				i := y*w.TerrainW + x
				if w.TerrainW <= 0 || i >= len(w.Terrain) || !isRockWallCode(w.Terrain[i]) {
					continue
				}
				if ld, ok := r.rockDraw(w, x, y); ok {
					draws = append(draws, artDraw{row: float64(y), col: float64(x), ld: ld})
				}
			}
		}
		if objects {
			for cy := cy0; cy <= cy1; cy++ {
				for cx := cx0; cx <= cx1; cx++ {
					cv := r.game.ChunkLayer(cx, cy)
					for _, i := range cv.Buildings {
						if int(i) >= len(w.Buildings) {
							continue
						}
						b := &w.Buildings[i]
						if b.Floor || onFloor(b.Kind) || !in(b.X, b.Y, b.W, b.H+2) {
							continue
						}
						draws = append(draws, artDraw{row: float64(b.Y + max(b.H, 1) - 1), col: float64(b.X), kind: objBuilding, i: i})
					}
					for _, i := range cv.Plants {
						if int(i) >= len(w.Plants) {
							continue
						}
						if p := &w.Plants[i]; in(p.X, p.Y, 1, 1) {
							draws = append(draws, artDraw{row: float64(p.Y), col: float64(p.X), kind: objPlantMark, i: i})
						}
					}
				}
			}
		}
		sortDraws(draws)
		for i := range draws {
			r.runDraw(dst, w, &draws[i], tile, sw, sh)
		}
		r.lod.bakeDraws = draws[:0]
	})
}

// whiteImg is a 1×1 white image for solid marks.
func (l *lodState) whiteImg() *ebiten.Image {
	if l.white == nil {
		l.white = ebiten.NewImage(1, 1)
		l.white.Fill(color.White)
	}
	return l.white
}

// solidRect fills the screen rectangle x, y, w, h with c as one sprite
// draw of the white pixel: the pixels vector.FillRect fills without
// anti-aliasing, without its path fill (a tessellation and a flush of the
// vector batch each: per-animal health bars cost milliseconds a frame
// zoomed out).
func (r *Renderer) solidRect(dst *ebiten.Image, x, y, w, h float32, c color.Color) {
	if w <= 0 || h <= 0 {
		return
	}
	op := &r.lod.rectOp
	op.GeoM.Reset()
	op.GeoM.Scale(float64(w), float64(h))
	op.GeoM.Translate(float64(x), float64(y))
	op.ColorScale.Reset()
	op.ColorScale.ScaleWithColor(c)
	dst.DrawImage(r.lod.whiteImg(), op)
}

// plantMarkColor is a plant's mark in the map bake: trees dark, crops
// golden green, other plants green.
func plantMarkColor(p *game.PlantView) (color.RGBA, float64) {
	switch {
	case isTreePlant(p.Species):
		return color.RGBA{34, 70, 36, 255}, 0.8
	case isCropSpecies(p.Species):
		return color.RGBA{150, 160, 70, 255}, 0.5
	}
	if c, ok := wildMarkColors[p.Species]; ok {
		return c, 0.4
	}
	return color.RGBA{70, 120, 52, 255}, 0.45
}

// wildMarkColors are the far-zoom marks of the PLAY-010 wild plants.
var wildMarkColors = map[string]color.RGBA{
	"bramble":      {92, 84, 52, 255},
	"moss":         {96, 118, 92, 255},
	"duskmoss":     {70, 92, 140, 255},
	"agave":        {80, 124, 112, 255},
	"ball_cactus":  {88, 130, 80, 255},
	"elephant_ear": {50, 110, 60, 255},
	"fire_lily":    {196, 110, 44, 255},
	"dandelion":    {200, 188, 90, 255},
}

// isCropSpecies reports a sown crop.
func isCropSpecies(s string) bool {
	_, ok := plantKeys.crops[s]
	return ok
}

// drawPlantMark draws a plant as a solid square mark (the map bake).
func (r *Renderer) drawPlantMark(dst *ebiten.Image, p *game.PlantView, tile float64) {
	c, k := plantMarkColor(p)
	k *= 0.6 + 0.4*float64(min(max(p.Growth, 0), 100))/100
	sx, sy := r.worldToScreenF(float64(p.X)+0.5, float64(p.Y)+0.5)
	s := tile * k
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(s, s)
	op.GeoM.Translate(float64(sx)-s/2, float64(sy)-s/2)
	op.ColorScale.ScaleWithColor(c)
	dst.DrawImage(r.lod.whiteImg(), op)
}

// drawBaked draws tiles tx0..tx1 × ty0..ty1 (inclusive) of src, a bake at
// bpx pixels per tile whose tile (bx, by) starts at source pixel (sx0,
// sy0), onto screen as one lit triangle batch: a quad per tile carrying
// that tile's sky and lamp light, lit flat like the live ground.
func (r *Renderer) drawBaked(screen, src *ebiten.Image, bx, by int, sx0, sy0, bpx float64, tx0, ty0, tx1, ty1 int) {
	if tx1 < tx0 || ty1 < ty0 {
		return
	}
	l := &r.lod
	l.verts, l.idx = l.verts[:0], l.idx[:0]
	tile := r.tilePx()
	for y := ty0; y <= ty1; y++ {
		sy := float32(float64(y)*tile - r.cam.offsetY)
		sy1 := float32(float64(y+1)*tile - r.cam.offsetY)
		vy := float32(sy0 + float64(y-by)*bpx)
		for x := tx0; x <= tx1; x++ {
			// a run of tiles lit alike is one quad (open ground under the
			// sky mostly is): the same pixels, far fewer vertices
			sky, lamp := r.lightTile(x, y)
			xe := x
			for xe < tx1 {
				if s, lm := r.lightTile(xe+1, y); s != sky || lm != lamp {
					break
				}
				xe++
			}
			sx := float32(float64(x)*tile - r.cam.offsetX)
			sx1 := float32(float64(xe+1)*tile - r.cam.offsetX)
			vx := float32(sx0 + float64(x-bx)*bpx)
			vx1 := float32(sx0 + float64(xe+1-bx)*bpx)
			x = xe
			n := uint32(len(l.verts))
			for k := 0; k < 4; k++ {
				dx, dy := sx, sy
				ux, uy := vx, vy
				if k&1 != 0 {
					dx, ux = sx1, vx1
				}
				if k&2 != 0 {
					dy, uy = sy1, vy+float32(bpx)
				}
				l.verts = append(l.verts, ebiten.Vertex{DstX: dx, DstY: dy, SrcX: ux, SrcY: uy,
					ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1, Custom0: sky, Custom1: lamp})
			}
			l.idx = append(l.idx, n, n+1, n+2, n+1, n+3, n+2)
		}
	}
	r.lit.calls++
	sh := r.litShader()
	if sh == nil { // no shader: unlit
		screen.DrawTriangles32(l.verts, l.idx, src, &ebiten.DrawTrianglesOptions{Filter: ebiten.FilterLinear})
		return
	}
	r.lit.syncUniforms()
	r.lit.opts.Images[0] = src
	screen.DrawTrianglesShader32(l.verts, l.idx, sh, &r.lit.opts)
}

// drawGroundLOD draws the visible ground from the bakes (chunk or map
// tier). Chunks without a fresh bake this frame are drawn live.
func (r *Renderer) drawGroundLOD(screen *ebiten.Image, w *game.WorldView, sw, sh int, tile float64) {
	l := &r.lod
	l.budget = bakeBudget
	x0, y0, x1, y1 := l.vx0, l.vy0, l.vx1, l.vy1
	if x1 < x0 || y1 < y0 {
		return
	}
	cs := game.ChunkSize
	for cy := y0 / cs; cy <= y1/cs; cy++ {
		for cx := x0 / cs; cx <= x1/cs; cx++ {
			cv := r.chunkView(cx, cy)
			tx0, ty0 := max(cx*cs, x0), max(cy*cs, y0)
			tx1, ty1 := min(cx*cs+cs-1, x1), min(cy*cs+cs-1, y1)
			var ok bool
			if l.tier == lodMap {
				ok = r.drawMapChunk(screen, w, cx, cy, cv, tx0, ty0, tx1, ty1)
			} else {
				ok = r.drawChunkBake(screen, w, cx, cy, cv, tx0, ty0, tx1, ty1)
			}
			if !ok { // not baked yet: live this frame
				for y := ty0; y <= ty1; y++ {
					for x := tx0; x <= tx1; x++ {
						r.drawGroundTile(screen, w, x, y, tile, sw, sh)
					}
				}
			}
		}
	}
}

// chunkView is this frame's view of chunk (cx, cy) (fetched when outside
// the frame's range).
func (r *Renderer) chunkView(cx, cy int) game.ChunkView {
	l := &r.lod
	if cx >= l.cx0 && cx <= l.cx1 && cy >= l.cy0 && cy <= l.cy1 {
		if i := (cy-l.cy0)*(l.cx1-l.cx0+1) + cx - l.cx0; i < len(l.views) {
			return l.views[i]
		}
	}
	return r.game.ChunkLayer(cx, cy)
}

// mapBake is the whole-map bake: pages of up to mapPageTiles² tiles at
// mapBakePx, each chunk stamped with the versions it was baked at.
type mapBake struct {
	w, h  int // map tiles
	pages []*ebiten.Image
	pcw   int // pages per row
	ver   [][2]uint64
	baked []bool
}

// mapPageTiles is a map page's side in tiles: 63 chunks, so a page with
// its padding stays within 4096 px.
const mapPageTiles = 63 * 16

func (m *mapBake) reset(w, h int) {
	for _, p := range m.pages {
		p.Deallocate()
	}
	*m = mapBake{w: w, h: h}
	m.pcw = (w + mapPageTiles - 1) / mapPageTiles
	pch := (h + mapPageTiles - 1) / mapPageTiles
	m.pages = make([]*ebiten.Image, m.pcw*pch)
	n := ((w + 15) / 16) * ((h + 15) / 16)
	m.ver = make([][2]uint64, n)
	m.baked = make([]bool, n)
}

// page is the page holding tile (x, y) and that tile's pixel in it.
func (m *mapBake) page(x, y int) (img *ebiten.Image, px, py float64) {
	pi := (y/mapPageTiles)*m.pcw + x/mapPageTiles
	if m.pages[pi] == nil {
		tw := min(mapPageTiles, m.w-(x/mapPageTiles)*mapPageTiles)
		th := min(mapPageTiles, m.h-(y/mapPageTiles)*mapPageTiles)
		m.pages[pi] = ebiten.NewImage(tw*mapBakePx+2*bakePad, th*mapBakePx+2*bakePad)
	}
	return m.pages[pi], float64((x%mapPageTiles)*mapBakePx + bakePad), float64((y%mapPageTiles)*mapBakePx + bakePad)
}

// drawMapChunk draws tiles tx0..tx1 × ty0..ty1 of chunk (cx, cy) from the
// map bake, baking the chunk first when it is missing or stale (within
// the frame's budget). False: nothing drawn.
func (r *Renderer) drawMapChunk(screen *ebiten.Image, w *game.WorldView, cx, cy int, cv game.ChunkView, tx0, ty0, tx1, ty1 int) bool {
	m := &r.lod.pages
	mw, mh := r.game.Width(), r.game.Height()
	if m.w != mw || m.h != mh || m.pages == nil {
		m.reset(mw, mh)
	}
	cs := game.ChunkSize
	c := cy*((mw+cs-1)/cs) + cx
	if c < 0 || c >= len(m.baked) {
		return false
	}
	x0, y0 := cx*cs, cy*cs
	img, px, py := m.page(x0, y0)
	want := [2]uint64{grassBakeKey(cv.Ver, r.grassMapSig(w, cx, cy, cv.Ver)), cv.PlantVer}
	if !m.baked[c] || m.ver[c] != want {
		if r.lod.budget <= 0 {
			if !m.baked[c] {
				return false
			}
		} else {
			r.lod.budget--
			x1, y1 := min(x0+cs, mw), min(y0+cs, mh)
			// the chunk's own pixels (the page edge keeps its padding)
			rx0, ry0 := int(px), int(py)
			rx1, ry1 := rx0+(x1-x0)*mapBakePx, ry0+(y1-y0)*mapBakePx
			if x0%mapPageTiles == 0 {
				rx0 -= bakePad
			}
			if y0%mapPageTiles == 0 {
				ry0 -= bakePad
			}
			if x1 == mw || x1%mapPageTiles == 0 {
				rx1 += bakePad
			}
			if y1 == mh || y1%mapPageTiles == 0 {
				ry1 += bakePad
			}
			sub := img.SubImage(rectOf(rx0, ry0, rx1, ry1)).(*ebiten.Image)
			sub.Clear()
			r.bakeGround(sub, w, x0, y0, x1, y1, mapBakePx, px, py, true)
			m.baked[c], m.ver[c] = true, want
		}
	}
	r.drawBaked(screen, img, x0, y0, px, py, mapBakePx, tx0, ty0, tx1, ty1)
	return true
}

// lodTile is the tile size creatures are drawn at: the camera's, or the
// minimum below it.
func lodTile(tile float64) float64 { return math.Max(tile, creatureMinTile) }
