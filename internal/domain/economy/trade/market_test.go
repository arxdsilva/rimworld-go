// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package trade

import (
	"math"
	"reflect"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
)

const testSeed = 0xC0FFEE

func TestMarketDeterministic(t *testing.T) {
	b := item.Book{}
	now := uint64(3*restockTicks + 17)
	// Interleave other rolls: the order of calls must not matter.
	a := NewMarket(b, faction.Settlers, testSeed, 42, 3, now)
	_ = NewMarket(b, faction.Clans, testSeed, 7, 1, now)
	_ = NewMarket(b, faction.Settlers, 99, 42, 3, now)
	c := NewMarket(b, faction.Settlers, testSeed, 42, 3, now)
	if !reflect.DeepEqual(a, c) {
		t.Fatalf("same inputs differ:\n%+v\n%+v", a, c)
	}
	if len(a.Stock.Lines) == 0 {
		t.Fatal("empty stock")
	}
	// Any tick inside the same period gives the same stock.
	d := NewMarket(b, faction.Settlers, testSeed, 42, 3, a.NextAt-1)
	if !reflect.DeepEqual(a, d) {
		t.Fatal("stock changed within a period")
	}
}

func TestMarketKindByTech(t *testing.T) {
	cases := []struct {
		ft   faction.Type
		want TraderKind
	}{
		{faction.Clans, BulkHauler},
		{faction.Settlers, Quartermaster},
		{faction.Drifters, Quartermaster},
		{faction.Swarm, BulkHauler},
		{faction.Player, BulkHauler},
		{faction.NumTypes + 3, BulkHauler},
	}
	for _, c := range cases {
		if got := SettlementKind(c.ft); got != c.want {
			t.Errorf("SettlementKind(%d) = %d, want %d", c.ft, got, c.want)
		}
	}
	b := item.Book{}
	for tile := 0; tile < 40; tile++ {
		m := NewMarket(b, faction.Clans, testSeed, tile, 0, 0)
		if m.Stock.Kind != BulkHauler {
			t.Fatalf("clans stock kind %d", m.Stock.Kind)
		}
		for _, l := range m.Stock.Lines {
			if l.Kind == item.Rifle || l.Kind == item.Components {
				t.Fatalf("neolithic stock holds %v", l.Kind)
			}
		}
		if q := NewMarket(b, faction.Settlers, testSeed, tile, 1, 0); q.Stock.Kind != Quartermaster {
			t.Fatalf("settlers stock kind %d", q.Stock.Kind)
		}
	}
}

func TestMarketDue(t *testing.T) {
	b := item.Book{}
	m := NewMarket(b, faction.Settlers, testSeed, 5, 2, 100)
	if m.Due(m.NextAt-1) || !m.Due(m.NextAt) || !m.Due(m.NextAt+1) {
		t.Fatalf("Due boundary wrong at NextAt %d", m.NextAt)
	}
	before := m
	if m.Restock(b, faction.Settlers, 2, testSeed, m.NextAt-1) {
		t.Fatal("restocked within a period")
	}
	if !reflect.DeepEqual(before, m) {
		t.Fatal("market changed within a period")
	}
	if !m.Restock(b, faction.Settlers, 2, testSeed, m.NextAt) {
		t.Fatal("no restock when due")
	}
	if m.Restocked != before.NextAt {
		t.Fatalf("Restocked %d, want %d", m.Restocked, before.NextAt)
	}
}

func TestMarketFactionChangeRestocks(t *testing.T) {
	b := item.Book{}
	m := NewMarket(b, faction.Clans, testSeed, 5, 2, 100)
	if !m.Restock(b, faction.Settlers, 4, testSeed, 101) {
		t.Fatal("faction change did not restock")
	}
	if m.Faction != 4 || m.Stock.Kind != Quartermaster {
		t.Fatalf("market not re-rolled for the new faction: %+v", m)
	}
}

func TestMarketCatchUpOnce(t *testing.T) {
	b := item.Book{}
	m := NewMarket(b, faction.Settlers, testSeed, 11, 1, 0)
	now := uint64(5*restockTicks + 123)
	if !m.Restock(b, faction.Settlers, 1, testSeed, now) {
		t.Fatal("no restock after 5 periods")
	}
	if want := NewMarket(b, faction.Settlers, testSeed, 11, 1, now); !reflect.DeepEqual(m, want) {
		t.Fatalf("catch-up differs from NewMarket:\n%+v\n%+v", m, want)
	}
	if m.NextAt <= now {
		t.Fatalf("NextAt %d not after now %d", m.NextAt, now)
	}
	if m.Restocked > now {
		t.Fatalf("Restocked %d after now", m.Restocked)
	}
}

func TestMarketPhasePerTile(t *testing.T) {
	b := item.Book{}
	phases := map[uint64]bool{}
	stocks := 0
	var first Market
	for tile := 0; tile < 20; tile++ {
		m := NewMarket(b, faction.Settlers, testSeed, tile, 1, 0)
		if m.NextAt == 0 || m.NextAt > restockTicks || m.Restocked != 0 {
			t.Fatalf("tile %d: NextAt %d Restocked %d at tick 0", tile, m.NextAt, m.Restocked)
		}
		phases[m.NextAt] = true
		if tile == 0 {
			first = m
		} else if !reflect.DeepEqual(first.Stock, m.Stock) {
			stocks++
		}
	}
	if len(phases) < 2 {
		t.Fatal("every tile shares one phase")
	}
	if stocks == 0 {
		t.Fatal("every tile shares one stock")
	}
}

func TestMarketSeedChangesStock(t *testing.T) {
	b := item.Book{}
	a := NewMarket(b, faction.Settlers, testSeed, 42, 3, 0)
	diff := false
	for s := uint64(1); s <= 5 && !diff; s++ {
		o := NewMarket(b, faction.Settlers, testSeed+s, 42, 3, 0)
		diff = !reflect.DeepEqual(a.Stock, o.Stock)
	}
	if !diff {
		t.Fatal("different seeds gave identical stocks")
	}
}

func TestMarketSilverNeverNegative(t *testing.T) {
	b := item.Book{}
	for tile := 0; tile < 300; tile++ {
		for _, ft := range []faction.Type{faction.Clans, faction.Settlers} {
			m := NewMarket(b, ft, testSeed, tile, 1, uint64(tile)*restockTicks/3)
			if m.Stock.Silver < 0 {
				t.Fatalf("tile %d: silver %d", tile, m.Stock.Silver)
			}
		}
	}
}

func TestMarketNextAtLargeNow(t *testing.T) {
	b := item.Book{}
	for _, now := range []uint64{math.MaxInt64 / 2, math.MaxInt64 / 2, 1 << 50, 1000 * restockTicks * 365} {
		m := NewMarket(b, faction.Settlers, testSeed, 9, 1, now)
		if m.NextAt <= now || m.NextAt-now > restockTicks {
			t.Fatalf("now %d: NextAt %d", now, m.NextAt)
		}
		if m.Restocked > now {
			t.Fatalf("now %d: Restocked %d", now, m.Restocked)
		}
	}
	m := NewMarket(b, faction.Settlers, testSeed, 9, 1, 0)
	if !m.Restock(b, faction.Settlers, 1, testSeed, math.MaxInt64/2) || m.NextAt <= math.MaxInt64/2 {
		t.Fatalf("long-gap restock: NextAt %d", m.NextAt)
	}
}

func TestMarketNoCrates(t *testing.T) {
	b := item.Book{}
	for tile := 0; tile < 200; tile++ {
		for _, ft := range []faction.Type{faction.Clans, faction.Settlers, faction.Drifters} {
			m := NewMarket(b, ft, testSeed, tile, 1, uint64(tile)*restockTicks)
			for _, l := range m.Stock.Lines {
				if l.Building != 0 {
					t.Fatalf("tile %d: crate line %+v", tile, l)
				}
			}
		}
	}
}
