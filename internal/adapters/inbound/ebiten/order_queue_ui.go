package ebiten

import (
	"fmt"
	"image/color"
	"strconv"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
)

// Order queue UI: Shift while choosing an order (or right-clicking with a
// drafted colonist) queues it behind the current order. The selected
// colonist's queue is listed under the inspector with a Clear button, and
// its route is drawn on the map as numbered waypoints.

const queueRowH = 18

// queueing reports the queue modifier held.
func queueing() bool {
	return ebiten.IsKeyPressed(ebiten.KeyShift)
}

// orderQueueRect is the panel listing n queued orders below the inspector.
func orderQueueRect(n int) button {
	r := inspectorRect()
	return button{x: r.x, y: r.y + r.h + 6, w: r.w, h: float32(30 + n*queueRowH)}
}

func (a *App) orderQueueButtons() []button {
	p, ok := a.selectedPawn()
	if !ok || len(p.QueuedOrders) == 0 || a.workOpen || a.archOpen {
		return nil
	}
	r := orderQueueRect(len(p.QueuedOrders))
	return []button{{id: "pawn.clearorders", label: "Clear", x: r.x + r.w - 70, y: r.y + 4, w: 62, h: 20}}
}

func (a *App) drawOrderQueue(screen *ebiten.Image) {
	p, ok := a.selectedPawn()
	if !ok || len(p.QueuedOrders) == 0 || a.workOpen || a.archOpen {
		return
	}
	r := orderQueueRect(len(p.QueuedOrders))
	drawPanel(screen, r.x, r.y, r.w, r.h)
	drawLabel(screen, int(r.x)+12, int(r.y)+7, fmt.Sprintf("Queued orders %d/%d", len(p.QueuedOrders), game.MaxQueuedOrders))
	for i, q := range p.QueuedOrders {
		drawLabel(screen, int(r.x)+16, int(r.y)+28+i*queueRowH, fmt.Sprintf("%d. %s", i+1, q.Label))
	}
	mx, my := a.currentCursor()
	a.drawButtons(screen, a.orderQueueButtons(), mx, my)
}

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
	line := color.RGBA{250, 220, 120, 200}
	for i := 1; i < len(pts); i++ {
		ax, ay := r.worldToScreenF(pts[i-1].x, pts[i-1].y)
		bx, by := r.worldToScreenF(pts[i].x, pts[i].y)
		dashed(screen, ax, ay, bx, by, float32(tile*0.12), line)
	}
	n := 0
	for i := 1; i < len(pts); i++ {
		sx, sy := r.worldToScreenF(pts[i].x, pts[i].y)
		rad := float32(tile * 0.17)
		fill := color.RGBA{60, 46, 24, 230}
		if i == 1 && p.HasOrder {
			fill = color.RGBA{150, 110, 30, 235} // the order in progress
		}
		vector.FillCircle(screen, sx, sy, rad, fill, true)
		vector.StrokeCircle(screen, sx, sy, rad, 1.5, line, true)
		label := ">" // the order in progress
		if i > 1 || !p.HasOrder {
			n++
			label = strconv.Itoa(n)
		}
		ebitenutil.DebugPrintAt(screen, label, int(sx)-3, int(sy)-8)
	}
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
