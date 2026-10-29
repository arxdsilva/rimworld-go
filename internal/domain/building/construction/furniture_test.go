package construction

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
)

func TestFurnitureOf(t *testing.T) {
	cases := []struct {
		k     Kind
		stuff item.Kind
		q     quality.Level
		want  Furniture
	}{
		{Bed, item.Wood, quality.Normal, Furniture{750, 1000, 1000}},
		{DoubleBed, item.Steel, quality.Normal, Furniture{750, 1000, 2000}},
		{Bedroll, item.Cloth, quality.Normal, Furniture{680, 950, 0}},
		{DoubleBedroll, item.Leather, quality.Normal, Furniture{680, 950, 0}},
		{HospitalBed, item.Steel, quality.Normal, Furniture{800, 1000, 0}},
		{SleepingSpot, item.Wood, quality.Legendary, Furniture{400, 800, 0}}, // never crafted: no quality
		{AnimalSleepingSpot, item.Wood, quality.Normal, Furniture{0, 800, 0}},
		{AnimalBed, item.Cloth, quality.Normal, Furniture{750, 1000, 0}},
		{EndTable, item.Wood, quality.Normal, Furniture{0, 0, 3000}},
		{Dresser, item.Wood, quality.Normal, Furniture{0, 0, 5000}},
		{Wall, item.Wood, quality.Normal, Furniture{}},
		{kindCount, item.Wood, quality.Normal, Furniture{}},
		// wiki quality curves: bedroll 0.52 comfort / 0.82 rest at Awful,
		// bed 1.28 / 1.60 at Legendary
		{Bedroll, item.Cloth, quality.Awful, Furniture{516, 817, 0}},
		{Bed, item.Wood, quality.Legendary, Furniture{1275, 1600, 8000}},
		// stone keeps 90% of the rest effectiveness
		{Bed, item.GraniteBlocks, quality.Normal, Furniture{750, 900, 1000}},
	}
	for _, c := range cases {
		if got := FurnitureOf(c.k, c.stuff, c.q); got != c.want {
			t.Errorf("FurnitureOf(%v, %v, %v) = %+v, want %+v", c.k, c.stuff, c.q, got, c.want)
		}
	}
}

func TestFacilityComfort(t *testing.T) {
	if FacilityComfort(quality.Normal) != 50 || FacilityComfort(quality.Legendary) != 90 || FacilityComfort(quality.Awful) != 40 {
		t.Fatal("facility comfort curve")
	}
	if FacilityComfort(quality.Level(99)) != 50 {
		t.Fatal("unknown quality reads as Normal")
	}
	if !EndTable.IsFacility() || !Dresser.IsFacility() || Bed.IsFacility() {
		t.Fatal("IsFacility")
	}
}

func TestBedKindsBED1(t *testing.T) {
	for _, k := range []Kind{Bedroll, DoubleBedroll} {
		if !k.Sleeps() || k.AnimalSleeps() || !Movable(k) || !DefOf(k).Fabric {
			t.Errorf("%v: a movable sleeping place sewn from fabric", k)
		}
	}
	if DoubleBedroll.Sleepers() != 2 || Bedroll.Sleepers() != 1 {
		t.Fatal("sleepers")
	}
	for _, k := range []Kind{AnimalSleepingSpot, AnimalBed} {
		if k.Sleeps() || !k.AnimalSleeps() || k.Sleepers() != 0 {
			t.Errorf("%v: for animals only", k)
		}
	}
	if !AnimalSleepingSpot.Instant() || AnimalBed.Instant() {
		t.Fatal("animal spot is free and instant, the bed is built")
	}
	if DefaultStuff(Bedroll) != item.Cloth || !StuffOK(AnimalBed, item.Leather) || StuffOK(Bedroll, item.Wood) ||
		StuffOK(Bed, item.Cloth) || !StuffOK(Dresser, item.GraniteBlocks) || !StuffOK(EndTable, item.Steel) {
		t.Fatal("stuff rules")
	}
	if s := Stuffs(DoubleBedroll); len(s) != 2 || s[0] != item.Cloth || s[1] != item.Leather {
		t.Fatalf("fabric stuffs %v", s)
	}
	if k, ok := Parse("dresser"); !ok || k != Dresser {
		t.Fatal("parse dresser")
	}
	if c, _, _ := Footprint(Dresser, East, 0, 0); len(c) != 2 || c[0][0] != c[1][0] {
		t.Fatalf("dresser footprint %v", c)
	}
}

// TestSmallFootprint matches Footprint for every kind of up to four tiles
// and every facing, with no allocation.
func TestSmallFootprint(t *testing.T) {
	for k := Kind(0); k < kindCount; k++ {
		for f := South; f <= West; f++ {
			want, _, _ := Footprint(k, f, 3, 4)
			got, n := SmallFootprint(k, f, 3, 4)
			if len(want) > 4 {
				if n != 0 {
					t.Errorf("%v: n %d for %d cells", k, n, len(want))
				}
				continue
			}
			if n != len(want) {
				t.Fatalf("%v %v: %d cells, want %d", k, f, n, len(want))
			}
			for i := range n {
				if got[i] != want[i] {
					t.Errorf("%v %v cell %d: %v, want %v", k, f, i, got[i], want[i])
				}
			}
		}
	}
	if a := testing.AllocsPerRun(100, func() { SmallFootprint(DoubleBed, East, 1, 1) }); a != 0 {
		t.Fatalf("%v allocs", a)
	}
}
