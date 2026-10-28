package uikit

import (
	"fmt"
	"math"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

const progressBarH = 14

// DrawProgress draws a progress bar in r: the label top-left, the phase
// text top-right and the bar under them with the percentage in it.
func DrawProgress(dst *ebiten.Image, r gfx.Rect, label string, frac float64, phase string) {
	frac = math.Max(0, math.Min(1, frac))
	lh := float32(gfx.LineH(gfx.Label))
	hd := gfx.Label
	hd.Color = Heading
	gfx.Text(dst, label, float64(r.X), float64(r.Y), hd)
	if phase != "" {
		ps := gfx.Small
		ps.Color, ps.Align = TextDim, gfx.AlignRight
		gfx.Text(dst, FitText(phase, gfx.Small, r.W*0.6), float64(r.Right()), float64(r.Y)+(float64(lh)-gfx.LineH(ps))/2, ps)
	}
	bar := gfx.Rect{X: r.X, Y: r.Y + lh + 4, W: r.W, H: min(progressBarH, max(8, r.H-lh-4))}
	FillRect(dst, bar, SliderTrack)
	FillRect(dst, gfx.Rect{X: bar.X, Y: bar.Y, W: bar.W * float32(frac), H: bar.H}, Accent)
	StrokeRect(dst, bar, PanelBorder)
	ps := gfx.Small
	ps.Align, ps.Color = gfx.AlignCenter, Text
	if frac > 0.5 {
		ps.Color = Field
	}
	cx, cy := bar.Center()
	gfx.Text(dst, fmt.Sprintf("%d%%", int(math.Round(frac*100))), float64(cx), float64(cy)-gfx.LineH(ps)/2, ps)
}
