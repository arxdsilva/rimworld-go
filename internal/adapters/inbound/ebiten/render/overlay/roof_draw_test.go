package overlay

import (
	"image/color"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/world/roof"
)

func TestRoofFlagsMirrorDomain(t *testing.T) {
	t.Parallel()
	pairs := [][2]uint8{
		{RoofBuilt, uint8(roof.Built)}, {RoofNatural, uint8(roof.Natural)},
		{RoofPlanBuild, uint8(roof.PlanBuild)}, {RoofPlanRemove, uint8(roof.PlanRemove)},
		{RoofSupported, uint8(roof.Supported)}, {RoofNoAuto, uint8(roof.NoAuto)}, {RoofThick, uint8(roof.NaturalThick)},
	}
	for _, p := range pairs {
		if p[0] != p[1] {
			t.Fatalf("renderer flag %d != domain flag %d", p[0], p[1])
		}
	}
	if RoofAutoMax != roof.AutoMaxCells {
		t.Fatalf("roofAutoMax %d != roof.AutoMaxCells %d", RoofAutoMax, roof.AutoMaxCells)
	}
}

func TestRoofLabels(t *testing.T) {
	t.Parallel()
	w := roofLabelWorld()
	cases := []struct {
		x, y int
		want string
	}{
		{0, 1, "Roof removal planned"},
		{1, 1, "Roof planned"},
		{2, 1, "Natural roof (rock)"},
		{3, 1, "Roofed"},
		{0, 2, "Thick rock overhead (permanent)"},
		{6, 2, "Unroofed (room too large to auto-roof)"},
		{0, 3, "Unroofed"},
		{4, 0, ""},  // a wall
		{-1, 0, ""}, // out of bounds
		{10, 0, ""},
		{0, 6, ""},
	}
	for _, c := range cases {
		if got := RoofLabelAt(w, c.x, c.y); got != c.want {
			t.Errorf("roofLabelAt(%d,%d) = %q, want %q", c.x, c.y, got, c.want)
		}
	}
	// outdoors next to a large room is plain unroofed
	w.RoomGrid[2*10+9] = -1
	if got := RoofLabelAt(w, 9, 2); got != "Unroofed" {
		t.Errorf("outdoors: %q", got)
	}
	if got := RoofLabelAt(game.WorldView{TerrainW: 10, RoomGrid: make([]int, 60)}, 1, 1); got != "" {
		t.Errorf("no roof data: %q", got)
	}
}

func TestRoofOverlayColorsPerFlag(t *testing.T) {
	t.Parallel()
	cases := []struct {
		c    uint8
		want RoofStyle
	}{
		{RoofBuilt, RoofStyle{Fill: color.RGBA{90, 140, 210, 70}, Border: color.RGBA{120, 170, 235, 150}}},
		{RoofBuilt | RoofSupported, RoofStyle{Fill: color.RGBA{90, 140, 210, 70}, Border: color.RGBA{120, 170, 235, 150}}},
		{RoofNatural, RoofStyle{Fill: color.RGBA{110, 110, 125, 85}, Stipple: RoofNaturalDots}},
		{RoofPlanBuild, RoofStyle{Fill: color.RGBA{235, 175, 60, 30}, Dash: color.RGBA{235, 175, 60, 200}}},
		{RoofBuilt | RoofPlanRemove, RoofStyle{Fill: color.RGBA{90, 140, 210, 70}, Border: color.RGBA{120, 170, 235, 150},
			Hatch: color.RGBA{220, 70, 60, 120}}},
		{RoofSupported | RoofNoAuto, RoofStyle{}},
	}
	for _, c := range cases {
		if got := RoofStyleFor(c.c); got != c.want {
			t.Errorf("flags %06b: got %+v, want %+v", c.c, got, c.want)
		}
	}
	if RoofNaturalDots.A == 0 {
		t.Error("natural roofs have no stipple")
	}
}

func roofLabelWorld() game.WorldView {
	w := game.WorldView{TerrainW: 10, RoomGrid: make([]int, 60), RoofCells: make([]uint8, 60),
		Rooms: []game.RoomView{{ID: 0, Cells: 20}, {ID: 1, Cells: 151}}}
	for i := range w.RoomGrid {
		if i%10 >= 5 {
			w.RoomGrid[i] = 1
		}
	}
	w.RoomGrid[4] = -2
	w.RoofCells[4] = RoofBuilt // a roof flag on a wall still reads nothing
	w.RoofCells[10] = RoofBuilt | RoofPlanRemove
	w.RoofCells[11] = RoofPlanBuild
	w.RoofCells[12] = RoofNatural
	w.RoofCells[13] = RoofBuilt
	w.RoofCells[20] = RoofNatural | RoofThick
	return w
}
