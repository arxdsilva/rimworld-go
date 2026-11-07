package overlay

import (
	"math"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// Selection brackets (UIS-092): four white corner pieces drawn in code at
// the footprint of what is selected, a pawn's or an item's cell or a
// building's whole footprint. They follow the zoom only: an arm is a third
// of a cell and the stroke max(1.5, cell/14) map pixels.
const (
	bracketArm    = 1.0 / 3  // of a cell
	bracketStroke = 1.0 / 14 // of a cell
	bracketMinW   = 1.5      // map pixels
)

// BracketGeom is the arm length and stroke width of the brackets for a
// cell of the given size (map pixels).
func BracketGeom(cell float64) (arm, stroke float64) {
	return cell * bracketArm, math.Max(bracketMinW, cell*bracketStroke)
}

// DrawSelectBrackets draws the corner brackets of the rectangle x, y, w, h
// (map pixels, the footprint's top-left and size) for cells of size cell,
// each corner two strips of white (a 1x1 white image), inside the
// rectangle so neighbouring footprints never share a stroke.
func DrawSelectBrackets(dst, white *ebiten.Image, x, y, w, h, cell float64) {
	if dst == nil || white == nil || w <= 0 || h <= 0 || cell <= 0 {
		return
	}
	arm, th := BracketGeom(cell)
	arm = math.Min(arm, math.Min(w, h)/2)
	th = math.Min(th, arm)
	var op ebiten.DrawImageOptions
	strip := func(sx, sy, sw, sh float64) {
		op.GeoM.Reset()
		op.GeoM.Scale(sw, sh)
		op.GeoM.Translate(sx, sy)
		dst.DrawImage(white, &op)
	}
	r, b := x+w, y+h
	strip(x, y, arm, th) // top-left
	strip(x, y+th, th, arm-th)
	strip(r-arm, y, arm, th) // top-right
	strip(r-th, y+th, th, arm-th)
	strip(x, b-th, arm, th) // bottom-left
	strip(x, b-arm, th, arm-th)
	strip(r-arm, b-th, arm, th) // bottom-right
	strip(r-th, b-arm, th, arm-th)
}
