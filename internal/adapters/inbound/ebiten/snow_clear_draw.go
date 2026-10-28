package ebiten

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// The snow clearing area overlay (PLAY-015 §4): a translucent cyan tint
// over the painted cells with a 1 px edge on its outline, like the Home
// overlay (home_draw.go). Shown while a snow clearing tool is held; drawn
// in world space after the Plan overlay (draw_art.go).

var (
	snowClearFill   = color.RGBA{0x3c, 0xd2, 0xe6, 255}
	snowClearFillA  = 0.2 // the tint's alpha
	snowClearEdgeA  = 0.8 // the outline's alpha
	snowClearEdgePx = 1.0 // the outline's width
	snowClearTools  = [2]string{"snow_clear", "unsnow_clear"}
)

// snowClearToolID reports id is one of the two snow clearing tools.
func snowClearToolID(id string) bool { return id == snowClearTools[0] || id == snowClearTools[1] }

// syncSnowClearForce shows the overlay while a snow clearing tool is held.
func (a *App) syncSnowClearForce() {
	if a.renderer != nil {
		a.renderer.env.snowClear.forced = a.archOpen && snowClearToolID(a.archTool)
	}
}

// snowClearDrawState caches the overlay's tile-space geometry, rebuilt
// only when WorldView.SnowClearVer (or the grid's shape) changes.
type snowClearDrawState struct {
	forced bool // a snow clearing tool is held
	valid  bool
	ver    uint64
	w, n   int
	runs   []homeRun
	edges  []homeEdge
	builds int // rebuild count (tests)
	verts  int // vertices in the last drawn batch (tests)
	batch  triBatch
}

// rebuild recomputes the runs and the outline from the area grid.
func (s *snowClearDrawState) rebuild(w game.WorldView) {
	s.valid, s.ver, s.w, s.n = true, w.SnowClearVer, w.TerrainW, len(w.SnowClear)
	s.runs, s.edges = s.runs[:0], s.edges[:0]
	s.builds++
	if w.TerrainW <= 0 {
		return
	}
	m := w.SnowClear
	rows := len(m) / w.TerrainW
	nb := [4][2]int{sideN: {0, -1}, sideE: {1, 0}, sideS: {0, 1}, sideW: {-1, 0}}
	for y := 0; y < rows; y++ {
		start := -1
		for x := 0; x <= w.TerrainW; x++ {
			in := x < w.TerrainW && homeAt(m, w.TerrainW, x, y)
			if in && start < 0 {
				start = x
			}
			if !in && start >= 0 {
				s.runs = append(s.runs, homeRun{y, start, x})
				start = -1
			}
			if !in {
				continue
			}
			for side, d := range nb {
				if !homeAt(m, w.TerrainW, x+d[0], y+d[1]) {
					s.edges = append(s.edges, homeEdge{x, y, side})
				}
			}
		}
	}
}

// drawSnowClearOverlay draws the snow clearing area when a tool is held,
// in one batch.
func (r *Renderer) drawSnowClearOverlay(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	st := &r.env.snowClear
	st.verts = 0
	if !st.forced || len(w.SnowClear) == 0 || w.TerrainW <= 0 {
		return
	}
	if !st.valid || st.ver != w.SnowClearVer || st.w != w.TerrainW || st.n != len(w.SnowClear) {
		st.rebuild(w)
	}
	r.setView(sw, sh, tile)
	b := &st.batch
	b.Reset()
	a := snowClearFillA
	for _, run := range st.runs {
		sx, sy := r.worldToScreen(run.x0, run.y)
		wd := float64(run.x1-run.x0) * tile
		if !visible(float32(sx), float32(sy), float32(wd), float32(tile), sw, sh) {
			continue
		}
		b.Rect(float64(sx), float64(sy), wd, tile, snowClearFill, a, a, a, a)
	}
	ec := snowClearFill
	ec.A = uint8(snowClearEdgeA * 255)
	for _, e := range st.edges {
		sx, sy := r.worldToScreen(e.x, e.y)
		if !visible(float32(sx), float32(sy), float32(tile), float32(tile), sw, sh) {
			continue
		}
		r.edgeStrip(b, float64(sx), float64(sy), tile, e.side, snowClearEdgePx, ec, 0, 1)
	}
	st.verts = len(b.Verts)
	b.Flush(screen, r.env.whiteImg(), ebiten.BlendSourceOver)
}
