package ebiten

import (
	"image/color"
	"strconv"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
)

// Order queue UI: Shift while choosing an order (or right-clicking with a
// drafted colonist) queues it behind the current order. The selected
// colonist's queue is listed in the pane's lines ("Queued orders 2/3" and
// each order; cmds_pawn.go) and cleared by the Clear orders command; its
// route is drawn on the map as numbered waypoints.

// drawOrderRoute draws the selected colonist's current order target and
// queued orders as a dotted route with numbered waypoints.
func (r *Renderer) drawOrderRoute(screen *ebiten.Image, tile float64) {
	if !r.hasSelect {
		return
	}
	var p game.PawnView
	found := false
	for _, q := range r.drawPawns {
		if q.ID == r.selected {
			p, found = q, true
		}
	}
	if !found || !p.HasOrder && len(p.QueuedOrders) == 0 {
		return
	}
	wx, wy := r.interpWorld(p)
	type pt struct{ x, y float64 }
	pts := []pt{{wx + 0.5, wy + 0.5}}
	if p.HasOrder {
		pts = append(pts, pt{float64(p.OrderX) + 0.5, float64(p.OrderY) + 0.5})
	}
	for _, q := range p.QueuedOrders {
		pts = append(pts, pt{float64(q.X) + 0.5, float64(q.Y) + 0.5})
	}
	line := routeTint(uiAccent, 200)
	for i := 1; i < len(pts); i++ {
		ax, ay := r.worldToScreenF(pts[i-1].x, pts[i-1].y)
		bx, by := r.worldToScreenF(pts[i].x, pts[i].y)
		dashed(screen, ax, ay, bx, by, float32(tile*0.12), line)
	}
	n := 0
	for i := 1; i < len(pts); i++ {
		sx, sy := r.worldToScreenF(pts[i].x, pts[i].y)
		rad := float32(tile * 0.17)
		fill := routeTint(uiBtn, 230)
		if i == 1 && p.HasOrder {
			fill = routeTint(uiBtnSelected, 235) // the order in progress
		}
		vector.FillCircle(screen, sx, sy, rad, fill, true)
		vector.StrokeCircle(screen, sx, sy, rad, 1.5, line, true)
		label := ">" // the order in progress
		if i > 1 || !p.HasOrder {
			n++
			label = strconv.Itoa(n)
		}
		k := 1 / uiScale() // sx, sy are physical (world); text is logical
		drawText(screen, label, float64(sx)*k, float64(sy)*k-lineH(tsSmall)/2, textStyle{Size: tsSmall.Size, Bold: true, Align: alignCenter})
	}
}

// routeTint is palette token c at alpha a, premultiplied the way the
// vector fills expect.
func routeTint(c color.RGBA, a uint8) color.RGBA {
	k := func(v uint8) uint8 { return uint8(uint16(v) * uint16(a) / 255) }
	return color.RGBA{R: k(c.R), G: k(c.G), B: k(c.B), A: a}
}

// dashed draws a dashed line from a to b.
func dashed(screen *ebiten.Image, ax, ay, bx, by, dash float32, c color.RGBA) {
	dx, dy := bx-ax, by-ay
	l := float32(0)
	if d := dx*dx + dy*dy; d > 0 {
		l = sqrt32(d)
	}
	if l < 1 || dash < 2 {
		return
	}
	steps := int(l / dash)
	for i := 0; i < steps; i += 2 {
		t0, t1 := float32(i)/float32(steps), float32(i+1)/float32(steps)
		vector.StrokeLine(screen, ax+dx*t0, ay+dy*t0, ax+dx*t1, ay+dy*t1, 2, c, true)
	}
}

func sqrt32(v float32) float32 {
	x := v
	for i := 0; i < 12; i++ {
		x = (x + v/x) / 2
	}
	return x
}
