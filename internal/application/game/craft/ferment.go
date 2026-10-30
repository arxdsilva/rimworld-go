package craft

import (
	"fmt"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Fermenting barrels (PRD-4, wiki: Fermenting barrel). A barrel takes up
// to recipe.BarrelWort wort through its fill bill and ferments it into as
// much beer over FermentTicks, but only between FermentMinC and
// FermentMaxC; from SpoilMinC to FermentMinC it pauses, and outside
// SpoilMinC..FermentMaxC it spoils, ruining the batch after RuinTicks
// there. Wort added later stretches the time left in proportion. The
// finished beer is set down on the barrel's spot. A barrel's batch shares
// the saved rack map (Game.Racks): Kind wort, Qty its wort, Left the
// full-speed ticks still to ferment, Rot the ticks it has spent too hot
// or too cold.

// Barrel constants. wiki: Fermenting barrel (6 days, ideal 7 to 32 C,
// paused from -1 to 7 C, ruined outside -1 to 32 C).
const (
	FermentTicks = 6 * clock.TicksPerDay
	FermentMinC  = 7
	FermentMaxC  = 32
	SpoilMinC    = -1
	// RuinTicks is how long a batch lasts too hot or too cold before it
	// is ruined (the wiki gives no figure: original tuning, one day).
	RuinTicks = clock.TicksPerDay
	// TopUpPct: a barrel takes more wort only while it is under this far
	// along (original tuning). Once past it, the barrel stays closed
	// until it is emptied, so haulers never flap between topping up and
	// waiting (TestBarrelFillNoFlap).
	TopUpPct = 10
)

// isBarrel reports a fermenting barrel.
func isBarrel(k construction.Kind) bool { return k == construction.FermentingBarrel }

// barrelOpen reports barrel id takes wort now: empty, or not full and
// still under TopUpPct fermented.
func (d *Drying) barrelOpen(id entity.EntityID) bool {
	b := d.racks[id]
	return b.Qty == 0 || b.Qty < recipe.BarrelWort && b.Percent() < TopUpPct
}

// barrelRoom is how much more wort barrel id takes.
func (d *Drying) barrelRoom(id entity.EntityID) int {
	if !d.barrelOpen(id) {
		return 0
	}
	return recipe.BarrelWort - d.racks[id].Qty
}

// pour adds stack st (wort) to barrel id, stretching the time left in
// proportion; what does not fit is left on the barrel's spot (x, y).
func (d *Drying) pour(id entity.EntityID, st item.Stack, x, y int) {
	b := d.racks[id]
	add := min(st.Qty, d.barrelRoom(id))
	if add > 0 {
		total := b.Qty + add
		b.Left = (b.Left*b.Qty + FermentTicks*add) / total
		b.Kind, b.Qty = item.Wort, total
		d.racks[id] = b
	}
	if rest := st.Qty - add; rest > 0 {
		st.Qty, st.Carrier, st.X, st.Y = rest, 0, x, y
		d.h.DropStackOf(st, x, y)
	}
}

// ferment moves barrel bid's batch on by interval ticks at the barrel's
// air temperature: it ferments, pauses or spoils; a finished batch is set
// down as beer on the spot, a ruined one is lost.
func (d *Drying) ferment(bid entity.EntityID, b construction.Building, rb Batch, interval int) {
	t := d.h.TemperatureAt(b.X, b.Y)
	switch {
	case t >= FermentMinC && t <= FermentMaxC:
		rb.Left -= interval
	case t < SpoilMinC || t > FermentMaxC:
		rb.Rot += interval
	}
	_, spot, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
	switch {
	case rb.Rot >= RuinTicks:
		delete(d.racks, bid)
		d.h.Message(spot[0], spot[1], fmt.Sprintf("%d wort spoiled in a fermenting barrel", rb.Qty))
	case rb.Left <= 0:
		delete(d.racks, bid)
		d.h.DropStack(item.Beer, rb.Qty, spot[0], spot[1])
		d.h.Message(spot[0], spot[1], fmt.Sprintf("%d beer ready", rb.Qty))
	default:
		d.racks[bid] = rb
	}
}
