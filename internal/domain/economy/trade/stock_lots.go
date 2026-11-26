// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package trade

import (
	"cmp"
	"slices"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/statehash"
)

// Lot is a physical item acquired by a trader. ID is trader-local,
// monotonically assigned, and distinct from a colony EntityID.
type Lot struct {
	ID    uint64
	Stack item.Stack
}

// CarryStack keeps item attributes without transferring world placement,
// access permissions or provenance to the trader's inventory.
func CarryStack(s item.Stack, qty int) item.Stack {
	return item.Stack{
		Kind: s.Kind, Qty: qty, Rot: s.Rot, Dried: s.Dried, Of: s.Of,
		DiedAt: s.DiedAt, Animal: s.Animal, Yield: s.Yield, Hatch: s.Hatch,
		Tame: s.Tame, Poison: s.Poison, Quality: s.Quality, Wear: s.Wear,
		Stuff: s.Stuff, Ingredients: s.Ingredients, IngredientSpecies: s.IngredientSpecies,
		Building: s.Building, MaterialBasis: s.MaterialBasis, Book: s.Book,
	}
}

// Detailed records even a non-priced property (e.g. a book's seed or
// cooked meal's ingredients) that must survive a later trade or drop.
func Detailed(s item.Stack) bool {
	return CarryStack(s, s.Qty) != (item.Stack{Kind: s.Kind, Qty: s.Qty})
}

// OrdinaryQty is the portion represented only by the stock's base kind.
func (l Line) OrdinaryQty() int {
	n := l.Qty
	for _, lot := range l.Lots {
		n -= lot.Stack.Qty
	}
	return n
}

// CloneLines takes independent copies of the optional detailed items.
func CloneLines(lines []Line) []Line {
	out := slices.Clone(lines)
	for i := range out {
		out[i].Lots = slices.Clone(lines[i].Lots)
	}
	return out
}

// AddLine merges one acquired or unmodified line into the Goods key.
// Lot IDs are already assigned by the validated settlement.
func AddLine(lines []Line, part Line) []Line {
	i, found := slices.BinarySearchFunc(lines, part.Goods(), func(l Line, g Goods) int {
		return l.Goods().Compare(g)
	})
	if !found {
		part.Lots = slices.Clone(part.Lots)
		return slices.Insert(lines, i, part)
	}
	lines[i].Qty += part.Qty
	for _, lot := range part.Lots {
		j, found := slices.BinarySearchFunc(lines[i].Lots, lot.ID, func(l Lot, id uint64) int {
			return cmp.Compare(l.ID, id)
		})
		if found {
			lines[i].Lots[j].Stack.Qty += lot.Stack.Qty
		} else {
			lines[i].Lots = slices.Insert(lines[i].Lots, j, lot)
		}
	}
	return lines
}

// canTake reports whether an individual owned line can fulfill a
// request. Carrier manifests may have several lines with the same Goods.
func canTake(line, part Line) bool {
	if line.Goods() != part.Goods() || part.Qty <= 0 || part.Qty > line.Qty {
		return false
	}
	lotQty := 0
	var previous uint64
	for _, wanted := range part.Lots {
		if wanted.ID <= previous {
			return false
		}
		previous = wanted.ID
		j, ok := slices.BinarySearchFunc(line.Lots, wanted.ID, func(l Lot, id uint64) int {
			return cmp.Compare(l.ID, id)
		})
		if !ok || wanted.Stack.Qty <= 0 || wanted.Stack.Qty > line.Lots[j].Stack.Qty {
			return false
		}
		actual, expected := line.Lots[j].Stack, wanted.Stack
		actual.Qty, expected.Qty = 0, 0
		if actual != expected {
			return false
		}
		lotQty += wanted.Stack.Qty
	}
	return part.Qty >= lotQty && part.Qty-lotQty <= line.OrdinaryQty()
}

// TakeLine removes exactly the represented ordinary portion and
// particular lots. The caller applies it to a copy before committing.
func TakeLine(lines []Line, part Line) ([]Line, bool) {
	i, found := slices.BinarySearchFunc(lines, part.Goods(), func(l Line, g Goods) int {
		return l.Goods().Compare(g)
	})
	if !found {
		return lines, false
	}
	for i < len(lines) && lines[i].Goods() == part.Goods() && !canTake(lines[i], part) {
		i++
	}
	if i >= len(lines) || lines[i].Goods() != part.Goods() {
		return lines, false
	}
	target := &lines[i]
	for _, wanted := range part.Lots {
		j, _ := slices.BinarySearchFunc(target.Lots, wanted.ID, func(l Lot, id uint64) int {
			return cmp.Compare(l.ID, id)
		})
		target.Lots[j].Stack.Qty -= wanted.Stack.Qty
		if target.Lots[j].Stack.Qty == 0 {
			target.Lots = slices.Delete(target.Lots, j, j+1)
		}
	}
	target.Qty -= part.Qty
	if target.Qty == 0 {
		lines = slices.Delete(lines, i, i+1)
	}
	return lines, true
}

// ApplyTrade updates the authoritative stock with the same physical
// buy/sell lines that were validated and assigned to the cargo manifest.
func (s *Stock) ApplyTrade(res Result) bool {
	lines := CloneLines(s.Lines)
	for _, bought := range res.Buy {
		var ok bool
		lines, ok = TakeLine(lines, bought)
		if !ok {
			return false
		}
	}
	for _, sold := range res.Sell {
		if sold.Kind != item.Silver {
			lines = AddLine(lines, sold)
		}
	}
	for _, sold := range res.Sell {
		for _, lot := range sold.Lots {
			s.NextLotID = max(s.NextLotID, lot.ID)
		}
	}
	s.Lines = lines
	s.Silver -= res.Silver
	for _, sold := range res.Sell {
		if sold.Kind == item.Silver {
			s.Silver += sold.Qty
		}
	}
	return true
}

// DropStacks returns the exact purchased or lost physical items and the
// remaining ordinary stock in standard stack-size units.
func DropStacks(b item.Book, l Line) []item.Stack {
	if l.Kind == item.None || int(l.Kind) >= b.Len() {
		return nil
	}
	var out []item.Stack
	for _, lot := range l.Lots {
		for left := lot.Stack.Qty; left > 0; {
			n := min(left, b.StackLimit(lot.Stack.Kind))
			out = append(out, CarryStack(lot.Stack, n))
			left -= n
		}
	}
	for left := l.OrdinaryQty(); left > 0; {
		n := min(left, b.StackLimit(l.Kind))
		out = append(out, item.Stack{
			Kind: l.Kind, Building: l.Building, Stuff: CrateStuff(l.Building), Qty: n,
		})
		left -= n
	}
	return out
}

// PhysicalStacks groups interchangeable world cargo by the complete transferable
// payload. IDs do not depend on slice position, quantity or save/load order.
// The side bit separates the two inventories within a single offer.
func PhysicalStacks(stacks []item.Stack, trader bool) ([]GiftStack, bool) {
	byID := make(map[uint64]item.Stack)
	for _, s := range stacks {
		if s.Forbidden || s.Qty <= 0 {
			continue
		}
		key := CarryStack(s, 1)
		id := max(uint64(1), statehash.Of(key)&(1<<63-1))
		if trader {
			id |= 1 << 63
		}
		if previous, ok := byID[id]; ok {
			if CarryStack(previous, 1) != key || s.Qty > int(^uint(0)>>1)-previous.Qty {
				return nil, false
			}
			s.Qty += previous.Qty
		}
		byID[id] = CarryStack(s, s.Qty)
	}
	out := make([]GiftStack, 0, len(byID))
	for id, st := range byID {
		out = append(out, GiftStack{ID: id, Stack: st})
	}
	slices.SortFunc(out, func(a, b GiftStack) int { return cmp.Compare(a.ID, b.ID) })
	return out, true
}

// Clone detaches stock and all physical lot slices for transaction preflight.
func (s Stock) Clone() Stock {
	s.Lines = CloneLines(s.Lines)
	return s
}
