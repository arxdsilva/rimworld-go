package ebiten

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// The Plan overlay (PLAY-012): soft blue-white fills with a dashed outline
// around each marked region. Marks are purely visual sketches; they are
// visible by default, hidden by the HUD "Plans" button, and forced visible
// while a Plan tool is held. Drawn in world space right after the zones
// (draw_art.go), under blueprints and objects.

const (
	planEdgePx  = 1.5 // the outline's width
	planDashEnd = 0.4 // each tile side carries two dashes: [0, 0.4] and [0.6, 1]
)

// planOverlayOn reports the Plan overlay is showing.
func (r *Renderer) planOverlayOn() bool { return !r.planHidden || r.planForced }

// planDrawState caches the overlay's tile-space geometry, rebuilt only when
// WorldView.PlanVer (or the grid's shape) changes.
type planDrawState struct {
	valid  bool
	ver    uint32
	w, n   int
	Runs   []homeRun
	Edges  []homeEdge
	Builds int // rebuild count (tests)
	Verts  int // vertices in the last drawn batch (tests)
	batch  triBatch
}

// Rebuild recomputes the runs and the outline from the Plan grid.
func (s *planDrawState) Rebuild(w game.WorldView) {
	s.valid, s.ver, s.w, s.n = true, w.PlanVer, w.TerrainW, len(w.Plan)
	s.Runs, s.Edges = s.Runs[:0], s.Edges[:0]
	s.Builds++
	if w.TerrainW <= 0 {
		return
	}
	rows := len(w.Plan) / w.TerrainW
	nb := [4][2]int{sideN: {0, -1}, sideE: {1, 0}, sideS: {0, 1}, sideW: {-1, 0}}
	for y := 0; y < rows; y++ {
		start := -1
		for x := 0; x <= w.TerrainW; x++ {
			in := x < w.TerrainW && homeAt(w.Plan, w.TerrainW, x, y)
			if in && start < 0 {
				start = x
			}
			if !in && start >= 0 {
				s.Runs = append(s.Runs, homeRun{y, start, x})
				start = -1
			}
			if !in {
				continue
			}
			for side, d := range nb {
				if !homeAt(w.Plan, w.TerrainW, x+d[0], y+d[1]) {
					s.Edges = append(s.Edges, homeEdge{x, y, side})
				}
			}
		}
	}
}

// drawPlanOverlay draws the Plan marks when the overlay is on, in one batch.
func (r *Renderer) drawPlanOverlay(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	st := &r.planDraw
	st.Verts = 0
	if !r.planOverlayOn() || len(w.Plan) == 0 || w.TerrainW <= 0 {
		return
	}
	if !st.valid || st.ver != w.PlanVer || st.w != w.TerrainW || st.n != len(w.Plan) {
		st.Rebuild(w)
	}
	r.setView(sw, sh, tile)
	b := &st.batch
	b.Reset()
	a := alphaOf(uiPlanFill)
	for _, run := range st.Runs {
		sx, sy := r.worldToScreen(run.x0, run.y)
		wd := float64(run.x1-run.x0) * tile
		if !visible(float32(sx), float32(sy), float32(wd), float32(tile), sw, sh) {
			continue
		}
		b.Rect(float64(sx), float64(sy), wd, tile, uiPlanFill, a, a, a, a)
	}
	for _, e := range st.Edges {
		sx, sy := r.worldToScreen(e.x, e.y)
		if !visible(float32(sx), float32(sy), float32(tile), float32(tile), sw, sh) {
			continue
		}
		r.edgeStrip(b, float64(sx), float64(sy), tile, e.side, planEdgePx, uiPlanEdge, 0, planDashEnd)
		r.edgeStrip(b, float64(sx), float64(sy), tile, e.side, planEdgePx, uiPlanEdge, 1-planDashEnd, 1)
	}
	st.Verts = len(b.Verts)
	b.Flush(screen, r.env.WhiteImg(), ebiten.BlendSourceOver)
}
