package game

import (
	"fmt"
	"hash/fnv"
	"testing"

	"rimworld-go/internal/domain/sim/clock"
)

// goldenTemperate pins the temperate-forest map (terrain, grass, plants)
// and climate as they were before biomes (SIM-023): adding biomes must not
// change a temperate game. Wildlife is left out on purpose.
// Recompute only for a deliberate change to temperate generation.
// TEMP-2: wiki biome extremes (temperate 35/-25 C) moved only the climate
// part; terrain, grass and plants hash as before.
var goldenTemperate = map[uint64]uint64{1: 0x65ce86679a377794, 7: 0xa2d9fbbd56bd7382, 42: 0xe90d9897ae0bb1ce}

func temperateHash(t *testing.T, seed uint64) uint64 {
	t.Helper()
	g := NewColony(Config{Seed: seed, Width: 48, Height: 40})
	w := g.World()
	h := fnv.New64a()
	h.Write(w.Terrain)
	h.Write(w.Grass)
	for _, p := range w.Plants {
		fmt.Fprintf(h, "%s@%d,%d:%d;", p.Species, p.X, p.Y, p.Growth)
	}
	for d := 0; d < clock.DaysPerYear; d++ {
		for hr := 0; hr < 24; hr += 5 {
			fmt.Fprintf(h, "%d;", g.climate.Outdoor(clock.Tick(d*clock.TicksPerDay+hr*clock.TicksPerHour)))
		}
	}
	return h.Sum64()
}

func TestTemperateGenerationUnchanged(t *testing.T) {
	for _, seed := range []uint64{1, 7, 42} {
		got := temperateHash(t, seed)
		if w := goldenTemperate[seed]; got != w {
			t.Errorf("seed %d: temperate world hash %#x, want %#x", seed, got, w)
		}
	}
}
