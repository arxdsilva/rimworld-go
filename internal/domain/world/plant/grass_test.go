package plant

import (
	"testing"

	"rimworld-go/internal/domain/world/terrain"
)

func TestGrassAtOnlyOnSoil(t *testing.T) {
	soil, rich, other := 0, 0, 0
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			if GrassAt(terrain.Soil, x, y) {
				soil++
			}
			if GrassAt(terrain.RichSoil, x, y) {
				rich++
			}
			if GrassAt(terrain.Rock, x, y) || GrassAt(terrain.Stone, x, y) || GrassAt(terrain.Gravel, x, y) {
				other++
			}
		}
	}
	if other != 0 {
		t.Fatalf("grass on non-soil: %d", other)
	}
	if soil != rich || soil < 600 || soil > 1000 {
		t.Fatalf("soil=%d rich=%d, want equal and ~1/5 of 4096", soil, rich)
	}
}

// Species helpers the grass work touches indirectly (crops clear grass).
func TestSpeciesNamesRoundTrip(t *testing.T) {
	s, ok := Parse("potato")
	if !ok || s.String() != "potato" || !DefOf(s).ZoneSowable() {
		t.Fatalf("potato: %v %v", s, ok)
	}
}
