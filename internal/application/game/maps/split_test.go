// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package maps

import (
	"reflect"
	"testing"

	"rimworld-go/internal/application/game/travel"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
)

type splitHost struct {
	Host
	cargo    travel.Ops
	holders  map[entity.EntityID]entity.EntityID
	prisoner entity.EntityID
	next     uint64
}

func (h *splitHost) Cargo() Freight                             { return h.cargo }
func (h *splitHost) SplitCargo() SplitFreight                   { return h.cargo }
func (*splitHost) MarketCarriers(*Caravan) travel.TradeCarriers { return travel.TradeCarriers{} }
func (h *splitHost) HeldMember(id, holder entity.EntityID) (bool, bool) {
	ok := h.holders[id] == holder
	return ok, ok && id != h.prisoner
}
func (h *splitHost) TransferHeld(id, holder entity.EntityID)     { h.holders[id] = holder }
func (h *splitHost) NewID() uint64                               { h.next++; return h.next }
func (*splitHost) Alive(entity.EntityID) bool                    { return true }
func (*splitHost) Capacity(entity.EntityID, body.Capacity) int32 { return 1000 }

func TestSplitLifecycleAndPrisonerAdmission(t *testing.T) {
	for _, state := range []caravan.State{caravan.Travelling, caravan.Returning, caravan.Arrived, caravan.Visiting} {
		t.Run(state.Label(), func(t *testing.T) {
			h := &splitHost{cargo: (&travel.State{}).Ops(nil), holders: map[entity.EntityID]entity.EntityID{1: 7, 2: 7, 3: 7}, prisoner: 3, next: 7}
			c := Caravan{C: caravan.Caravan{ID: 7, Members: []uint64{1, 2, 3}, Tile: 11, Dest: 22, State: state, Progress: 190, FoodDays: 1250, VisitUntil: 98, Path: []int{12, 22}}, Home: 10, Out: []int{10, 11, 12, 22}, Back: state == caravan.Returning, NextEnc: 150, NextTalk: 60}
			list := []Caravan{c}
			a := &Atlas{}
			f := Of(&list, a, h)
			selection := travel.EncodeSplit(travel.SplitSelection{Members: []uint64{2, 3}})
			q := f.SplitView(&list[0], nil).Quote
			if q == 0 {
				t.Fatal("legal caravan cannot split")
			}
			// A prisoner alone cannot form another caravan.
			if _, why := f.Split(7, q, travel.EncodeSplit(travel.SplitSelection{Members: []uint64{3}})); why == "" {
				t.Fatal("prisoner-only split accepted")
			}
			// No change when a map transition or encounter takes ownership.
			a.Queue = []Request{{Op: OpEnter, Caravan: 7}}
			if _, why := f.Split(7, q, selection); why == "" {
				t.Fatal("pending transition accepted")
			}
			a.Queue = nil
			list[0].Ask.Letter = 9
			if _, why := f.Split(7, q, selection); why == "" {
				t.Fatal("pending encounter accepted")
			}
			list[0].Ask.Letter = 0
			if h.next != 7 || !reflect.DeepEqual(list[0], c) || h.holders[3] != 7 {
				t.Fatal("rejected split changed owner or spent ID")
			}
			id, why := f.Split(7, q, selection)
			if why != "" || id != 8 || len(list) != 2 {
				t.Fatalf("split: %d %s", id, why)
			}
			if list[1].C.State != state || list[1].C.VisitUntil != 98 || list[1].Back != c.Back || list[1].C.Progress != 190 || list[1].C.FoodDays != 1250 || list[1].NextEnc != 150 || list[1].NextTalk != 60 || h.holders[3] != 8 || h.prisoner != 3 {
				t.Fatal("split changed state, deadlines or prisoner identity")
			}
			list[1].Out[0] = 99
			list[1].C.Path[0] = 99
			if list[0].Out[0] != 10 || list[0].C.Path[0] != 12 {
				t.Fatal("routes alias")
			}
		})
	}
}
