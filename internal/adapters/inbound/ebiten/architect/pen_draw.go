package architect

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
)

// Pen region outline (PLAY-015 §2.2): the selected pen marker's cells, or
// every pen while the Pen marker tool is held, get an outline; an open pen
// (no region) is a red dashed box around its marker with "Not enclosed".

var (
	penEdgeClr = color.RGBA{R: 240, G: 220, B: 120, A: 230}
	penOpenClr = color.RGBA{R: 220, G: 60, B: 50, A: 240}
)

const (
	penMarkerKind = "pen marker" // the marker building (pen_ui.go's penMarkerKind)
	penToolID     = "pen_marker" // its Architect tool (pen_ui.go's penToolID)
)

// PenEdge is one unit outline segment: the Side (0 top, 1 right, 2
// bottom, 3 left) of cell (X, Y) that faces outside the pen.
type PenEdge struct{ X, Y, Side int }

// bitAt reports cell (x, y) is set in a row-major bitset w cells wide
// (as pen_ui.go reads a pen's cells).
func bitAt(bits []uint64, w, x, y int) bool {
	if w <= 0 || x < 0 || y < 0 || x >= w {
		return false
	}
	n := y*w + x
	return n/64 < len(bits) && bits[n/64]&(1<<(n%64)) != 0
}

// PenEdges lists the outline of the cells set in bits (row-major, w wide,
// h tall).
func PenEdges(bits []uint64, w, h int) []PenEdge {
	if w <= 0 || h <= 0 {
		return nil
	}
	var out []PenEdge
	for wi, word := range bits {
		for word != 0 {
			b := 0
			for word&(1<<b) == 0 {
				b++
			}
			word &^= 1 << b
			n := wi*64 + b
			x, y := n%w, n/w
			if y >= h {
				return out
			}
			if y == 0 || !bitAt(bits, w, x, y-1) {
				out = append(out, PenEdge{x, y, 0})
			}
			if !bitAt(bits, w, x+1, y) {
				out = append(out, PenEdge{x, y, 1})
			}
			if !bitAt(bits, w, x, y+1) {
				out = append(out, PenEdge{x, y, 2})
			}
			if !bitAt(bits, w, x-1, y) {
				out = append(out, PenEdge{x, y, 3})
			}
		}
	}
	return out
}

// penToolHeld reports the Pen marker architect tool is armed.
func (m *Marks) penToolHeld() bool { return m.Open && m.ToolID == penToolID }

// PenOutlined is the pen of the selected marker.
func (m *Marks) PenOutlined() (game.PenView, bool) {
	env := m.sh.Env()
	if env.Sim == nil || !env.Sel.HasBuilding {
		return game.PenView{}, false
	}
	w := m.sh.World()
	for i := range w.Buildings {
		if b := &w.Buildings[i]; b.ID == env.Sel.SelBuilding {
			if b.Kind != penMarkerKind {
				return game.PenView{}, false
			}
			for _, p := range w.Pens {
				if p.ID == b.ID {
					return p, true
				}
			}
			return game.PenView{}, false
		}
	}
	return game.PenView{}, false
}

// PenOutlines are the pens to outline now: all of them while the pen tool
// is held, else the selected marker's.
func (m *Marks) PenOutlines() []game.PenView {
	if m.sim() == nil {
		return nil
	}
	if m.penToolHeld() {
		return m.sh.World().Pens
	}
	if pv, ok := m.PenOutlined(); ok {
		return []game.PenView{pv}
	}
	return nil
}

// DrawPenOutline draws the outlines (physical pixels, over the map).
func (m *Marks) DrawPenOutline(screen *ebiten.Image) {
	pens := m.PenOutlines()
	if len(pens) == 0 {
		return
	}
	sw, sh := m.sh.Size()
	w := m.sh.World()
	tile := float32(m.sh.TilePx())
	for _, pv := range pens {
		if !pv.Enclosed || len(pv.Bits) == 0 || w.TerrainW <= 0 {
			m.drawOpenPen(screen, pv, tile, sw, sh)
			continue
		}
		h := len(pv.Bits) * 64 / w.TerrainW
		for _, e := range PenEdges(pv.Bits, w.TerrainW, h) {
			sx, sy := m.sh.WorldToScreen(e.X, e.Y)
			if !sprite.Visible(sx, sy, tile, tile, sw, sh) {
				continue
			}
			penSide(screen, sx, sy, tile, e.Side, penEdgeClr, false)
		}
	}
}

// drawOpenPen is a red dashed box around the marker and a label.
func (m *Marks) drawOpenPen(screen *ebiten.Image, pv game.PenView, tile float32, sw, sh int) {
	sx, sy := m.sh.WorldToScreen(pv.X, pv.Y)
	if !sprite.Visible(sx, sy, tile, tile, sw, sh) {
		return
	}
	for side := 0; side < 4; side++ {
		penSide(screen, sx, sy, tile, side, penOpenClr, true)
	}
	k := 1 / gfx.UIScale()
	gfx.Text(screen, "Not enclosed", float64(sx)*k, float64(sy-tile*0.4)*k, gfx.TextStyle{Size: 11, Bold: true, Color: penOpenClr})
}

// penSide strokes one side of the tile at (sx, sy); dashed draws it as
// short dashes.
func penSide(screen *ebiten.Image, sx, sy, tile float32, side int, clr color.RGBA, dashed bool) {
	x0, y0, x1, y1 := sx, sy, sx+tile, sy
	switch side {
	case 1:
		x0, y0, x1, y1 = sx+tile, sy, sx+tile, sy+tile
	case 2:
		x0, y0, x1, y1 = sx, sy+tile, sx+tile, sy+tile
	case 3:
		x0, y0, x1, y1 = sx, sy, sx, sy+tile
	}
	if !dashed {
		vector.StrokeLine(screen, x0, y0, x1, y1, 2, clr, false)
		return
	}
	const dashes = 3
	for i := 0; i < dashes; i++ { // dash: the first half of each third
		a, b := float32(i)/dashes, (float32(i)+0.5)/dashes
		vector.StrokeLine(screen, x0+(x1-x0)*a, y0+(y1-y0)*a, x0+(x1-x0)*b, y0+(y1-y0)*b, 2, clr, false)
	}
}
