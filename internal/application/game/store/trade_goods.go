// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package store

import (
	"math/bits"
	"slices"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/sim/entity"
)

// TradeGoods is the tradeable physical stock selected from a map or a
// beacon's reach. Selection and consumption share the same item IDs.
type TradeGoods struct {
	ids    map[item.Kind][]entity.EntityID
	counts map[item.Kind]int
	stacks []trade.GiftStack // captured physical attributes at selection time
}

func NewTradeGoods() TradeGoods {
	return TradeGoods{ids: map[item.Kind][]entity.EntityID{}, counts: map[item.Kind]int{}}
}

// Add records one already-validated loose stack in the current selection.
func (g *TradeGoods) Add(id entity.EntityID, st item.Stack) {
	g.ids[st.Kind] = append(g.ids[st.Kind], id)
	g.counts[st.Kind] += st.Qty
	g.stacks = append(g.stacks, trade.GiftStack{ID: uint64(id), Stack: st})
}

// Sort stabilizes the physical removal order for every kind.
func (g *TradeGoods) Sort() {
	for kind := range g.ids {
		slices.Sort(g.ids[kind])
	}
	slices.SortFunc(g.stacks, func(a, b trade.GiftStack) int {
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		default:
			return 0
		}
	})
}

// CollectTrade selects the live, tradeable stacks with a caller-owned
// location rule. The stock module owns selection and ordering.
func CollectTrade(ids []entity.EntityID, stack func(entity.EntityID) (item.Stack, bool),
	allowed func(item.Stack) bool) TradeGoods {
	goods := NewTradeGoods()
	for _, id := range ids {
		if st, ok := stack(id); ok && st.Carrier == 0 && !st.Forbidden &&
			st.Qty > 0 && allowed(st) {
			goods.Add(id, st)
		}
	}
	goods.Sort()
	return goods
}

// Stacks exposes detached item-valued trade selections, in ID order.
// The game must settle against a newly collected selection per command.
func (g TradeGoods) Stacks() []trade.GiftStack { return slices.Clone(g.stacks) }

// Counts exposes the per-kind quantities for an immediate trade quote.
// The result belongs to this ephemeral selection.
func (g TradeGoods) Counts() map[item.Kind]int { return g.counts }

func (g TradeGoods) Count(kind item.Kind) int { return g.counts[kind] }

func (g TradeGoods) Kinds() []item.Kind {
	kinds := make([]item.Kind, 0, len(g.ids))
	for kind := range g.ids {
		kinds = append(kinds, kind)
	}
	slices.Sort(kinds)
	return kinds
}

// IDs reports physical stack IDs in deterministic order for one kind.
func (g TradeGoods) IDs(kind item.Kind) []entity.EntityID { return g.ids[kind] }

// Take consumes up to n from the originally selected stacks. A stack may
// have disappeared or changed since the initial quote; the caller owns the
// live transfer through consume, so no stale quantity gets applied.
func (g TradeGoods) Take(kind item.Kind, n int,
	stack func(entity.EntityID) (item.Stack, bool),
	consume func(entity.EntityID, item.Stack, int)) {
	for _, id := range g.ids[kind] {
		if n <= 0 {
			return
		}
		st, ok := stack(id)
		if !ok {
			continue
		}
		take := min(n, st.Qty)
		consume(id, st, take)
		n -= take
	}
}

// Debits consolidates priced allocations and verifies every physical
// source still matches the captured eligible stack before any mutation.
func (g TradeGoods) Debits(plan []trade.GiftAllocation, live func(entity.EntityID) (item.Stack, bool)) ([]trade.GiftAllocation, bool) {
	debits, valid := trade.CoalesceAllocations(plan)
	if !valid {
		return nil, false
	}
	for _, a := range debits {
		st, exists := live(entity.EntityID(a.ID))
		if !exists || st != a.Stack || a.Qty > st.Qty {
			return nil, false
		}
	}
	return debits, true
}

// CollectBeacon selects loose, visible goods in the powered reach bitset.
func CollectBeacon(cells []uint64, width int, loose func(x, y int) []entity.EntityID,
	stack func(entity.EntityID) (item.Stack, bool), fogged func(x, y int) bool) TradeGoods {
	gd := NewTradeGoods()
	for wi, word := range cells {
		for ; word != 0; word &= word - 1 {
			c := wi<<6 + bits.TrailingZeros64(word)
			for _, id := range loose(c%width, c/width) {
				if st, ok := stack(id); ok && st.Carrier == 0 && !st.Forbidden && st.Qty > 0 && !fogged(c%width, c/width) {
					gd.Add(id, st)
				}
			}
		}
	}
	gd.Sort()
	return gd
}
