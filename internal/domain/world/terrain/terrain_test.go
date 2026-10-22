package terrain

import "testing"

func TestTerrainRules(t *testing.T) {
	t.Parallel()
	if Rock.Walkable() || !Stone.Walkable() {
		t.Fatal("rock blocks, stone ground does not")
	}
	if RichSoil.Fertility() <= Soil.Fertility() || Stone.Fertility() != 0 {
		t.Fatal("fertility ordering")
	}
	m := NewMap(3, 2)
	m.Set(2, 1, Gravel)
	if m.At(2, 1) != Gravel || m.At(0, 0) != Soil || m.At(-1, 0) != Rock {
		t.Fatal("map access")
	}
	if len(Names()) != int(numTypes) || Rock.String() != "rock" {
		t.Fatal("names")
	}
}

func TestRockSmoothBlocksAndIsRockWall(t *testing.T) {
	t.Parallel()
	tests := []struct {
		t        Type
		wall     bool
		fertile  bool
		walkCost int
	}{
		{Rock, true, false, 100},
		{RockSmooth, true, false, 100},
		{Stone, false, false, 100},
		{StoneSmooth, false, false, 100},
		{Soil, false, true, 100},
		{Sand, false, true, 120},
	}
	for _, tc := range tests {
		if IsRockWall(tc.t) != tc.wall || tc.t.Walkable() == tc.wall {
			t.Errorf("%v: IsRockWall=%v Walkable=%v, want wall=%v", tc.t, IsRockWall(tc.t), tc.t.Walkable(), tc.wall)
		}
		if (tc.t.Fertility() > 0) != tc.fertile {
			t.Errorf("%v: fertility %d", tc.t, tc.t.Fertility())
		}
		if tc.t.WalkCost() != tc.walkCost {
			t.Errorf("%v: walk cost %d, want %d", tc.t, tc.t.WalkCost(), tc.walkCost)
		}
	}
}

func TestTerrainNamesRoundTrip(t *testing.T) {
	t.Parallel()
	names := Names()
	seen := map[string]bool{}
	for i, n := range names {
		if Type(i).String() != n || n == "" || seen[n] {
			t.Fatalf("type %d: name %q (String %q)", i, n, Type(i).String())
		}
		seen[n] = true
	}
	if RockSmooth.String() != "rock_smooth" || StoneSmooth.String() != "stone_smooth" {
		t.Fatalf("smooth names: %q %q", RockSmooth.String(), StoneSmooth.String())
	}
	if Type(numTypes).String() != "unknown" {
		t.Fatal("out-of-range type name")
	}
}
