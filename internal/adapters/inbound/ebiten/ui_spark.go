package ebiten

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const sparkPad = 3

// sparkPoints maps vals onto r: x evenly across, y from the smallest value
// (bottom) to the largest (top); a flat series sits mid-height.
func sparkPoints(vals []int, r rect) [][2]float32 {
	if len(vals) == 0 {
		return nil
	}
	lo, hi := vals[0], vals[0]
	for _, v := range vals {
		lo, hi = min(lo, v), max(hi, v)
	}
	in := r.inset(sparkPad)
	pts := make([][2]float32, len(vals))
	for i, v := range vals {
		x := in.X + in.W/2
		if len(vals) > 1 {
			x = in.X + in.W*float32(i)/float32(len(vals)-1)
		}
		y := in.Y + in.H/2
		if hi > lo {
			y = in.bottom() - in.H*float32(v-lo)/float32(hi-lo)
		}
		pts[i] = [2]float32{x, y}
	}
	return pts
}

// drawSparkline draws vals as a small line chart in r over a faint
// baseline, with a dot on the last value.
func drawSparkline(dst *ebiten.Image, r rect, vals []int, c color.Color) {
	base := uiPanelBorder
	base.A = 0x90
	vector.StrokeLine(dst, r.X, r.bottom()-0.5, r.right(), r.bottom()-0.5, 1, base, false)
	pts := sparkPoints(vals, r)
	for i := 1; i < len(pts); i++ {
		vector.StrokeLine(dst, pts[i-1][0], pts[i-1][1], pts[i][0], pts[i][1], 1.5, c, true)
	}
	if n := len(pts); n > 0 {
		vector.FillCircle(dst, pts[n-1][0], pts[n-1][1], 2.5, c, true)
	}
}
