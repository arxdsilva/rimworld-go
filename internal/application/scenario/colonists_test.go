// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package scenario

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
)

func TestCacheFileNames(t *testing.T) {
	v := game.SaveVersion
	tests := []struct {
		colonists int
		raid      bool
		want      string
	}{
		{20, false, fmt.Sprintf("grown-v%d-s1-120x120.sav", v)},
		{20, true, fmt.Sprintf("grown-v%d-s1-120x120-raid.sav", v)},
		{3, false, fmt.Sprintf("grown-v%d-s1-120x120-c3.sav", v)},
		{3, true, fmt.Sprintf("grown-v%d-s1-120x120-c3-raid.sav", v)},
	}
	for _, tc := range tests {
		got := filepath.Base(CacheFile(1, 120, 120, tc.colonists, tc.raid))
		if got != tc.want {
			t.Errorf("CacheFile(c%d, raid %v) = %s, want %s", tc.colonists, tc.raid, got, tc.want)
		}
	}
}

func TestBuildColonistCounts(t *testing.T) {
	for _, n := range []int{3, 10, 20} {
		g, c, err := Build(1, 120, 120, n)
		if err != nil {
			t.Fatalf("n=%d: %v", n, err)
		}
		if c.Colonists != n || len(g.Snapshot()) != n {
			t.Errorf("n=%d: counts %d, snapshot %d", n, c.Colonists, len(g.Snapshot()))
		}
	}
}

func TestBuildColonistErrors(t *testing.T) {
	for _, n := range []int{2, 0, -1, 1 << 20} {
		g, _, err := Build(1, 120, 120, n)
		if err == nil || g != nil {
			t.Fatalf("n=%d: want an error and no game, got %v", n, err)
		}
		if !strings.HasPrefix(err.Error(), "grown colony: ") || !strings.Contains(err.Error(), "colonists") {
			t.Errorf("n=%d: error %q", n, err)
		}
	}
}

// TestBuildExtras places 20 colonists beyond the fixed layout without a
// warm-up (the only scenario test above 20 colonists that builds).
func TestBuildExtras(t *testing.T) {
	build := func() (*game.Game, Counts) {
		g, c, err := Build(1, 120, 120, 40)
		if err != nil {
			t.Fatal(err)
		}
		return g, c
	}
	g, c := build()
	if c.Colonists != 40 {
		t.Fatalf("colonists = %d, want 40", c.Colonists)
	}
	site := siteRect(60, 60)
	seen := map[cell]bool{}
	pawns := g.Snapshot()
	if len(pawns) != 40 {
		t.Fatalf("snapshot = %d pawns, want 40", len(pawns))
	}
	for _, p := range pawns {
		k := cell{p.X, p.Y}
		if seen[k] {
			t.Errorf("two pawns on (%d, %d)", p.X, p.Y)
		}
		seen[k] = true
	}
	w := g.World()
	for _, p := range pawns[20:] {
		if site.contains(p.X, p.Y) {
			t.Errorf("extra %s at (%d, %d) is inside the site", p.Name, p.X, p.Y)
		}
		for _, b := range w.Buildings {
			if b.X == p.X && b.Y == p.Y {
				t.Errorf("extra %s stands on a building", p.Name)
			}
		}
		for _, it := range w.Items {
			if it.X == p.X && it.Y == p.Y {
				t.Errorf("extra %s stands on an item", p.Name)
			}
		}
		for _, a := range w.Animals {
			if a.X == p.X && a.Y == p.Y {
				t.Errorf("extra %s stands on an animal", p.Name)
			}
		}
	}
	g2, _ := build()
	if g.StateHash() != g2.StateHash() {
		t.Error("two builds of 40 colonists differ")
	}
}

func TestExtraOrder(t *testing.T) {
	site := rect{2, 2, 3, 3}
	order := extraOrder(6, 6, site)
	if len(order) != 36-4 || len(order) != site.cellsOutside(6, 6) {
		t.Fatalf("len = %d", len(order))
	}
	for i, c := range order {
		if site.contains(c.x, c.y) {
			t.Fatalf("cell %v is inside the site", c)
		}
		if i == 0 {
			continue
		}
		p := order[i-1]
		dp, dc := site.dist(p.x, p.y), site.dist(c.x, c.y)
		if dp > dc || (dp == dc && (p.y > c.y || (p.y == c.y && p.x >= c.x))) {
			t.Fatalf("order broken at %d: %v then %v", i, p, c)
		}
	}
	if order[0] != (cell{1, 1}) {
		t.Errorf("first = %v, want {1 1}", order[0])
	}
	if !slices.Equal(order, extraOrder(6, 6, site)) {
		t.Error("order differs between calls")
	}
}

func TestSiteCellsOutsideClipped(t *testing.T) {
	if got := (rect{-5, -5, 2, 2}).cellsOutside(10, 10); got != 100-9 {
		t.Errorf("clipped = %d, want 91", got)
	}
	if got := (rect{20, 20, 30, 30}).cellsOutside(10, 10); got != 100 {
		t.Errorf("disjoint = %d, want 100", got)
	}
}

func TestPlaceExtras(t *testing.T) {
	order := extraOrder(6, 6, rect{2, 2, 3, 3})
	busy := map[cell]bool{{1, 1}: true, {2, 1}: true}
	bad := map[cell]bool{{3, 1}: true}
	run := func(want int) (int, []cell) {
		var got []cell
		n := placeExtras(order, want,
			func(x, y int) bool { return busy[cell{x, y}] },
			func(x, y, k int) error {
				if bad[cell{x, y}] {
					return errors.New("not walkable")
				}
				if k != len(got) {
					t.Errorf("k = %d, want %d", k, len(got))
				}
				got = append(got, cell{x, y})
				return nil
			})
		return n, got
	}
	n, got := run(3)
	want := []cell{{4, 1}, {1, 2}, {4, 2}}
	if n != 3 || !slices.Equal(got, want) {
		t.Fatalf("placed %d %v, want 3 %v", n, got, want)
	}
	n2, got2 := run(3)
	if n2 != n || !slices.Equal(got, got2) {
		t.Error("two runs differ")
	}
	// more than the map has: a shortfall, every free walkable cell used
	if n, got := run(1000); n != 32-2-1 || len(got) != n {
		t.Errorf("placed %d, want %d", n, 32-2-1)
	}
	if n, _ := run(0); n != 0 {
		t.Errorf("want 0 placed, got %d", n)
	}
}

func TestGrownColonyError(t *testing.T) {
	if _, err := GrownColony(91004, 120, 120, 2, false); err == nil || !strings.Contains(err.Error(), "colonists") {
		t.Fatalf("want a colonists error, got %v", err)
	}
	defer func() {
		if recover() == nil {
			t.Error("MustGrownColony did not panic")
		}
	}()
	MustGrownColony(91004, 120, 120, 2, false)
}
