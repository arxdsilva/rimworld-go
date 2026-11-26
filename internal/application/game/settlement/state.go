// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package settlement

import (
	"cmp"
	"slices"

	"rimworld-go/internal/application/game/tradeview"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/world/caravan"
	"rimworld-go/internal/domain/world/planet"
)

// State is the settlement markets (ascending Tile) and the open
// settlement trade session, if any. The session is never saved. A nil
// *State is safe for every read (and for Drop and Close); Open and
// Propose need a State.
type State struct {
	markets []trade.Market
	sess    *session
}

// session is the open settlement session: caravan cid at tile, its
// negotiator and the last refusal (tradeview.Trade.Reason).
type session struct {
	cid, negotiator uint64
	tile            int
	reason          string
}

// Save is State's saved form: the markets, ascending Tile.
type Save struct {
	Version uint8
	Markets []trade.Market
}

// saveVersion is the format Export writes.
const saveVersion = 1

// Import bounds: a save holds at most maxMarkets markets of at most
// maxLines lines each (no planet has that many settlements, no stock that
// many goods).
const (
	maxMarkets = 4096
	maxLines   = 512
	// maxSilver caps a market's silver (Import clamps to it, a trade
	// saturates at it): far below MaxInt64/2, so adding any one trade's
	// silver cannot overflow.
	maxSilver = 1 << 40
)

// Reasons a caravan can't trade where it stands (see Reason).
const (
	ReasonNoCaravan    = "No such caravan."
	ReasonForming      = "Still forming at the colony."
	ReasonNoSettlement = "No settlement here."
	ReasonNotVisiting  = "Only a caravan visiting a settlement can trade."
)

// SiteIn is Host.Site over the game's factions fs: the first faction
// holding tile among its Settlements (-1, nil: none) and that
// settlement's name ("" when it has none).
func SiteIn(fs []faction.Faction, tile int) (int, *faction.Faction, string) {
	for i := range fs {
		if k := slices.Index(fs[i].Settlements, tile); k >= 0 {
			name := ""
			if k < len(fs[i].SettlementNames) {
				name = fs[i].SettlementNames[k]
			}
			return i, &fs[i], name
		}
	}
	return -1, nil, ""
}

// VisitOf is caravan c as settlement trade sees it; back: it is heading
// home, so it is no longer visiting.
func VisitOf(c *caravan.Caravan, back bool) Visit {
	return Visit{Tile: c.Tile, Visiting: c.State == caravan.Visiting && !back, Forming: c.State == caravan.Forming,
		Until: c.VisitUntil, Members: c.Members}
}

// SeedOf is planet p's seed for Host.Seed (0: no planet).
func SeedOf(p *planet.Planet) uint64 {
	if p == nil {
		return 0
	}
	return p.Seed
}

// TilesOf is planet p's tile count for Host.Tiles (0: no planet).
func TilesOf(p *planet.Planet) int {
	if p == nil {
		return 0
	}
	return p.TileCount()
}

// Import rebuilds the state from a save (nil: none yet). Hostile data is
// bounded: negative and repeated tiles go (the first stays), at most
// maxMarkets markets and maxLines lines each are kept, quantities clamp
// to [0, trade.MaxOfferQty] and silver to [0, maxSilver], and crates (never in
// a market) and empty lines go. A far NextAt is clamped at Open, which
// knows the time.
func Import(s *Save) *State {
	if s == nil {
		return nil
	}
	ms := make([]trade.Market, 0, min(len(s.Markets), maxMarkets))
	for _, m := range s.Markets {
		if m.Tile >= 0 {
			ms = append(ms, m)
		}
	}
	slices.SortStableFunc(ms, func(a, b trade.Market) int { return cmp.Compare(a.Tile, b.Tile) })
	ms = slices.CompactFunc(ms, func(a, b trade.Market) bool { return a.Tile == b.Tile })
	ms = ms[:min(len(ms), maxMarkets)]
	for i := range ms {
		ms[i].Stock = cleanStock(ms[i].Stock)
	}
	return &State{markets: slices.Clip(ms)}
}

// cleanStock is st bounded for Import: a fresh, sorted copy.
func cleanStock(st trade.Stock) trade.Stock {
	out := trade.Stock{Kind: st.Kind, Silver: min(max(st.Silver, 0), maxSilver)}
	for _, l := range st.Lines[:min(len(st.Lines), maxLines)] {
		if l.Qty > 0 && l.Building == 0 {
			out.AddGoods(l.Goods(), min(l.Qty, trade.MaxOfferQty))
		}
	}
	for i := range out.Lines {
		out.Lines[i].Qty = min(out.Lines[i].Qty, trade.MaxOfferQty)
	}
	return out
}

// Export is s's saved form (a copy), nil when there is nothing to save.
func (s *State) Export() *Save {
	if s == nil || len(s.markets) == 0 {
		return nil
	}
	sv := &Save{Version: saveVersion, Markets: slices.Clone(s.markets)}
	for i := range sv.Markets {
		sv.Markets[i].Stock.Lines = slices.Clone(sv.Markets[i].Stock.Lines)
	}
	return sv
}

// find is the index of tile's market (or where it goes) and whether it
// is there.
func (s *State) find(tile int) (int, bool) {
	return slices.BinarySearchFunc(s.markets, tile, func(m trade.Market, t int) int { return cmp.Compare(int(m.Tile), t) })
}

// Drop forgets the market on tile (its settlement was lost) and ends a
// session there.
func (s *State) Drop(tile int) {
	if s == nil {
		return
	}
	if i, ok := s.find(tile); ok {
		s.markets = slices.Delete(s.markets, i, i+1)
	}
	if s.sess != nil && s.sess.tile == tile {
		s.sess = nil
	}
}

// Reason is why caravan cid can't trade where it stands ("" when it
// can), first match wins: no such caravan, still forming, no settlement
// on its tile, a faction that does not welcome the colony
// (tradeview.ReasonNoTrade), not visiting, then busy (the colony
// session's refusal, "" when none is open). It reads only Host.Caravan
// and Host.Site.
func (s *State) Reason(h Host, cid uint64, busy string) string {
	v, ok := h.Caravan(cid)
	switch {
	case !ok:
		return ReasonNoCaravan
	case v.Forming:
		return ReasonForming
	}
	fi, f, _ := h.Site(v.Tile)
	switch {
	case fi < 0 || f == nil:
		return ReasonNoSettlement
	case !f.Welcomes():
		return tradeview.ReasonNoTrade
	case !v.Visiting:
		return ReasonNotVisiting
	}
	return busy
}
