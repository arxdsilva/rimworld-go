package learn

import (
	"fmt"
	"image/color"
	"math"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
)

// mix blends a toward b by t (0: a, 1: b), keeping a's alpha.
func mix(a, b color.RGBA, t float64) color.RGBA {
	m := func(x, y uint8) uint8 { return uint8(math.Round(float64(x)*(1-t) + float64(y)*t)) }
	return color.RGBA{m(a.R, b.R), m(a.G, b.G), m(a.B, b.B), a.A}
}

// fade scales a colour (alpha included) by k, clamped to 0..1.
func fade(c color.RGBA, k float64) color.RGBA {
	k = max(0, min(k, 1))
	return color.RGBA{uint8(float64(c.R) * k), uint8(float64(c.G) * k), uint8(float64(c.B) * k), uint8(float64(c.A) * k)}
}

// Draw draws the helper: the panel, its lessons and the detail card.
func (h *Helper) Draw(screen *ebiten.Image) {
	lay := h.Layout()
	p := lay.Panel
	if p.Empty() {
		return
	}
	mx, my := h.sh.Cursor()
	flashTitle, flashing := h.Flash()
	pulse := 1.0
	if flashing && !h.sh.Settings().Accessibility.ReducedFlashing {
		pulse = 0.65 + 0.35*math.Sin(float64(h.sh.Updates())/5)
	}
	collapsed := h.Folded(p)

	gfx.FillRect(screen, p.X, p.Y, p.W, p.H, uikit.TutorPanel, false)
	if flashing {
		gfx.FillRect(screen, p.X, p.Y, p.W, p.H, fade(uikit.TutorHeading, 0.28*pulse), false)
	}
	gfx.StrokeRect(screen, p.X, p.Y, p.W, p.H, 1, uikit.TutorBorder, false)

	// header: title, what is left, and the fold arrow
	if lay.Header.Hit(mx, my) {
		gfx.FillRect(screen, p.X+1, p.Y+1, p.W-2, learnHeaderH-1, fade(uikit.TutorHeading, 0.14), false)
	}
	hy := float64(p.Y) + (learnHeaderH-gfx.LineH(gfx.Label))/2
	gfx.Text(screen, "Learning helper", float64(p.X)+10, hy, gfx.WithColor(gfx.Label, uikit.TutorHeading))
	right := "Done!"
	if !flashing {
		right = fmt.Sprintf("%d left", h.count())
	}
	st := gfx.WithColor(gfx.Small, uikit.TutorText)
	st.Align = gfx.AlignRight
	gfx.Text(screen, right, float64(p.Right())-28, float64(p.Y)+(learnHeaderH-gfx.LineH(gfx.Small))/2, st)
	drawChevron(screen, p.Right()-16, p.Y+learnHeaderH/2, collapsed)

	if !collapsed {
		drawRows(screen, lay, mx, my)
		note := fmt.Sprintf("%d of %d learned", len(Catalog)-h.count(), len(Catalog))
		nst := gfx.WithColor(gfx.Small, mix(uikit.TutorText, uikit.TutorPanel, 0.2))
		if flashing {
			note, nst = "Done: "+flashTitle, gfx.WithColor(gfx.Small, uikit.TutorHeading)
		}
		gfx.Text(screen, uikit.FitText(note, nst, p.W-20), float64(p.X)+10, float64(lay.Footer.Y)+(learnFooterH-gfx.LineH(gfx.Small))/2, nst)
	}
	if lay.Open != nil && !lay.Card.Empty() {
		drawCard(screen, lay, mx, my)
	}
}

// drawChevron draws the fold arrow at (cx, cy): down while open, right
// while folded.
func drawChevron(screen *ebiten.Image, cx, cy float32, collapsed bool) {
	if collapsed {
		gfx.Line(screen, cx-2, cy-5, cx+3, cy, 2, uikit.TutorHeading, true)
		gfx.Line(screen, cx+3, cy, cx-2, cy+5, 2, uikit.TutorHeading, true)
		return
	}
	gfx.Line(screen, cx-5, cy-2, cx, cy+3, 2, uikit.TutorHeading, true)
	gfx.Line(screen, cx, cy+3, cx+5, cy-2, 2, uikit.TutorHeading, true)
}

// drawRows draws the listed lessons.
func drawRows(screen *ebiten.Image, lay Layout, mx, my int) {
	dark := mix(uikit.TutorPanel, color.RGBA{A: 0xFF}, 0.28)
	for _, rw := range lay.Rows {
		r := rw.R
		fill := dark
		switch {
		case lay.Open == rw.L:
			fill = mix(dark, uikit.TutorBorder, 0.45)
		case r.Hit(mx, my):
			fill = mix(dark, uikit.TutorBorder, 0.25)
		}
		gfx.FillRect(screen, r.X, r.Y, r.W, r.H, fill, false)
		accent := uikit.TutorBorder
		if lay.Open == rw.L {
			accent = uikit.TutorHeading
		}
		gfx.FillRect(screen, r.X, r.Y, 4, r.H, accent, false)
		gfx.StrokeRect(screen, r.X, r.Y, r.W, r.H, 1, uikit.TutorBorder, false)
		inner := r.W - learnRowPad*2
		block := gfx.LineH(gfx.Label) + gfx.LineH(gfx.Small)
		ty := float64(r.Y) + (float64(r.H)-block)/2
		gfx.Text(screen, uikit.FitText(rw.L.Title, gfx.Label, inner), float64(r.X)+learnRowPad, ty, gfx.WithColor(gfx.Label, uikit.TutorHeading))
		gfx.Text(screen, uikit.FitText(rw.L.Goal, gfx.Small, inner), float64(r.X)+learnRowPad, ty+gfx.LineH(gfx.Label), gfx.WithColor(gfx.Small, uikit.TutorText))
	}
}

// drawCard draws the detail card of the open lesson.
func drawCard(screen *ebiten.Image, lay Layout, mx, my int) {
	c := lay.Card
	gfx.FillRect(screen, c.X, c.Y, c.W, c.H, uikit.TutorPanel, false)
	gfx.StrokeRect(screen, c.X, c.Y, c.W, c.H, 1, uikit.TutorBorder, false)
	x := float64(c.X) + learnCardPad
	y := float64(c.Y) + learnCardPad
	gfx.Text(screen, uikit.FitText(lay.Open.Title, gfx.Label, c.W-2*learnCardPad), x, y, gfx.WithColor(gfx.Label, uikit.TutorHeading))
	y += float64(gfx.LineStep(gfx.Label)) + 6
	for _, ln := range lay.Body {
		gfx.Text(screen, ln, x, y, gfx.WithColor(gfx.Body, uikit.TutorText))
		y += float64(gfx.LineStep(gfx.Body))
	}
	y += 8
	for _, ln := range lay.Goal {
		gfx.Text(screen, ln, x, y, gfx.WithColor(gfx.Small, uikit.TutorHeading))
		y += float64(gfx.LineStep(gfx.Small))
	}
	g := lay.GotIt
	fill := mix(uikit.TutorPanel, color.RGBA{A: 0xFF}, 0.3)
	if g.Hit(mx, my) {
		fill = mix(fill, uikit.TutorBorder, 0.45)
	}
	gfx.FillRect(screen, g.X, g.Y, g.W, g.H, fill, false)
	gfx.StrokeRect(screen, g.X, g.Y, g.W, g.H, 1, uikit.TutorBorder, false)
	cx, cy := g.Center()
	uikit.DrawCentered(screen, "Got it", float64(cx), float64(cy), gfx.WithColor(gfx.Label, uikit.TutorHeading))
}
