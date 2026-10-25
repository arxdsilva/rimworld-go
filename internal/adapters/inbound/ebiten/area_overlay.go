package ebiten

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// The allowed-area overlay (PLAY-015 §1.3): while an area tool is armed,
// or an area row is hovered in a picker, that area's cells draw as a
// translucent tint in its colour. The cells (AreaView.Bits, row-major over
// the map) are baked into row runs once per area version (AreaView.Ver);
// the draw only walks the runs on screen, in the physical-pixel map pass
// (drawRoofOverlay calls it with the other map overlays).

const areaOverlayA = 0.34 // the tint's alpha

// areaBake is one area's cached row runs.
type areaBake struct {
	built bool
	ver   uint64
	w     int // map width the runs were built for
	n     int // len(Bits) they were built from
	runs  []homeRun
}

// areaOverlay is the renderer's overlay state. armed and hover are area
// ids (0: none), set by the UI each frame; the bakes are rebuilt only when
// an area's Ver changes.
type areaOverlay struct {
	armed, hover int
	bakes        map[uint16]*areaBake
	builds       int        // bakes rebuilt (tests)
	verts        int        // vertices in the last batch (tests)
	tint         color.RGBA // the last tint drawn (tests)
	batch        triBatch
}

// areaBitAt reports cell i of a row-major bitset is set.
func areaBitAt(bits []uint64, i int) bool {
	return i >= 0 && i>>6 < len(bits) && bits[i>>6]&(1<<(uint(i)&63)) != 0
}

// bake builds ar's row runs over a map w cells wide.
func (b *areaBake) bake(ar game.AreaView, w int) {
	b.built, b.ver, b.w, b.n = true, ar.Ver, w, len(ar.Bits)
	b.runs = b.runs[:0]
	if w <= 0 {
		return
	}
	rows := len(ar.Bits) * 64 / w
	for y := 0; y < rows; y++ {
		start := -1
		for x := 0; x <= w; x++ {
			in := x < w && areaBitAt(ar.Bits, y*w+x)
			if in && start < 0 {
				start = x
			}
			if !in && start >= 0 {
				b.runs = append(b.runs, homeRun{y, start, x})
				start = -1
			}
		}
	}
}

// drawAreaOverlay is the map-pass hook: it draws the armed and the
// hovered area, if any.
func (r *Renderer) drawAreaOverlay(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	st := &r.areaOv
	st.verts = 0
	if st.armed == 0 && st.hover == 0 {
		return
	}
	r.drawAreas(screen, w, r.game.Areas(), sw, sh, tile)
}

// drawAreas draws the overlay of the armed and hovered ids among areas.
func (r *Renderer) drawAreas(screen *ebiten.Image, w game.WorldView, areas []game.AreaView, sw, sh int, tile float64) {
	st := &r.areaOv
	st.verts = 0
	if w.TerrainW <= 0 {
		return
	}
	if st.bakes == nil {
		st.bakes = map[uint16]*areaBake{}
	}
	r.setView(sw, sh, tile)
	b := &st.batch
	b.reset()
	for _, ar := range areas {
		if int(ar.ID) != st.armed && int(ar.ID) != st.hover {
			continue
		}
		bk := st.bakes[ar.ID]
		if bk == nil {
			bk = &areaBake{}
			st.bakes[ar.ID] = bk
		}
		if !bk.built || bk.ver != ar.Ver || bk.w != w.TerrainW || bk.n != len(ar.Bits) {
			bk.bake(ar, w.TerrainW)
			st.builds++
		}
		c := color.RGBA{ar.Color[0], ar.Color[1], ar.Color[2], 255}
		st.tint = c
		for _, run := range bk.runs {
			sx, sy := r.worldToScreen(run.x0, run.y)
			wd := float64(run.x1-run.x0) * tile
			if !visible(float32(sx), float32(sy), float32(wd), float32(tile), sw, sh) {
				continue
			}
			b.rect(float64(sx), float64(sy), wd, tile, c, areaOverlayA, areaOverlayA, areaOverlayA, areaOverlayA)
		}
	}
	st.verts = len(b.verts)
	b.flush(screen, r.env.whiteImg(), ebiten.BlendSourceOver)
}

// syncAreaOverlay points the overlay at the area the armed Architect tool
// paints (none when no area tool is armed); the pickers set hover.
func (a *App) syncAreaOverlay() {
	if a.renderer == nil {
		return
	}
	id := 0
	if _, ok := areaToolPaints(a.archTool); ok && a.archOpen {
		id = a.areaPaintIndex()
	}
	a.renderer.areaOv.armed = id
}
