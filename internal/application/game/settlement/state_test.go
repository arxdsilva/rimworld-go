// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package settlement

import (
	"bytes"
	"encoding/gob"
	"math"
	"slices"
	"testing"

	"rimworld-go/internal/application/game/tradeview"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/world/caravan"
	"rimworld-go/internal/domain/world/planet"
)

// pawnRec is a fake colonist: its name and Social level.
type pawnRec struct {
	name   string
	social int
}

// fakeHost is a Host over plain data: caravans by id, the factions (sites
// resolve through SiteIn, as the game's do) and the pawns.
type fakeHost struct {
	now, seed uint64
	tiles     int
	day       int
	caravans  map[uint64]Visit
	facs      []faction.Faction
	pawns     map[uint64]pawnRec
	seeds     int // Seed calls
}

func (h *fakeHost) Now() uint64      { return h.now }
func (h *fakeHost) Seed() uint64     { h.seeds++; return h.seed }
func (h *fakeHost) Tiles() int       { return h.tiles }
func (h *fakeHost) Items() item.Book { return item.Book{} }
func (h *fakeHost) Day() int         { return h.day }
func (h *fakeHost) Caravan(cid uint64) (Visit, bool) {
	v, ok := h.caravans[cid]
	return v, ok
}
func (h *fakeHost) Site(tile int) (int, *faction.Faction, string) { return SiteIn(h.facs, tile) }
func (h *fakeHost) Pawn(id uint64) (string, int) {
	p := h.pawns[id]
	return p.name, p.social
}

// fakeCargo is the caravans' goods. failAt > 0 makes that Yield call (1
// based) fail.
type fakeCargo struct {
	goods  map[uint64][]item.Stack
	failAt int
	yields int
}

func (c *fakeCargo) Goods(cid uint64) []item.Stack { return slices.Clone(c.goods[cid]) }

func (c *fakeCargo) Yield(cid uint64, k item.Kind, qty int) bool {
	c.yields++
	if c.failAt > 0 && c.yields == c.failAt {
		return false
	}
	if count(c.goods[cid])[k] < qty {
		return false
	}
	st := c.goods[cid]
	for i := range st {
		if st[i].Kind == k {
			n := min(st[i].Qty, qty)
			st[i].Qty -= n
			qty -= n
		}
	}
	c.goods[cid] = slices.DeleteFunc(st, func(s item.Stack) bool { return s.Qty <= 0 })
	return true
}

func (c *fakeCargo) CarryLines(cid uint64, lines []trade.Line) {
	for _, l := range lines {
		c.goods[cid] = append(c.goods[cid], item.Stack{Kind: l.Kind, Qty: l.Qty})
	}
}

// count sums stacks by kind.
func count(st []item.Stack) map[item.Kind]int {
	m := map[item.Kind]int{}
	for _, s := range st {
		m[s.Kind] += s.Qty
	}
	return m
}

const (
	testTile = 42
	testCid  = 7
)

// world is a welcoming settlers settlement on testTile ("Oakmere", faction
// index 1 behind a clans faction elsewhere) with caravan testCid visiting
// it: members 3 (Social 4), 5 (Social 9) and 9 (Social 9), carrying silver
// and steel.
func world() (*fakeHost, *fakeCargo) {
	h := &fakeHost{now: 1000, seed: 99, tiles: 500, day: 3,
		caravans: map[uint64]Visit{testCid: {Tile: testTile, Visiting: true, Until: 1000 + 2*2500 + 1, Members: []uint64{9, 3, 5}}},
		facs: []faction.Faction{
			{ID: 1, Type: faction.Clans, Name: "Clans", Relation: faction.Neutral, Settlements: []int{10}, SettlementNames: []string{"Elsewhere"}},
			{ID: 2, Type: faction.Settlers, Name: "Union", Relation: faction.Neutral, Goodwill: 20, Settlements: []int{11, testTile}, SettlementNames: []string{"Capital", "Oakmere"}},
		},
		pawns: map[uint64]pawnRec{3: {"Ada", 4}, 5: {"Bo", 9}, 9: {"Cy", 9}},
	}
	c := &fakeCargo{goods: map[uint64][]item.Stack{testCid: {{Kind: item.Silver, Qty: 1000}, {Kind: item.Steel, Qty: 50}, {Kind: item.Components, Qty: 4}}}}
	return h, c
}

// TestSettlementReasons: Open refuses each case with its reason, in
// item 3's precedence, and a refusal leaves no market and no session.
func TestSettlementReasons(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		edit func(h *fakeHost)
		busy string
		want string
	}{
		{"no caravan", func(h *fakeHost) { delete(h.caravans, testCid) }, "", ReasonNoCaravan},
		{"forming", func(h *fakeHost) {
			h.caravans[testCid] = Visit{Tile: testTile, Forming: true}
			h.facs[1].Relation = faction.Hostile
		}, "x", ReasonForming},
		{"missing settlement", func(h *fakeHost) { v := h.caravans[testCid]; v.Tile = 12; h.caravans[testCid] = v }, "x", ReasonNoSettlement},
		{"hostile", func(h *fakeHost) {
			h.facs[1].Relation = faction.Hostile
			v := h.caravans[testCid]
			v.Visiting = false
			h.caravans[testCid] = v
		}, "x", tradeview.ReasonNoTrade},
		{"defeated", func(h *fakeHost) { h.facs[1].Defeated = true }, "", tradeview.ReasonNoTrade},
		{"not visiting", func(h *fakeHost) { v := h.caravans[testCid]; v.Visiting = false; h.caravans[testCid] = v }, "x", ReasonNotVisiting},
		{"busy", func(*fakeHost) {}, "another trade is open", "another trade is open"},
		{"out of range", func(h *fakeHost) { h.tiles = testTile }, "", ReasonNoSettlement},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			h, cargo := world()
			c.edit(h)
			s := &State{}
			if got := s.Open(h, cargo, testCid, c.busy); got != c.want {
				t.Fatalf("Open = %q, want %q", got, c.want)
			}
			if c.name != "out of range" {
				if got := s.Reason(h, testCid, c.busy); got != c.want {
					t.Fatalf("Reason = %q, want %q", got, c.want)
				}
			}
			if s.Export() != nil || s.Live(h) || h.seeds != 0 {
				t.Fatalf("a refusal changed the state: %+v, seeds %d", s.Export(), h.seeds)
			}
			if _, ok := s.View(h, cargo); ok {
				t.Fatal("a refused session has a view")
			}
		})
	}
	h, _ := world()
	if got := (*State)(nil).Reason(h, testCid, ""); got != "" {
		t.Fatalf("a welcome visit's reason %q", got)
	}
}

// TestSettlementImportBounds: hostile save data is bounded. Negative and
// duplicate tiles go, markets cap at maxMarkets and lines at maxLines,
// negative quantities and silver clamp, crates and empty lines go, and the
// result is sorted.
func TestSettlementImportBounds(t *testing.T) {
	t.Parallel()
	var lines []trade.Line
	for i := range maxLines + 40 {
		lines = append(lines, trade.Line{Kind: item.Kind(i % 30), Qty: i + 1})
	}
	lines = append([]trade.Line{{Kind: item.Steel, Qty: -5}, {Kind: item.MinifiedBuilding, Qty: 2, Building: 1}}, lines...)
	sv := &Save{Version: 1, Markets: []trade.Market{
		{Tile: 9, Stock: trade.Stock{Silver: -40, Lines: lines}},
		{Tile: -3},
		{Tile: 9, Stock: trade.Stock{Silver: 7}},
		{Tile: 2, Stock: trade.Stock{Lines: []trade.Line{{Kind: item.Steel, Qty: trade.MaxOfferQty * 9}}}},
	}}
	for i := range maxMarkets + 10 {
		sv.Markets = append(sv.Markets, trade.Market{Tile: int32(100 + i)})
	}
	s := Import(sv)
	if len(s.markets) != maxMarkets {
		t.Fatalf("%d markets, want %d", len(s.markets), maxMarkets)
	}
	if !slices.IsSortedFunc(s.markets, func(a, b trade.Market) int { return int(a.Tile) - int(b.Tile) }) {
		t.Fatal("markets unsorted")
	}
	if s.markets[0].Tile != 2 || s.markets[1].Tile != 9 || s.markets[2].Tile != 100 {
		t.Fatalf("tiles %d %d %d", s.markets[0].Tile, s.markets[1].Tile, s.markets[2].Tile)
	}
	if q := s.markets[0].Stock.Lines[0].Qty; q > trade.MaxOfferQty {
		t.Fatalf("line qty %d past the offer bound", q)
	}
	m := s.markets[1]
	if m.Stock.Silver != 0 || len(m.Stock.Lines) > maxLines {
		t.Fatalf("silver %d, %d lines", m.Stock.Silver, len(m.Stock.Lines))
	}
	for i, l := range m.Stock.Lines {
		if l.Qty <= 0 || l.Building != 0 {
			t.Fatalf("line %+v kept", l)
		}
		if i > 0 && m.Stock.Lines[i-1].Goods().Compare(l.Goods()) >= 0 {
			t.Fatal("lines unsorted or duplicated")
		}
	}
	if sv.Markets[0].Stock.Lines[0].Qty != -5 {
		t.Fatal("Import changed the save")
	}
	if Import(nil) != nil || (*State)(nil).Export() != nil || (&State{}).Export() != nil {
		t.Fatal("an empty state saves something")
	}
}

// TestSettlementSaveRoundTrip: export, import and export again give the
// same gob bytes, and the export is a copy.
func TestSettlementSaveRoundTrip(t *testing.T) {
	t.Parallel()
	h, c := world()
	s := &State{}
	if why := s.Open(h, c, testCid, ""); why != "" {
		t.Fatal(why)
	}
	a := s.Export()
	b := Import(a).Export()
	if !bytes.Equal(gobOf(t, a), gobOf(t, b)) {
		t.Fatal("a round trip changed the save")
	}
	a.Markets[0].Stock.Lines[0].Qty = -1
	if s.markets[0].Stock.Lines[0].Qty < 0 {
		t.Fatal("Export shares the stock")
	}
}

func gobOf(t *testing.T, v any) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(v); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestSettlementDrop: Drop forgets the tile's market and ends a session
// there; other markets stay, and nil is safe.
func TestSettlementDrop(t *testing.T) {
	t.Parallel()
	h, c := world()
	s := &State{}
	if why := s.Open(h, c, testCid, ""); why != "" {
		t.Fatal(why)
	}
	s.markets = append([]trade.Market{{Tile: 1}}, s.markets...)
	s.Drop(testTile)
	if s.Live(h) || len(s.markets) != 1 || s.markets[0].Tile != 1 {
		t.Fatalf("after Drop: live %v, markets %+v", s.Live(h), s.markets)
	}
	s.Drop(5)
	(*State)(nil).Drop(1)
	(*State)(nil).Close()
	if len(s.markets) != 1 {
		t.Fatal("dropping an unknown tile changed the markets")
	}
}

// TestSiteInAndVisitOf: SiteIn finds the faction holding a tile and the
// settlement's name; VisitOf maps a caravan's state.
func TestSiteInAndVisitOf(t *testing.T) {
	t.Parallel()
	h, _ := world()
	if fi, f, name := SiteIn(h.facs, testTile); fi != 1 || f != &h.facs[1] || name != "Oakmere" {
		t.Fatalf("SiteIn = %d %p %q", fi, f, name)
	}
	h.facs[0].SettlementNames = nil
	if fi, _, name := SiteIn(h.facs, 10); fi != 0 || name != "" {
		t.Fatalf("unnamed settlement: %d %q", fi, name)
	}
	if fi, f, _ := SiteIn(h.facs, 77); fi != -1 || f != nil {
		t.Fatal("an empty tile has a site")
	}
	if SeedOf(nil) != 0 || TilesOf(nil) != 0 {
		t.Fatal("no planet has a seed or tiles")
	}
	if p := (&planet.Planet{Seed: 5, Center: make([][3]float32, 3)}); SeedOf(p) != 5 || TilesOf(p) != 3 {
		t.Fatalf("SeedOf %d, TilesOf %d", SeedOf(p), TilesOf(p))
	}
	c := caravan.Caravan{Tile: 4, State: caravan.Visiting, VisitUntil: 90, Members: []uint64{1}}
	if v := VisitOf(&c, false); !v.Visiting || v.Forming || v.Tile != 4 || v.Until != 90 || len(v.Members) != 1 {
		t.Fatalf("visiting: %+v", v)
	}
	if v := VisitOf(&c, true); v.Visiting {
		t.Fatal("a caravan heading home is visiting")
	}
	c.State = caravan.Forming
	if v := VisitOf(&c, false); !v.Forming || v.Visiting {
		t.Fatalf("forming: %+v", v)
	}
}

// TestSettlementImportHugeSilver (security S1): a save with a market's
// silver near MaxInt imports capped at maxSilver, and a following buy (the
// caravan pays into the stock) and a silver gift saturate there instead of
// overflowing.
func TestSettlementImportHugeSilver(t *testing.T) {
	t.Parallel()
	h, c := world()
	huge := trade.Market{Tile: testTile, Faction: 1, NextAt: h.now + 1,
		Stock: trade.Stock{Kind: trade.Quartermaster, Silver: math.MaxInt, Lines: []trade.Line{{Kind: item.Rifle, Qty: 5}}}}
	s := Import(&Save{Version: 1, Markets: []trade.Market{huge}})
	if got := s.markets[0].Stock.Silver; got != maxSilver {
		t.Fatalf("imported silver %d, want the cap %d", got, maxSilver)
	}
	if why := s.Open(h, c, testCid, ""); why != "" {
		t.Fatal(why)
	}
	if s.markets[0].Stock.Silver != maxSilver {
		t.Fatal("Open re-rolled the capped market")
	}
	if note := s.Propose(h, c, offer(trade.Delta{Kind: item.Rifle, Qty: 1}), false); note != "" {
		t.Fatalf("buy refused: %q", note)
	}
	if got := s.markets[0].Stock.Silver; got != maxSilver {
		t.Fatalf("stock silver after a buy %d, want saturated at %d", got, maxSilver)
	}
	if count(c.goods[testCid])[item.Silver] >= 1000 || count(c.goods[testCid])[item.Rifle] != 1 {
		t.Fatalf("the buy did not happen: %v", count(c.goods[testCid]))
	}
	if note := s.Propose(h, c, offer(trade.Delta{Kind: item.Silver, Qty: -100}), true); note != "" {
		t.Fatalf("silver gift refused: %q", note)
	}
	if got := s.markets[0].Stock.Silver; got != maxSilver {
		t.Fatalf("stock silver after a gift %d", got)
	}
	if addSilver(maxSilver-5, 3) != maxSilver-2 || addSilver(maxSilver-5, math.MaxInt) != maxSilver {
		t.Fatal("addSilver does not saturate")
	}
}
