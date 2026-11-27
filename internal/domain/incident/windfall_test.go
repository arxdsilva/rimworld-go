// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package incident

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/geology"
)

// seq returns a roll func replaying vals (0 once spent) and the list of n
// it was asked for.
func seq(vals ...int) (func(int) int, *[]int) {
	var asked []int
	return func(n int) int {
		asked = append(asked, n)
		if len(asked) <= len(vals) {
			return vals[len(asked)-1]
		}
		return 0
	}, &asked
}

func TestSkySongTiming(t *testing.T) {
	var st State
	if st.Song(0) || st.SongView(0) != 0 {
		t.Fatal("zero State must be quiet")
	}
	if st.StartSong(10, 0) || st.SongUntil != 0 {
		t.Fatal("ticks 0 must refuse")
	}
	if !st.StartSong(10, 100) || st.SongUntil != 110 {
		t.Fatalf("start: until %d", st.SongUntil)
	}
	if st.StartSong(50, 500) || st.SongUntil != 110 {
		t.Fatal("must refuse while playing")
	}
	if !st.Song(109) || st.SongView(109) != 110 {
		t.Fatal("playing view")
	}
	if st.Song(110) || st.SongView(110) != 0 {
		t.Fatal("ended view")
	}
	if !st.StartSong(200, 5) || st.SongUntil != 205 {
		t.Fatal("restart after end")
	}
}

func TestSongTicksBounds(t *testing.T) {
	span := (SongMax-SongMin)*clock.TicksPerDay/100 + 1
	for _, c := range []struct {
		r    int
		want uint64
	}{
		{0, SongMin * clock.TicksPerDay / 100},
		{span - 1, SongMax * clock.TicksPerDay / 100},
		{-5, SongMin * clock.TicksPerDay / 100},
	} {
		roll, asked := seq(c.r)
		if got := SongTicks(roll); got != c.want {
			t.Fatalf("roll %d: %d want %d", c.r, got, c.want)
		}
		if !slices.Equal(*asked, []int{span}) {
			t.Fatalf("asked %v want one draw of %d", *asked, span)
		}
	}
}

func TestSongDays(t *testing.T) {
	d := uint64(clock.TicksPerDay)
	for _, c := range []struct {
		ticks uint64
		want  int
	}{{0, 1}, {d / 4, 1}, {d * 3 / 2, 2}, {d*3/2 - 1, 1}, {5 * d, 5}} {
		if got := SongDays(c.ticks); got != c.want {
			t.Fatalf("SongDays(%d)=%d want %d", c.ticks, got, c.want)
		}
	}
}

func TestMeteorCells(t *testing.T) {
	for r, want := range map[int]int{-1: 4, 0: 4, 3: 7, 99: 7} {
		roll, asked := seq(r)
		if got := MeteorCells(roll); got != want {
			t.Fatalf("roll %d: %d want %d", r, got, want)
		}
		if !slices.Equal(*asked, []int{4}) {
			t.Fatalf("asked %v", *asked)
		}
	}
}

func TestPickOreWeights(t *testing.T) {
	got := map[geology.Ore]int{}
	for r := 0; r < 10; r++ {
		roll, asked := seq(r)
		o := PickOre(roll)
		if o == geology.NoOre || !slices.Equal(*asked, []int{10}) {
			t.Fatalf("roll %d: ore %v asked %v", r, o, *asked)
		}
		got[o]++
	}
	if got[geology.Steel] != 6 || got[geology.Silver] != 3 || got[geology.Gold] != 1 {
		t.Fatalf("weights %v", got)
	}
}

func TestPickSupplies(t *testing.T) {
	ranges := map[item.Kind][2]int{}
	for _, e := range lootTable() {
		ranges[e.kind] = [2]int{e.min, e.max}
	}
	seed := 7
	next := func(n int) int { // deterministic LCG
		seed = (seed*1103515245 + 12345) & 0x7fffffff
		return seed % n
	}
	counts := map[int]bool{}
	for i := 0; i < 500; i++ {
		saved := seed
		a := PickSupplies(next)
		seed = saved
		b := PickSupplies(next)
		if !slices.Equal(a, b) {
			t.Fatal("same rolls must give the same drop")
		}
		seen := map[item.Kind]bool{}
		for _, s := range a {
			r, ok := ranges[s.Kind]
			if !ok || seen[s.Kind] || s.Qty < r[0] || s.Qty > r[1] {
				t.Fatalf("bad stack %+v", s)
			}
			seen[s.Kind] = true
		}
		counts[len(a)] = true
	}
	if len(counts) != 2 || !counts[2] || !counts[3] {
		t.Fatalf("counts %v", counts)
	}
}

func TestPickSuppliesDrawOrder(t *testing.T) {
	roll, asked := seq(1, 0, 0, 0, 0, 0, 0)
	got := PickSupplies(roll)
	// count (2), then kind of 6 and qty 31 (steel), kind of 5 and qty 26
	// (cloth swapped in), kind of 4 and qty 5 (herbal medicine).
	if want := []int{2, 6, 31, 5, 26, 4, 5}; !slices.Equal(*asked, want) {
		t.Fatalf("asked %v want %v", *asked, want)
	}
	if len(got) != 3 {
		t.Fatalf("got %v", got)
	}
	// Out-of-range rolls clamp instead of panicking.
	if g := PickSupplies(func(int) int { return 1 << 20 }); len(g) != 3 {
		t.Fatalf("clamp high: %v", g)
	}
	if g := PickSupplies(func(int) int { return -3 }); len(g) != 2 || g[0] != (Supply{item.Steel, 30}) {
		t.Fatalf("clamp low: %v", g)
	}
}

func TestSupplyFitsOneStack(t *testing.T) {
	tab := lootTable()
	if len(tab) != 6 {
		t.Fatal("six loot kinds")
	}
	for _, e := range tab {
		if e.max > e.kind.StackLimit() || e.min < 1 || e.min > e.max {
			t.Fatalf("kind %v: %d-%d vs limit %d", e.kind, e.min, e.max, e.kind.StackLimit())
		}
	}
}
