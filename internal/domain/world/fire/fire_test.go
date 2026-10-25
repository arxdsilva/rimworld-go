package fire

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/world/plant"
)

func TestNextGrowsWithFuelAndStarvesWithout(t *testing.T) {
	wood := Fuel{Flammability: 700}
	if got := Next(10, wood, false); got != 14 {
		t.Fatalf("with fuel: %d, want 14", got)
	}
	if got := Next(MaxSize-1, wood, false); got != MaxSize {
		t.Fatalf("growth caps at MaxSize: %d", got)
	}
	if got := Next(10, Fuel{}, false); got != 4 {
		t.Fatalf("no fuel: %d, want 4", got)
	}
	if got := Next(5, Fuel{}, false); got != 0 {
		t.Fatalf("no fuel burns out at 0: %d", got)
	}
}

func TestNextDousedLosesTwelve(t *testing.T) {
	wood := Fuel{Flammability: 700}
	if got := Next(50, wood, true); got != 50+Grow-Drench {
		t.Fatalf("doused with fuel: %d, want %d", got, 50+Grow-Drench)
	}
	if got := Next(50, Fuel{}, true); got != 50-Starve-Drench {
		t.Fatalf("doused without fuel: %d", got)
	}
	if got := Next(3, wood, true); got != 0 {
		t.Fatalf("doused small fire dies: %d", got)
	}
}

func TestDousedOnlyOutdoorsInHardRainOrSnow(t *testing.T) {
	cases := []struct {
		name      string
		falling   bool
		intensity float64
		roofed    bool
		want      bool
	}{
		{"hard rain outdoors", true, 0.6, false, true},
		{"threshold", true, DouseIntensity, false, true},
		{"drizzle", true, 0.2, false, false},
		{"under a roof", true, 1, true, false},
		{"dry", false, 1, false, false},
	}
	for _, c := range cases {
		if got := Doused(c.falling, c.intensity, c.roofed); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

func TestSpreadChance(t *testing.T) {
	grass := Fuel{Flammability: Grass}
	if got := SpreadChance(SpreadSize-1, grass, false, false); got != 0 {
		t.Fatalf("below spread size: %d", got)
	}
	if got := SpreadChance(80, Fuel{}, false, false); got != 0 {
		t.Fatalf("no fuel: %d", got)
	}
	// 0.5 x 0.8 x 0.06 = 0.024 = 24000 ppm
	base := SpreadChance(80, grass, false, false)
	if base != 24000 {
		t.Fatalf("base chance %d ppm, want 24000", base)
	}
	if got := SpreadChance(80, grass, true, false); got != base*3/2 {
		t.Fatalf("downwind %d, want %d", got, base*3/2)
	}
	if got := SpreadChance(80, grass, false, true); got != base/5 {
		t.Fatalf("roof shield %d, want %d", got, base/5)
	}
	if got := SpreadChance(80, grass, true, true); got != base*3/10 {
		t.Fatalf("both %d, want %d", got, base*3/10)
	}
}

func TestDownwind(t *testing.T) {
	if !Downwind(1, 0, 2, 0) {
		t.Fatal("east of a fire with an east wind is downwind")
	}
	if Downwind(-1, 0, 2, 0) || Downwind(0, 1, 2, 0) {
		t.Fatal("upwind and crosswind are not downwind")
	}
	if !Downwind(1, 1, 1, 1) || Downwind(1, 0, 0.01, 0) {
		t.Fatal("diagonal wind; still air never biases")
	}
}

func TestPlantFlammability(t *testing.T) {
	cases := map[plant.Species]int{
		plant.Tree: 700, plant.Pine: 700, plant.JungleTree: 700,
		plant.Duskmoss: 900, plant.Corn: 600, plant.Potato: 600,
		plant.BerryBush: 600, plant.Shrub: 600, plant.Moss: 600,
	}
	for s, want := range cases {
		if got := PlantFlammability(s); got != want {
			t.Errorf("%v: %d, want %d", s, got, want)
		}
	}
}

func TestBuildingFlammability(t *testing.T) {
	cases := []struct {
		k     construction.Kind
		stuff item.Kind
		want  int
	}{
		{construction.Wall, item.Wood, 700},
		{construction.Bridge, item.Wood, 700},
		{construction.Wall, item.GraniteBlocks, 0},
		{construction.Door, item.Steel, 0},
		{construction.Wall, item.Food, 700}, // legacy zero stuff: the default, wood
		{construction.Carpet, item.CarpetCloth, 800},
		{construction.Campfire, item.Wood, 0},
		{construction.Grave, item.Wood, 0},
		{construction.SteamVent, item.Food, 0},
	}
	for _, c := range cases {
		if got := BuildingFlammability(c.k, c.stuff); got != c.want {
			t.Errorf("%v of %v: %d, want %d", c.k, c.stuff, got, c.want)
		}
	}
}

func TestItemFlammability(t *testing.T) {
	cases := map[item.Kind]int{
		item.Wood: 800, item.Cloth: 900, item.Leather: 700, item.Straw: 900, item.Wool: 900,
		item.CorpseHuman: 700, item.CorpseAnimal: 700, item.Parka: 800, item.CarpetStraw: 800,
		item.Meal: 300, item.Potatoes: 300, item.Beer: 300,
		item.Steel: 0, item.GraniteBlocks: 0, item.GraniteChunk: 0, item.Gold: 0, item.Rifle: 0,
	}
	for k, want := range cases {
		if got := ItemFlammability(k); got != want {
			t.Errorf("%v: %d, want %d", k, got, want)
		}
	}
	for k := item.Kind(0); k < 200; k++ {
		if f := ItemFlammability(k); f < 0 || f > 1000 {
			t.Fatalf("%v: %d out of range", k, f)
		}
	}
}

func TestDamageScalesWithSize(t *testing.T) {
	if BuildingDamage(0) != 0 || CreatureDamage(0) != 0 {
		t.Fatal("no fire, no damage")
	}
	if !(BuildingDamage(10) < BuildingDamage(50) && BuildingDamage(50) < BuildingDamage(MaxSize)) {
		t.Fatal("building damage grows with size")
	}
	if !(CreatureDamage(10) <= CreatureDamage(50) && CreatureDamage(50) < CreatureDamage(MaxSize)) {
		t.Fatal("creature damage grows with size")
	}
	if CreatureDamage(1) < 1 || BuildingDamage(1) < 1 {
		t.Fatal("any fire hurts")
	}
}

func TestBurnStepsByFuel(t *testing.T) {
	if !(GrassBurnSteps > 0 && PlantBurnSteps(plant.Moss) > 0) {
		t.Fatal("burn steps positive")
	}
	if PlantBurnSteps(plant.Tree) <= PlantBurnSteps(plant.Shrub) {
		t.Fatal("trees burn longer than shrubs")
	}
	// grass alone burns long enough to reach spread size from a spark
	size, steps := IgniteSize, 0
	for steps < GrassBurnSteps {
		size = Next(size, Fuel{Flammability: Grass}, false)
		steps++
	}
	if size < SpreadSize {
		t.Fatalf("a grass fire peaks at %d, below spread size", size)
	}
}

func TestFuelBest(t *testing.T) {
	f := Fuel{}
	f = f.With(300)
	f = f.With(700)
	f = f.With(500)
	if f.Flammability != 700 || !f.Burns() {
		t.Fatalf("best fuel %d", f.Flammability)
	}
	if (Fuel{}).Burns() {
		t.Fatal("empty fuel burns")
	}
}

func TestAdjacentDamageOnlyBesideBigFires(t *testing.T) {
	if AdjacentDamage(adjacentSize-1) != 0 || AdjacentDamage(adjacentSize) != 1 || AdjacentDamage(MaxSize) != 1 {
		t.Fatal("only big fires singe their neighbours, a little")
	}
}
