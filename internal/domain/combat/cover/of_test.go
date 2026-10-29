package cover

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/world/plant"
)

func TestOfBuilding(t *testing.T) {
	t.Parallel()
	want := map[construction.Kind]Kind{
		construction.Sandbags: Sandbags, construction.Barricade: Barricade, construction.Wall: Wall,
		construction.Door: Door, construction.Autodoor: Door, construction.Fence: Fence, construction.FenceGate: Fence,
		construction.Stool: Stool, construction.Table: Table, construction.Bed: Table, construction.DoubleBed: Table,
		construction.HospitalBed: Table, construction.Chair: Chair, construction.SentryTurret: Turret,
		construction.Autocannon: HeavyTurret, construction.UraniumSlugTurret: HeavyTurret,
		construction.Floor: None, construction.SpikeTrap: None, construction.Mortar: None,
	}
	for k, c := range want {
		if got := OfBuilding(k); got != c {
			t.Errorf("%v: %v, want %v", k, got, c)
		}
	}
}

func TestOfPlant(t *testing.T) {
	t.Parallel()
	if OfPlant(plant.Tree, true) != Tree || OfPlant(plant.Pine, true) != Tree || OfPlant(plant.Cactus, true) != Cactus ||
		OfPlant(plant.BerryBush, true) != Bush || OfPlant(plant.Shrub, true) != Bush || OfPlant(plant.Corn, true) != None ||
		OfPlant(plant.Tree, false) != None {
		t.Fatal("plant cover")
	}
}
