// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package incident

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/geology"
)

// Windfalls (#13, original tuning): an ore meteor raises a small cluster
// of ore-bearing rough rock near home, a supply drop lands a few forbidden
// stacks near home, and the sky song lifts every colonist's mood until
// State.SongUntil.

// The sky song's length, hundredths of a day.
const SongMin, SongMax = 150, 250

// An ore meteor's cell count.
const MeteorCellsMin, MeteorCellsMax = 4, 7

// An ore meteor lands at most MeteorRadius and at least MeteorMinDist
// tiles (Chebyshev) from home.
const MeteorRadius, MeteorMinDist = 14, 5

// A supply drop's distinct stacks.
const SupplyStacksMin, SupplyStacksMax = 2, 3

// A supply drop lands at most SupplyRadius tiles (Chebyshev) from home.
const SupplyRadius = 12

// Supply is one stack a supply drop lands.
type Supply struct {
	Kind item.Kind
	Qty  int
}

// StartSong starts the sky song at tick now lasting ticks; false when
// ticks is 0 or a song already plays.
func (st *State) StartSong(now, ticks uint64) bool {
	if ticks == 0 || st.Song(now) {
		return false
	}
	st.SongUntil = now + ticks
	return true
}

// Song reports the sky song playing at tick now.
func (st State) Song(now uint64) bool { return now < st.SongUntil }

// SongView is SongUntil while the song plays at tick now, else 0.
func (st State) SongView(now uint64) uint64 {
	if st.Song(now) {
		return st.SongUntil
	}
	return 0
}

// SongTicks is the sky song's length for one draw of roll: between
// SongMin and SongMax hundredths of a day.
func SongTicks(roll func(int) int) uint64 {
	r := roll((SongMax-SongMin)*clock.TicksPerDay/100 + 1)
	return SongMin*clock.TicksPerDay/100 + uint64(max(r, 0))
}

// SongDays is ticks in whole days, rounded, at least 1 (letter text).
func SongDays(ticks uint64) int {
	return max(1, int((ticks+clock.TicksPerDay/2)/clock.TicksPerDay))
}

// MeteorCells is an ore meteor's cell count for one draw of roll, in
// [MeteorCellsMin, MeteorCellsMax].
func MeteorCells(roll func(int) int) int {
	n := MeteorCellsMax - MeteorCellsMin + 1
	return MeteorCellsMin + min(max(roll(n), 0), n-1)
}

// PickOre is the ore a meteor bears: steel 6, silver 3, gold 1 by weight.
func PickOre(roll func(int) int) geology.Ore {
	r := roll(10) // original tuning (#13): total weight 6+3+1
	switch {
	case r < 6:
		return geology.Steel
	case r < 9:
		return geology.Silver
	}
	return geology.Gold
}

// lootEntry is one row of the supply loot table.
type lootEntry struct {
	kind     item.Kind
	min, max int
}

// lootTable is the supply drop's loot, original tuning (#13); every max
// fits one stack of its kind.
// original tuning (#13)
func lootTable() [6]lootEntry {
	return [6]lootEntry{
		{item.Steel, 30, 60},
		{item.Silver, 60, 150},
		{item.Components, 2, 5},
		{item.IndustrialMedicine, 2, 4},
		{item.HerbalMedicine, 4, 8},
		{item.Cloth, 25, 50},
	}
}

// PickSupplies is a supply drop's stacks: 2-3 distinct kinds with rolled
// quantities. Draw order: the count, then each entry's kind (from the
// kinds left) and quantity.
func PickSupplies(roll func(int) int) []Supply {
	pool := lootTable()
	left := len(pool)
	span := SupplyStacksMax - SupplyStacksMin + 1
	n := SupplyStacksMin + min(max(roll(span), 0), span-1)
	out := make([]Supply, 0, n)
	for range n {
		i := min(max(roll(left), 0), left-1)
		e := pool[i]
		pool[i] = pool[left-1]
		left--
		q := e.max - e.min + 1
		out = append(out, Supply{Kind: e.kind, Qty: e.min + min(max(roll(q), 0), q-1)})
	}
	return out
}
