package terrain

import "testing"

func TestTerrainRules(t *testing.T) {
	t.Parallel()
	if Rock.Walkable() || !Stone.Walkable() {
		t.Fatal("rock blocks, stone ground does not")
	}
	if RichSoil.Fertility() <= Soil.Fertility() || Stone.Fertility() != 0 {
		t.Fatal("fertility ordering")
	}
	m := NewMap(3, 2)
	m.Set(2, 1, Gravel)
	if m.At(2, 1) != Gravel || m.At(0, 0) != Soil || m.At(-1, 0) != Rock {
		t.Fatal("map access")
	}
	if len(Names()) != int(numTypes) || Rock.String() != "rock" {
		t.Fatal("names")
	}
}
