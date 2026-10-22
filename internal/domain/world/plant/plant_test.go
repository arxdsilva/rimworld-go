package plant

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
)

func TestPlantStagesAndYields(t *testing.T) {
	t.Parallel()
	p := Plant{Species: Potato}
	if p.Ripe() || p.Stage() != 0 {
		t.Fatal("fresh crop is not ripe")
	}
	p.Growth = DefOf(Potato).GrowTicks
	if !p.Harvestable() || p.Stage() != 3 || p.Percent() != 100 {
		t.Fatal("full growth is ripe")
	}
	tree := Plant{Species: Tree, Growth: DefOf(Tree).GrowTicks / 2}
	if tree.Harvestable() || tree.WoodYield() != DefOf(Tree).Wood/2 {
		t.Fatalf("tree: harvestable=%v wood=%d", tree.Harvestable(), tree.WoodYield())
	}
	if s, ok := Parse("berry_bush"); !ok || s != BerryBush || !DefOf(s).Regrows {
		t.Fatal("parse berry bush")
	}
}

func TestGrowRespectsFertility(t *testing.T) {
	t.Parallel()
	normal, rich, barren := Plant{Species: Potato}, Plant{Species: Potato}, Plant{Species: Potato}
	for tick := uint64(1); tick <= 1000; tick++ {
		normal = normal.Grow(100, tick)
		rich = rich.Grow(140, tick)
		barren = barren.Grow(0, tick)
	}
	if normal.Growth != 1000 || rich.Growth != 1400 || barren.Growth != 0 {
		t.Fatalf("growth normal=%d rich=%d barren=%d", normal.Growth, rich.Growth, barren.Growth)
	}
}

func TestCropsGrowLikeTheRealThing(t *testing.T) {
	t.Parallel()
	// order of maturity matches real crops: radish < lettuce < beans <
	// (healroot) < potato < corn < pumpkin < (cotton) < wheat
	prev := 0.0
	for _, c := range Crops() {
		d := GrowDays(c, 100)
		if d <= prev {
			t.Fatalf("%s matures in %.1f days, not after the previous crop (%.1f)", c, d, prev)
		}
		prev = d
		useful := DefOf(c).Yields == item.HerbalMedicine || DefOf(c).Yields == item.Cloth
		if !DefOf(c).Crop || DefOf(c).Yield <= 0 || DefOf(c).Yields.Nutrition() <= 0 && !useful {
			t.Fatalf("%s must be an edible, medicinal or fibre crop", c)
		}
	}
	if GrowDays(Potato, 100) != 7.5 || GrowDays(Radish, 100) != 2 {
		t.Fatalf("potato %.2f radish %.2f days", GrowDays(Potato, 100), GrowDays(Radish, 100))
	}
	// potatoes barely care about poor soil; corn suffers on gravel
	if Rate(Potato, 70) <= Rate(Corn, 70) || Rate(Corn, 70) != 70 || Rate(Potato, 70) != 91 {
		t.Fatalf("gravel rates: potato %d corn %d", Rate(Potato, 70), Rate(Corn, 70))
	}
	if Rate(Corn, 140) != 140 || Rate(Potato, 0) != 0 {
		t.Fatal("rich soil helps sensitive crops most; barren grows nothing")
	}
	if !GrowingHour(6) || GrowingHour(20) || GrowingHour(2) {
		t.Fatal("daylight growth hours")
	}
}

func TestTemperatureEffects(t *testing.T) {
	if TempFactor(-30) != 0 || TempFactor(50) != 500 || TempFactor(200) != 1000 || TempFactor(400) != 500 {
		t.Error("growth temperature curve wrong")
	}
	if !Frostbitten(Corn, -20) || Frostbitten(Wheat, -20) {
		t.Error("corn dies at -2C, wheat survives")
	}
	if Frostbitten(Tree, -300) || Frostbitten(BerryBush, -300) {
		t.Error("wild plants only go dormant")
	}
}
