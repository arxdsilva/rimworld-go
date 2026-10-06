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

func TestScoringAndRoles(t *testing.T) {
	t.Parallel()
	bare := Contents{Cells: 9}
	nice := Contents{Cells: 16, Floored: 16, Beds: 1, Tables: 1, Chairs: 2, Lamps: 1, StoneWalls: 16, TotalWalls: 16}
	if !(Impressiveness(nice) > Impressiveness(bare)) {
		t.Fatal("furnished floored room scores higher")
	}
	// a furnished, floored 3x3 hut is nice, not impressive
	hut := Contents{Cells: 9, Floored: 9, Beds: 2, Tables: 1, Chairs: 1, Lamps: 1, TotalWalls: 16}
	if got := Label(Impressiveness(hut)); got != "nice" {
		t.Fatalf("3x3 hut label = %q (score %d), want nice", got, Impressiveness(hut))
	}
	if Label(Impressiveness(Contents{Cells: 3})) != "awful" || Label(Impressiveness(nice)) == "dull" {
		t.Fatal("labels")
	}
	if RoleOf(Contents{Beds: 1}) != Bedroom || RoleOf(Contents{Beds: 3}) != Barracks ||
		RoleOf(Contents{Tables: 1}) != Dining || RoleOf(Contents{Benches: 1, Beds: 1}) != Workshop {
		t.Fatal("roles")
	}
}
