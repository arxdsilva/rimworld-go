package hud

import (
	"slices"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/render/creature"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"strconv"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// The colonist bar's right button (CTL-011, CTL-012): press on a portrait and
// drag it onto another of the same map to move it there; release on the same
// one, having barely moved, to open the world view on its map.

const (
	// BarClickSlop is how far (px) a right press may wander and still be a click.
	BarClickSlop = 6
	// BarCrossMapDragHint is why a drag onto another map's portrait does nothing.
	BarCrossMapDragHint = "The bar orders each map's colonists apart: drop on a colonist of the same map."
)

// barDrag is the right-button gesture in flight.
type barDrag struct {
	on     bool
	id     entity.EntityID
	m      game.MapID
	x, y   int  // where it was pressed
	cx, cy int  // where the cursor is
	moved  bool // it has strayed past BarClickSlop since the press
}

// byBarOrder orders the bar; stable, so equal orders keep the colonists' own.
func byBarOrder(a, b game.PawnView) int { return a.BarOrder - b.BarOrder }

// barSorted is pawns in bar order. A slice already in order comes back as is;
// otherwise a copy sits in the bar's reused buffer (never the caller's slice).
func (c ColonistBar) barSorted(pawns []game.PawnView) []game.PawnView {
	if slices.IsSortedFunc(pawns, byBarOrder) {
		return pawns
	}
	buf := append(c.bar.ord[:0], pawns...)
	slices.SortStableFunc(buf, byBarOrder)
	c.bar.ord = buf
	return buf
}

// barSlotAt is the index in the bar's list of the portrait under (x, y).
func (c ColonistBar) barSlotAt(pawns []game.PawnView, x, y int) (int, bool) {
	l := c.sh.Layout()
	for i := range min(BarShown(l), len(pawns), len(l.BarSlots)) {
		if l.BarSlots[i].Hit(x, y) {
			return i, true
		}
	}
	return 0, false
}

// HandleRight drives the right-button gesture from this frame's input and
// reports the bar owns it (a press on a portrait, taken; then every frame
// until the release).
func (c ColonistBar) HandleRight(f *inputframe.Frame) bool {
	d := &c.bar.drag
	x, y := f.Ui.Mx, f.Ui.My
	if !d.on {
		if !c.Visible() || !f.Clicked(inputframe.Right) {
			return false
		}
		pawns := c.Pawns()
		i, ok := c.barSlotAt(pawns, x, y)
		if !ok {
			return false
		}
		f.Take(inputframe.Right)
		*d = barDrag{on: true, id: pawns[i].ID, m: pawns[i].Map, x: x, y: y, cx: x, cy: y}
		return true
	}
	d.cx, d.cy = x, y
	d.moved = d.moved || abs(x-d.x) > BarClickSlop || abs(y-d.y) > BarClickSlop
	if f.Down(inputframe.Right) && !f.Released(inputframe.Right) {
		return true
	}
	g := *d
	*d = barDrag{}
	if !f.Released(inputframe.Right) {
		return false // the button came up unseen: drop the gesture
	}
	c.finishDrag(g, x, y)
	return true
}

// finishDrag ends the gesture at (x, y).
func (c ColonistBar) finishDrag(g barDrag, x, y int) {
	pawns := c.Pawns()
	i, ok := c.barSlotAt(pawns, x, y)
	if !ok {
		return
	}
	over := pawns[i]
	switch {
	case over.ID == g.id:
		if !g.moved {
			if d, ok := c.sh.(ports.Dispatcher); ok {
				d.Do(ports.OpenWorld{Select: "map:" + strconv.FormatUint(uint64(g.m), 10)})
			}
		}
	case over.Map != g.m:
		c.Toast(BarCrossMapDragHint)
	default:
		c.sh.Enqueue(command.Command{Kind: command.SetColonistOrder, Pawn: uint64(g.id), Count: over.BarOrder}) // one rank across every map; i counts this list's groups
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// drawDrag paints the gesture: the portrait under the cursor and, over a
// colonist of the same map, a mark where the dragged one would land.
func (c ColonistBar) drawDrag(screen *ebiten.Image, pawns []game.PawnView) {
	d := c.bar.drag
	if !d.on || !d.moved {
		return
	}
	l := c.sh.Layout()
	if i, ok := c.barSlotAt(pawns, d.cx, d.cy); ok && pawns[i].ID != d.id && pawns[i].Map == d.m {
		s := l.BarSlots[i]
		gfx.FillRect(screen, s.X-2, s.Y, 3, s.H, uikit.Accent, false)
	}
	for _, p := range pawns {
		if p.ID != d.id {
			continue
		}
		w, h := l.BarSlots[0].W, l.BarSlots[0].H
		card := barCardOf(gfx.Rect{X: float32(d.cx) - w/2, Y: float32(d.cy) - h/2, W: w, H: h}, l.BarScale)
		c.drawCard(screen, card, p, true, false, nil)
		gfx.StrokeRect(screen, card.slot.X, card.slot.Y, card.slot.W, card.slot.H, 1, creature.Fade(uikit.Accent, 0.8), false)
		return
	}
}
