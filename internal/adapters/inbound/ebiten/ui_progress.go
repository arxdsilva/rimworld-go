package ebiten

import (
	"fmt"
	"math"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

const progressBarH = 14

// drawProgress draws a progress bar in r: the label top-left, the phase
// text top-right and the bar under them with the percentage in it.
func drawProgress(dst *ebiten.Image, r rect, label string, frac float64, phase string) {
	frac = math.Max(0, math.Min(1, frac))
	lh := float32(lineH(tsLabel))
	hd := tsLabel
	hd.Color = uiHeading
	drawText(dst, label, float64(r.X), float64(r.Y), hd)
	if phase != "" {
		ps := tsSmall
		ps.Color, ps.Align = uiTextDim, alignRight
		drawText(dst, fitText(phase, tsSmall, r.W*0.6), float64(r.Right()), float64(r.Y)+(float64(lh)-lineH(ps))/2, ps)
	}
	bar := rect{X: r.X, Y: r.Y + lh + 4, W: r.W, H: min(progressBarH, max(8, r.H-lh-4))}
	fillRect(dst, bar, uiSliderTrack)
	fillRect(dst, rect{X: bar.X, Y: bar.Y, W: bar.W * float32(frac), H: bar.H}, uiAccent)
	strokeRect(dst, bar, uiPanelBorder)
	ps := tsSmall
	ps.Align, ps.Color = alignCenter, uiText
	if frac > 0.5 {
		ps.Color = uiField
	}
	cx, cy := bar.Center()
	drawText(dst, fmt.Sprintf("%d%%", int(math.Round(frac*100))), float64(cx), float64(cy)-lineH(ps)/2, ps)
}
