package plant

import (
	"testing"

	"rimworld-go/internal/domain/world/terrain"
)

func TestGrassAtBiomeDensity(t *testing.T) {
	share := func(tt terrain.Type, permil int) int {
		n := 0
		for y := 0; y < 100; y++ {
			for x := 0; x < 100; x++ {
				if GrassAtBiome(tt, x, y, permil) {
					n++
				}
			}
		}
		return n / 10 // permille of 10,000 tiles
	}
	for _, c := range []struct {
		t          terrain.Type
		permil, lo int
		hi         int
	}{
		{terrain.Soil, 700, 670, 730}, {terrain.Gravel, 600, 570, 630}, {terrain.Marsh, 800, 770, 830},
		{terrain.Sand, 400, 80, 120}, {terrain.Ice, 1000, 0, 0}, {terrain.Stone, 1000, 0, 0}, {terrain.Soil, 0, 0, 0},
	} {
		if got := share(c.t, c.permil); got < c.lo || got > c.hi {
			t.Errorf("%s at %d permille: %d", c.t, c.permil, got)
		}
	}
}

func TestGrassGround(t *testing.T) {
	if GrassGround(terrain.Gravel, true) || !GrassGround(terrain.Gravel, false) {
		t.Fatal("gravel grows grass only outside the temperate map")
	}
	if GrassGround(terrain.Ice, false) || GrassGround(terrain.Rock, false) || !GrassGround(terrain.Soil, true) {
		t.Fatal("grass ground")
	}
}
