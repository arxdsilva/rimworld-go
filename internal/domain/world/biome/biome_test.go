package biome

import (
	"testing"

	"rimworld-go/internal/domain/world/temperature"
)

func TestEveryBiomeDefined(t *testing.T) {
	seen := map[string]bool{}
	for _, d := range All() {
		if d.Key == "" || d.Name == "" || d.Blurb == "" || seen[d.Key] {
			t.Fatalf("biome %d: incomplete or duplicate %+v", d.ID, d)
		}
		seen[d.Key] = true
		if id, ok := Parse(d.Key); !ok || id != d.ID {
			t.Fatalf("Parse(%q) = %v, %v", d.Key, id, ok)
		}
		if g := d.Ground; !d.Legacy && g.Soil+g.Rich+g.Gravel+g.Sand+g.Marsh+g.Ice != 1000 {
			t.Fatalf("%s ground mix sums to %d", d.Key, g.Soil+g.Rich+g.Gravel+g.Sand+g.Marsh+g.Ice)
		}
	}
	if len(seen) != int(NumBiomes) {
		t.Fatalf("%d biomes", len(seen))
	}
}

func TestTemperateIsTheOriginalClimate(t *testing.T) {
	if d := DefOf(TemperateForest); d.Climate != temperature.Temperate || !d.Legacy {
		t.Fatalf("temperate forest %+v", d)
	}
	if DefOf(NumBiomes+3).ID != TemperateForest {
		t.Fatal("unknown biome should read as temperate forest")
	}
}

func TestClimatesOrderedByCold(t *testing.T) {
	cold, _ := DefOf(SeaIce).Climate.Range()
	_, hot := DefOf(ExtremeDesert).Climate.Range()
	for _, d := range All() {
		lo, hi := d.Climate.Range()
		if lo < cold || hi > hot {
			t.Fatalf("%s range %v..%v outside sea ice..extreme desert", d.Key, lo, hi)
		}
	}
}

func TestSet(t *testing.T) {
	s := Of(Tundra, Desert)
	if !s.Has(Tundra) || !s.Has(Desert) || s.Has(TemperateForest) {
		t.Fatalf("set %b", s)
	}
}
