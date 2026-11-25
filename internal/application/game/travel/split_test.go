// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package travel

import (
	"reflect"
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/world/caravan"
)

func TestSplitCargoExactLotsAndIndependentCapacity(t *testing.T) {
	goods := []item.Stack{{Kind: item.Steel, Qty: 40, Forbidden: true}, {Kind: item.Steel, Qty: 20, Quality: 3, Wear: 4}}
	held := Beast{ID: 71, State: animal.State{Species: animal.Donkey, Tame: true, Name: "Pack"}}
	stash := Beast{ID: 72, State: animal.State{Species: animal.Horse, Tame: true, Name: "Mount"}}
	s := &State{cargo: map[uint64]*Hold{7: {Items: goods, Animals: []Beast{held}}}}
	o := s.Ops(nil)
	roster := TradeCarriers{Stashed: []Beast{stash}}
	before := s.Export()
	for _, sel := range []SplitSelection{
		{Lots: []SplitLot{{0, 1}, {0, 1}}},
		{Lots: []SplitLot{{1, 21}}},
		{Lots: []SplitLot{{-1, 1}}},
	} {
		if _, why := o.PlanSplit(7, sel, 2, 1, roster); why == "" {
			t.Fatal("invalid lot accepted")
		}
		if !reflect.DeepEqual(s.Export(), before) {
			t.Fatal("invalid plan changed cargo")
		}
	}
	sel := SplitSelection{Members: []uint64{2}, Animals: []uint64{71}, Lots: []SplitLot{{0, 13}, {1, 20}}}
	plan, why := o.PlanSplit(7, sel, 2, 1, roster)
	if why != "" {
		t.Fatal(why)
	}
	if !reflect.DeepEqual(s.Export(), before) {
		t.Fatal("preflight changed cargo")
	}
	o.CommitSplit(7, 8, plan)
	a, b := s.cargo[7], s.cargo[8]
	l, r := goods[0], goods[0]
	l.Qty, r.Qty = 27, 13
	if !reflect.DeepEqual(a.Items, []item.Stack{l}) || !reflect.DeepEqual(b.Items, []item.Stack{r, goods[1]}) || len(a.Animals) != 0 || len(b.Animals) != 1 || b.Animals[0].ID != 71 {
		t.Fatalf("physical split changed payload: %+v %+v", a, b)
	}
	if plan.Load[0] <= 0 || plan.Load[1] <= 0 {
		t.Fatal("missing native load facts")
	}
	s.cargo[9] = &Hold{Items: []item.Stack{{Kind: item.Steel, Qty: 100}}}
	if _, why = o.PlanSplit(9, SplitSelection{}, 2, 1, TradeCarriers{}); why == "" {
		t.Fatal("one overloaded half accepted")
	}
}

func TestPaceUsesSharedDeduplicatedRoster(t *testing.T) {
	held := Beast{ID: 71, State: animal.State{Species: animal.Donkey, Tame: true}}
	horse := Beast{ID: 72, State: animal.State{Species: animal.Horse, Tame: true}}
	st := item.Stack{Kind: item.Steel, Qty: 20}
	s := &State{cargo: map[uint64]*Hold{7: {Items: []item.Stack{st}, Animals: []Beast{held, horse}}}}
	o := s.Ops(nil)
	roster := TradeCarriers{Stashed: []Beast{horse}}
	cap, mount, n := carrierStats(2, s.cargo[7], roster)
	want := int64(2*caravan.PersonCarryKg+caravan.PackKg(animal.BodySize(animal.Donkey))+caravan.PackKg(animal.BodySize(animal.Horse))) * 1000
	if cap != want || n != 2 || mount == 0 {
		t.Fatalf("capacity=%d want=%d mount=%d count=%d", cap, want, mount, n)
	}
	load, riding := o.CarrierPace(7, 2, 2, roster)
	if load != int32(int64(item.Core().MassGrams(st.Kind)*st.Qty)*1000/want) || riding != mount {
		t.Fatalf("load=%d mount=%d", load, riding)
	}
	_, riding = o.CarrierPace(7, 2, 3, roster)
	if riding != 0 {
		t.Fatal("insufficient mounts")
	}
	roster.Stashed[0].State.Tame = false
	cap, _, n = carrierStats(2, s.cargo[7], roster)
	if cap != want-int64(caravan.PackKg(animal.BodySize(animal.Horse)))*1000 || n != 1 {
		t.Fatalf("stashed eligibility did not override duplicate: %d %d", cap, n)
	}
}
