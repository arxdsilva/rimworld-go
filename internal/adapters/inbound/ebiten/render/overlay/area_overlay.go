package overlay

import (
	"image/color"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"

	"github.com/hajimehoshi/ebiten/v2"
)

// The allowed-area overlay (PLAY-015 §1.3): while an area tool is armed,
// or an area row is hovered in a picker, that area's cells draw as a
// translucent tint in its colour. The cells (AreaView.Bits, row-major over
// the map) are baked into row runs once per area version (AreaView.Ver);
// the draw only walks the runs on screen, in the physical-pixel map pass
// (DrawRoofOverlay calls it with the other map overlays).

const areaOverlayA = 0.34 // the tint's alpha

// areaBake is one area's cached row runs.
type areaBake struct {
	built bool
	ver   uint64
	w     int // map width the runs were built for
	n     int // len(Bits) they were built from
	runs  []HomeRun
}

// AreaOverlay is the renderer's overlay state. armed and hover are area
// ids (0: none), set by the UI each frame; the bakes are rebuilt only when
// an area's Ver changes.
type AreaOverlay struct {
	Armed, Hover int
	bakes        map[uint16]*areaBake
	Builds       int        // bakes rebuilt (tests)
	Verts        int        // vertices in the last batch (tests)
	Tint         color.RGBA // the last tint drawn (tests)
	batch        gfx.TriBatch
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
				b.runs = append(b.runs, HomeRun{y, start, x})
				start = -1
			}
		}
	}
}

// DrawAreaOverlay is the map-pass hook: it draws the armed and the
// hovered area, if any.
func (o *Set) DrawAreaOverlay(v *render.View, screen *ebiten.Image, w game.WorldView, areas func() []game.AreaView, sw, sh int, tile float64) {
	st := &o.Area
	st.Verts = 0
	if st.Armed == 0 && st.Hover == 0 {
		return
	}
	o.DrawAreas(v, screen, w, areas(), sw, sh, tile)
}

// DrawAreas draws the overlay of the armed and hovered ids among areas.
func (o *Set) DrawAreas(v *render.View, screen *ebiten.Image, w game.WorldView, areas []game.AreaView, sw, sh int, tile float64) {
	st := &o.Area
	st.Verts = 0
	if w.TerrainW <= 0 {
		return
	}
	if st.bakes == nil {
		st.bakes = map[uint16]*areaBake{}
	}
	b := &st.batch
	b.Reset()
	for _, ar := range areas {
		if int(ar.ID) != st.Armed && int(ar.ID) != st.Hover {
			continue
		}
		bk := st.bakes[ar.ID]
		if bk == nil {
			bk = &areaBake{}
			st.bakes[ar.ID] = bk
		}
		if !bk.built || bk.ver != ar.Ver || bk.w != w.TerrainW || bk.n != len(ar.Bits) {
			bk.bake(ar, w.TerrainW)
			st.Builds++
		}
		c := color.RGBA{ar.Color[0], ar.Color[1], ar.Color[2], 255}
		st.Tint = c
		for _, run := range bk.runs {
			sx, sy := v.WorldToScreen(run.x0, run.y)
			wd := float64(run.x1-run.x0) * tile
			if !sprite.Visible(float32(sx), float32(sy), float32(wd), float32(tile), sw, sh) {
				continue
			}
			b.Rect(float64(sx), float64(sy), wd, tile, c, areaOverlayA, areaOverlayA, areaOverlayA, areaOverlayA)
		}
	}
	st.Verts = len(b.Verts)
	b.Flush(screen, o.whiteImg(), ebiten.BlendSourceOver)
}
