// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package settlement

import (
	"reflect"
	"strings"
	"testing"

	"rimworld-go/internal/application/game/tradeview"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// opened is world() with the session open.
func opened(t *testing.T) (*State, *fakeHost, *fakeCargo) {
	t.Helper()
	h, c := world()
	s := &State{}
	if why := s.Open(h, c, testCid, ""); why != "" {
		t.Fatalf("Open: %q", why)
	}
	return s, h, c
}

// TestSettlementOpen: a welcome visit gets the market trade.NewMarket
// rolls at now, the negotiator is the member with the highest Social (ties
// to the lowest id), and the view names the settlement and its faction.
func TestSettlementOpen(t *testing.T) {
	t.Parallel()
	s, h, c := opened(t)
	want := trade.NewMarket(item.Book{}, faction.Settlers, h.seed, testTile, 1, h.now)
	if len(s.markets) != 1 || !reflect.DeepEqual(s.markets[0], want) {
		t.Fatalf("market %+v, want %+v", s.markets, want)
	}
	if !s.Live(h) || s.sess.negotiator != 5 {
		t.Fatalf("live %v, negotiator %d (want 5)", s.Live(h), s.sess.negotiator)
	}
	v, ok := s.View(h, c)
	if !ok {
		t.Fatal("no view")
	}
	if v.Trader != "Oakmere" || v.Faction != "Union" || v.FactionID != 2 || v.Orbital || v.Goodwill != 20 {
		t.Fatalf("view %+v", v)
	}
	if v.Negotiator != entity.EntityID(5) || v.NegotiatorName != "Bo" || v.Social != 9 || v.HoursLeft != 3 {
		t.Fatalf("negotiator or hours: %+v", v)
	}
	if v.TraderKind != strings.ToLower(trade.DefOf(want.Stock.Kind).Label) || v.ColonySilver != 1000 || v.TraderSilver != want.Stock.Silver {
		t.Fatalf("kind or silver: %+v", v)
	}
	terms := trade.Terms{Social: 9, Goodwill: 20}
	if v.PriceModPct != tradeview.PriceModPct(terms) {
		t.Fatalf("price mod %d", v.PriceModPct)
	}
	if rows := tradeview.Rows(item.Book{}, count(c.goods[testCid]), want.Stock, terms, item.Kind.String); len(rows) != len(v.Rows) {
		t.Fatalf("%d rows, want %d", len(v.Rows), len(rows))
	}
	if got := s.Open(h, c, testCid, ""); got != "" || h.seeds != 1 {
		t.Fatalf("reopening the open session: %q, seeds %d", got, h.seeds)
	}
	h.caravans[8] = h.caravans[testCid]
	if got := s.Open(h, c, 8, ""); got != tradeview.ReasonTradeOpen {
		t.Fatalf("a second caravan opened %q", got)
	}
	s.Close()
	if s.Live(h) {
		t.Fatal("Close left the session open")
	}
}

// setStock gives the open session's market a known stock.
func setStock(s *State) *trade.Market {
	m := &s.markets[0]
	m.Stock = trade.Stock{Kind: trade.Quartermaster, Silver: 2000, Lines: []trade.Line{{Kind: item.Rifle, Qty: 5}}}
	return m
}

func offer(d ...trade.Delta) string { return trade.EncodeOffer(d) }

// TestSettlementBuySell: buying and selling move goods and silver both
// ways; an offer the cargo can't cover and a Yield failing half way
// change nothing.
func TestSettlementBuySell(t *testing.T) {
	t.Parallel()
	terms := trade.Terms{Social: 9, Goodwill: 20}
	t.Run("buy", func(t *testing.T) {
		t.Parallel()
		s, h, c := opened(t)
		m := setStock(s)
		res := trade.Settle(item.Book{}, []trade.Delta{{Kind: item.Rifle, Qty: 2}}, count(c.goods[testCid]), m.Stock, terms, false)
		if note := s.Propose(h, c, offer(trade.Delta{Kind: item.Rifle, Qty: 2}), false); note != "" {
			t.Fatalf("refused: %q", note)
		}
		got := count(c.goods[testCid])
		if got[item.Rifle] != 2 || got[item.Silver] != 1000+res.Silver || res.Silver >= 0 {
			t.Fatalf("cargo %v, silver change %d", got, res.Silver)
		}
		if m.Stock.Lines[0].Qty != 3 || m.Stock.Silver != 2000-res.Silver {
			t.Fatalf("stock %+v", m.Stock)
		}
	})
	t.Run("sell", func(t *testing.T) {
		t.Parallel()
		s, h, c := opened(t)
		m := setStock(s)
		res := trade.Settle(item.Book{}, []trade.Delta{{Kind: item.Steel, Qty: -10}}, count(c.goods[testCid]), m.Stock, terms, false)
		if note := s.Propose(h, c, offer(trade.Delta{Kind: item.Steel, Qty: -10}), false); note != "" {
			t.Fatalf("refused: %q", note)
		}
		got := count(c.goods[testCid])
		if got[item.Steel] != 40 || got[item.Silver] != 1000+res.Silver || res.Silver <= 0 {
			t.Fatalf("cargo %v, silver change %d", got, res.Silver)
		}
		if m.Stock.Silver != 2000-res.Silver || m.Stock.Lines[len(m.Stock.Lines)-1].Kind != item.Steel || m.Stock.Lines[len(m.Stock.Lines)-1].Qty != 10 {
			t.Fatalf("stock %+v", m.Stock)
		}
	})
	t.Run("uncovered", func(t *testing.T) {
		t.Parallel()
		s, h, c := opened(t)
		m := setStock(s)
		before, stock := count(c.goods[testCid]), cloneStock(m.Stock)
		for _, o := range []string{offer(trade.Delta{Kind: item.Steel, Qty: -51}), offer(trade.Delta{Kind: item.Rifle, Qty: 6}), "nonsense"} {
			if note := s.Propose(h, c, o, false); note == "" {
				t.Fatalf("offer %q accepted", o)
			}
		}
		if !reflect.DeepEqual(count(c.goods[testCid]), before) || !reflect.DeepEqual(m.Stock, stock) {
			t.Fatal("a refused offer changed something")
		}
		if v, _ := s.View(h, c); v.Reason != tradeview.ReasonOfferBroken {
			t.Fatalf("view reason %q", v.Reason)
		}
	})
	t.Run("yield fails", func(t *testing.T) {
		t.Parallel()
		s, h, c := opened(t)
		m := setStock(s)
		before, stock := count(c.goods[testCid]), cloneStock(m.Stock)
		c.failAt = 2
		note := s.Propose(h, c, offer(trade.Delta{Kind: item.Steel, Qty: -10}, trade.Delta{Kind: item.Components, Qty: -2}), false)
		if note != tradeview.ReasonOfferBroken {
			t.Fatalf("note %q", note)
		}
		if !reflect.DeepEqual(count(c.goods[testCid]), before) || !reflect.DeepEqual(m.Stock, stock) {
			t.Fatalf("a broken yield changed something: %v", count(c.goods[testCid]))
		}
	})
}

func cloneStock(st trade.Stock) trade.Stock {
	st.Lines = append([]trade.Line(nil), st.Lines...)
	return st
}

// TestSettlementGift: a gift moves the goods to the stock and raises the
// faction's goodwill with a dated event; nothing comes back.
func TestSettlementGift(t *testing.T) {
	t.Parallel()
	s, h, c := opened(t)
	m := setStock(s)
	if note := s.Propose(h, c, offer(trade.Delta{Kind: item.Steel, Qty: -50}), true); note != "" {
		t.Fatalf("gift refused: %q", note)
	}
	f := h.facs[1]
	if f.Goodwill <= 20 || len(f.Events) == 0 || !strings.HasPrefix(f.Events[len(f.Events)-1], "Day 3: gifts from the colony") {
		t.Fatalf("goodwill %d, events %q", f.Goodwill, f.Events)
	}
	if got := count(c.goods[testCid]); got[item.Steel] != 0 || got[item.Silver] != 1000 || m.Stock.Silver != 2000 {
		t.Fatalf("cargo %v, stock silver %d", got, m.Stock.Silver)
	}
}

// TestSettlementRestockLazy: reopening within the period keeps the traded
// stock; reopening at or after NextAt rolls once; a NextAt beyond one
// period from now (a hostile save) restocks at once.
func TestSettlementRestockLazy(t *testing.T) {
	t.Parallel()
	s, h, c := opened(t)
	m := setStock(s)
	traded := cloneStock(m.Stock)
	next := m.NextAt
	s.Close()
	h.now = next - 1
	if why := s.Open(h, c, testCid, ""); why != "" {
		t.Fatal(why)
	}
	if !reflect.DeepEqual(s.markets[0].Stock, traded) {
		t.Fatal("reopened within the period, the stock re-rolled")
	}
	s.Close()
	h.now = next + 3*trade.RestockDays*clock.TicksPerDay
	if why := s.Open(h, c, testCid, ""); why != "" {
		t.Fatal(why)
	}
	if want := trade.NewMarket(item.Book{}, faction.Settlers, h.seed, testTile, 1, h.now); !reflect.DeepEqual(s.markets[0], want) {
		t.Fatal("a late reopen is not one roll at now")
	}
	s.Close()
	sv := s.Export()
	sv.Markets[0].NextAt = 1 << 62
	sv.Markets[0].Stock = traded
	s = Import(sv)
	if why := s.Open(h, c, testCid, ""); why != "" {
		t.Fatal(why)
	}
	if m := s.markets[0]; reflect.DeepEqual(m.Stock, traded) || m.NextAt > h.now+trade.RestockDays*clock.TicksPerDay {
		t.Fatalf("a far NextAt was kept: %d", m.NextAt)
	}
}

// TestSettlementLiveness: once the visit ends, the caravan heads home (and
// leaves the tile) or the faction turns hostile, View is not ok and
// Propose changes nothing.
func TestSettlementLiveness(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		edit func(h *fakeHost)
	}{
		{"visit over", func(h *fakeHost) { v := h.caravans[testCid]; v.Visiting = false; h.caravans[testCid] = v }},
		{"home", func(h *fakeHost) {
			v := h.caravans[testCid]
			v.Visiting, v.Tile = false, testTile+1
			h.caravans[testCid] = v
		}},
		{"moved", func(h *fakeHost) { v := h.caravans[testCid]; v.Tile = 11; h.caravans[testCid] = v }},
		{"hostile", func(h *fakeHost) { h.facs[1].Relation = faction.Hostile }},
		{"gone", func(h *fakeHost) { delete(h.caravans, testCid) }},
	}
	for _, cs := range cases {
		t.Run(cs.name, func(t *testing.T) {
			t.Parallel()
			s, h, c := opened(t)
			m := setStock(s)
			before, stock := count(c.goods[testCid]), cloneStock(m.Stock)
			cs.edit(h)
			if _, ok := s.View(h, c); ok || s.Live(h) {
				t.Fatal("a dead session is still live")
			}
			if note := s.Propose(h, c, offer(trade.Delta{Kind: item.Rifle, Qty: 1}), false); note == "" {
				t.Fatal("a dead session took an offer")
			}
			if !reflect.DeepEqual(count(c.goods[testCid]), before) || !reflect.DeepEqual(m.Stock, stock) {
				t.Fatal("a dead session's offer changed something")
			}
		})
	}
}
