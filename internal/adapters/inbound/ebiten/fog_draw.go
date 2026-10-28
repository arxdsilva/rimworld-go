package ebiten

import (
	"image/color"
	"math"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// Fog of war on screen (MAPGEN-2 §5): unexplored tiles (WorldView.Fog == 1)
// draw solid near-black over everything on the map, with a soft edge one
// tile wide that fades into the revealed side.
//
// The fog is baked per map chunk (fogChunkTiles tiles square) into a small
// alpha image at fogPx pixels per tile and stamped scaled to the camera's
// physical tile size. A chunk is rebuilt, only when it is in view, once the
// read model's FogVer differs from the one it was baked at, so an unchanged
// fog costs one image draw per visible chunk and no per-frame bake. An
// empty Fog (legacy colonies) returns before any work.

// fogClr is the colour of unexplored ground (#0B0B0E); the minimap uses it
// too.
var fogClr = color.RGBA{R: 0x0B, G: 0x0B, B: 0x0E, A: 255}

const (
	fogChunkTiles = 32 // tiles per chunk side
	fogPx         = 4  // baked pixels per tile
	fogChunkPx    = fogChunkTiles * fogPx
	fogEdgeTiles  = 1.0 // width of the soft edge, in tiles
)

// fogChunk is one baked chunk.
type fogChunk struct {
	img   *ebiten.Image
	ver   uint64
	built bool
}

// fogCache holds the baked chunks of the current map.
type fogCache struct {
	Chunks map[int]*fogChunk
	n      int // len(Fog) the chunks were baked for
	Pix    []byte
	Bakes  int // chunk bakes (tests)
}

// fogHidden reports tile (x, y) is unexplored in a read model's Fog.
func fogHidden(fog []uint8, w, x, y int) bool {
	if w <= 0 || x < 0 || y < 0 || x >= w {
		return false
	}
	i := y*w + x
	return i < len(fog) && fog[i] != 0
}

// worldFogged reports tile (x, y) is unexplored in the world view.
func worldFogged(w game.WorldView, x, y int) bool {
	return len(w.Fog) > 0 && fogHidden(w.Fog, w.TerrainW, x, y)
}

// fogAlpha is the fog's opacity (0..1) at the tile-space point (px, py):
// 1 on a hidden tile, fading to 0 over fogEdgeTiles outside the nearest
// hidden tile (smoothstep).
func fogAlpha(fog []uint8, mw, mh int, px, py float64) float64 {
	tx, ty := int(math.Floor(px)), int(math.Floor(py))
	best := 0.0
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			x, y := tx+dx, ty+dy
			if x < 0 || y < 0 || x >= mw || y >= mh || !fogHidden(fog, mw, x, y) {
				continue
			}
			ddx := math.Max(math.Max(float64(x)-px, px-float64(x+1)), 0)
			ddy := math.Max(math.Max(float64(y)-py, py-float64(y+1)), 0)
			d := math.Hypot(ddx, ddy) / fogEdgeTiles
			if d >= 1 {
				continue
			}
			s := 1 - d
			best = math.Max(best, s*s*(3-2*s))
		}
	}
	return best
}

// Bake renders chunk (cx, cy) into c as premultiplied fog-coloured pixels.
func (fc *fogCache) Bake(c *fogChunk, fog []uint8, mw, mh, cx, cy int) {
	if c.img == nil {
		c.img = ebiten.NewImage(fogChunkPx, fogChunkPx)
	}
	if cap(fc.Pix) < 4*fogChunkPx*fogChunkPx {
		fc.Pix = make([]byte, 4*fogChunkPx*fogChunkPx)
	}
	pix := fc.Pix[:4*fogChunkPx*fogChunkPx]
	for y := 0; y < fogChunkPx; y++ {
		py := float64(cy*fogChunkTiles) + (float64(y)+0.5)/fogPx
		for x := 0; x < fogChunkPx; x++ {
			px := float64(cx*fogChunkTiles) + (float64(x)+0.5)/fogPx
			a := fogAlpha(fog, mw, mh, px, py)
			o := 4 * (y*fogChunkPx + x)
			pix[o] = uint8(float64(fogClr.R)*a + 0.5)
			pix[o+1] = uint8(float64(fogClr.G)*a + 0.5)
			pix[o+2] = uint8(float64(fogClr.B)*a + 0.5)
			pix[o+3] = uint8(255*a + 0.5)
		}
	}
	c.img.WritePixels(pix)
	fc.Bakes++
}

// drawFogLayer stamps the fog over the map for a fog slice mw x mh tiles
// at FogVer ver, tile physical pixels per tile, into a sw x sh screen.
func (r *Renderer) drawFogLayer(screen *ebiten.Image, fog []uint8, ver uint64, mw, mh int, tile float64, sw, sh int) {
	fc := &r.fogc
	if len(fog) == 0 || mw <= 0 {
		if fc.Chunks != nil { // fog gone (another map): drop the bakes
			for _, c := range fc.Chunks {
				if c.img != nil {
					c.img.Deallocate()
				}
			}
			fc.Chunks, fc.n = nil, 0
		}
		return
	}
	if fc.Chunks == nil || fc.n != len(fog) {
		for _, c := range fc.Chunks {
			if c.img != nil {
				c.img.Deallocate()
			}
		}
		fc.Chunks, fc.n = map[int]*fogChunk{}, len(fog)
	}
	ncx := (mw + fogChunkTiles - 1) / fogChunkTiles
	span := tile * fogChunkTiles
	cx0 := max(int(math.Floor(r.cam.OffsetX/span)), 0)
	cy0 := max(int(math.Floor(r.cam.OffsetY/span)), 0)
	cx1 := min(int(math.Floor((r.cam.OffsetX+float64(sw))/span)), ncx-1)
	cy1 := min(int(math.Floor((r.cam.OffsetY+float64(sh))/span)), (mh+fogChunkTiles-1)/fogChunkTiles-1)
	k := tile / fogPx
	for cy := cy0; cy <= cy1; cy++ {
		for cx := cx0; cx <= cx1; cx++ {
			id := cy*ncx + cx
			c := fc.Chunks[id]
			if c == nil {
				c = &fogChunk{}
				fc.Chunks[id] = c
			}
			if !c.built || c.ver != ver {
				fc.Bake(c, fog, mw, mh, cx, cy)
				c.built, c.ver = true, ver
			}
			var op ebiten.DrawImageOptions
			op.Filter = ebiten.FilterLinear
			op.GeoM.Scale(k, k)
			sx, sy := r.worldToScreenF(float64(cx*fogChunkTiles), float64(cy*fogChunkTiles))
			op.GeoM.Translate(float64(sx), float64(sy))
			screen.DrawImage(c.img, &op)
		}
	}
}

// drawFog is the map pass's fog layer (physical pixels, r.cam).
func (r *Renderer) drawFog(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	if len(w.Fog) == 0 && r.fogc.Chunks == nil {
		return
	}
	r.drawFogLayer(screen, w.Fog, w.FogVer, w.TerrainW, r.game.Height(), tile, sw, sh)
}

// unexploredLines is the hover readout of a fogged tile; nil elsewhere.
func unexploredLines(w game.WorldView, x, y int) []string {
	if worldFogged(w, x, y) {
		return []string{"Unexplored"}
	}
	return nil
}
