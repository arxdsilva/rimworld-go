package construction

import "testing"

func TestDefsAreConsistent(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for i, d := range All() {
		if Kind(i) != d.Kind {
			t.Fatalf("def %d has kind %d", i, d.Kind)
		}
		if d.Name == "" || seen[d.Name] {
			t.Fatalf("def %d: empty or duplicate name %q", i, d.Name)
		}
		seen[d.Name] = true
		switch {
		case d.Dug:
			if d.Wood != 0 || d.Work <= 0 {
				t.Fatalf("%s: a dug kind needs work and no materials", d.Name)
			}
		case (d.Wood <= 0 || d.Work <= 0) && !d.Kind.Instant():
			t.Fatalf("%s: cost and work must be positive (or both zero for an instant marking)", d.Name)
		}
		if k, ok := Parse(d.Name); !ok || k != d.Kind {
			t.Fatalf("Parse(%q) = %v %v", d.Name, k, ok)
		}
	}
	if !DefOf(Wall).BlocksMove || DefOf(Door).BlocksMove {
		t.Fatal("walls block movement, doors do not")
	}
	if DefOf(Floor).Layer != LayerFloor || DefOf(Table).Layer != LayerObject {
		t.Fatal("floor is the floor layer, furniture the object layer")
	}
	if _, ok := Parse("castle"); ok {
		t.Fatal("unknown names must not parse")
	}
}

func TestRefundAndFacing(t *testing.T) {
	t.Parallel()
	if Refund(SleepingSpot) != 0 || Refund(Bed) != 2 || Refund(Floor) != 0 {
		t.Fatalf("refund spot=%d bed=%d floor=%d", Refund(SleepingSpot), Refund(Bed), Refund(Floor))
	}
	if !SleepingSpot.Instant() || Bed.Instant() || !SleepingSpot.Sleeps() {
		t.Fatal("sleeping spots are instant sleeping places")
	}
	if ParseFacing(9) != South || ParseFacing(uint8(West)) != West {
		t.Fatal("facing parse")
	}
	if West.String() != "west" {
		t.Fatal("facing string")
	}
}

func TestFootprintRotation(t *testing.T) {
	t.Parallel()
	cells, spot, ok := Footprint(Workbench, South, 5, 5)
	if len(cells) != 2 || cells[0] != [2]int{5, 5} || cells[1] != [2]int{6, 5} || !ok || spot != [2]int{5, 6} {
		t.Fatalf("south bench cells=%v spot=%v", cells, spot)
	}
	cells, spot, _ = Footprint(Workbench, East, 5, 5)
	// vertical 1x2, front faces east
	if len(cells) != 2 || !Covers(Workbench, East, 5, 5, 5, 6) || spot[0] != 6 {
		t.Fatalf("east bench cells=%v spot=%v", cells, spot)
	}
	_, spot, _ = Footprint(Workbench, North, 5, 5)
	if spot[1] != 4 {
		t.Fatalf("north bench spot=%v", spot)
	}
	_, spot, _ = Footprint(Workbench, West, 5, 5)
	if spot[0] != 4 {
		t.Fatalf("west bench spot=%v", spot)
	}
	if c, _, ok := Footprint(Wall, East, 2, 3); len(c) != 1 || c[0] != [2]int{2, 3} || ok {
		t.Fatal("walls are 1x1 without a spot")
	}
	if c, _, _ := Footprint(Bed, East, 0, 0); len(c) != 2 || c[1][0] != 1 {
		t.Fatalf("east bed = %v", c)
	}
}

func TestDoubleBedHoldsTwo(t *testing.T) {
	b := Building{Kind: DoubleBed}
	b = b.Claim(7)
	if !b.CanClaim(9) {
		t.Fatal("second slot is free")
	}
	b = b.Claim(9)
	if b.CanClaim(11) || b.SlotOf(7) != 0 || b.SlotOf(9) != 1 || b.Owner(11) != 7 {
		t.Fatalf("full double bed: %+v", b)
	}
	b = b.Release(7)
	if !b.CanClaim(11) || b.SlotOf(9) != 1 {
		t.Fatal("releasing frees exactly that slot")
	}
	single := Building{Kind: Bed}.Claim(7)
	if single.CanClaim(9) || single.Claim(9).SlotOf(9) != -1 {
		t.Fatal("a single bed holds one")
	}
	x0, y0 := SleepCell(DoubleBed, South, 4, 4, 0)
	x1, y1 := SleepCell(DoubleBed, South, 4, 4, 1)
	if x0 != 4 || y0 != 4 || x1 != 5 || y1 != 4 {
		t.Fatalf("south slots side by side at the head row: %d,%d %d,%d", x0, y0, x1, y1)
	}
	x0, y0 = SleepCell(DoubleBed, East, 4, 4, 0)
	x1, y1 = SleepCell(DoubleBed, East, 4, 4, 1)
	if x0 != x1 || y0 == y1 {
		t.Fatalf("east-facing slots share the head column: %d,%d %d,%d", x0, y0, x1, y1)
	}
}
