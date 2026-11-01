package room

import "testing"

func TestDetectRoomsAndOutdoors(t *testing.T) {
	t.Parallel()
	// 8x6 map with a 4x4 walled box (interior 2x2) at x 1..4, y 1..4.
	walls := map[[2]int]bool{}
	for x := 1; x <= 4; x++ {
		walls[[2]int{x, 1}], walls[[2]int{x, 4}] = true, true
	}
	for y := 1; y <= 4; y++ {
		walls[[2]int{1, y}], walls[[2]int{4, y}] = true, true
	}
	grid, sizes := Detect(8, 6, func(x, y int) bool { return walls[[2]int{x, y}] })
	if len(sizes) != 1 || sizes[0] != 4 {
		t.Fatalf("rooms = %v", sizes)
	}
	if grid[2*8+2] != 0 || grid[0] != Outdoors || grid[1*8+1] != Wall {
		t.Fatal("room / outdoors / wall ids")
	}
	// a gap in the wall makes it outdoors
	delete(walls, [2]int{2, 1})
	_, sizes = Detect(8, 6, func(x, y int) bool { return walls[[2]int{x, y}] })
	if len(sizes) != 0 {
		t.Fatalf("open box is outdoors, rooms = %v", sizes)
	}
}

func TestRoles(t *testing.T) {
	t.Parallel()
	if RoleOf(Contents{Beds: 1}) != Bedroom || RoleOf(Contents{Beds: 3}) != Barracks ||
		RoleOf(Contents{Tables: 1}) != Dining || RoleOf(Contents{Benches: 1, Beds: 1}) != Workshop {
		t.Fatal("roles")
	}
}

// TestRoomCapTakesNoRole (TEMP-6, wiki: Rooms): an enclosed area past
// MaxCells (5184 tiles) is no room: a bed in it makes no bedroom, a
// prisoner bed no cell, a stove no kitchen.
func TestRoomCapTakesNoRole(t *testing.T) {
	t.Parallel()
	at, over := Contents{Cells: MaxCells, Beds: 1}, Contents{Cells: MaxCells + 1, Beds: 1}
	if RoleOf(at) != Bedroom || RoleOf(over) != General {
		t.Fatalf("at the cap %v, past it %v", RoleOf(at), RoleOf(over))
	}
	for _, c := range []FullContents{{Contents: over, PrisonerBeds: 1}, {Contents: over, Stoves: 1}, {Contents: over, MedicalBeds: 1}} {
		if r := RoleOfFull(c); r != General {
			t.Errorf("%+v: role %v past the cap", c, r)
		}
	}
	if r := RoleOfFull(FullContents{Contents: at, PrisonerBeds: 1}); r != PrisonCell {
		t.Errorf("at the cap a prisoner bed makes %v", r)
	}
}
