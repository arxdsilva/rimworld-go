package craft

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// applySetting runs a PRD-1 bill-settings command on bill c.Index (in
// range) of bench bid's bills. A changed bill searches for ingredients
// again at once.
func (b *Bills) applySetting(bid entity.EntityID, bills []recipe.Bill, c command.Command) {
	bl := bills[c.Index]
	switch c.Kind {
	case command.ConfigureBill:
		bl = configured(bl, c)
	case command.SetBillIngredient:
		k := item.Kind(c.Stuff)
		if !recipe.DefOf(bl.Recipe).Accepts(k) {
			return
		}
		bl = bl.SetAllowed(k, c.Enabled)
	case command.ReorderBill:
		b.reorder(bid, bills, c.Index, c.Index+c.Count)
		return
	case command.SetStorageSource: // RES-1: take ingredients only from this storage (0: anywhere)
		bl.Source = c.Zone
	}
	bl.SearchAt = 0
	bills[c.Index] = bl
}

// configured is bill bl with ConfigureBill c's settings, each brought
// into range: an unknown store mode is the best stockpile, the radius
// and resume level are at least 0, the skill range lies in 0..20 with
// its ends in order.
func configured(bl recipe.Bill, c command.Command) recipe.Bill {
	bl.Store = recipe.StoreMode(c.Store)
	if !bl.Store.Valid() {
		bl.Store = recipe.BestStockpile
	}
	bl.Zone = c.Zone
	bl.Radius = max(0, c.Radius)
	bl.SkillMin, bl.SkillMax = min(c.SkillMin, skill.MaxLevel), min(c.SkillMax, skill.MaxLevel)
	if bl.SkillMax > 0 && bl.SkillMin > bl.SkillMax {
		bl.SkillMin = bl.SkillMax
	}
	bl.Worker = entity.EntityID(c.Worker)
	bl.UnpauseAt = max(0, c.UnpauseAt)
	return bl.PinStuff(recipe.DefOf(bl.Recipe), item.Kind(c.Stuff)) // PRD-2: 0 or a kind it doesn't take unpins
}

// reorder moves bill from to place to (clamped) in bench bid's bills, and
// points the jobs working the bench's bills at their bills' new places.
func (b *Bills) reorder(bid entity.EntityID, bills []recipe.Bill, from, to int) {
	to = max(0, min(to, len(bills)-1))
	if to == from {
		return
	}
	moved := bills[from]
	if to < from {
		copy(bills[to+1:from+1], bills[to:from])
	} else {
		copy(bills[from:to], bills[from+1:to+1])
	}
	bills[to] = moved
	for _, pid := range b.h.PawnIDs() {
		j, ok := b.h.Job(pid)
		if !ok || j.Dest != bid || j.Kind != job.Bill && j.Kind != job.HaulIngredients {
			continue
		}
		if n := movedIndex(j.BillIndex, from, to); n != j.BillIndex {
			j.BillIndex = n
			b.h.SetJob(pid, j)
		}
	}
}

// movedIndex is where bill i sits after the bill at from moved to to.
func movedIndex(i, from, to int) int {
	switch {
	case i == from:
		return to
	case from < to && i > from && i <= to:
		return i - 1
	case to < from && i >= to && i < from:
		return i + 1
	}
	return i
}

// spot is bench bid's work spot (the bench's own tile when it is gone).
func (b *Bills) spot(bid entity.EntityID) [2]int {
	bench, ok := b.h.Building(bid)
	if !ok {
		return [2]int{}
	}
	_, spot, _ := construction.Footprint(bench.Kind, bench.Facing, bench.X, bench.Y)
	return spot
}
