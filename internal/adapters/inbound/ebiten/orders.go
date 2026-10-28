package ebiten

import (
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
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
	// pick, when set, makes this a command's options menu (PLAY-018): the
	// entries carry labels and reasons only, picking entry i calls pick(i),
	// and the menu has no Shift-queue footer.
	pick func(i int)
	up   bool // the menu opens upward from y (y is the anchor's top)
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
	h := len(a.float.opts) * floatRowH
	if a.float.pick == nil {
		h += floatHintH
	}
	x, y := a.float.x, a.float.y
	if a.float.up {
		y -= float32(h)
	}
	if sw, sh := a.size(); sw > 0 {
		if x+float32(w) > float32(sw) {
			x = float32(sw - w - 4)
		}
		// the menu stays above the tab strip (the layout's Tabs rect)
		if tabsTop := float32(sh - tabsH); y+float32(h) > tabsTop {
			y = tabsTop - float32(h) - 4
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
			id: ids.PrefixFloat + strconv.Itoa(i), label: floatLabel(o),
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
	if a.float.pick == nil {
		drawText(screen, "Shift: queue (max 3)", float64(r.x)+10, float64(r.y+r.h)-floatHintH, withColor(tsSmall, uiTextDim))
	}
	mx, my := a.currentCursor()
	for i, b := range a.floatButtons() {
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
		if ic := floatIcon(a.float.opts[i]); ic != "" {
			drawIcon(screen, a.icon(ic), float64(b.x+b.w)-floatRowH+3, float64(b.y)+3, floatRowH-6)
		}
		drawText(screen, ellipsize(b.label, ts, float64(b.w)-20), float64(b.x)+10, float64(b.y)+(float64(b.h)-lineH(ts))/2, ts)
	}
}

func (a *App) chooseFloat(id string) {
	i, err := strconv.Atoi(strings.TrimPrefix(id, ids.PrefixFloat))
	if err != nil || i < 0 || i >= len(a.float.opts) || a.renderer == nil {
		return
	}
	o := a.float.opts[i]
	if o.Disabled {
		a.shell.Hint(o.Reason)
		return
	}
	if pick := a.float.pick; pick != nil {
		a.closeFloat()
		pick(i)
		return
	}
	c := o.Cmd
	c.Queue = a.queueing() // Shift: queue behind the current order
	a.shell.Enqueue(c)
	a.closeFloat()
}

// openPick opens a menu over entries (labels and reasons; their Cmd is
// unused) at (x, y), upward from y when up. Picking entry i closes the menu
// and calls pick(i); a disabled entry shows its reason and keeps it open.
func (a *App) openPick(x, y float32, up bool, entries []game.OrderOption, pick func(i int)) {
	a.float = floatMenu{open: true, x: x, y: y, opts: entries, pick: pick, up: up}
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
// gets the order menu instead, so Shoot / Melee are reachable, and on a
// wounded colonist or prisoner too, so "Tend <name>" is (MED-03).
func (a *App) rightClickTile(tx, ty int, sx, sy float32) {
	p, ok := a.selectedPawn()
	if !ok {
		return
	}
	if opts := fireOrders(p, a.shell.World(), tx, ty); opts != nil && !a.creatureAt(tx, ty, p.ID) {
		a.float = floatMenu{open: true, x: sx + 6, y: sy + 4, opts: opts, tx: tx, ty: ty} // fire_world_draw.go
		return
	}
	if p.Drafted && !a.creatureAt(tx, ty, p.ID) {
		if opts := a.renderer.game.OrdersAt(p.ID, tx, ty); hasTendOrder(opts) {
			// MED-03: a wounded pawn here: the menu offers "Tend <name>"
			a.float = floatMenu{open: true, x: sx + 6, y: sy + 4, opts: opts, tx: tx, ty: ty}
			return
		}
		a.closeFloat()
		a.shell.Enqueue(command.Command{Kind: command.MovePawn, Pawn: uint64(p.ID), X: tx, Y: ty, Queue: a.queueing()})
		return
	}
	a.openOrders(tx, ty, sx, sy)
}

// hasTendOrder reports an emergency tend among opts (a wounded colonist
// or prisoner on the tile, MED-03).
func hasTendOrder(opts []game.OrderOption) bool {
	for _, o := range opts {
		if o.Cmd.Kind == command.EmergencyTend {
			return true
		}
	}
	return false
}

// creatureAt reports whether a raider or animal stands on the tile, so a
// drafted colonist's right-click offers Shoot / Melee instead of walking.
func (a *App) creatureAt(tx, ty int, self entity.EntityID) bool {
	for _, q := range a.renderer.hostiles() {
		if q.ID != self && q.X == tx && q.Y == ty {
			return true
		}
	}
	_, ok := animalAt(a.shell.World(), tx, ty)
	return ok
}

func (a *App) toggleDraft() {
	p, ok := a.selectedPawn()
	if !ok || a.renderer == nil {
		return
	}
	a.shell.Enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(p.ID), Enabled: !p.Drafted})
}
