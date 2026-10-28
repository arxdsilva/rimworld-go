package orders

import (
	"image/color"
	"strconv"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
)

// Order queue UI: Shift while choosing an order (or right-clicking with a
// drafted colonist) queues it behind the current order. The selected
// colonist's queue is listed in the pane's lines ("Queued orders 2/3" and
// each order; cmds_pawn.go) and cleared by the Clear orders command; its
// route is drawn on the map as numbered waypoints.

// DrawRoute draws the colonist p's current order target and queued orders
// as a dotted route with numbered waypoints. (wx, wy) is where p is drawn
// now (interpolated) and toScreen maps a world point to map pixels. The
// renderer calls it, so it is a function, not a Controller method.
func DrawRoute(screen *ebiten.Image, tile float64, p game.PawnView, wx, wy float64, toScreen func(x, y float64) (float32, float32)) {
	if !p.HasOrder && len(p.QueuedOrders) == 0 {
		return
	}
	type pt struct{ x, y float64 }
	pts := []pt{{wx + 0.5, wy + 0.5}}
	if p.HasOrder {
		pts = append(pts, pt{float64(p.OrderX) + 0.5, float64(p.OrderY) + 0.5})
	}
	for _, q := range p.QueuedOrders {
		pts = append(pts, pt{float64(q.X) + 0.5, float64(q.Y) + 0.5})
	}
	line := RouteTint(uikit.Accent, 200)
	for i := 1; i < len(pts); i++ {
		ax, ay := toScreen(pts[i-1].x, pts[i-1].y)
		bx, by := toScreen(pts[i].x, pts[i].y)
		Dashed(screen, ax, ay, bx, by, float32(tile*0.12), line)
	}
	n := 0
	for i := 1; i < len(pts); i++ {
		sx, sy := toScreen(pts[i].x, pts[i].y)
		rad := float32(tile * 0.17)
		fill := RouteTint(uikit.Btn, 230)
		if i == 1 && p.HasOrder {
			fill = RouteTint(uikit.BtnSelected, 235) // the order in progress
		}
		vector.FillCircle(screen, sx, sy, rad, fill, true)
		vector.StrokeCircle(screen, sx, sy, rad, 1.5, line, true)
		label := ">" // the order in progress
		if i > 1 || !p.HasOrder {
			n++
			label = strconv.Itoa(n)
		}
		k := 1 / gfx.UIScale() // sx, sy are physical (world); text is logical
		gfx.Text(screen, label, float64(sx)*k, float64(sy)*k-gfx.LineH(gfx.Small)/2, gfx.TextStyle{Size: gfx.Small.Size, Bold: true, Align: gfx.AlignCenter})
	}
}

// RouteTint is palette token c at alpha a, premultiplied the way the
// vector fills expect.
func RouteTint(c color.RGBA, a uint8) color.RGBA {
	k := func(v uint8) uint8 { return uint8(uint16(v) * uint16(a) / 255) }
	return color.RGBA{R: k(c.R), G: k(c.G), B: k(c.B), A: a}
}

// Dashed draws a dashed line from a to b.
func Dashed(screen *ebiten.Image, ax, ay, bx, by, dash float32, c color.RGBA) {
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
