package ebiten

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
)

// Pen region outline (PLAY-015 §2.2): the selected pen marker's cells, or
// every pen while the Pen marker tool is held, get an outline; an open pen
// (no region) is a red dashed box around its marker with "Not enclosed".

var (
	penEdgeClr = color.RGBA{R: 240, G: 220, B: 120, A: 230}
	penOpenClr = color.RGBA{R: 220, G: 60, B: 50, A: 240}
)

// penEdge is one unit outline segment: the side (0 top, 1 right, 2
// bottom, 3 left) of cell (x, y) that faces outside the pen.
type penEdge struct{ x, y, side int }

// penEdges lists the outline of the cells set in bits (row-major, w wide,
// h tall).
func penEdges(bits []uint64, w, h int) []penEdge {
	if w <= 0 || h <= 0 {
		return nil
	}
	var out []penEdge
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
				out = append(out, penEdge{x, y, 0})
			}
			if !bitAt(bits, w, x+1, y) {
				out = append(out, penEdge{x, y, 1})
			}
			if !bitAt(bits, w, x, y+1) {
				out = append(out, penEdge{x, y, 2})
			}
			if !bitAt(bits, w, x-1, y) {
				out = append(out, penEdge{x, y, 3})
			}
		}
	}
	return out
}

// penToolHeld reports the Pen marker architect tool is armed.
func (a *App) penToolHeld() bool { return a.archOpen && a.archTool == penToolID }

// penOutlined is the pen of the selected marker.
func (a *App) penOutlined() (game.PenView, bool) {
	if a.renderer == nil {
		return game.PenView{}, false
	}
	b, ok := a.selectedBuilding()
	if !ok || !isPenMarker(b) {
		return game.PenView{}, false
	}
	return a.penOf(b)
}

// penOutlines are the pens to outline now: all of them while the pen tool
// is held, else the selected marker's.
func (a *App) penOutlines() []game.PenView {
	if a.renderer == nil {
		return nil
	}
	if a.penToolHeld() {
		return a.shell.World().Pens
	}
	if pv, ok := a.penOutlined(); ok {
		return []game.PenView{pv}
	}
	return nil
}

// drawPenOutline draws the outlines (physical pixels, over the map).
func (a *App) drawPenOutline(screen *ebiten.Image) {
	pens := a.penOutlines()
	if len(pens) == 0 {
		return
	}
	sw, sh := a.size()
	w := a.shell.World()
	tile := float32(a.shell.TilePx())
	for _, pv := range pens {
		if !pv.Enclosed || len(pv.Bits) == 0 || w.TerrainW <= 0 {
			a.drawOpenPen(screen, pv, tile, sw, sh)
			continue
		}
		h := len(pv.Bits) * 64 / w.TerrainW
		for _, e := range penEdges(pv.Bits, w.TerrainW, h) {
			sx, sy := a.shell.WorldToScreen(e.x, e.y)
			if !visible(sx, sy, tile, tile, sw, sh) {
				continue
			}
			penSide(screen, sx, sy, tile, e.side, penEdgeClr, false)
		}
	}
}

// drawOpenPen is a red dashed box around the marker and a label.
func (a *App) drawOpenPen(screen *ebiten.Image, pv game.PenView, tile float32, sw, sh int) {
	sx, sy := a.shell.WorldToScreen(pv.X, pv.Y)
	if !visible(sx, sy, tile, tile, sw, sh) {
		return
	}
	for side := 0; side < 4; side++ {
		penSide(screen, sx, sy, tile, side, penOpenClr, true)
	}
	k := 1 / uiScale()
	drawText(screen, "Not enclosed", float64(sx)*k, float64(sy-tile*0.4)*k, textStyle{Size: 11, Bold: true, Color: penOpenClr})
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
