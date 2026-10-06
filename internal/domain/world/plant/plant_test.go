package plant

import "testing"

func TestPlantStagesAndYields(t *testing.T) {
	t.Parallel()
	p := Plant{Species: Amberroot}
	if p.Ripe() || p.Stage() != 0 {
		t.Fatal("fresh crop is not ripe")
	}
	p.Growth = DefOf(Amberroot).GrowTicks
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
	normal, rich, barren := Plant{Species: Amberroot}, Plant{Species: Amberroot}, Plant{Species: Amberroot}
	for tick := uint64(1); tick <= 1000; tick++ {
		normal = normal.Grow(100, tick)
		rich = rich.Grow(140, tick)
		barren = barren.Grow(0, tick)
	}
	if normal.Growth != 1000 || rich.Growth != 1400 || barren.Growth != 0 {
		t.Fatalf("growth normal=%d rich=%d barren=%d", normal.Growth, rich.Growth, barren.Growth)
	}
}
