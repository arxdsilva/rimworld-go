// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package maps

import (
	"slices"

	"rimworld-go/internal/application/game/travel"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/sim/statehash"
	"rimworld-go/internal/domain/world/caravan"
)

// SplitView is a detached, quote-bound view of the native partition
// options. No simulation rules need to be reproduced in presentation.
func (f Fleet) SplitView(c *Caravan, goods []item.Stack) travel.SplitView {
	var v travel.SplitView
	switch {
	case c == nil || c.C.State != caravan.Travelling && c.C.State != caravan.Returning && c.C.State != caravan.Arrived && c.C.State != caravan.Visiting:
		v.Reason = "Only an established caravan can split."
	case c.Ask.Letter != 0:
		v.Reason = "Resolve the encounter before splitting."
	case len(c.C.Members) < 2:
		v.Reason = "Both caravans need a member."
	case slices.ContainsFunc(f.a.Queue, func(r Request) bool { return r.Caravan == c.C.ID }):
		v.Reason = "Finish the pending map transition before splitting."
	}
	if v.Reason != "" {
		return v
	}
	v.Animals = f.h.SplitCargo().SplitAnimals(c.C.ID)
	v.Animals = append(v.Animals, travel.StashedSplitAnimals(f.h.MarketCarriers(c))...)
	slices.SortFunc(v.Animals, func(a, b travel.SplitAnimal) int {
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
	seen := make(map[uint64]bool)
	for _, a := range v.Animals {
		if a.ID == 0 || seen[uint64(a.ID)] {
			v.Reason = "The animal ownership is inconsistent."
			return v
		}
		seen[uint64(a.ID)] = true
	}
	for _, id := range c.C.Animals {
		if !seen[id] {
			v.Reason = "The animal ownership is inconsistent."
			return v
		}
	}
	for _, id := range c.C.Members {
		held, leader := f.h.HeldMember(entity.EntityID(id), entity.EntityID(c.C.ID))
		if !held || seen[id] {
			v.Reason = "The member ownership is inconsistent."
			return v
		}
		seen[id] = true
		if leader {
			v.Leaders = append(v.Leaders, id)
		}
	}
	if len(v.Leaders) < 2 {
		v.Reason = "Both caravans need an able free colonist."
		return v
	}
	v.Quote = max(uint64(1), statehash.Of(struct {
		ID               uint64
		Tile             int
		State            caravan.State
		Members, Animals []uint64
		Choices          []travel.SplitAnimal
		Leaders          []uint64
		Cargo            []item.Stack
	}{c.C.ID, c.C.Tile, c.C.State, c.C.Members, c.C.Animals, v.Animals, v.Leaders, goods}))
	return v
}

func splitIDs(ids, chosen []uint64) (left, right []uint64) {
	for _, id := range ids {
		if slices.Contains(chosen, id) {
			right = append(right, id)
		} else {
			left = append(left, id)
		}
	}
	return
}
func validSelection(ids, allowed []uint64) bool {
	seen := make(map[uint64]bool, len(ids))
	for _, id := range ids {
		if seen[id] || !slices.Contains(allowed, id) {
			return false
		}
		seen[id] = true
	}
	return true
}

// Split partitions an established caravan, preserving its route,
// progress, return/visit state, cooldown and per-member food budget.
// Both sides must retain an able non-prisoner and fit their own cargo.
// Invalid or stale requests consume no IDs, goods, food or RNG.
func (f Fleet) Split(cid, quote uint64, payload string) (uint64, string) {
	c := f.At(entity.EntityID(cid))
	if c == nil {
		return 0, "No such caravan."
	}
	sel, ok := travel.DecodeSplit(payload)
	if !ok {
		return 0, "Invalid caravan split selection."
	}
	v := f.SplitView(c, f.h.Cargo().Goods(cid))
	if v.Reason != "" {
		return 0, v.Reason
	}
	if quote == 0 || quote != v.Quote {
		return 0, "The caravan changed; refresh the split selection."
	}
	var animals []uint64
	for _, a := range v.Animals {
		animals = append(animals, uint64(a.ID))
	}
	if !validSelection(sel.Members, c.C.Members) || !validSelection(sel.Animals, animals) {
		return 0, "Invalid member or animal selection."
	}
	l, r := splitIDs(v.Leaders, sel.Members)
	if len(l) == 0 || len(r) == 0 {
		return 0, "Both caravans need an able free colonist."
	}
	a, b := c.Clone(), c.Clone()
	a.C.Members, b.C.Members = splitIDs(c.C.Members, sel.Members)
	a.C.Animals, b.C.Animals = splitIDs(c.C.Animals, sel.Animals)
	cargo := f.h.SplitCargo()
	plan, why := cargo.PlanSplit(cid, sel, CarrierCount(c.C.Members, f.h.Alive), CarrierCount(b.C.Members, f.h.Alive), f.h.MarketCarriers(c))
	if why != "" {
		return 0, why
	}
	b.C.ID = f.h.NewID()
	cargo.CommitSplit(cid, b.C.ID, plan)
	for _, id := range b.C.Members {
		f.h.TransferHeld(entity.EntityID(id), entity.EntityID(b.C.ID))
	}
	for _, animal := range v.Animals {
		if animal.Stashed && slices.Contains(sel.Animals, uint64(animal.ID)) {
			f.h.TransferHeld(animal.ID, entity.EntityID(b.C.ID))
		}
	}
	f.pace(&a)
	f.pace(&b)
	*c = a
	*f.list = append(*f.list, b)
	return b.C.ID, ""
}
