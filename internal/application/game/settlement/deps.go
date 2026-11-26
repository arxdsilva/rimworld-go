// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package settlement

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
)

// Host is what settlement trade reads from the game.
type Host interface {
	// Now is the current tick.
	Now() uint64
	// Seed is the built planet's seed (called only from Open, the
	// command path).
	Seed() uint64
	// Tiles is the built planet's tile count (0: no planet; called only
	// from Open, like Seed).
	Tiles() int
	// Items is the item book.
	Items() item.Book
	// Caravan is player caravan cid's visit state (ok false: no such
	// caravan).
	Caravan(cid uint64) (Visit, bool)
	// Site resolves the settlement on tile from the game's factions
	// (Faction.Settlements, SettlementNames), never from the planet: its
	// faction index fi (-1 none), the faction and the settlement's name.
	Site(tile int) (fi int, f *faction.Faction, name string)
	// Pawn is colonist id's name and Social level.
	Pawn(id uint64) (name string, social int)
	// Day is the colony day.
	Day() int
}

// Cargo is the caravan's goods (travel.Ops satisfies it as it is).
type Cargo interface {
	// Goods are caravan cid's goods.
	Goods(cid uint64) []item.Stack
	// Yield takes qty of kind k out of caravan cid's goods, reporting
	// whether it had them.
	Yield(cid uint64, k item.Kind, qty int) bool
	// CarryLines loads lines into caravan cid's goods.
	CarryLines(cid uint64, lines []trade.Line)
}

// Visit is a player caravan as settlement trade sees it: the Tile it is
// on, whether it is Visiting a settlement there (until tick Until) or
// still Forming, and its Members (pawn ids).
type Visit struct {
	Tile              int
	Visiting, Forming bool
	Until             uint64
	Members           []uint64
}
