// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package travel

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/entity"
)

// A hold weighs its people at 60 kg, its goods by their wiki masses
// (rounded up) and is worth its goods' market value.
func TestHoldMassAndValue(t *testing.T) {
	h := Hold{Pawns: []entity.EntityID{7}, Items: []item.Stack{{Kind: item.Steel, Qty: 30}}}
	h.addItem(item.Stack{Kind: item.Steel, Qty: 10, X: 3, Y: 4})
	if len(h.Items) != 1 || h.Items[0].Qty != 40 {
		t.Fatalf("like stacks did not merge: %+v", h.Items)
	}
	if kg := h.massKg(item.Core(), animalKg); kg != 60+20 {
		t.Fatalf("mass %d kg, want 80", kg)
	}
	if v := h.value(item.Core()); v != 40*item.Steel.Value() {
		t.Fatalf("value %d", v)
	}
}

// The saved form keeps everything and nothing more (an empty state saves
// nothing).
func TestSaveRoundTrip(t *testing.T) {
	if (&State{}).Export() != nil || (*State)(nil).Export() != nil {
		t.Fatal("an empty state saved something")
	}
	s := &State{}
	s.setSleeper(4, 9)
	s.holdOf(5).addItem(item.Stack{Kind: item.Gold, Qty: 3})
	s.sites = []Site{{Tile: 11, Mineral: item.Jade, Qty: 700, Cells: 18, Expires: 99}}
	s.flights = []Flight{{ID: 1, From: 2, To: 3, Arrive: 50, Pods: 2}}
	s.setScan(6, Scan{Work: 12, For: item.Gold})
	back := Import(s.Export())
	if !back.Asleep(9) || back.holds[5].Items[0].Qty != 3 || back.sites[0] != s.sites[0] || back.flights[0].Pods != 2 ||
		back.scans[6] != (Scan{Work: 12, For: item.Gold}) {
		t.Fatalf("round trip lost state: %+v", back.Export())
	}
}

// Ring visits rings outward in row order; EdgeTile draws in a fixed
// order from its roll and falls back to the first dry cell.
func TestRingAndEdgeTile(t *testing.T) {
	var seen [][2]int
	x, y, ok := Ring(5, 5, 2, func(x, y int) bool { seen = append(seen, [2]int{x, y}); return x == 6 && y == 6 })
	if !ok || x != 6 || y != 6 || seen[0] != [2]int{5, 5} || seen[1] != [2]int{4, 4} {
		t.Fatalf("ring found (%d, %d) after %v", x, y, seen)
	}
	rolls := []int{1, 3, 0, 7}
	i := 0
	roll := func(n int) int { v := rolls[i%len(rolls)] % n; i++; return v }
	ex, ey, ok := EdgeTile(10, 8, roll, func(int, int) bool { return true }, func(x, y int) bool { return false })
	if !ok || ex != 3 || ey != 7 {
		t.Fatalf("fallback edge (%d, %d) ok=%v, want the first dry cell (3, 7)", ex, ey, ok)
	}
}

// The settlement list skips the defeated, names unnamed settlements and
// marks hostile peoples unvisitable; the id list parser keeps each
// accepted id once, ascending.
func TestSettlementViewsAndIDs(t *testing.T) {
	fs := []faction.Faction{
		{Name: "Ash", Type: faction.Settlers, Relation: faction.Hostile, Settlements: []int{4}},
		{Name: "Gone", Type: faction.Settlers, Defeated: true, Settlements: []int{5}},
	}
	v := SettlementViews(fs, nil)
	if len(v) != 1 || v[0].Name != "a settlement of Ash" || v[0].Visitable || v[0].Relation != "hostile" {
		t.Fatalf("views %+v", v)
	}
	ids := ParseIDs("9, 3,x,9,4", func(id entity.EntityID) bool { return id != 4 })
	if len(ids) != 2 || ids[0] != 3 || ids[1] != 9 {
		t.Fatalf("ids %v", ids)
	}
	if JoinNames([]string{"A", "B", "C"}) != "A, B and C" || JoinNames(nil) != "Nobody" {
		t.Fatal("join names")
	}
}

// A gift's eligible selling price differs by delivery method and follows
// the item stack's physical condition.
func TestPodGiftUsesStackQualityAndDeliveryRate(t *testing.T) {
	book := item.Book{}
	pod := Hold{Items: []item.Stack{{Kind: item.Lung, Qty: 1}}}
	if got := pod.giftValue(book); got != 612 {
		t.Fatalf("a lung pod gift credited %d, want 612", got)
	}
	pod.Items = append(pod.Items, item.Stack{Kind: item.Silver, Qty: 1000})
	if got := pod.giftValue(book); got != 1612 {
		t.Fatalf("gift silver must keep its face value: %d", got)
	}
	rifle := item.Stack{Kind: item.Rifle, Qty: 1}
	pod = Hold{Items: []item.Stack{rifle}}
	if got := pod.giftValue(book); got != 14 {
		t.Fatalf("a normal rifle pod gift credited %d", got)
	}
	rifle.Wear = rifle.MaxHP() / 2
	pod.Items[0] = rifle
	if got, market := pod.giftValue(book), item.StackValue(rifle); market != 12 || got != 1 {
		t.Fatalf("worn rifle gift %d with market value %d", got, market)
	}
}

func TestInTransportPodHasNoPersistentIndex(t *testing.T) {
	var nilState *State
	if nilState.InTransportPod(9) {
		t.Fatal("nil state claims a passenger")
	}
	s := &State{}
	s.setSleeper(7, 9)
	if s.InTransportPod(9) {
		t.Fatal("casket sleeper is not a transport pod passenger")
	}
	s.holdOf(10).Pawns = []entity.EntityID{11}
	s.flights = []Flight{{ID: 3, Load: Hold{Pawns: []entity.EntityID{12, 13}}}}
	for _, id := range []entity.EntityID{11, 12, 13} {
		if !s.InTransportPod(id) {
			t.Fatalf("passenger %d missing from pod state", id)
		}
	}
	for _, id := range []entity.EntityID{0, 9, 14} {
		if s.InTransportPod(id) {
			t.Fatalf("non-passenger %d claimed by pod state", id)
		}
	}
	back := Import(s.Export())
	for _, id := range []entity.EntityID{11, 12, 13} {
		if !back.InTransportPod(id) {
			t.Fatalf("save/load lost pod passenger %d", id)
		}
	}
	delete(back.holds, 10)
	back.flights = nil
	if back.InTransportPod(11) || back.InTransportPod(12) {
		t.Fatal("disembarked passengers remain in transport pod state")
	}
}

func TestLegacySiteCellsPreserveOldGeometry(t *testing.T) {
	for _, tc := range []struct {
		k          item.Kind
		qty, cells int
	}{{item.Steel, 200, 10}, {item.Silver, 2400, 96}, {item.Gold, 80, 10}, {item.Jade, 700, 18}, {item.Wood, 75, 0}} {
		v := &Save{Sites: []Site{{Tile: 11, Mineral: tc.k, Qty: tc.qty, Expires: 99}}}
		s := Import(v)
		got := s.Export().Sites[0]
		if got.Cells != tc.cells || got.Qty != tc.qty || v.Sites[0].Cells != 0 {
			t.Fatalf("legacy site %+v, want %d cells", got, tc.cells)
		}
		if Import(s.Export()).Export().Sites[0] != got {
			t.Fatal("migration was not idempotent")
		}
	}
}
