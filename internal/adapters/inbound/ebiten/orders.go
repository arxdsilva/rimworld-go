package ebiten

import (
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
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
		if n := len(floatLabel(o))*6 + 28; n > w {
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
	ebitenutil.DebugPrintAt(screen, "Shift: queue (max 3)", int(r.x)+10, int(r.y+r.h)-floatHintH+1)
	mx, my := a.currentCursor()
	for _, b := range a.floatButtons() {
		fill := uiBtn
		if b.disabled {
			fill = uiBtnDisabled
		} else if b.contains(mx, my) {
			fill = uiBtnHover
		}
		vector.FillRect(screen, b.x, b.y, b.w, b.h, fill, false)
		vector.StrokeRect(screen, b.x, b.y, b.w, b.h, 1, uiPanelBorder, false)
		drawLabel(screen, int(b.x)+10, int(b.y)+4, b.label)
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
	c.Queue = queueing() // Shift: queue behind the current order
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

// handleOrders: right-click with a pawn selected. Drafted pawns move at
// once; undrafted pawns get the order menu.
func (a *App) handleOrders() {
	if a.renderer == nil || !a.renderer.HasSelect() {
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		a.toggleDraft()
	}
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		return
	}
	mx, my := a.currentCursor()
	sw, sh := a.size()
	if a.colonyChromeHit(mx, my, sw, sh) && !a.floatRect().contains(mx, my) {
		return
	}
	tx, ty := a.renderer.TileAt(mx, my)
	p, ok := a.selectedPawn()
	if !ok {
		return
	}
	if p.Drafted {
		a.closeFloat()
		a.renderer.enqueue(command.Command{Kind: command.MovePawn, Pawn: uint64(p.ID), X: tx, Y: ty, Queue: queueing()})
		return
	}
	a.openOrders(tx, ty, float32(mx), float32(my))
}

func (a *App) toggleDraft() {
	p, ok := a.selectedPawn()
	if !ok || a.renderer == nil {
		return
	}
	a.renderer.enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(p.ID), Enabled: !p.Drafted})
}
