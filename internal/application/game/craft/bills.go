package craft

import (
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Bill status strings for the read model (game.BillView.Status).
const (
	BillActive     = ""
	BillNoInput    = "waiting for ingredients"
	BillDrying     = "drying a batch"
	BillFermenting = "fermenting" // PRD-4: a barrel full or under way
	BillNoFuel     = "fire needs wood"
	BillNoPower    = "no power"
	BillDone       = "done"
	BillSuspended  = "suspended"
	BillHaveEnough = "have enough"
)

// Bills is every bench's bills. Saved (Game.Bills) as the map itself.
type Bills struct {
	h      Host
	p      planner
	drying *Drying
	list   map[entity.EntityID][]recipe.Bill
}

// Of is bench bid's bills: the live slice, so a caller may change a bill
// in place but never appends to it.
func (b *Bills) Of(bid entity.EntityID) []recipe.Bill { return b.list[bid] }

// Set replaces bench bid's bills.
func (b *Bills) Set(bid entity.EntityID, bills []recipe.Bill) { b.list[bid] = bills }

// Forget drops a removed bench's bills.
func (b *Bills) Forget(bid entity.EntityID) { delete(b.list, bid) }

// Export is the bills for a save (the live map, read only).
func (b *Bills) Export() map[entity.EntityID][]recipe.Bill { return b.list }

// Import loads saved bills (nil, an older save: kept as they are).
func (b *Bills) Import(m map[entity.EntityID][]recipe.Bill) {
	if m != nil {
		b.list = m
	}
}

// Apply runs a bill command (AddBill, SetBill, RemoveBill; PRD-1's
// ConfigureBill, SetBillIngredient, ReorderBill and RES-1's
// SetStorageSource in settings.go) on the bench at (c.X, c.Y).
func (b *Bills) Apply(c command.Command) {
	bid, bench, ok := b.benchAt(c.X, c.Y)
	if !ok {
		return
	}
	bills := b.list[bid]
	switch c.Kind {
	case command.AddBill:
		d := recipe.DefOf(recipe.ID(c.Recipe))
		if d.Name == "" || !d.At(bench.Kind) || b.h.RecipeLocked(d.ID) {
			return
		}
		target := c.Count
		if target <= 0 {
			target = 1
		}
		bills = append(bills, recipe.Bill{Recipe: d.ID, Mode: recipe.Mode(c.Mode), Target: target})
	case command.SetBill:
		if c.Index < 0 || c.Index >= len(bills) {
			return
		}
		bl := bills[c.Index]
		bl.Mode = recipe.Mode(c.Mode)
		if c.Count > 0 {
			bl.Target = c.Count
		}
		bl.Suspended = c.Enabled
		bills[c.Index] = bl
	case command.RemoveBill:
		if c.Index < 0 || c.Index >= len(bills) {
			return
		}
		bills = append(bills[:c.Index:c.Index], bills[c.Index+1:]...)
	case command.ConfigureBill, command.SetBillIngredient, command.ReorderBill, command.SetStorageSource:
		if c.Index < 0 || c.Index >= len(bills) {
			return
		}
		b.applySetting(bid, bills, c)
	}
	b.list[bid] = bills
}

// benchAt is the completed bench building covering (x, y).
func (b *Bills) benchAt(x, y int) (entity.EntityID, construction.Building, bool) {
	bid := b.h.ObjectAt(x, y)
	bench, ok := b.h.Bench(bid)
	if !ok {
		return 0, construction.Building{}, false
	}
	return bid, bench, true
}

// Defaults gives a new bench the bills nearly everyone wants: a butcher
// table or spot butchers every carcass type and a drying rack dries any
// perishable food, forever; a campfire cooks until there are 10 meals; a
// funeral pyre cremates every body brought to it.
func (b *Bills) Defaults(id entity.EntityID, k construction.Kind) {
	k = construction.RecipeBench(k) // PLAY-021: an electric bench gets its base kind's bills
	if k == construction.ButcherSpot {
		k = construction.ButcherTable // PRD-3: the spot butchers like the table
	}
	if k == construction.Pyre {
		b.list[id] = append(b.list[id], recipe.Bill{Recipe: recipe.Cremate, Mode: recipe.Forever, Target: 1})
		return
	}
	if k == construction.Campfire {
		b.list[id] = append(b.list[id], recipe.Bill{Recipe: recipe.CampfireMeal, Mode: recipe.UntilHave, Target: 10})
		return
	}
	if k != construction.ButcherTable && k != construction.DryingRack && k != construction.FermentingBarrel {
		return
	}
	for _, d := range recipe.For(k) {
		if d.Output == item.Kibble {
			continue // PRD-4: kibble is made to order, never by default
		}
		b.list[id] = append(b.list[id], recipe.Bill{Recipe: d.ID, Mode: recipe.Forever, Target: 1})
	}
}

// AnyFor reports a bench holding a bill of work type t. It spares a
// doctor with nothing to tend the full bench scan when no bench has
// doctoring bills (the usual case). It ranges over the bills map, a
// handful of benches, rather than every building: an existence check, so
// the map's order never shows in the result.
func (b *Bills) AnyFor(t work.Type) bool {
	for _, bills := range b.list {
		for _, bl := range bills {
			if recipe.DefOf(bl.Recipe).Skill == t {
				return true
			}
		}
	}
	return false
}

// UpgradeLegacy rewrites an older save's recipes to today's: the
// per-species butcher bills become one butcher-animal bill. Each bench is
// rewritten on its own, so the map's order never shows.
func (b *Bills) UpgradeLegacy() {
	for bid, bills := range b.list {
		out, has := bills[:0:0], false
		changed := false
		for _, bl := range bills {
			up := recipe.Upgrade(bl.Recipe)
			if up != bl.Recipe {
				changed = true
				if has {
					continue // one butcher-animal bill does them all
				}
				bl.Recipe = up
			}
			if bl.Recipe == recipe.ButcherAnimal {
				has = true
			}
			out = append(out, bl)
		}
		if changed {
			b.list[bid] = out
		}
	}
}

// stock is the colony's stock of a bill's output: dried food for a
// drying bill, the standing pieces for a sculpture (PRD-3), otherwise
// the output kind.
func (b *Bills) stock(d recipe.Def) int {
	switch {
	case d.Dry:
		return driedStock(b.h)
	case d.Builds != 0:
		n := 0
		for _, id := range b.h.BuildingIDs() {
			if o, ok := b.h.Building(id); ok && o.Kind == d.Builds {
				n++
			}
		}
		return n
	}
	return b.h.Stock(d.Output)
}

// Line is bill i of bench bid as the read model shows it: the bill with
// its "until you have" state brought up to date (a copy: views never
// mutate) and its status (one of the Bill* strings).
func (b *Bills) Line(bid entity.EntityID, i int) (recipe.Bill, string) {
	bl := b.list[bid][i]
	d := recipe.DefOf(bl.Recipe)
	stock := b.stock(d)
	bl = bl.Update(stock)
	switch {
	case bl.Suspended:
		return bl, BillSuspended
	case !bl.Wants(stock) && bl.Mode == recipe.DoCount:
		return bl, BillDone
	case !bl.Wants(stock):
		return bl, BillHaveEnough
	case d.Dry && b.drying.busy(bid):
		return bl, BillDrying
	case d.Ferment && !b.drying.barrelOpen(bid):
		return bl, BillFermenting
	case b.unfueled(bid), d.ID == recipe.Cremate && b.h.FreeWood() < d.ExtraQty:
		return bl, BillNoFuel
	case b.unpowered(bid):
		return bl, BillNoPower
	}
	var spot [2]int // only the radius needs it
	if bl.Radius > 0 {
		spot = b.spot(bid)
	}
	if !b.p.available(bl, d, spot) {
		return bl, BillNoInput
	}
	return bl, BillActive
}

// unfueled: bid is a campfire or forge with no fuel in it (PRD-3).
func (b *Bills) unfueled(bid entity.EntityID) bool {
	bench, ok := b.h.Building(bid)
	return ok && Fueled(bench.Kind) && b.h.Fuel(bid) <= 0
}

// unpowered: bid is an electric bench without power (PLAY-021).
func (b *Bills) unpowered(bid entity.EntityID) bool {
	bench, ok := b.h.Building(bid)
	return ok && ElectricBench(bench.Kind) && !b.h.Powered(bid)
}
