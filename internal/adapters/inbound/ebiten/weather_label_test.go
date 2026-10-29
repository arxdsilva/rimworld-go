package ebiten

import (
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/application/game"
)

// PLAY-017: the HUD weather readout lays out in logical pixels. Its wind
// arrow was stroked with raw vector calls at those logical coordinates, so
// at UI scale > 1 it landed at 1/scale of the way across the screen,
// nowhere near its text. It changes curUIScale and drawTrace, so it does
// not run in parallel.
func TestWeatherLabelArrowSitsBetweenItsTextAtScale(t *testing.T) {
	for _, sc := range wlScales {
		for _, wind := range [][2]float64{{1, 0}, {0, 1}, {-1, -1}} {
			wlSetScale(t, sc)
			got := wlTrace(t)
			w := game.WorldView{Weather: "thunderstorm", WindX: wind[0], WindY: wind[1]}
			drawWeatherLabel(ebiten.NewImage(64, 64), 300, 10, w)

			texts, lines := wlOfKind(*got, "text"), wlOfKind(*got, "line")
			if len(texts) != 2 || len(lines) != 3 {
				t.Fatalf("%s wind %v: %d texts and %d lines, want the weather name, the speed and 3 arrow strokes: %+v",
					sc.name, wind, len(texts), len(lines), *got)
			}
			name, speed := texts[0], texts[1]
			x0, y0, x1, y1 := lines[0].X, lines[0].Y, lines[0].X+lines[0].W, lines[0].Y+lines[0].H
			for _, l := range lines[1:] {
				x0, y0 = min(x0, l.X), min(y0, l.Y)
				x1, y1 = max(x1, l.X+l.W), max(y1, l.Y+l.H)
			}
			// a pixel or two of rounding between the logical layout and the glyph boxes
			const tol = 2
			if x0 < name.X+name.W-tol || x1 > speed.X+tol {
				t.Errorf("%s wind %v: arrow x %g..%g not between the name (ends %g) and the speed (starts %g)",
					sc.name, wind, x0, x1, name.X+name.W, speed.X)
			}
			if y0 < name.Y || y1 > name.Y+name.H {
				t.Errorf("%s wind %v: arrow y %g..%g outside the text row %g..%g", sc.name, wind, y0, y1, name.Y, name.Y+name.H)
			}
			// the arrow sits where the layout puts it: 10 logical px past the name
			s := float32(gfx.UIScale())
			cx := (x0 + x1) / 2
			wlNear(t, sc.name+" arrow centre x", float64(cx), float64((300+gfx.TextW(texts[0].Text, gfx.Body)+10)*s), float64(1.5*s))
		}
	}
}
