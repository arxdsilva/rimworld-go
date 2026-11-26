// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package travel

import (
	"encoding/json"
	"slices"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// SplitSelection moves these member and animal IDs and exact cargo lot
// quantities into the new caravan; omitted IDs/lots remain in the old one.
// Lot indices refer to the quoted CaravanView.Cargo order (zero-based).
type SplitSelection struct {
	Members []uint64
	Animals []uint64
	Lots    []SplitLot
}
type SplitLot struct{ Index, Qty int }

// EncodeSplit creates a typed command payload without adding a saved field.
func EncodeSplit(s SplitSelection) string { b, _ := json.Marshal(s); return string(b) }
func DecodeSplit(s string) (SplitSelection, bool) {
	var v SplitSelection
	err := json.Unmarshal([]byte(s), &v)
	return v, err == nil
}

// SplitAnimal contains just the detached facts needed to allocate pack
// animals. Stashed animals and cargo-held animals have different owners.
type SplitAnimal struct {
	ID      entity.EntityID
	Species animal.Species
	Name    string
	Stashed bool
	Carries bool
}
type SplitView struct {
	Quote   uint64
	Reason  string
	Animals []SplitAnimal
	Leaders []uint64
}

func (o Ops) SplitAnimals(cid uint64) []SplitAnimal {
	var out []SplitAnimal
	if h := o.s.cargo[cid]; h != nil {
		for _, b := range h.Animals {
			out = append(out, splitAnimal(b, false))
		}
	}
	return out
}

func splitAnimal(b Beast, stashed bool) SplitAnimal {
	return SplitAnimal{ID: b.ID, Species: b.State.Species, Name: b.State.Name, Stashed: stashed, Carries: b.State.Tame && !b.State.Dead()}
}

func StashedSplitAnimals(roster TradeCarriers) []SplitAnimal {
	var out []SplitAnimal
	for _, b := range roster.Stashed {
		out = append(out, splitAnimal(b, true))
	}
	return out
}

// SplitPlan holds the two complete private cargo replacements; no live
// data is mutated before both halves pass capacity and ownership checks.
type SplitPlan struct {
	source      uint64
	left, right Hold
	Load        [2]int32
	valid       bool
}

func (o Ops) PlanSplit(cid uint64, sel SplitSelection, members, moved int, roster TradeCarriers) (SplitPlan, string) {
	if moved < 1 || moved >= members {
		return SplitPlan{}, "Both caravans need members."
	}
	p := SplitPlan{source: cid}
	h := o.s.cargo[cid]
	if h == nil {
		h = &Hold{}
	}
	// Strictly validate lot selection before touching either hold.
	quantities := make([]int, len(h.Items))
	for _, lot := range sel.Lots {
		if lot.Index < 0 || lot.Index >= len(h.Items) || lot.Qty <= 0 || quantities[lot.Index] != 0 || lot.Qty > h.Items[lot.Index].Qty {
			return SplitPlan{}, "A selected cargo lot changed or is invalid."
		}
		quantities[lot.Index] = lot.Qty
	}
	for i, st := range h.Items {
		qty := quantities[i]
		if qty > 0 {
			part := st
			part.Qty = qty
			p.right.Items = append(p.right.Items, part)
		}
		if st.Qty > qty {
			st.Qty -= qty
			p.left.Items = append(p.left.Items, st)
		}
		if h.Items[i].Qty <= 0 {
			return SplitPlan{}, "The caravan contains invalid cargo."
		}
	}
	for _, id := range h.Pawns {
		if slices.Contains(sel.Members, uint64(id)) {
			p.right.Pawns = append(p.right.Pawns, id)
		} else {
			p.left.Pawns = append(p.left.Pawns, id)
		}
	}
	for _, b := range h.Animals {
		if slices.Contains(sel.Animals, uint64(b.ID)) {
			p.right.Animals = append(p.right.Animals, b)
		} else {
			p.left.Animals = append(p.left.Animals, b)
		}
	}
	var left, right TradeCarriers
	for _, b := range roster.Stashed {
		if slices.Contains(sel.Animals, uint64(b.ID)) {
			right.Stashed = append(right.Stashed, b)
		} else {
			left.Stashed = append(left.Stashed, b)
		}
	}
	book := item.Book{}
	if o.h != nil {
		book = o.h.Items()
	}
	for i, part := range []struct {
		h       *Hold
		members int
		roster  TradeCarriers
	}{
		{&p.left, members - moved, left}, {&p.right, moved, right},
	} {
		cap := carrierCapacity(part.members, part.h, part.roster)
		room := cap
		for _, st := range part.h.Items {
			grams := int64(book.MassGrams(st.Kind))
			if grams <= 0 || st.Qty <= 0 || int64(st.Qty) > room/grams {
				return SplitPlan{}, "One caravan cannot carry its assigned goods."
			}
			room -= int64(st.Qty) * grams
		}
		p.Load[i] = int32((cap - room) * 1000 / cap)
	}
	p.valid = true
	return p, ""
}

// CommitSplit consumes a validated plan on the single simulation owner.
// A fresh ID is allocated by the fleet only after preflight succeeds.
func (o Ops) CommitSplit(cid, newID uint64, p SplitPlan) {
	if !p.valid || p.source != cid || newID == 0 || newID == cid {
		return
	}
	if p.left.empty() {
		delete(o.s.cargo, cid)
	} else {
		*o.s.cargoOf(cid) = p.left
	}
	if !p.right.empty() {
		*o.s.cargoOf(newID) = p.right
	}
	if o.h != nil {
		o.h.Changed()
	}
}
