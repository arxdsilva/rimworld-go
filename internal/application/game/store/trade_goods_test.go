// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package store

import (
	"reflect"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

func TestTradeGoodsSelectionAndPhysicalConsumption(t *testing.T) {
	stacks := map[entity.EntityID]item.Stack{
		9:  {Kind: item.Silver, Qty: 7},
		3:  {Kind: item.Silver, Qty: 4},
		7:  {Kind: item.Steel, Qty: 3, Forbidden: true},
		5:  {Kind: item.Silver, Qty: 1, Carrier: 12},
		6:  {Kind: item.Silver},
		11: {Kind: item.Wood, Qty: 8, X: 1},
	}
	fetch := func(id entity.EntityID) (item.Stack, bool) {
		stack, ok := stacks[id]
		return stack, ok
	}
	selected := CollectTrade([]entity.EntityID{9, 3, 7, 5, 6, 11}, fetch,
		func(st item.Stack) bool { return st.X == 0 })
	if got := selected.Kinds(); !reflect.DeepEqual(got, []item.Kind{item.Silver}) {
		t.Fatalf("selected kinds: %v", got)
	}
	if got := selected.IDs(item.Silver); !reflect.DeepEqual(got, []entity.EntityID{3, 9}) {
		t.Fatalf("physical ID order: %v", got)
	}
	if selected.Count(item.Silver) != 11 || selected.Count(item.Wood) != 0 {
		t.Fatalf("wrong stock quantities: %v", selected.Counts())
	}
	var spent []entity.EntityID
	selected.Take(item.Silver, 6, fetch, func(id entity.EntityID, st item.Stack, qty int) {
		spent = append(spent, id)
		st.Qty -= qty
		stacks[id] = st
	})
	if !reflect.DeepEqual(spent, []entity.EntityID{3, 9}) ||
		stacks[3].Qty != 0 || stacks[9].Qty != 5 {
		t.Fatalf("selection spent incorrect physical stacks: ids %v, state %v", spent, stacks)
	}
}

func TestTradeGoodsSkipsConsumedStackAtCommit(t *testing.T) {
	stacks := map[entity.EntityID]item.Stack{
		1: {Kind: item.Silver, Qty: 5},
		2: {Kind: item.Silver, Qty: 5},
	}
	fetch := func(id entity.EntityID) (item.Stack, bool) {
		s, ok := stacks[id]
		return s, ok
	}
	selected := CollectTrade([]entity.EntityID{1, 2}, fetch, func(item.Stack) bool { return true })
	delete(stacks, 1)
	selected.Take(item.Silver, 4, fetch, func(id entity.EntityID, s item.Stack, qty int) {
		s.Qty -= qty
		stacks[id] = s
	})
	if stacks[2].Qty != 1 {
		t.Fatalf("remaining physical goods after stale removal: %v", stacks)
	}
}
