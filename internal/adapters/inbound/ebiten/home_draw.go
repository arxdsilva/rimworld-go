package ebiten

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// The Home area overlay (SIM-021 / ART §16.3): a translucent blue tint over
// the tiles colonists keep clean, with a 1 px edge on its outline. Shown
// while a Home tool is held or when toggled next to Roof (r.homeOverlay,
// r.homeOverlayForced); drawn with the Roof overlay (roof_draw.go).

var (
	homeFill   = color.RGBA{0x4a, 0x7f, 0xd4, 255}
	homeFillA  = 0.18 // the tint's alpha
	homeEdgeA  = 0.75 // the outline's alpha
	homeEdgePx = 1.0  // the outline's width
)

// homeOverlayOn reports the Home overlay is showing.
func (r *Renderer) homeOverlayOn() bool { return r.homeOverlay || r.homeOverlayForced }

// homeRun is a horizontal run of Home tiles [x0, x1) on row y.
type homeRun struct{ y, x0, x1 int }

// homeEdge is one outline segment: the side of Home tile (x, y) that faces
// a tile outside Home.
type homeEdge struct {
	x, y int
	side int
}

// homeDrawState caches the overlay's geometry in tile space, rebuilt only
// when WorldView.HomeVer (or the grid's shape) changes.
type homeDrawState struct {
	valid  bool
	ver    uint32
	w, n   int
	Runs   []homeRun
	Edges  []homeEdge
	Builds int // how many times the cache was rebuilt (tests)
	Verts  int // vertices in the last drawn batch (tests)
	batch  triBatch
}

// homeAt reports tile (x, y) of the grid (w wide) is Home.
func homeAt(home []uint8, w, x, y int) bool {
	if x < 0 || y < 0 || x >= w {
		return false
	}
	i := y*w + x
	return i < len(home) && home[i] != 0
}

// Rebuild recomputes the runs and the outline from w's Home grid.
func (s *homeDrawState) Rebuild(w game.WorldView) {
	s.valid, s.ver, s.w, s.n = true, w.HomeVer, w.TerrainW, len(w.Home)
	s.Runs, s.Edges = s.Runs[:0], s.Edges[:0]
	s.Builds++
	if w.TerrainW <= 0 {
		return
	}
	rows := len(w.Home) / w.TerrainW
	nb := [4][2]int{sideN: {0, -1}, sideE: {1, 0}, sideS: {0, 1}, sideW: {-1, 0}}
	for y := 0; y < rows; y++ {
		start := -1
		for x := 0; x <= w.TerrainW; x++ {
			in := x < w.TerrainW && homeAt(w.Home, w.TerrainW, x, y)
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
				if !homeAt(w.Home, w.TerrainW, x+d[0], y+d[1]) {
					s.Edges = append(s.Edges, homeEdge{x, y, side})
				}
			}
		}
	}
}

// drawHomeOverlay draws the Home overlay when it is on, in one batch.
func (r *Renderer) drawHomeOverlay(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	st := &r.env.Home
	st.Verts = 0
	if !r.homeOverlayOn() || len(w.Home) == 0 || w.TerrainW <= 0 {
		return
	}
	if !st.valid || st.ver != w.HomeVer || st.w != w.TerrainW || st.n != len(w.Home) {
		st.Rebuild(w)
	}
	r.setView(sw, sh, tile)
	b := &st.batch
	b.Reset()
	a := homeFillA
	for _, run := range st.Runs {
		sx, sy := r.worldToScreen(run.x0, run.y)
		wd := float64(run.x1-run.x0) * tile
		if !visible(float32(sx), float32(sy), float32(wd), float32(tile), sw, sh) {
			continue
		}
		b.Rect(float64(sx), float64(sy), wd, tile, homeFill, a, a, a, a)
	}
	ec := homeFill
	for _, e := range st.Edges {
		sx, sy := r.worldToScreen(e.x, e.y)
		if !visible(float32(sx), float32(sy), float32(tile), float32(tile), sw, sh) {
			continue
		}
		ec.A = uint8(homeEdgeA * 255)
		r.edgeStrip(b, float64(sx), float64(sy), tile, e.side, homeEdgePx, ec, 0, 1)
	}
	st.Verts = len(b.Verts)
	b.Flush(screen, r.env.WhiteImg(), ebiten.BlendSourceOver)
}
