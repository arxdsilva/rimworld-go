package generation

import (
	"reflect"
	"testing"

	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/terrain"
)

func TestLegacyBiomeIsGenerate(t *testing.T) {
	t.Parallel()
	for _, seed := range []uint64{1, 7, 42} {
		if !reflect.DeepEqual(GenerateBiome(seed, 48, 40, biome.DefOf(biome.TemperateForest)), Generate(seed, 48, 40)) {
			t.Fatalf("seed %d: temperate forest must be the original generator", seed)
		}
	}
}

func TestBiomeGroundMix(t *testing.T) {
	t.Parallel()
	for _, d := range biome.All() {
		if d.Legacy {
			continue
		}
		res := GenerateBiome(3, 120, 120, d)
		count := map[terrain.Type]int{}
		soft := 0
		for _, c := range res.Terrain.Cells {
			if c == terrain.Rock || c == terrain.Stone {
				continue
			}
			count[c]++
			soft++
		}
		if soft == 0 {
			t.Fatalf("%s: no soft ground", d.Key)
		}
		g := d.Ground
		want := map[terrain.Type]int{terrain.Soil: g.Soil, terrain.RichSoil: g.Rich, terrain.Gravel: g.Gravel,
			terrain.Sand: g.Sand, terrain.Marsh: g.Marsh, terrain.Ice: g.Ice}
		for tt, w := range want {
			got := count[tt] * 1000 / soft
			if got < w-50 || got > w+50 {
				t.Errorf("%s: %s %d permille, want %d +-50", d.Key, tt, got, w)
			}
		}
	}
}

func TestBiomePlants(t *testing.T) {
	t.Parallel()
	species := func(b biome.ID) map[plant.Species]int {
		out := map[plant.Species]int{}
		for _, p := range GenerateBiome(5, 120, 120, biome.DefOf(b)).Plants {
			out[p.Species]++
		}
		return out
	}
	for _, c := range []struct {
		b    biome.ID
		want []plant.Species
		none []plant.Species
	}{
		{biome.Desert, []plant.Species{plant.Cactus, plant.Shrub}, []plant.Species{plant.Tree, plant.Pine}},
		{biome.BorealForest, []plant.Species{plant.Pine, plant.Tree, plant.BerryBush}, []plant.Species{plant.JungleTree, plant.Cactus}},
		{biome.TropicalRainforest, []plant.Species{plant.JungleTree, plant.BerryBush}, []plant.Species{plant.Pine}},
		{biome.ColdBog, []plant.Species{plant.Pine, plant.Reeds}, nil},
		{biome.Tundra, []plant.Species{plant.Shrub}, []plant.Species{plant.Tree}},
	} {
		got := species(c.b)
		for _, s := range c.want {
			if got[s] == 0 {
				t.Errorf("%s: no %s (%v)", c.b, s, got)
			}
		}
		for _, s := range c.none {
			if got[s] != 0 {
				t.Errorf("%s: unexpected %s (%v)", c.b, s, got)
			}
		}
	}
	for _, b := range []biome.ID{biome.IceSheet, biome.SeaIce} {
		if n := len(GenerateBiome(5, 120, 120, biome.DefOf(b)).Plants); n != 0 {
			t.Errorf("%s: %d plants on ice", b, n)
		}
	}
	// a dense biome holds about its permille of fertile tiles
	res := GenerateBiome(5, 120, 120, biome.DefOf(biome.TropicalRainforest))
	fertile := 0
	for _, c := range res.Terrain.Cells {
		if c.Fertility() > 0 {
			fertile++
		}
	}
	if got := len(res.Plants) * 1000 / fertile; got < 140 || got > 180 {
		t.Errorf("rainforest plants %d permille of fertile tiles, want ~160", got)
	}
}

func TestBiomeMapsDeterministicWithOpenStart(t *testing.T) {
	t.Parallel()
	for _, d := range biome.All() {
		a, b := GenerateBiome(11, 64, 48, d), GenerateBiome(11, 64, 48, d)
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("%s: same seed gave different maps", d.Key)
		}
		for y := a.ClearY0; y <= a.ClearY1; y++ {
			for x := a.ClearX0; x <= a.ClearX1; x++ {
				if !a.Terrain.At(x, y).Walkable() {
					t.Fatalf("%s: start area blocked at %d,%d", d.Key, x, y)
				}
			}
		}
		for _, p := range a.Plants {
			if a.Terrain.At(p.X, p.Y).Fertility() == 0 {
				t.Fatalf("%s: %s on barren ground", d.Key, p.Species)
			}
			if p.Growth <= 0 || p.Growth > plant.DefOf(p.Species).GrowTicks {
				t.Fatalf("%s: %s growth %d", d.Key, p.Species, p.Growth)
			}
		}
	}
}

func TestSeaIceHasNoRock(t *testing.T) {
	t.Parallel()
	for _, c := range GenerateBiome(2, 64, 48, biome.DefOf(biome.SeaIce)).Terrain.Cells {
		if c != terrain.Ice {
			t.Fatalf("sea ice map holds %s", c)
		}
	}
	rock := 0
	for _, c := range GenerateBiome(2, 64, 48, biome.DefOf(biome.IceSheet)).Terrain.Cells {
		if c == terrain.Rock {
			rock++
		}
	}
	if rock == 0 {
		t.Fatal("an ice sheet keeps its rock outcrops")
	}
}

// TestNativePlantsSurviveTheirBiome: a biome's own wild plants live through
// its coldest night and hottest afternoon, with a blizzard's chill on top.
func TestNativePlantsSurviveTheirBiome(t *testing.T) {
	t.Parallel()
	const weatherMargin = 50
	for _, d := range biome.All() {
		lo, hi := d.Climate.Range()
		for _, pd := range d.Plants {
			if plant.Killed(pd.Species, int(lo)-weatherMargin) || plant.Killed(pd.Species, int(hi)) {
				t.Errorf("%s: native %s dies in its own climate (%v..%v)", d.Key, pd.Species, lo, hi)
			}
		}
	}
}
