// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package faction

import (
	"math/rand/v2"
	"slices"
	"testing"
)

// fakeGraph is an explicit adjacency list; water marks non-land tiles.
type fakeGraph struct {
	adj   [][]int32
	water map[int]bool
}

func (g *fakeGraph) TileCount() int           { return len(g.adj) }
func (g *fakeGraph) Land(i int) bool          { return !g.water[i] }
func (g *fakeGraph) Neighbours(i int) []int32 { return g.adj[i] }

// ring is n tiles in a cycle.
func ring(n int) *fakeGraph {
	g := &fakeGraph{adj: make([][]int32, n), water: map[int]bool{}}
	for i := range n {
		g.adj[i] = []int32{int32((i + n - 1) % n), int32((i + 1) % n)}
	}
	return g
}

// grid is a w x h four-connected grid.
func grid(w, h int) *fakeGraph {
	g := &fakeGraph{adj: make([][]int32, w*h), water: map[int]bool{}}
	for y := range h {
		for x := range w {
			i := y*w + x
			if x > 0 {
				g.adj[i] = append(g.adj[i], int32(i-1))
			}
			if x < w-1 {
				g.adj[i] = append(g.adj[i], int32(i+1))
			}
			if y > 0 {
				g.adj[i] = append(g.adj[i], int32(i-w))
			}
			if y < h-1 {
				g.adj[i] = append(g.adj[i], int32(i+w))
			}
		}
	}
	return g
}

func TestTerritoryNearestAndTies(t *testing.T) {
	t.Parallel()
	type row struct {
		name    string
		g       *fakeGraph
		seeds   []Seed
		radius  int
		wantOwn map[int]int // tile -> owner
		wantSet map[int]int // tile -> seat
	}
	rows := []row{
		{"ring nearest", ring(12), []Seed{{0, 0}, {6, 1}}, 6,
			map[int]int{0: 0, 1: 0, 2: 0, 4: 1, 5: 1, 6: 1, 7: 1, 11: 0},
			map[int]int{1: 0, 5: 1}},
		{"ring tie goes to lower seed index", ring(12), []Seed{{0, 0}, {6, 1}}, 6,
			map[int]int{3: 0, 9: 0}, map[int]int{3: 0, 9: 0}},
		{"tie lower index wins even when listed second tile first", ring(12), []Seed{{6, 1}, {0, 0}}, 6,
			map[int]int{3: 1, 9: 1}, map[int]int{3: 0, 9: 0}},
		{"grid tie", grid(5, 1), []Seed{{0, 2}, {4, 3}}, 4,
			map[int]int{2: 2, 1: 2, 3: 3}, map[int]int{2: 0, 3: 1}},
		{"same tile twice: first seed keeps it", ring(8), []Seed{{2, 0}, {2, 1}}, 1,
			map[int]int{2: 0, 1: 0, 3: 0}, map[int]int{2: 0}},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			var tr Territory
			tr.Build(r.g, r.seeds, 4, r.radius)
			for tile, want := range r.wantOwn {
				if got := tr.Owner(tile); got != want {
					t.Errorf("Owner(%d) = %d, want %d", tile, got, want)
				}
			}
			for tile, want := range r.wantSet {
				if got := tr.Seat(tile); got != want {
					t.Errorf("Seat(%d) = %d, want %d", tile, got, want)
				}
			}
		})
	}
}

// TestTerritoryEquidistantSeat pins the tie-break: tile 3 of a 7-ring is
// three hops from both seeds, and the lower seed index must own it in both
// input orderings (the faction follows the seat).
func TestTerritoryEquidistantSeat(t *testing.T) {
	t.Parallel()
	a, b := Seed{Tile: 0, Faction: 1}, Seed{Tile: 6, Faction: 2}
	for _, c := range []struct {
		name      string
		seeds     []Seed
		seat, own int
	}{
		{"a first", []Seed{a, b}, 0, 1},
		{"b first", []Seed{b, a}, 0, 2},
	} {
		var tr Territory
		g := ring(12)
		tr.Build(g, c.seeds, 3, 3)
		if tr.Seat(3) != c.seat || tr.Owner(3) != c.own {
			t.Errorf("%s: tile 3 seat %d owner %d, want %d %d", c.name, tr.Seat(3), tr.Owner(3), c.seat, c.own)
		}
		if tr.Seat(9) != c.seat || tr.Owner(9) != c.own {
			t.Errorf("%s: tile 9 seat %d owner %d, want %d %d", c.name, tr.Seat(9), tr.Owner(9), c.seat, c.own)
		}
	}
}

func TestTerritoryWaterAndRadius(t *testing.T) {
	t.Parallel()
	g := ring(20)
	g.water[3] = true
	g.water[10] = true
	var tr Territory
	tr.Build(g, []Seed{{0, 0}, {15, 1}, {10, 2}}, 3, 2)
	// Seed on water is skipped, water is never owned.
	for _, i := range []int{3, 10} {
		if tr.Owner(i) != -1 || tr.Seat(i) != -1 {
			t.Errorf("water tile %d owned by %d seat %d", i, tr.Owner(i), tr.Seat(i))
		}
	}
	// Radius 2 from tile 0: 18,19,0,1,2 (3 is water).
	for _, i := range []int{18, 19, 0, 1, 2} {
		if tr.Owner(i) != 0 {
			t.Errorf("Owner(%d) = %d, want 0", i, tr.Owner(i))
		}
	}
	// Beyond the radius, or behind water, stays unclaimed.
	for _, i := range []int{4, 5, 8, 11, 12} {
		if tr.Owner(i) != -1 {
			t.Errorf("Owner(%d) = %d, want -1", i, tr.Owner(i))
		}
	}
	if got := tr.Tiles(2); len(got) != 0 {
		t.Errorf("water seed faction owns %v", got)
	}
	if got, want := tr.Tiles(1), []int32{13, 14, 15, 16, 17}; !slices.Equal(got, want) {
		t.Errorf("Tiles(1) = %v, want %v", got, want)
	}
	// Radius 0 claims only the seed tile; negative claims nothing.
	tr.Build(g, []Seed{{0, 0}}, 1, 0)
	if tr.Owner(0) != 0 || tr.Owner(1) != -1 {
		t.Errorf("radius 0: owners %d %d", tr.Owner(0), tr.Owner(1))
	}
	tr.Build(g, []Seed{{0, 0}}, 1, -1)
	if tr.Owner(0) != 0 || tr.Owner(1) != -1 {
		t.Errorf("radius -1: owners %d %d", tr.Owner(0), tr.Owner(1))
	}
}

func TestTerritoryBadSeedsSkipped(t *testing.T) {
	t.Parallel()
	var tr Territory
	tr.Build(ring(6), []Seed{{-1, 0}, {99, 0}, {1, 5}, {2, -1}, {3, 1}}, 2, 0)
	if got := tr.Tiles(1); !slices.Equal(got, []int32{3}) {
		t.Errorf("Tiles(1) = %v", got)
	}
	if tr.Seat(3) != 4 {
		t.Errorf("Seat(3) = %d, want 4 (index in the given slice)", tr.Seat(3))
	}
	if len(tr.Tiles(0)) != 0 {
		t.Errorf("Tiles(0) = %v", tr.Tiles(0))
	}
}

func TestTerritoryOrderFree(t *testing.T) {
	t.Parallel()
	g := grid(15, 15)
	seeds := []Seed{{16, 0}, {40, 1}, {100, 2}, {120, 0}, {200, 1}, {30, 2}}
	var want Territory
	want.Build(g, seeds, 3, 6)
	if len(want.Tiles(0))+len(want.Tiles(1))+len(want.Tiles(2)) == 0 {
		t.Fatal("nothing claimed")
	}
	rng := rand.New(rand.NewPCG(1, 2))
	for trial := range 20 {
		sh := &fakeGraph{adj: make([][]int32, len(g.adj)), water: g.water}
		for i, nb := range g.adj {
			sh.adj[i] = slices.Clone(nb)
			rng.Shuffle(len(nb), func(a, b int) { sh.adj[i][a], sh.adj[i][b] = sh.adj[i][b], sh.adj[i][a] })
		}
		var got Territory
		got.Build(sh, seeds, 3, 6)
		if !slices.Equal(got.Owners(), want.Owners()) {
			t.Fatalf("trial %d: neighbour order changed the owners", trial)
		}
		for i := range g.adj {
			if got.Seat(i) != want.Seat(i) {
				t.Fatalf("trial %d: seat %d differs", trial, i)
			}
		}
	}
	// Permuting the seeds (seat indexes follow the new order) keeps each
	// tile's owner unless a tie flips it, so check the tie-free tiles: the
	// nearest seed is unique there.
	rev := slices.Clone(seeds)
	slices.Reverse(rev)
	var r Territory
	r.Build(g, rev, 3, 6)
	for i := range g.adj {
		w, got := want.Seat(i), r.Seat(i)
		if (w == -1) != (got == -1) {
			t.Fatalf("tile %d claimed in one order only", i)
		}
	}
}

func TestTerritoryRebuildResets(t *testing.T) {
	t.Parallel()
	g := ring(30)
	var tr Territory
	tr.Build(g, []Seed{{0, 0}, {15, 1}}, 2, 4)
	v := tr.Version()
	tr.Build(g, []Seed{{20, 1}}, 2, 2)
	if tr.Version() != v+1 {
		t.Errorf("version %d, want %d", tr.Version(), v+1)
	}
	if len(tr.Tiles(0)) != 0 || tr.Owner(0) != -1 || tr.Owner(15) != -1 || tr.Seat(1) != -1 {
		t.Errorf("stale claims after rebuild: %v %d %d", tr.Tiles(0), tr.Owner(0), tr.Owner(15))
	}
	if !slices.Equal(tr.Tiles(1), []int32{18, 19, 20, 21, 22}) {
		t.Errorf("Tiles(1) = %v", tr.Tiles(1))
	}
	// A different graph size and a growing faction count reallocate.
	tr.Build(ring(10), []Seed{{1, 5}}, 6, 1)
	if got := tr.Tiles(5); !slices.Equal(got, []int32{0, 1, 2}) {
		t.Errorf("Tiles(5) = %v", got)
	}
	// Shrinking the faction count drops the extra lists.
	tr.Build(ring(10), nil, 1, 1)
	if tr.Tiles(5) != nil || tr.Tiles(-1) != nil {
		t.Error("out-of-range faction must give nil")
	}
}

func TestTerritoryNilAndZero(t *testing.T) {
	t.Parallel()
	var nilT *Territory
	if nilT.Owner(0) != -1 || nilT.Seat(0) != -1 || nilT.Tiles(0) != nil || nilT.Owners() != nil || nilT.Version() != 0 {
		t.Error("nil territory must read as empty")
	}
	var z Territory
	if z.Owner(0) != -1 || z.Seat(5) != -1 || z.Tiles(0) != nil || z.Version() != 0 {
		t.Error("zero territory must read as empty")
	}
	g := ring(4)
	z.Build(g, nil, 0, 3)
	if z.Owner(-1) != -1 || z.Owner(4) != -1 || z.Seat(-1) != -1 || z.Seat(4) != -1 {
		t.Error("out-of-range tile must be -1")
	}
}

// bigGrid is a w x h grid with a few lakes.
func benchSetup() (*fakeGraph, []Seed) {
	g := grid(150, 140) // 21000 tiles
	for i := 0; i < len(g.adj); i += 97 {
		g.water[i] = true
	}
	var seeds []Seed
	for k := 0; k < 60; k++ {
		seeds = append(seeds, Seed{Tile: int32(k*341 + 7), Faction: int16(k % 5)})
	}
	return g, seeds
}

func BenchmarkTerritoryBuild(b *testing.B) {
	g, seeds := benchSetup()
	var tr Territory
	tr.Build(g, seeds, 5, TerritoryHops)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		tr.Build(g, seeds, 5, TerritoryHops)
	}
}

func TestTerritoryBuildNoAllocsAfterFirst(t *testing.T) {
	g, seeds := benchSetup()
	var tr Territory
	tr.Build(g, seeds, 5, TerritoryHops)
	tr.Build(g, seeds, 5, TerritoryHops)
	if a := testing.AllocsPerRun(5, func() { tr.Build(g, seeds, 5, TerritoryHops) }); a != 0 {
		t.Errorf("allocs per Build = %v, want 0", a)
	}
}
