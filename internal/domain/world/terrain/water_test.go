package terrain

import "testing"

func TestWaterAndWetGround(t *testing.T) {
	t.Parallel()
	tests := []struct {
		t        Type
		walk     bool
		water    bool
		walkCost int
		fert     int
		wet      int
	}{
		{ShallowWater, true, true, 300, 0, 1000},
		{DeepWater, false, true, 100, 0, 1000},
		{Mud, true, false, 140, 70, 800},
		{Marsh, true, false, 200, 60, 600},
		{Track, true, false, 90, 0, 0},
		{Sand, true, false, 120, 10, 0},
		{Ice, true, false, 130, 0, 0},
		{Gravel, true, false, 100, 70, 0},
		{Soil, true, false, 100, 100, 0},
		{Rock, false, false, 100, 0, 0},
	}
	for _, tc := range tests {
		if tc.t.Walkable() != tc.walk || IsWater(tc.t) != tc.water || tc.t.WalkCost() != tc.walkCost ||
			tc.t.Fertility() != tc.fert || tc.t.Wetness() != tc.wet {
			t.Errorf("%s: walk %v water %v cost %d fert %d wet %d", tc.t, tc.t.Walkable(), IsWater(tc.t),
				tc.t.WalkCost(), tc.t.Fertility(), tc.t.Wetness())
		}
	}
	if Type(200).String() != "unknown" || Track.String() != "track" || ShallowWater.String() != "shallow_water" {
		t.Fatal("names")
	}
	m := NewMap(2, 2)
	m.Set(-1, 0, Mud)
	m.Set(2, 2, Mud)
	for _, c := range m.Cells {
		if c != Soil {
			t.Fatal("out-of-bounds Set wrote")
		}
	}
}
