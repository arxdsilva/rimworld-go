// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package craft

import (
	"slices"
	"strconv"

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
	edits  map[entity.EntityID]command.ConfigEdit
	book   recipe.Book // the run's recipes (PLG-025; zero: core)
}

// UseBook sets the run's recipes (game.Content.Recipes): every bill's
// recipe ID is read in it.
func (b *Bills) UseBook(book recipe.Book) { b.book = book }

// Book is the run's recipes.
func (b *Bills) Book() recipe.Book { return b.book }

// Of is bench bid's bills: the live slice, so a caller may change a bill
// in place but never appends to it.
func (b *Bills) Of(bid entity.EntityID) []recipe.Bill { return b.list[bid] }

// Set replaces bench bid's bills.
func (b *Bills) Set(bid entity.EntityID, bills []recipe.Bill) { b.list[bid] = bills }

// Forget drops a removed bench's bills.
func (b *Bills) Forget(bid entity.EntityID) { delete(b.list, bid); delete(b.edits, bid) }

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
func (b *Bills) Apply(c command.Command) (entity.EntityID, command.ConfigEdit) {
	bid, bench, ok := b.benchAt(c.X, c.Y)
	if !ok {
		return 0, command.ConfigEdit{}
	}
	bills := b.list[bid]
	idx := c.Index
	if c.Kind != command.AddBill && (idx < 0 || idx >= len(bills)) {
		return 0, command.ConfigEdit{}
	}
	var row uint64
	rev := b.rowHighWater(bid) + 1
	switch c.Kind {
	case command.AddBill:
		d := b.book.DefOf(recipe.ID(c.Recipe))
		if d.Name == "" || !d.At(bench.Kind) || d.Legacy || b.h.RecipeLocked(d.ID) {
			return 0, command.ConfigEdit{}
		}
		target := max(1, c.Count)
		idx = len(bills)
		bills = append(bills, recipe.Bill{Recipe: d.ID, Mode: recipe.Mode(c.Mode), Target: target})
	case command.SetBill:
		bl := bills[idx]
		bl.Mode = recipe.Mode(c.Mode)
		if c.Count > 0 {
			bl.Target = c.Count
		}
		bl.Suspended = c.Enabled
		bills[idx] = bl
	case command.RemoveBill:
		row = bills[idx].Identity
		bills = append(bills[:idx:idx], bills[idx+1:]...)
		b.remapJobs(bid, bills, idx, -1)
	case command.ConfigureBill, command.SetBillIngredient, command.ReorderBill, command.SetStorageSource, command.SetBillRange:
		idx = b.applySetting(bid, bills, c)
		if idx < 0 {
			return 0, command.ConfigEdit{}
		}
	default:
		return 0, command.ConfigEdit{}
	}
	if b.edits == nil {
		b.edits = make(map[entity.EntityID]command.ConfigEdit)
	}
	if c.Kind != command.RemoveBill {
		row = bills[idx].Identity
		if row == 0 {
			row = rev
		}
		bills[idx].Identity, bills[idx].Revision = row, rev
	} else if row == 0 {
		row = rev
	}
	stamp := command.ConfigEdit{Revision: rev, Tick: b.h.Now(), Kind: c.Kind, Row: row, Index: idx}
	b.list[bid], b.edits[bid] = bills, stamp
	return bid, stamp
}

// rowHighWater includes the identities of existing rows even before their
// first configuration edit. Removed identities cannot be reused because
// the next edit advances the saved revision beyond this high water mark.
func (b *Bills) rowHighWater(bid entity.EntityID) uint64 {
	v := b.edits[bid].Revision
	for _, row := range b.list[bid] {
		v = max(v, row.Identity)
	}
	return v
}

// BindOrder records the actual bench and selected row at admission, not
// the transient screen index. Name is the existing saved command payload.
func (b *Bills) BindOrder(c *command.Command) bool {
	bid, ok := b.BenchID(c.X, c.Y)
	if !ok || c.Target != 0 && c.Target != uint64(bid) || c.Index < -1 || c.Index >= len(b.list[bid]) {
		return false
	}
	c.Target = uint64(bid)
	if c.Index == -1 {
		c.Name = "*"
		return true
	}
	c.Name = strconv.FormatUint(b.ensureIdentity(bid, c.Index), 10)
	return true
}

// ResolveOrder refuses removed rows or replacement benches instead of
// silently running a different recipe. Unbound historical queued orders
// cannot prove their original row and are refused as stale.
func (b *Bills) ResolveOrder(c command.Command) (entity.EntityID, int, bool) {
	bid, ok := b.BenchID(c.X, c.Y)
	if !ok || c.Target != uint64(bid) {
		return 0, 0, false
	}
	if c.Name == "*" && c.Index == -1 {
		return bid, -1, true
	}
	row, err := strconv.ParseUint(c.Name, 10, 64)
	if err != nil || row == 0 {
		return 0, 0, false
	}
	for i, bl := range b.list[bid] {
		if bl.Identity == row {
			return bid, i, true
		}
	}
	return 0, 0, false
}

// Configuration returns a detached copy of the current bill rows and
// the bench's last recorded configuration edit. Runtime work progress
// does not advance the configuration revision.
func (b *Bills) Configuration(bid entity.EntityID) ([]recipe.Bill, command.ConfigEdit) {
	rows := slices.Clone(b.list[bid])
	for i := range rows {
		rows[i].Forbid = slices.Clone(rows[i].Forbid)
	}
	return rows, b.edits[bid]
}

// ExportEdits and ImportEdits carry sparse bench edit metadata in saves.
func (b *Bills) ExportEdits() map[entity.EntityID]command.ConfigEdit  { return b.edits }
func (b *Bills) ImportEdits(m map[entity.EntityID]command.ConfigEdit) { b.edits = m }

// benchAt is the completed bench building covering (x, y).
func (b *Bills) benchAt(x, y int) (entity.EntityID, construction.Building, bool) {
	bid := b.h.ObjectAt(x, y)
	bench, ok := b.h.Bench(bid)
	if !ok {
		return 0, construction.Building{}, false
	}
	return bid, bench, true
}

// BenchID is the completed bench covering (x, y).
func (b *Bills) BenchID(x, y int) (entity.EntityID, bool) {
	bid, _, ok := b.benchAt(x, y)
	return bid, ok
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
	if k == construction.Pyre || k == construction.ElectricCrematorium { // AX-4: the crematorium too
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
	for _, d := range b.book.For(k) {
		if d.Output == item.Kibble || d.Output == item.HumanMeat || int(d.ID) >= recipe.Core().Len() {
			continue // PRD-4: kibble is made to order, never by default; FOOD-3: nor is butchering humans; PLG-025: nor a plugin's recipe
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
			if b.book.DefOf(bl.Recipe).WorkType() == t {
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
			up := b.book.Upgrade(bl.Recipe)
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
	d := b.book.DefOf(bl.Recipe)
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
	case b.unfueled(bid), d.ID == recipe.Cremate && b.pyre(bid) && b.h.FreeWood() < d.ExtraQty:
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

// pyre reports bench bid a funeral pyre (its cremation burns wood).
func (b *Bills) pyre(bid entity.EntityID) bool {
	bench, ok := b.h.Building(bid)
	return ok && bench.Kind == construction.Pyre
}

// CrematoriumWork is the electric crematorium's work per corpse (AX-4,
// wiki: Electric crematorium, 180 ticks).
const CrematoriumWork = 180

// BurnsWood reports a cremation on bench k that burns the recipe's wood:
// the funeral pyre's, never the electric crematorium's.
func BurnsWood(k construction.Kind, d recipe.Def) bool {
	return d.ID == recipe.Cremate && k == construction.Pyre
}

// BillWork is bill d's work on bench k: the crematorium burns a corpse in
// CrematoriumWork, everything else takes the recipe's work.
func BillWork(k construction.Kind, d recipe.Def) int {
	if d.ID == recipe.Cremate && k == construction.ElectricCrematorium {
		return CrematoriumWork
	}
	return d.Work
}

// ensureIdentity binds old/default rows only when work or a selected order
// first needs durable row identity.
func (b *Bills) ensureIdentity(bid entity.EntityID, index int) uint64 {
	row := &b.list[bid][index]
	if row.Identity == 0 {
		row.Identity = b.rowHighWater(bid) + 1
	}
	return row.Identity
}
