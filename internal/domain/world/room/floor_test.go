package room

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
)

// TestFloorStatsAreTheWikis (AX-3): every built floor's beauty and
// cleanliness per cell. wiki: Floors, Sterile tile.
func TestFloorStatsAreTheWikis(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		k             construction.Kind
		stuff         item.Kind
		beauty, clean int
	}{
		{construction.Floor, item.Wood, 0, 0},
		{construction.Floor, item.GraniteBlocks, 10, 0}, // a pre-v11 stone floor
		{construction.StrawMatting, item.Hay, -10, -10},
		{construction.Concrete, item.Steel, -10, 0},
		{construction.PavedTile, item.Steel, 0, 0},
		{construction.SteelTile, item.Steel, 0, 20},
		{construction.SilverTile, item.Silver, 40, 20},
		{construction.GoldTile, item.Gold, 110, 20},
		{construction.SterileTile, item.Steel, -10, 60},
		{construction.StoneTile, item.MarbleBlocks, 10, 0},
		{construction.Flagstone, item.SlateBlocks, 0, 0},
		{construction.CarpetFloor, item.Cloth, 20, 0},
	} {
		if b, cl := FloorStats(c.k, c.stuff); b != c.beauty || cl != c.clean {
			t.Errorf("%s of %v: beauty %d clean %d, want %d %d", c.k, c.stuff, b, cl, c.beauty, c.clean)
		}
	}
}

// TestSterileRoomTally (AX-3): a room laid in sterile tile reads
// cleanliness +0.6, one in gold tile beauty +11 a cell, and every floor
// kind counts as floored.
func TestSterileRoomTally(t *testing.T) {
	t.Parallel()
	var c Contents
	var tl Tally
	for i := range 9 {
		Add(&c, &tl, construction.Building{Kind: construction.SterileTile, Stuff: item.Steel, X: i}, 1, Bare)
	}
	if in := tl.Inputs(9, 0, 0, 0); in.CleanlinessHundredths != 60 || c.Floored != 9 || tl.Covered != 9 {
		t.Fatalf("sterile room %+v floored %d", in, c.Floored)
	}
	var gold Tally
	Add(&c, &gold, construction.Building{Kind: construction.GoldTile, Stuff: item.Gold}, 1, Bare)
	if gold.BeautyTenths != 110 || gold.Wealth <= BuildingValue(construction.SteelTile, item.Steel, quality.Default) {
		t.Fatalf("gold tile %+v", gold)
	}
}

// TestSurgeryCleanliness (AX-3): bare ground costs 1, filth 2 per permille
// a cell, floors add their own (sterile tile +0.6), floored at -5.
func TestSurgeryCleanliness(t *testing.T) {
	t.Parallel()
	var sterile Tally
	var c Contents
	for range 4 {
		Add(&c, &sterile, construction.Building{Kind: construction.SterileTile, Stuff: item.Steel}, 1, Bare)
	}
	c.Cells = 4
	for _, tc := range []struct {
		c     Contents
		t     Tally
		filth int
		want  int32
	}{
		{c, sterile, 0, 60},
		{c, sterile, 40, 40},
		{Contents{Cells: 4}, Tally{}, 0, -100},
		{Contents{Cells: 4}, Tally{}, 100000, -500},
	} {
		if got := SurgeryCleanliness(tc.c, tc.t, tc.filth); got != tc.want {
			t.Errorf("SurgeryCleanliness(%+v, filth %d) = %d, want %d", tc.c, tc.filth, got, tc.want)
		}
	}
}
