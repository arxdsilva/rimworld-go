// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package faction

import "slices"

// Territory (#11): the tiles a faction owns, derived from its standing
// settlements. Each settlement (a seed) claims the land within
// TerritoryHops of it; the seat of a tile is the settlement that owns it.

// TerritoryHops is how far, in land hops, a settlement's territory
// reaches (original tuning).
const TerritoryHops = 6

// Seed is one standing settlement a territory grows from: its world Tile
// and the index of the Faction that holds it.
type Seed struct {
	Tile    int32
	Faction int16
}

// Graph is what a territory build needs from the planet (planet
// implements it; faction never imports planet).
type Graph interface {
	TileCount() int
	Land(i int) bool
	Neighbours(i int) []int32
}

// Territory is each tile's owning faction and seat, rebuilt from the
// seeds by Build. The zero value is an empty territory (every tile
// unclaimed). Reads never mutate it, so a built Territory may be read
// from several goroutines.
type Territory struct {
	owner []int16 // per tile: owning faction index, -1 unclaimed
	seat  []int32 // per tile: owning seed index, -1 unclaimed
	dist  []int32 // per tile: hops from its seed, -1 unclaimed (Build scratch)
	tiles [][]int32

	claimed     []int32 // every tile claimed by the last Build
	front, next []int32 // ring scratch
	version     uint64
}

// Build rebuilds t from seeds over g: each land tile within radius hops
// of a seed goes to the nearest one (ties to the lowest seed index), for
// factions factions. The search is level-synchronous over land tiles only,
// so the result depends on neither neighbour nor queue order. A seed that
// is not on land, repeats an earlier seed's tile or names a faction
// outside [0, factions) is skipped. Buffers are reused, and only the tiles
// the last build claimed are reset. It bumps Version.
func (t *Territory) Build(g Graph, seeds []Seed, factions, radius int) {
	n := g.TileCount()
	t.reset(n, max(factions, 0))
	t.version++
	t.front = t.front[:0]
	for k, s := range seeds {
		i := int(s.Tile)
		if i < 0 || i >= n || int(s.Faction) < 0 || int(s.Faction) >= factions || t.dist[i] >= 0 || !g.Land(i) {
			continue
		}
		t.dist[i], t.seat[i] = 0, int32(k)
		t.claimed = append(t.claimed, s.Tile)
		t.front = append(t.front, s.Tile)
	}
	for d := int32(1); int(d) <= radius && len(t.front) > 0; d++ {
		t.next = t.next[:0]
		for _, u := range t.front {
			su := t.seat[u]
			for _, nb := range g.Neighbours(int(u)) {
				switch {
				case t.dist[nb] < 0:
					if !g.Land(int(nb)) {
						continue
					}
					t.dist[nb], t.seat[nb] = d, su
					t.next = append(t.next, nb)
				case t.dist[nb] == d && su < t.seat[nb]:
					t.seat[nb] = su
				}
			}
		}
		t.claimed = append(t.claimed, t.next...)
		t.front, t.next = t.next, t.front
	}
	slices.Sort(t.claimed)
	for _, i := range t.claimed {
		f := seeds[t.seat[i]].Faction
		t.owner[i] = f
		t.tiles[f] = append(t.tiles[f], i)
	}
}

// reset clears what the last Build claimed and sizes the buffers for n
// tiles and factions factions.
func (t *Territory) reset(n, factions int) {
	if len(t.owner) != n {
		t.owner, t.seat, t.dist = make([]int16, n), make([]int32, n), make([]int32, n)
		for i := range n {
			t.owner[i], t.seat[i], t.dist[i] = -1, -1, -1
		}
	} else {
		for _, i := range t.claimed {
			t.owner[i], t.seat[i], t.dist[i] = -1, -1, -1
		}
	}
	t.claimed = t.claimed[:0]
	if cap(t.tiles) < factions {
		t.tiles = append(t.tiles[:cap(t.tiles)], make([][]int32, factions-cap(t.tiles))...)
	}
	t.tiles = t.tiles[:factions]
	for f := range t.tiles {
		t.tiles[f] = t.tiles[f][:0]
	}
}

// Owner is the faction index that owns tile i, -1 when unclaimed or out
// of range.
func (t *Territory) Owner(i int) int {
	if t == nil || i < 0 || i >= len(t.owner) {
		return -1
	}
	return int(t.owner[i])
}

// Seat is the seed (settlement) index that owns tile i, -1 when
// unclaimed or out of range.
func (t *Territory) Seat(i int) int {
	if t == nil || i < 0 || i >= len(t.seat) {
		return -1
	}
	return int(t.seat[i])
}

// Tiles are the tiles faction f owns, ascending (shared, read-only); nil
// for an out-of-range f.
func (t *Territory) Tiles(f int) []int32 {
	if t == nil || f < 0 || f >= len(t.tiles) {
		return nil
	}
	return t.tiles[f]
}

// Owners is the owning faction index per tile (-1 unclaimed), shared and
// read-only.
func (t *Territory) Owners() []int16 {
	if t == nil {
		return nil
	}
	return t.owner
}

// Version counts the builds, so a reader can tell a stale cache.
func (t *Territory) Version() uint64 {
	if t == nil {
		return 0
	}
	return t.version
}
