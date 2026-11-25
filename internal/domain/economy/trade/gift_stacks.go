// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package trade

import (
	"cmp"
	"slices"

	"rimworld-go/internal/domain/economy/item"
)

// GiftStack is an eligible physical colony stack. The game supplies only
// stacks available at the actual trading location.
type GiftStack struct {
	ID    uint64
	Stack item.Stack
}

// GiftAllocation is one resolved physical removal from a gift offer.
type GiftAllocation struct {
	ID    uint64
	Qty   int
	Stack item.Stack
}

// GiftIndividual is true for items whose material, damage, craftsmanship
// or construction identity needs a separate gift row.
func GiftIndividual(st item.Stack) bool {
	return st.Kind == item.MinifiedBuilding || item.HasQuality(st.Kind) ||
		st.Quality != 0 || st.Stuff != 0 || st.Wear != 0 ||
		item.StackValue(st) != st.Kind.Value()
}

// GiftStackCenti is a gift stack's selling price per item.
// wiki: Factions / Goodwill from gifts; StackValue is the project's
// authoritative physical quality, material and wear adjustment.
func GiftStackCenti(b item.Book, st item.Stack, method GiftMethod, social int) int {
	if st.Kind == item.MinifiedBuilding {
		return GiftGoodsCenti(b, Goods{Kind: st.Kind, Building: st.Building}, method, social)
	}
	return GiftUnitCenti(st.Kind, stackMarketValue(b, st), method, social)
}

// PlanGift resolves specific IDs first and then generic kind requests
// against the same eligible inventory. Generic rows consume ordinary
// stacks first; named high-value items cannot silently replace a bulk
// gift. Stale IDs, wrong kinds, repeated IDs and exhausted stacks fail
// before the caller makes any mutation.
func PlanGift(offer []Delta, stock []GiftStack) ([]GiftAllocation, bool) {
	return planSales(offer, stock, true, false)
}

// planSales separates paid ordinary inventory from individually quoted lots.
// Legacy kind-only offers can consume detailed lots only after ordinary stock;
// each allocation is still valued from its actual physical attributes.
func planSales(offer []Delta, stock []GiftStack, gift, exact bool) ([]GiftAllocation, bool) {
	individual := Detailed
	if gift {
		individual = GiftIndividual
	}
	ordered := slices.Clone(stock)
	slices.SortFunc(ordered, func(a, b GiftStack) int { return cmp.Compare(a.ID, b.ID) })
	byID := make(map[uint64]item.Stack, len(ordered))
	for _, v := range ordered {
		if v.ID == 0 || v.Stack.Qty <= 0 {
			return nil, false
		}
		if _, duplicate := byID[v.ID]; duplicate {
			return nil, false
		}
		byID[v.ID] = v.Stack
	}
	used := make(map[uint64]int, len(ordered))
	var plan []GiftAllocation
	for _, d := range offer {
		if d.Qty >= 0 {
			return nil, false
		}
		if d.StackID == 0 {
			continue
		}
		st, ok := byID[d.StackID]
		if !ok || st.Kind != d.Kind || st.Building != d.Building ||
			used[d.StackID] != 0 || -d.Qty > st.Qty {
			return nil, false
		}
		used[d.StackID] = -d.Qty
		plan = append(plan, GiftAllocation{d.StackID, -d.Qty, st})
	}
	for _, d := range offer {
		if d.StackID != 0 {
			continue
		}
		left := -d.Qty
		passes := 2
		if exact {
			passes = 1
		}
		for pass := 0; pass < passes && left > 0; pass++ {
			for _, v := range ordered {
				st := v.Stack
				if st.Kind != d.Kind || st.Building != d.Building ||
					individual(st) != (pass == 1) {
					continue
				}
				n := min(left, st.Qty-used[v.ID])
				if n <= 0 {
					continue
				}
				used[v.ID] += n
				plan = append(plan, GiftAllocation{v.ID, n, st})
				left -= n
				if left == 0 {
					break
				}
			}
		}
		if left != 0 {
			return nil, false
		}
	}
	return plan, true
}

// GiftValueAllocations prices the actual physical stacks selected and
// removed by the game. All fractional cents are aggregated once.
func GiftValueAllocations(b item.Book, plan []GiftAllocation, terms Terms) (int, bool) {
	method := GiftVisitor
	if terms.ImproveCenti == 200 {
		method = GiftSettlement
	}
	centi := 0
	for _, a := range plan {
		unit := GiftStackCenti(b, a.Stack, method, terms.Social)
		var ok bool
		centi, ok = addCenti(centi, a.Qty, unit)
		if !ok {
			return 0, false
		}
	}
	return centi / 100, true
}

// CoalesceAllocations returns one debit per source, preserving first-seen
// order, and checks that all allocations refer to the same original stack.
func CoalesceAllocations(plan []GiftAllocation) ([]GiftAllocation, bool) {
	var out []GiftAllocation
	index := make(map[uint64]int, len(plan))
	for _, a := range plan {
		if a.ID == 0 || a.Qty <= 0 || a.Qty > a.Stack.Qty {
			return nil, false
		}
		if i, ok := index[a.ID]; ok {
			if out[i].Stack != a.Stack || a.Qty > a.Stack.Qty-out[i].Qty {
				return nil, false
			}
			out[i].Qty += a.Qty
		} else {
			index[a.ID] = len(out)
			out = append(out, a)
		}
	}
	return out, true
}
