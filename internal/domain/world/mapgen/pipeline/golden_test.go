package pipeline

import (
	"encoding/binary"
	"hash/fnv"
	"testing"

	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/site"
)

// resultHash fingerprints everything a map hands the game.
func resultHash(r mapgen.Result) uint64 {
	h := fnv.New64a()
	w := func(vs ...int) {
		var b [8]byte
		for _, v := range vs {
			binary.LittleEndian.PutUint64(b[:], uint64(int64(v)))
			h.Write(b[:])
		}
	}
	for _, t := range r.Terrain.Cells {
		w(int(t))
	}
	for i := range r.Geology.Stone {
		w(int(r.Geology.Stone[i]), int(r.Geology.Ore[i]), int(r.Roof[i]), map[bool]int{true: 1}[r.Grass[i]])
	}
	for _, p := range r.Plants {
		w(p.X, p.Y, int(p.Species), p.Growth)
	}
	for _, c := range r.Chunks {
		w(c.X, c.Y, int(c.Stone))
	}
	for _, ru := range r.Ruins {
		w(int(ru.Kind), ru.X0, ru.Y0, ru.X1, ru.Y1, len(ru.Walls), len(ru.Floors), len(ru.Loot), len(ru.Rubble))
	}
	for _, v := range r.Vents {
		w(v.X, v.Y)
	}
	w(r.Start.X, r.Start.Y)
	return h.Sum64()
}

// goldenMaps pin the generator's output (re-pinned for MAPGEN-3: sealed
// vaults, road bridges, duskmoss in dark caves, ocean marks) for a spread of biomes and
// hilliness: a change of map output moves a pin. Re-pin only for an
// intended change of generation (and bump mapgen.Version if saved
// colonies' debug views would then disagree with their maps).
// Re-pinned (FOOD-2, 2026-10-30): plants spawn with a share of their
// grow time, and FOOD-2 moved every crop, tree and bush to the wiki grow
// days (trees about 30 days instead of 2), so the generated growth values
// change; terrain and plant placement are unchanged.
var goldenMaps = []struct {
	biome biome.ID
	hills site.Hills
	extra func(*site.Site)
	want  uint64
}{
	{biome.TemperateForest, site.Flat, nil, 0x170a38e55b0b80ce},
	{biome.TemperateForest, site.LargeHills, func(s *site.Site) { s.River = site.RiverPath{Size: site.River, In: 0, Out: 4}; s.Caves = true }, 0x70e4ae5c29325e99},
	{biome.BorealForest, site.Mountainous, func(s *site.Site) { s.Caves = true; s.Uphill, s.HasUphill = 2, true }, 0xddd61ef4abbb9f7c},
	{biome.Tundra, site.SmallHills, func(s *site.Site) { s.Lakes = 2 }, 0xe361f70f1f8e340d},
	{biome.ColdBog, site.Flat, func(s *site.Site) { s.Coast = 1 << 6 }, 0x0a5abc6055870b7a},
	{biome.IceSheet, site.LargeHills, nil, 0x9bb1e3e269aea4},
	{biome.SeaIce, site.Flat, nil, 0x9583980065b10be2},
	{biome.TemperateSwamp, site.SmallHills, func(s *site.Site) { s.River = site.RiverPath{Size: site.Creek, In: 2, Out: 6} }, 0xb16fbbae0c780e89},
	{biome.TropicalRainforest, site.LargeHills, func(s *site.Site) { s.Road = 1<<0 | 1<<4; s.Settled = true }, 0x60b5a25a2a1e1ad9},
	{biome.AridShrubland, site.Mountainous, func(s *site.Site) { s.Stones = []geology.Stone{geology.Sandstone, geology.Granite} }, 0xeee369698f9f1600},
	{biome.Desert, site.SmallHills, func(s *site.Site) { s.Coast = 1 << 2; s.CoastNear = 1 }, 0xccbbae807ecb88b4},
	{biome.ExtremeDesert, site.Flat, func(s *site.Site) { s.Road = 1 << 6 }, 0x732d39cb3f689572},
}

func TestGoldenMaps(t *testing.T) {
	for i, g := range goldenMaps {
		s := site.Site{Biome: g.biome, Hills: g.hills, Climate: biome.DefOf(g.biome).Climate}
		if g.extra != nil {
			g.extra(&s)
		}
		r, err := Generate(mapgen.Config{Seed: 7000 + uint64(i), W: 120, H: 120, Site: s, Biome: biome.DefOf(g.biome)})
		if err != nil {
			t.Fatal(err)
		}
		if got := resultHash(r); got != g.want {
			t.Errorf("golden %d (%s, %s): hash %#x, want %#x", i, g.biome, g.hills.Label(), got, g.want)
		}
	}
}
