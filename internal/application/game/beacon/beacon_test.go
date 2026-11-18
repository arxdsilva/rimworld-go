// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package beacon

import (
	"slices"
	"testing"
)

// reach collects Reach's cells.
func reach(w, h, bx, by int, blocked func(x, y int) bool) []int {
	var got []int
	Reach(w, h, bx, by, blocked, func(i int) { got = append(got, i) })
	return got
}

// disc is every cell of the w x h map in the open disc around (bx, by),
// ascending (brute force over the whole map).
func disc(w, h, bx, by int) []int {
	var want []int
	for y := range h {
		for x := range w {
			if (x-bx)*(x-bx)+(y-by)*(y-by) <= 56 {
				want = append(want, y*w+x)
			}
		}
	}
	return want
}

// walls is a blocked func over a set of cells.
func walls(cells ...[2]int) func(x, y int) bool {
	return func(x, y int) bool { return slices.Contains(cells, [2]int{x, y}) }
}

// TestReachOpenDisc: on open ground the reach is the disc 15 cells across
// (177 cells; the widest row, through the beacon, holds 15).
func TestReachOpenDisc(t *testing.T) {
	t.Parallel()
	got := reach(40, 40, 20, 20, func(int, int) bool { return false })
	if want := disc(40, 40, 20, 20); !slices.Equal(got, want) {
		t.Fatalf("Reach = %v, want %v", got, want)
	}
	if len(got) != 177 {
		t.Fatalf("Reach = %d cells, want 177", len(got))
	}
	rows := map[int]int{}
	for _, i := range got {
		rows[i/40]++
	}
	if rows[20] != Diameter || rows[13] != 5 || rows[27] != 5 || rows[12] != 0 || rows[28] != 0 {
		t.Fatalf("rows %v: want 15 across the beacon's row, 5 at the tips", rows)
	}
	if !slices.Equal(reach(40, 40, 20, 20, nil), got) {
		t.Fatal("a nil blocked func is not open ground")
	}
}

// TestReachWallRingCuts: a closed wall ring keeps the reach inside it,
// even one with a corner missing (the flood never steps diagonally), and
// a gap in a side lets it out.
func TestReachWallRingCuts(t *testing.T) {
	t.Parallel()
	ring := func(skip ...[2]int) [][2]int {
		var out [][2]int
		for y := 17; y <= 23; y++ {
			for x := 17; x <= 23; x++ {
				if (x == 17 || x == 23 || y == 17 || y == 23) && !slices.Contains(skip, [2]int{x, y}) {
					out = append(out, [2]int{x, y})
				}
			}
		}
		return out
	}
	inside := 5 * 5
	for _, tc := range []struct {
		name string
		ring [][2]int
		want int
	}{
		{"closed", ring(), inside},
		{"corner missing", ring([2]int{23, 23}), inside},
	} {
		got := reach(40, 40, 20, 20, walls(tc.ring...))
		if len(got) != tc.want {
			t.Fatalf("%s ring: %d cells, want %d", tc.name, len(got), tc.want)
		}
		for _, i := range got {
			if x, y := i%40, i/40; x <= 17 || x >= 23 || y <= 17 || y >= 23 {
				t.Fatalf("%s ring: cell %d,%d is outside it", tc.name, x, y)
			}
		}
	}
	open := reach(40, 40, 20, 20, walls(ring([2]int{23, 20})...))
	if len(open) <= inside || !slices.Contains(open, 20*40+26) {
		t.Fatalf("a gap in the ring: %d cells, want the reach out through it", len(open))
	}
	for _, c := range ring([2]int{23, 20}) {
		if slices.Contains(open, c[1]*40+c[0]) {
			t.Fatalf("wall cell %v is in the reach", c)
		}
	}
}

// TestReachColumnsAndDoorsDoNotBlock: only what blocked reports cuts the
// reach; a cell it doesn't report (a column, a door) is crossed, and the
// beacon's own cell is never asked about.
func TestReachColumnsAndDoorsDoNotBlock(t *testing.T) {
	t.Parallel()
	asked := false
	got := reach(40, 40, 20, 20, func(x, y int) bool {
		if x == 20 && y == 20 {
			asked = true
		}
		return false // a column at 21,20 and a door at 20,22: passable
	})
	if asked {
		t.Fatal("blocked was asked about the beacon's own cell")
	}
	if !slices.Equal(got, disc(40, 40, 20, 20)) {
		t.Fatal("passable cells cut the reach")
	}
	// a beacon walled in on all four sides keeps only its own cell
	if got := reach(40, 40, 20, 20, walls([2]int{21, 20}, [2]int{19, 20}, [2]int{20, 21}, [2]int{20, 19})); !slices.Equal(got, []int{20*40 + 20}) {
		t.Fatalf("walled-in beacon reaches %v, want only its cell", got)
	}
}

// TestReachMapEdge: at a corner the reach is exactly the on-map quarter
// disc; no cell wraps to the other side of a row or leaves the map.
func TestReachMapEdge(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ w, h, bx, by int }{
		{4, 4, 0, 0}, {20, 20, 0, 0}, {20, 20, 19, 19}, {20, 10, 0, 5}, {20, 10, 19, 0},
	} {
		got := reach(tc.w, tc.h, tc.bx, tc.by, nil)
		if want := disc(tc.w, tc.h, tc.bx, tc.by); !slices.Equal(got, want) {
			t.Fatalf("%dx%d beacon %d,%d: Reach = %v, want %v", tc.w, tc.h, tc.bx, tc.by, got, want)
		}
	}
	if got := reach(10, 10, 10, 3, nil); got != nil {
		t.Fatalf("an off-map beacon reaches %v", got)
	}
	// the beacon, 7 cells along each edge and a quarter of the rest of the disc
	if got := reach(20, 20, 0, 0, nil); len(got) != 1+7+7+(177-1-4*7)/4 {
		t.Fatalf("corner beacon: %d cells, want 52", len(got))
	}
}

// TestReachDeterministicOrder: cells come in ascending index order, the
// same on every call.
func TestReachDeterministicOrder(t *testing.T) {
	t.Parallel()
	blocked := walls([2]int{22, 18}, [2]int{22, 19}, [2]int{22, 20}, [2]int{22, 21}, [2]int{18, 23})
	first := reach(40, 40, 20, 20, blocked)
	if !slices.IsSorted(first) || len(first) == 0 {
		t.Fatalf("cells not ascending: %v", first)
	}
	for range 5 {
		if !slices.Equal(reach(40, 40, 20, 20, blocked), first) {
			t.Fatal("a second call answered differently")
		}
	}
}

// TestReachAllocatesNothing: the flood's queue and marks live on the
// stack (not parallel: AllocsPerRun refuses parallel tests).
func TestReachAllocatesNothing(t *testing.T) {
	n := 0
	add := func(int) { n++ }
	open := func(int, int) bool { return false }
	if a := testing.AllocsPerRun(20, func() { Reach(40, 40, 20, 20, open, add) }); a != 0 {
		t.Fatalf("Reach allocates %v times a call", a)
	}
}
