// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package travel

import (
	"slices"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
)

// TradePlan is a preflighted, private replacement for one caravan's cargo.
// The market owner must commit the corresponding settlement stock in the
// same authoritative game command.
type TradePlan struct {
	caravan uint64
	items   []item.Stack
	sold    []item.Stack
	valid   bool
}

// Sold returns the actual stacks offered to the settlement, including
// quality, material, wear and other item attributes.
func (p TradePlan) Sold() []item.Stack { return slices.Clone(p.sold) }

// appendTradeGood joins fully identical physical stacks, regardless of
// purchase source. Different quality, make or condition stay separate.
func appendTradeGood(items []item.Stack, st item.Stack) []item.Stack {
	for i := range items {
		if items[i].Kind != st.Kind {
			continue
		}
		same := items[i]
		same.Qty = st.Qty
		if same == st {
			items[i].Qty += st.Qty
			return items
		}
	}
	return append(items, st)
}

// TradeCounts returns amounts the caravan can physically trade. Forbidden
// stacks remain aboard and cannot satisfy an offer.
func (o Ops) TradeCounts(cid uint64) map[item.Kind]int {
	counts := make(map[item.Kind]int)
	if h := o.s.cargo[cid]; h != nil {
		for _, st := range h.Items {
			if st.Qty > 0 && !st.Forbidden {
				counts[st.Kind] += st.Qty
			}
		}
	}
	return counts
}

// PlanTrade checks the complete new cargo, including payment in silver,
// and its carrying capacity before any authoritative state is changed.
// incoming is the exact merchandise removed from settlement stock.
func (o Ops) PlanTrade(cid uint64, members int, res trade.Result, incoming []item.Stack, carriers ...TradeCarriers) (TradePlan, string) {
	if !res.OK() || members <= 0 {
		return TradePlan{}, "The caravan cannot make that trade."
	}
	var next []item.Stack
	if h := o.s.cargo[cid]; h != nil {
		next = make([]item.Stack, len(h.Items), len(h.Items)+len(incoming)+1)
		copy(next, h.Items)
	}
	var roster TradeCarriers
	if len(carriers) > 0 {
		roster = carriers[0]
	}
	capacity := carrierCapacity(members, o.s.cargo[cid], roster)
	plan := TradePlan{caravan: cid, valid: true}
	take := func(key item.Stack, qty int, selling bool) bool {
		for i := 0; i < len(next) && qty > 0; {
			st := &next[i]
			if trade.CarryStack(*st, 1) != key || st.Forbidden || st.Qty <= 0 {
				i++
				continue
			}
			n := min(qty, st.Qty)
			if selling {
				part := *st
				part.Qty = n
				plan.sold = append(plan.sold, part)
			}
			st.Qty -= n
			qty -= n
			if st.Qty == 0 {
				next = slices.Delete(next, i, i+1)
			} else {
				i++
			}
		}
		return qty == 0
	}
	for _, line := range res.Sell {
		if line.Qty <= 0 || line.OrdinaryQty() < 0 {
			return TradePlan{}, "The caravan lacks goods for that trade."
		}
		for _, lot := range line.Lots {
			if !take(trade.CarryStack(lot.Stack, 1), lot.Stack.Qty, true) {
				return TradePlan{}, "The caravan lacks goods for that trade."
			}
		}
		if !take(item.Stack{Kind: line.Kind, Building: line.Building, Qty: 1}, line.OrdinaryQty(), true) {
			return TradePlan{}, "The caravan lacks goods for that trade."
		}
	}
	if res.Silver < 0 && !take(item.Stack{Kind: item.Silver, Qty: 1}, -res.Silver, false) {
		return TradePlan{}, "The caravan lacks silver for that trade."
	}
	// A gift of silver is already represented by res.Sell.
	if res.Silver > 0 {
		next = appendTradeGood(next, item.Stack{Kind: item.Silver, Qty: res.Silver})
	}
	expected := make(map[trade.Goods]int, len(res.Buy))
	for _, line := range res.Buy {
		expected[line.Goods()] += line.Qty
	}
	for _, st := range incoming {
		k := trade.Goods{Kind: st.Kind, Building: st.Building}
		if st.Qty <= 0 || expected[k] < st.Qty {
			return TradePlan{}, "The settlement cargo is inconsistent."
		}
		expected[k] -= st.Qty
		st.X, st.Y, st.Carrier = 0, 0, 0
		next = appendTradeGood(next, st)
	}
	for _, remain := range expected {
		if remain != 0 {
			return TradePlan{}, "The settlement lacks the offered goods."
		}
	}
	book := item.Book{}
	if o.h != nil {
		book = o.h.Items()
	}
	room := capacity
	for _, st := range next {
		grams := int64(book.MassGrams(st.Kind))
		if st.Qty <= 0 || grams <= 0 {
			return TradePlan{}, "The trade contains an invalid item stack."
		}
		if int64(st.Qty) > room/grams {
			return TradePlan{}, "The caravan cannot carry the purchased goods."
		}
		room -= int64(st.Qty) * grams
	}
	plan.items = next
	return plan, ""
}

// CommitTrade installs a previously accepted cargo replacement. It is
// called once, immediately alongside the settlement stock update.
func (o Ops) CommitTrade(cid uint64, plan TradePlan) {
	if !plan.valid || plan.caravan != cid {
		return
	}
	o.s.cargoOf(cid).Items = plan.items
	o.h.Changed()
}

// TradeCarriers supplies the caravan's native stashed animals.
// The hold already owns its own animals.
type TradeCarriers struct {
	Stashed []Beast
}

func carrierCapacity(members int, h *Hold, roster TradeCarriers) int64 {
	capacity, _, _ := carrierStats(members, h, roster)
	return capacity
}

// carrierStats uses the authoritative cargo hold and native stash exactly
// once. Stash wins if a stale hold entry names the same animal.
func carrierStats(members int, h *Hold, roster TradeCarriers) (capacity int64, mount int32, mounts int) {
	capacity = int64(members) * caravan.PersonCarryKg * 1000
	seen := make(map[entity.EntityID]bool)
	add := func(b Beast) {
		if seen[b.ID] {
			return
		}
		seen[b.ID] = true
		if b.State.Tame && !b.State.Dead() {
			capacity += int64(caravan.PackKg(animal.BodySize(b.State.Species))) * 1000
			if v := caravan.MountFactor(b.State.Species); v > 0 {
				mounts++
				mount = max(mount, v)
			}
		}
	}
	for _, b := range roster.Stashed {
		add(b)
	}
	if h != nil {
		for _, b := range h.Animals {
			add(b)
		}
	}
	return capacity, mount, mounts
}
