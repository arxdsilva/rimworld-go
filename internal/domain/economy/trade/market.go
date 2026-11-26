// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package trade

import (
	"slices"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/pkg/randx"
)

// Settlement markets (#11): each settlement of a welcoming faction keeps a
// stock, rolled from the world seed and its tile and restocked lazily once
// its restock period is over.

// RestockDays is the restock period, in in-game days, a settlement's
// stock is rolled for (original tuning).
const RestockDays = 10

// Market is one settlement's trade stock, keyed by its world Tile, for
// the faction index Faction. Restocked is the tick the current period
// started (display only); NextAt the tick the next restock is due.
type Market struct {
	Tile              int32
	Faction           int16
	Restocked, NextAt uint64
	Stock             Stock
}

// stageSettlementStock keys the market's RNG stream (randx.NewKeyed stage).
const stageSettlementStock = 0x5E77_570C

// restockTicks is the restock period P in ticks.
const restockTicks = RestockDays * clock.TicksPerDay

// SettlementKind is the trader kind a settlement of faction type t
// stocks as: BulkHauler for Neolithic tech, Quartermaster for higher
// tech (original tuning). A non-human or unknown type is a BulkHauler.
func SettlementKind(t faction.Type) TraderKind {
	if !t.Human() || faction.DefOf(t).Tech == faction.Neolithic {
		return BulkHauler
	}
	return Quartermaster
}

// NewMarket rolls the market of settlement tile (faction index fi, type
// ft) at tick now, deterministically from seed and tile: the stock
// depends only on (seed, tile, fi) and the restock period now falls in.
func NewMarket(b item.Book, ft faction.Type, seed uint64, tile, fi int, now uint64) Market {
	phase := randx.Mix64(seed^uint64(tile)) % restockTicks
	k := (now + phase) / restockTicks
	m := Market{Tile: int32(tile), Faction: int16(fi), NextAt: (k+1)*restockTicks - phase}
	if k*restockTicks > phase {
		m.Restocked = k*restockTicks - phase
	}
	key := randx.NewKeyed(seed, k, stageSettlementStock, uint64(tile), uint64(fi))
	st := Generate(b, SettlementKind(ft), faction.DefOf(ft).Tech, randx.New(key.Uint64()))
	st.Lines = slices.DeleteFunc(st.Lines, func(l Line) bool { return l.Building != 0 }) // caravan cargo holds plain stacks
	m.Stock = st
	return m
}

// Due reports whether m's restock is due at tick now.
func (m Market) Due(now uint64) bool {
	return now >= m.NextAt
}

// Restock re-rolls m's stock for faction index fi (type ft) when it is
// due at now, or when its faction changed, and reports whether it did.
// After many elapsed periods it rolls once, at now's period.
func (m *Market) Restock(b item.Book, ft faction.Type, fi int, seed, now uint64) bool {
	if !m.Due(now) && int(m.Faction) == fi {
		return false
	}
	*m = NewMarket(b, ft, seed, int(m.Tile), fi, now)
	return true
}
