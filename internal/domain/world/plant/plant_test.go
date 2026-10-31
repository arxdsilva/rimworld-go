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
	// the crop menu runs fastest first (FOOD-2: rice first, corn last)
	prev := 0.0
	for _, c := range Crops() {
		d := GrowDays(c, 100)
		if d < prev {
			t.Fatalf("%s matures in %.1f days, before the previous crop (%.1f)", c, d, prev)
		}
		prev = d
		useful := DefOf(c).Yields == item.HerbalMedicine || DefOf(c).Yields == item.Cloth ||
			DefOf(c).Yields == item.AshleafLeaf || DefOf(c).Yields == item.Hops || DefOf(c).Yields == item.Hay
		if !DefOf(c).Crop || DefOf(c).Yield <= 0 || DefOf(c).Yields.Nutrition() <= 0 && !useful {
			t.Fatalf("%s must be an edible, medicinal, drug, feed or fibre crop", c)
		}
	}
	// wiki: Plants, real grow days = grow days / 0.5417 on soil
	for s, want := range map[Species]float64{Rice: 5.54, Potato: 10.71, Corn: 20.86, Strawberry: 8.49, Haygrass: 12.92,
		Cotton: 14.77, Hops: 9.23, Healroot: 12.92} {
		if got := GrowDays(s, 100); got < want-0.01 || got > want+0.01 {
			t.Errorf("%s ripens in %.2f days on soil, wiki %.2f", s, got, want)
		}
	}
	for s, want := range map[Species][3]int{Corn: {22, 100}, Potato: {11, 40}, Rice: {6, 100}, Strawberry: {8, 100},
		Haygrass: {18, 60}, Cotton: {10, 100}, Hops: {8, 70}, Healroot: {1, 100}} {
		if d := DefOf(s); d.Yield != want[0] || d.Sens != want[1] {
			t.Errorf("%s: yield %d sensitivity %d, wiki %v", s, d.Yield, d.Sens, want)
		}
	}
	// wiki: rice 170 sow and 200 harvest work; healroot 800 / 400 and Plants 8
	if d := DefOf(Rice); d.SowWork != 170 || d.HarvestWork != 200 || d.MinSkill != 0 {
		t.Errorf("rice work %+v", d)
	}
	if d := DefOf(Healroot); d.SowWork != 800 || d.HarvestWork != 400 || d.MinSkill != 8 {
		t.Errorf("healroot work %+v", d)
	}
	// potatoes barely care about poor soil; corn suffers on gravel
	if Rate(Potato, 70) <= Rate(Corn, 70) || Rate(Corn, 70) != 70 || Rate(Potato, 70) != 88 {
		t.Fatalf("gravel rates: potato %d corn %d", Rate(Potato, 70), Rate(Corn, 70))
	}
	if Rate(Corn, 140) != 140 || Rate(Potato, 0) != 0 || Rate(Rice, HydroFertile) != 280 {
		t.Fatal("rich soil helps sensitive crops most; barren grows nothing; a basin 280 %")
	}
	// wiki: plants rest from hour 19 through hour 5
	if !GrowingHour(6) || !GrowingHour(18) || GrowingHour(19) || GrowingHour(5) || GrowingHour(2) {
		t.Fatal("growth hours")
	}
	// trees (wiki: Plants tree table)
	if d := DefOf(Tree); d.GrowTicks != wikiDays(30) || d.Wood != 46 || d.MinSkill != 6 || d.SowWork != 4000 {
		t.Errorf("tree %+v", d)
	}
}

func TestTemperatureEffects(t *testing.T) {
	t.Parallel()
	// wiki: Plants, optimal 6-42 °C, T/6 below, (58-T)/16 above, none outside 0-58
	for tenths, want := range map[int]int{-30: 0, 0: 0, 30: 500, 60: 1000, 200: 1000, 420: 1000, 500: 500, 580: 0, 600: 0} {
		if got := TempFactor(tenths); got != want {
			t.Errorf("TempFactor(%d) = %d, want %d", tenths, got, want)
		}
	}
	if !Frostbitten(Corn, -110) || Frostbitten(Corn, -90) || Frostbitten(Healroot, -200) {
		t.Error("crops start to freeze below -10 °C, healroot survives -29")
	}
	if Frostbitten(Tree, -300) || Frostbitten(BerryBush, -300) {
		t.Error("wild plants only go dormant")
	}
}
