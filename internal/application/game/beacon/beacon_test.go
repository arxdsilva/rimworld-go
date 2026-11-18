// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package beacon

import (
	"slices"
	"testing"
)

func TestReachStubIsTheOldDisc(t *testing.T) {
	t.Parallel()
	var got []int
	Reach(20, 20, 10, 10, nil, func(i int) { got = append(got, i) })
	var want []int
	for y := range 20 {
		for x := range 20 {
			if (x-10)*(x-10)+(y-10)*(y-10) <= 36 {
				want = append(want, y*20+x)
			}
		}
	}
	if !slices.Equal(got, want) {
		t.Fatalf("Reach = %d cells, want %d", len(got), len(want))
	}
}

func TestReachClipsAtTheEdge(t *testing.T) {
	t.Parallel()
	n := 0
	Reach(4, 4, 0, 0, nil, func(i int) {
		if i < 0 || i >= 16 {
			t.Fatalf("cell %d is off the map", i)
		}
		n++
	})
	if n == 0 {
		t.Fatal("no cells")
	}
}
