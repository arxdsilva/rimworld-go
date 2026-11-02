package construction

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
)

// TestAX2Kinds (AX-2): seats by size (wiki: Table 6/8/12/12, Couch 2),
// the seat cells round a table, the wall-hung pieces' wall, the precious
// walls' cost and the doors and beds the new kinds join.
func TestAX2Kinds(t *testing.T) {
	t.Parallel()
	for k, want := range map[Kind]int{Table: 6, Table2x2: 8, Table2x4: 12, Table3x3: 12, Couch: 2, Chair: 1, Bed: 0} {
		if Seats(k) != want {
			t.Errorf("%s seats %d, want %d", k, Seats(k), want)
		}
	}
	cells := SeatCells(Table, East, 5, 5) // (5,5),(6,5)
	if len(cells) != 6 || cells[0] != [2]int{5, 4} || cells[5] != [2]int{7, 5} {
		t.Fatalf("seat cells %v", cells)
	}
	if x, y := Behind(South, 3, 3); x != 3 || y != 2 {
		t.Fatal("a south-facing lamp hangs on the wall north of it")
	}
	if x, y := Behind(West, 3, 3); x != 4 || y != 3 {
		t.Fatal("a west-facing lamp hangs on the wall east of it")
	}
	if CostFor(Wall, item.Gold) != 10*WoodCost(Wall) || CostFor(Wall, item.Plasteel) != WoodCost(Wall) || CostFor(Door, item.Wood) != WoodCost(Door) {
		t.Fatal("small-volume walls cost ten times the stuff")
	}
	if !StuffOK(Wall, item.Jade) || StuffOK(Door, item.Jade) || RefundFor(Wall, item.Silver) != 10*Refund(Wall) {
		t.Fatal("precious walls: walls only, refunded ten times")
	}
	if !OrnateDoor.IsDoor() || !AnimalFlap.IsDoor() || DoubleSleepingSpot.Sleepers() != 2 || !AnimalSleepingBox.AnimalSleeps() ||
		!WallLamp.WallHung() || WallLamp.WallMounted() || !Column.HoldsRoof() {
		t.Fatal("predicates")
	}
	b := Building{Kind: PlantPot, Plant: uint8(PotRose), Growth: 500}
	if PotBeauty(b) != 7000 || PotBeauty(Building{Kind: Chair, Plant: 1, Growth: 1000}) != 0 {
		t.Fatalf("pot beauty %d, want a half-grown rose's 7", PotBeauty(b))
	}
	if p, ok := ParsePotPlant("daylily"); !ok || p != PotDaylily || len(PotPlants()) != 2 {
		t.Fatal("pot plants")
	}
	if MaxHP(Table, item.Wood) != 75 || DefOf(Table).Size != [2]int{1, 2} {
		t.Fatal("the 1x2 table (wiki: Table, 75 HP)")
	}
}
