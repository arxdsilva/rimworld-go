package water

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/site"
)

func landMean(c *mapgen.Ctx) float64 {
	s, n := 0.0, 0
	for i, w := range c.F.Wetness {
		if !c.F.Water[i].Wet() {
			s += float64(w)
			n++
		}
	}
	return s / float64(n)
}

func TestWetnessDeterministicAndInRange(t *testing.T) {
	s := site.Site{Biome: biome.TemperateSwamp, Hills: site.Flat, Lakes: 2, River: site.RiverPath{Size: site.River, In: 0, Out: 2}}
	a, b, o := fixture(3, 140, 120, s), fixture(3, 140, 120, s), fixture(4, 140, 120, s)
	all(t, a)
	all(t, b)
	all(t, o)
	if !slices.Equal(a.F.Wetness, b.F.Wetness) {
		t.Fatal("same seed gave different wetness")
	}
	if slices.Equal(a.F.Wetness, o.F.Wetness) {
		t.Fatal("other seed gave the same wetness")
	}
	for i, w := range a.F.Wetness {
		if w < 0 || w > 1 {
			t.Fatalf("wetness %v at %d", w, i)
		}
		if a.F.Water[i].Wet() && w != 1 {
			t.Fatalf("water cell %d has wetness %v", i, w)
		}
	}
}

func TestWetnessByBiome(t *testing.T) {
	wet := []biome.ID{biome.ColdBog, biome.TemperateSwamp, biome.TropicalSwamp}
	dry := []biome.ID{biome.Desert, biome.ExtremeDesert}
	rains := map[biome.ID][]int16{ // 0 (unknown) plus a plausible planet rain
		biome.ColdBog: {0, 1250}, biome.TemperateSwamp: {0, 1550}, biome.TropicalSwamp: {0, 2600},
		biome.Desert: {0, 150, 240}, biome.ExtremeDesert: {0, 40},
	}
	for seed := uint64(1); seed <= 4; seed++ {
		for _, id := range append(wet, dry...) {
			for _, rain := range rains[id] {
				c := fixture(seed, 150, 150, site.Site{Biome: id, Hills: site.Flat, Rain: rain})
				all(t, c)
				m := landMean(c)
				if slices.Contains(wet, id) && m < 0.55 {
					t.Fatalf("seed %d %s rain %d: mean wetness %.3f < 0.55", seed, id, rain, m)
				}
				if slices.Contains(dry, id) && m > 0.25 {
					t.Fatalf("seed %d %s rain %d: mean wetness %.3f > 0.25", seed, id, rain, m)
				}
			}
		}
		// a desert with a river and lakes stays dry overall
		c := fixture(seed, 200, 200, site.Site{Biome: biome.Desert, Hills: site.Flat, Lakes: 2,
			River: site.RiverPath{Size: site.LargeRiver, In: 0, Out: 4}})
		all(t, c)
		if m := landMean(c); m > 0.25 {
			t.Fatalf("seed %d: desert with a river has mean wetness %.3f", seed, m)
		}
	}
	// more rain, wetter ground
	lo := fixture(1, 120, 120, site.Site{Biome: biome.TemperateForest, Hills: site.Flat, Rain: 600})
	hi := fixture(1, 120, 120, site.Site{Biome: biome.TemperateForest, Hills: site.Flat, Rain: 2000})
	all(t, lo)
	all(t, hi)
	if landMean(lo) >= landMean(hi) {
		t.Fatalf("rain 600 mean %.3f >= rain 2000 mean %.3f", landMean(lo), landMean(hi))
	}
}

func TestWetnessHigherNearWater(t *testing.T) {
	for _, s := range []site.Site{
		{Biome: biome.AridShrubland, Hills: site.Flat, River: site.RiverPath{Size: site.River, In: 6, Out: 2}},
		{Biome: biome.TemperateForest, Hills: site.Flat, Coast: dirs(0), CoastNear: 1},
		{Biome: biome.Tundra, Hills: site.Flat, Lakes: 3},
	} {
		for seed := uint64(1); seed <= 3; seed++ {
			c := fixture(seed, 200, 200, s)
			all(t, c)
			// distance to any open water
			src := make([]bool, c.W*c.H)
			for i, w := range c.F.Water {
				src[i] = w.Wet()
			}
			d := mapgen.BFSDist(c.W, c.H, src, nil, true)
			var near, far, nn, nf float64
			for i, w := range c.F.Wetness {
				switch {
				case d[i] >= 1 && d[i] <= 4:
					near += float64(w)
					nn++
				case d[i] >= 40:
					far += float64(w)
					nf++
				}
			}
			if nn == 0 || nf == 0 || near/nn < far/nf+0.1 {
				t.Fatalf("%+v seed %d: near water %.3f (%v cells), far %.3f (%v cells)", s, seed, near/nn, nn, far/nf, nf)
			}
		}
	}
}

func TestFeaturelessSiteHasNoWater(t *testing.T) {
	for _, sz := range [][2]int{{1, 1}, {24, 24}, {120, 90}} {
		c := fixture(11, sz[0], sz[1], site.Site{Biome: biome.Tundra, Hills: site.Flat})
		all(t, c)
		for i := range c.F.Water {
			if c.F.Water[i] != mapgen.Dry || c.F.River[i] || c.F.Coast[i] || c.F.Lake[i] ||
				c.F.RiverDist[i] != mapgen.Far || c.F.CoastDist[i] != mapgen.Far {
				t.Fatalf("%v: water at %d", sz, i)
			}
		}
		if len(c.F.RiverLine) != 0 {
			t.Fatal("river line on a featureless site")
		}
	}
}

func TestAllStepsTinyMaps(t *testing.T) {
	for _, sz := range [][2]int{{1, 1}, {2, 7}, {24, 24}, {50, 50}} {
		for in := site.Dir8(0); in < 8; in++ {
			for _, out := range []site.Dir8{in, (in + 4) % 8, (in + 1) % 8} {
				c := fixture(uint64(in)+1, sz[0], sz[1], site.Site{Biome: biome.TropicalSwamp, Hills: site.Flat,
					Lakes: 3, Coast: dirs(in, (in+2)%8), CoastNear: 1,
					River: site.RiverPath{Size: site.LargeRiver, In: in, Out: out}})
				all(t, c)
			}
		}
	}
}

func BenchmarkWaterSteps500(b *testing.B) {
	s := site.Site{Biome: biome.TemperateForest, Hills: site.LargeHills, Lakes: 3, Coast: dirs(2), CoastNear: 1,
		Rain: 1100, River: site.RiverPath{Size: site.LargeRiver, In: 0, Out: 4}}
	for _, st := range []mapgen.Step{Coast{}, River{}, Lakes{}, Wetness{}} {
		b.Run(st.Name(), func(b *testing.B) {
			for b.Loop() {
				b.StopTimer()
				c := fixture(1, 500, 500, s)
				for _, pre := range []mapgen.Step{Coast{}, River{}, Lakes{}, Wetness{}} {
					if pre.Name() == st.Name() {
						break
					}
					run(b, c, pre)
				}
				b.StartTimer()
				run(b, c, st)
			}
		})
	}
}
