package ebiten

import (
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Pawn orders (UI checklist §3.2, §13.1): draft toggle and the
// right-click order menu. All actions are typed commands; the menu
// content and its disabled reasons come from Simulation.OrdersAt.

const (
	floatRowH = 22
	floatMinW = 200
)

type floatMenu struct {
	open   bool
	x, y   float32
	opts   []game.OrderOption
	tx, ty int
}

// floatHintH is the footer row under the order menu.
const floatHintH = 18

func (a *App) closeFloat() { a.float = floatMenu{} }

func floatLabel(o game.OrderOption) string {
	if o.Disabled && o.Reason != "" {
		return o.Label + "  - " + o.Reason
	}
	return o.Label
}

func (a *App) floatRect() button {
	w := floatMinW
	for _, o := range a.float.opts {
		if n := int(textW(floatLabel(o), tsBody)) + 28; n > w {
			w = n
		}
	}
	h := len(a.float.opts)*floatRowH + floatHintH
	x, y := a.float.x, a.float.y
	if sw, sh := a.size(); sw > 0 {
		if x+float32(w) > float32(sw) {
			x = float32(sw - w - 4)
		}
		if y+float32(h) > hudBarY(sh) {
			y = hudBarY(sh) - float32(h) - 4
		}
	}
	return button{x: x, y: y, w: float32(w), h: float32(h)}
}

func (a *App) floatButtons() []button {
	if !a.float.open {
		return nil
	}
	r := a.floatRect()
	out := make([]button, 0, len(a.float.opts))
	for i, o := range a.float.opts {
		out = append(out, button{
			id: "float." + strconv.Itoa(i), label: floatLabel(o),
			x: r.x, y: r.y + float32(i*floatRowH), w: r.w, h: floatRowH,
			disabled: o.Disabled, reason: o.Reason,
		})
	}
	return out
}

func (a *App) drawFloat(screen *ebiten.Image) {
	if !a.float.open {
		return
	}
	r := a.floatRect()
	drawPanel(screen, r.x-2, r.y-2, r.w+4, r.h+4)
	drawText(screen, "Shift: queue (max 3)", float64(r.x)+10, float64(r.y+r.h)-floatHintH, withColor(tsSmall, uiTextDim))
	mx, my := a.currentCursor()
	for _, b := range a.floatButtons() {
		fill := uiBtn
		if b.disabled {
			fill = uiBtnDisabled
		} else if b.contains(mx, my) {
			fill = uiBtnHover
		}
		uiFillRect(screen, b.x, b.y, b.w, b.h, fill, false)
		uiStrokeRect(screen, b.x, b.y, b.w, b.h, 1, uiPanelBorder, false)
		ts := tsBody
		if b.disabled {
			ts = withColor(tsBody, uiTextDim)
		}
		drawText(screen, ellipsize(b.label, ts, float64(b.w)-20), float64(b.x)+10, float64(b.y)+(float64(b.h)-lineH(ts))/2, ts)
	}
}

func (a *App) chooseFloat(id string) {
	i, err := strconv.Atoi(strings.TrimPrefix(id, "float."))
	if err != nil || i < 0 || i >= len(a.float.opts) || a.renderer == nil {
		return
	}
	o := a.float.opts[i]
	if o.Disabled {
		a.hint = o.Reason
		return
	}
	c := o.Cmd
	c.Queue = a.queueing() // Shift: queue behind the current order
	a.renderer.enqueue(c)
	a.closeFloat()
}

// openOrders opens the order menu for the selected pawn at tile (tx, ty).
func (a *App) openOrders(tx, ty int, sx, sy float32) {
	p, ok := a.selectedPawn()
	if !ok {
		return
	}
	opts := a.renderer.game.OrdersAt(p.ID, tx, ty)
	if len(opts) == 0 {
		a.closeFloat()
		return
	}
	a.float = floatMenu{open: true, x: sx + 6, y: sy + 4, opts: opts, tx: tx, ty: ty}
}

// rightClickTile is the right-click on tile (tx, ty) with a pawn selected.
// A drafted colonist walks to open ground at once; on a raider or animal it
// gets the order menu instead, so Shoot / Melee are reachable.
func (a *App) rightClickTile(tx, ty int, sx, sy float32) {
	p, ok := a.selectedPawn()
	if !ok {
		return
	}
	if p.Drafted && !a.creatureAt(tx, ty, p.ID) {
		a.closeFloat()
		a.renderer.enqueue(command.Command{Kind: command.MovePawn, Pawn: uint64(p.ID), X: tx, Y: ty, Queue: a.queueing()})
		return
	}
	a.openOrders(tx, ty, sx, sy)
}

// creatureAt reports whether a raider or animal stands on the tile, so a
// drafted colonist's right-click offers Shoot / Melee instead of walking.
func (a *App) creatureAt(tx, ty int, self entity.EntityID) bool {
	for _, q := range a.renderer.hostiles() {
		if q.ID != self && q.X == tx && q.Y == ty {
			return true
		}
	}
	_, ok := animalAt(a.renderer.world(), tx, ty)
	return ok
}

func (a *App) toggleDraft() {
	p, ok := a.selectedPawn()
	if !ok || a.renderer == nil {
		return
	}
	a.renderer.enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(p.ID), Enabled: !p.Drafted})
}
