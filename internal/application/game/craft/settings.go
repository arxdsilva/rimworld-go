// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

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
func (b *Bills) applySetting(bid entity.EntityID, bills []recipe.Bill, c command.Command) int {
	bl := bills[c.Index]
	switch c.Kind {
	case command.ConfigureBill:
		bl = configured(bl, c, b.book.DefOf(bl.Recipe))
	case command.SetBillIngredient:
		k := item.Kind(c.Stuff)
		if !b.book.DefTakes(b.book.DefOf(bl.Recipe), k) {
			return -1
		}
		bl = bl.SetAllowed(k, c.Enabled)
	case command.SetBillRange:
		switch c.Mode {
		case command.BillRangeHP:
			bl.Condition = bl.Condition.WithHP(c.X2, c.Y2)
		case command.BillRangeQuality:
			bl.Condition = bl.Condition.WithQuality(c.X2, c.Y2)
		default:
			return -1
		}
	case command.ReorderBill:
		to := max(0, min(c.Index+c.Count, len(bills)-1))
		if to == c.Index {
			return -1
		}
		b.reorder(bid, bills, c.Index, to)
		return to
	case command.SetStorageSource: // RES-1: take ingredients only from this storage (0: anywhere)
		bl.Source = c.Zone
	}
	bl.SearchAt = 0
	bills[c.Index] = bl
	return c.Index
}

// configured is bill bl with ConfigureBill c's settings, each brought
// into range: an unknown store mode is the best stockpile, the radius
// and resume level are at least 0, the skill range lies in 0..20 with
// its ends in order. d is the bill's recipe.
func configured(bl recipe.Bill, c command.Command, d recipe.Def) recipe.Bill {
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
	return bl.PinStuff(d, item.Kind(c.Stuff)) // PRD-2: 0 or a kind it doesn't take unpins
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
	b.remapJobs(bid, bills, from, to)
}

// remapJobs keeps the live index accurate for UI and legacy records.
// A removed row becomes invalid, never the next row with the same recipe.
func (b *Bills) remapJobs(bid entity.EntityID, rows []recipe.Bill, from, to int) {
	for _, pid := range b.h.PawnIDs() {
		j, ok := b.h.Job(pid)
		if next, changed := RemapBillJob(j, bid, rows, from, to); ok && changed {
			b.h.SetJob(pid, next)
		}
	}
}

// RemapBillJob keeps a live or suspended job's saved index consistent.
// Removed rows get an invalid index and fail through the normal job-end
// owner, preserving physical carry, pack contents and reservations.
func RemapBillJob(j job.Record, bid entity.EntityID, rows []recipe.Bill, from, to int) (job.Record, bool) {
	if j.Dest != bid || !billJob(j.Kind) {
		return j, false
	}
	n := editedIndex(j.BillIndex, from, to)
	if j.BillIdentity != 0 {
		n = billOf(j, rows)
		if n >= len(rows) {
			n = -1
		}
	}
	if n == j.BillIndex {
		return j, false
	}
	j.BillIndex = n
	if n < 0 && j.Active() {
		j.State, j.Fail = job.Fail, job.TargetGone
	}
	return j, true
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

func billJob(k job.Kind) bool {
	return k == job.Bill || k == job.HaulIngredients || k == job.PackIngredients
}
func editedIndex(i, from, to int) int {
	if to >= 0 {
		return movedIndex(i, from, to)
	}
	if i == from {
		return -1
	}
	if i > from {
		return i - 1
	}
	return i
}
