package orders

import (
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Pawn orders (UI checklist §3.2, §13.1): draft toggle and the
// right-click order menu. All actions are typed commands; the menu
// content and its disabled reasons come from the host's OrdersAt.

// Row sizes of the float menu.
const (
	FloatRowH = 22
	floatMinW = 200
	// FloatHintH is the footer row under the order menu.
	FloatHintH = 18
)

// Float is the open float menu.
type Float struct {
	Open   bool
	X, Y   float32
	Opts   []game.OrderOption
	TX, TY int
	// Pick, when set, makes this a command's options menu (PLAY-018): the
	// entries carry labels and reasons only, picking entry i calls Pick(i),
	// and the menu has no Shift-queue footer.
	Pick func(i int)
	Up   bool // the menu opens upward from Y (Y is the anchor's top)
}

// CloseFloat closes the menu.
func (c *Controller) CloseFloat() { c.Float = Float{} }

func floatLabel(o game.OrderOption) string {
	if o.Disabled && o.Reason != "" {
		return o.Label + "  - " + o.Reason
	}
	return o.Label
}

// FloatRect is the menu's box, kept on the screen and above the tab strip.
func (c *Controller) FloatRect() uikit.Button {
	f := &c.Float
	w := floatMinW
	for _, o := range f.Opts {
		if n := int(gfx.TextW(floatLabel(o), gfx.Body)) + 28; n > w {
			w = n
		}
	}
	h := len(f.Opts) * FloatRowH
	if f.Pick == nil {
		h += FloatHintH
	}
	x, y := f.X, f.Y
	if f.Up {
		y -= float32(h)
	}
	if sw, sh := c.sh.Size(); sw > 0 {
		if x+float32(w) > float32(sw) {
			x = float32(sw - w - 4)
		}
		// the menu stays above the tab strip (the layout's Tabs rect)
		if tabsTop := float32(sh - hudlayout.TabsH); y+float32(h) > tabsTop {
			y = tabsTop - float32(h) - 4
		}
	}
	return uikit.Button{X: x, Y: y, W: float32(w), H: float32(h)}
}

// FloatButtons are the menu's rows ("float.N").
func (c *Controller) FloatButtons() []uikit.Button {
	if !c.Float.Open {
		return nil
	}
	r := c.FloatRect()
	out := make([]uikit.Button, 0, len(c.Float.Opts))
	for i, o := range c.Float.Opts {
		out = append(out, uikit.Button{
			ID: ids.PrefixFloat + strconv.Itoa(i), Label: floatLabel(o),
			X: r.X, Y: r.Y + float32(i*FloatRowH), W: r.W, H: FloatRowH,
			Disabled: o.Disabled, Reason: o.Reason,
		})
	}
	return out
}

// DrawFloat draws the menu.
func (c *Controller) DrawFloat(screen *ebiten.Image) {
	if !c.Float.Open {
		return
	}
	r := c.FloatRect()
	uikit.DrawPanel(screen, r.X-2, r.Y-2, r.W+4, r.H+4)
	if c.Float.Pick == nil {
		gfx.Text(screen, "Shift: queue (max 3)", float64(r.X)+10, float64(r.Y+r.H)-FloatHintH, gfx.WithColor(gfx.Small, uikit.TextDim))
	}
	mx, my := c.sh.Cursor()
	for i, b := range c.FloatButtons() {
		fill := uikit.Btn
		if b.Disabled {
			fill = uikit.BtnDisabled
		} else if b.Contains(mx, my) {
			fill = uikit.BtnHover
		}
		gfx.FillRect(screen, b.X, b.Y, b.W, b.H, fill, false)
		gfx.StrokeRect(screen, b.X, b.Y, b.W, b.H, 1, uikit.PanelBorder, false)
		ts := gfx.Body
		if b.Disabled {
			ts = gfx.WithColor(gfx.Body, uikit.TextDim)
		}
		if ic := FloatIcon(c.Float.Opts[i]); ic != "" {
			gfx.Icon(screen, c.sh.Icon(ic), float64(b.X+b.W)-FloatRowH+3, float64(b.Y)+3, FloatRowH-6)
		}
		gfx.Text(screen, gfx.Ellipsize(b.Label, ts, float64(b.W)-20), float64(b.X)+10, float64(b.Y)+(float64(b.H)-gfx.LineH(ts))/2, ts)
	}
}

// ChooseFloat runs the row the id names: an options menu calls back, an
// order menu enqueues the command (Shift queues it).
func (c *Controller) ChooseFloat(id string) {
	i, err := strconv.Atoi(strings.TrimPrefix(id, ids.PrefixFloat))
	if err != nil || i < 0 || i >= len(c.Float.Opts) {
		return
	}
	o := c.Float.Opts[i]
	if o.Disabled {
		c.sh.Hint(o.Reason)
		return
	}
	if pick := c.Float.Pick; pick != nil {
		c.CloseFloat()
		pick(i)
		return
	}
	cmd := o.Cmd
	cmd.Queue = c.queueing() // Shift: queue behind the current order
	c.sh.Enqueue(cmd)
	c.CloseFloat()
}

// OpenPick opens a menu over entries (labels and reasons; their Cmd is
// unused) at (x, y), upward from y when up. Picking entry i closes the menu
// and calls pick(i); a disabled entry shows its reason and keeps it open.
func (c *Controller) OpenPick(x, y float32, up bool, entries []game.OrderOption, pick func(i int)) {
	c.Float = Float{Open: true, X: x, Y: y, Opts: entries, Pick: pick, Up: up}
}

func (c *Controller) openAt(opts []game.OrderOption, tx, ty int, sx, sy float32) {
	c.Float = Float{Open: true, X: sx + 6, Y: sy + 4, Opts: opts, TX: tx, TY: ty}
}

// OpenOrders opens the order menu for the selected pawn at tile (tx, ty).
func (c *Controller) OpenOrders(tx, ty int, sx, sy float32) {
	p, ok := c.selectedPawn()
	if !ok {
		return
	}
	opts := c.sh.OrdersAt(p.ID, tx, ty)
	if len(opts) == 0 {
		c.CloseFloat()
		return
	}
	c.openAt(opts, tx, ty, sx, sy)
}

// RightClickTile is the right-click on tile (tx, ty) with a pawn selected.
// A drafted colonist walks to open ground at once; on a raider or animal it
// gets the order menu instead, so Shoot / Melee are reachable, and on a
// wounded colonist or prisoner too, so "Tend <name>" is (MED-03).
func (c *Controller) RightClickTile(tx, ty int, sx, sy float32) {
	p, ok := c.selectedPawn()
	if !ok {
		return
	}
	if opts := FireOrders(p, c.sh.World(), tx, ty); opts != nil && !c.creatureAt(tx, ty, p.ID) {
		c.openAt(opts, tx, ty, sx, sy)
		return
	}
	if p.Drafted && !c.creatureAt(tx, ty, p.ID) {
		if opts := c.sh.OrdersAt(p.ID, tx, ty); hasPawnOrder(opts) {
			// MED-03 / SIM-062: a pawn here: the menu offers Tend, Rescue, Arrest...
			c.openAt(opts, tx, ty, sx, sy)
			return
		}
		c.CloseFloat()
		c.sh.Enqueue(command.Command{Kind: command.MovePawn, Pawn: uint64(p.ID), X: tx, Y: ty, Queue: c.queueing()})
		return
	}
	c.OpenOrders(tx, ty, sx, sy)
}

// hasPawnOrder reports an order aimed at a pawn among opts that makes a
// drafted right-click open the menu: an emergency tend (MED-03) or a
// Prioritize with a Target (rescue, capture, tend, feed, strip, carry;
// SIM-062), or an attack (a colonist in a berserk break is no hostile the
// creature check sees; SIM-066). Arrest alone does not: every standing
// colonist offers it, and
// a drafted right-click onto one walks there; Arrest still shows whenever
// the menu opens.
func hasPawnOrder(opts []game.OrderOption) bool {
	for _, o := range opts {
		if o.Cmd.Kind == command.EmergencyTend || o.Cmd.Kind == command.Prioritize && o.Cmd.Order == command.OrderAttack ||
			o.Cmd.Kind == command.Prioritize && o.Cmd.Target != 0 && o.Cmd.Order != command.OrderArrest {
			return true
		}
	}
	return false
}

// creatureAt reports whether a raider or animal stands on the tile, so a
// drafted colonist's right-click offers Shoot / Melee instead of walking.
func (c *Controller) creatureAt(tx, ty int, self entity.EntityID) bool {
	for _, q := range c.sh.Hostiles() {
		if q.ID != self && q.X == tx && q.Y == ty {
			return true
		}
	}
	for _, an := range c.sh.World().Animals {
		if an.X == tx && an.Y == ty {
			return true
		}
	}
	return false
}

// ToggleDraft drafts or undrafts the selected colonist.
func (c *Controller) ToggleDraft() {
	p, ok := c.selectedPawn()
	if !ok {
		return
	}
	c.sh.Enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(p.ID), Enabled: !p.Drafted})
}

// selectedPawn is the selected colonist's view, if any.
func (c *Controller) selectedPawn() (game.PawnView, bool) {
	id, ok := c.sh.Selected()
	if !ok {
		return game.PawnView{}, false
	}
	for _, p := range c.sh.Pawns() {
		if p.ID == id {
			return p, true
		}
	}
	return game.PawnView{}, false
}

// queueing reports the queue modifier (Shift) held.
func (c *Controller) queueing() bool {
	in, _ := c.sh.Frame()
	return in.Shift()
}
