package planet

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"math"
	"strings"
	"sync"
	"testing"

	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/site"
)

var (
	builtV1   = map[string]*Planet{}
	builtV1Mu sync.Mutex
)

// worldV1 builds (once per test binary) a v1 planet with the real
// factions at default sliders.
func worldV1(t testing.TB, seed string, coverage int) *Planet {
	t.Helper()
	builtV1Mu.Lock()
	defer builtV1Mu.Unlock()
	k := fmt.Sprintf("%s/%d", seed, coverage)
	if p, ok := builtV1[k]; ok {
		return p
	}
	p := Build(Params{SeedText: seed, Coverage: coverage, Rain: 2, Temp: 2, Pop: 1, Version: CurrentVersion,
		Factions: faction.DefaultSlots()})
	builtV1[k] = p
	return p
}

// hashV1 is hashPlanet plus the v1 data: road classes, features and name.
func hashV1(p *Planet) uint64 {
	h := fnv.New64a()
	w := func(v any) { _ = binary.Write(h, binary.LittleEndian, v) }
	w(hashPlanet(p))
	for k := range p.Roads {
		w(uint8(p.RoadClassOf(k)))
	}
	for _, f := range p.Features() {
		h.Write([]byte(f.Name))
		w(uint8(f.Kind))
		w(int32(f.Centre))
		w(f.Tiles)
	}
	h.Write([]byte(p.Name()))
	return h.Sum64()
}

// goldenHashV1 pins the v1 generator on "amber-hollow" at 30% (no
// factions). Re-pin only for an intended v1 change, recorded in the
// handoff; a change that must not move saved worlds needs a new version.
const goldenHashV1 = 0xc9037e9c6efba9b1

func TestPlanetGoldenHashV1(t *testing.T) {
	p := Build(Params{SeedText: "amber-hollow", Coverage: 30, Rain: 2, Temp: 2, Pop: 1, Version: 1})
	if got := hashV1(p); got != goldenHashV1 {
		t.Errorf("v1 planet hash = %#x, want %#x", got, uint64(goldenHashV1))
	}
}

// WG-17: a v0 world (an old save's Setup.World) rebuilds byte-identically
// and carries none of the v1 data.
func TestV0WorldUnchanged(t *testing.T) {
	params := Params{SeedText: "amber-hollow", Coverage: 30, Rain: 2, Temp: 2, Pop: 1}
	a := Build(params)
	if hashPlanet(a) != goldenHash {
		t.Fatalf("v0 hash %#x moved from %#x", hashPlanet(a), uint64(goldenHash))
	}
	b := NewBuilder(params)
	for {
		if _, _, done := b.Step(333); done {
			break
		}
	}
	if hashV1(b.Planet()) != hashV1(a) {
		t.Fatal("v0 rebuild differs with the step size")
	}
	if a.Name() != "" || len(a.Features()) != 0 || a.FeatureAt(0) != -1 || len(a.ext.roadClass) != 0 {
		t.Fatal("v0 world carries v1 data")
	}
	for i := range a.River {
		if a.River[i] == site.HugeRiver {
			t.Fatal("v0 world has a huge river")
		}
	}
	for k := range a.Roads {
		if a.RoadClassOf(k) != DirtRoad {
			t.Fatal("v0 roads are dirt roads")
		}
	}
	if Seed(params) == Seed(Params{SeedText: params.SeedText, Version: CurrentVersion}) {
		t.Error("v1 must seed differently from v0")
	}
}

func TestV1StepBudgetDeterministic(t *testing.T) {
	params := Params{SeedText: "steps", Coverage: 30, Rain: 2, Temp: 2, Pop: 1, Version: CurrentVersion, Factions: faction.DefaultSlots()}
	big := Build(params)
	b := NewBuilder(params)
	for {
		if _, _, done := b.Step(97); done {
			break
		}
	}
	if hashV1(big) != hashV1(b.Planet()) {
		t.Fatal("step size changed the v1 planet")
	}
}

// WG-02: at default sliders the v1 biome mix keeps arid shrubland a
// fringe, has real temperate forests and rainforests, and every biome.
func TestBiomeSharesV1(t *testing.T) {
	var count [biome.NumBiomes]int
	land := 0
	for _, s := range []string{"amber-hollow", "a", "b", "c", "d", "e"} {
		p := worldV1(t, s, 100)
		for i := 0; i < p.TileCount(); i++ {
			if p.Water[i] == Land {
				land++
				count[p.Biome[i]]++
			}
		}
	}
	share := func(b biome.ID) float64 { return float64(count[b]) / float64(land) }
	for b := biome.ID(0); b < biome.NumBiomes; b++ {
		t.Logf("%-20s %5.1f%%", b, 100*share(b))
		if count[b] == 0 {
			t.Errorf("%s missing", b)
		}
	}
	if share(biome.AridShrubland) > 0.20 || share(biome.TemperateForest) < 0.15 || share(biome.TropicalRainforest) < 0.06 {
		t.Errorf("shares: shrubland %.3f (≤0.20), temperate %.3f (≥0.15), rainforest %.3f (≥0.06)",
			share(biome.AridShrubland), share(biome.TemperateForest), share(biome.TropicalRainforest))
	}
}

// Every v1 biome lies inside its wiki thresholds.
func TestScoreBiomeInsideWikiThresholds(t *testing.T) {
	for tc := -40.0; tc <= 35; tc += 0.5 {
		for r := 0; r <= 4000; r += 50 {
			for _, h := range []site.Hills{site.Flat, site.SmallHills, site.LargeHills, site.Mountainous} {
				for _, e := range []int{50, 300, 1500} {
					b := scoreBiome(tc, r, h, e)
					bad := false
					switch b {
					case biome.TropicalRainforest, biome.TropicalSwamp:
						bad = tc < tropicalMinT || r < tropicalMinRain
					case biome.TemperateForest, biome.BorealForest, biome.TemperateSwamp:
						bad = tc < forestMinT || r < forestMinRain
					case biome.ColdBog:
						bad = tc < forestMinT
					case biome.Tundra:
						bad = tc > tundraMaxT
					case biome.IceSheet:
						bad = tc > iceSheetMaxT
					case biome.AridShrubland:
						bad = tc < forestMinT || r < forestMinRain || r >= shrubMaxRain
					case biome.Desert:
						bad = tc < desertMinT || r >= desertMaxRain
					case biome.ExtremeDesert:
						bad = tc < extremeMinT || r > extremeMaxRain
					case biome.SeaIce:
						bad = true
					}
					if bad {
						t.Fatalf("scoreBiome(%v °C, %d mm, %v, %d m) = %v outside its thresholds", tc, r, h, e, b)
					}
				}
			}
		}
	}
	p := worldV1(t, "a", 100)
	for i := 0; i < p.TileCount(); i++ {
		if p.Biome[i] == biome.SeaIce && p.Temp[i] > seaIceTempV1X10 {
			t.Fatalf("tile %d: sea ice at %d tenths", i, p.Temp[i])
		}
	}
}

// WG-07: v1 roads carry every class; settlement roads follow tech.
func TestRoadClassesPresent(t *testing.T) {
	var seen [NumRoadClasses]int
	for _, s := range []string{"amber-hollow", "a", "b"} {
		p := worldV1(t, s, 100)
		if len(p.ext.roadClass) != len(p.Roads) {
			t.Fatalf("%s: %d classes for %d roads", s, len(p.ext.roadClass), len(p.Roads))
		}
		for k := range p.Roads {
			seen[p.RoadClassOf(k)]++
		}
	}
	for c := RoadClass(0); c < NumRoadClasses; c++ {
		if seen[c] == 0 {
			t.Errorf("no %s", c.Label())
		}
		if c.Label() == "" {
			t.Errorf("class %d unlabelled", c)
		}
	}
	if RoadClass(99).Label() != "" || (&Planet{}).RoadClassOf(3) != DirtRoad {
		t.Error("out-of-range classes")
	}
}

// WG-08: v1 sizes the largest rivers huge (and only there).
func TestHugeRiverV1(t *testing.T) {
	huge := 0
	for _, s := range []string{"amber-hollow", "a", "b", "c", "d", "e"} {
		p := worldV1(t, s, 100)
		for i := range p.River {
			if p.River[i] != site.HugeRiver {
				continue
			}
			huge++
			// downstream of a huge river stays huge until it ends
			if to := p.RiverTo[i]; to >= 0 && p.River[to] != site.NoRiver && p.River[to] != site.HugeRiver {
				t.Fatalf("%s: huge river %d flows into %v", s, i, p.River[to])
			}
		}
	}
	if huge == 0 {
		t.Error("no huge river on six full globes")
	}
}

// WG-09: features are named, unique, located and findable.
func TestFeaturesNamedAndFindable(t *testing.T) {
	p := worldV1(t, "amber-hollow", 100)
	fs := p.Features()
	if len(fs) == 0 {
		t.Fatal("no features")
	}
	kinds := map[FeatureKind]bool{}
	names := map[string]bool{}
	for k, f := range fs {
		kinds[f.Kind] = true
		if f.Name == "" || names[f.Name] || f.Kind.Label() == "" {
			t.Fatalf("feature %d: name %q kind %v (duplicate or blank)", k, f.Name, f.Kind)
		}
		names[f.Name] = true
		if len(f.Tiles) == 0 || p.FeatureAt(f.Centre) < 0 {
			t.Fatalf("feature %q: centre %d unmapped", f.Name, f.Centre)
		}
		inside := false
		for _, tl := range f.Tiles {
			inside = inside || int(tl) == f.Centre
		}
		if !inside {
			t.Fatalf("feature %q centre outside its tiles", f.Name)
		}
		hits := p.Find(strings.ToUpper(f.Name))
		if len(hits) == 0 || hits[0].Feature != k || hits[0].Tile != f.Centre {
			t.Fatalf("Find(%q) = %v, want feature %d first", f.Name, hits, k)
		}
	}
	for _, want := range []FeatureKind{FeatOcean, FeatMountains, FeatDesert, FeatForest, FeatIceCap, FeatLake} {
		if !kinds[want] {
			t.Errorf("no %s", want.Label())
		}
	}
	// prefix and substring searches, settlements, blank queries
	f := fs[0]
	bare := strings.TrimPrefix(f.Name, "the ")
	if hits := p.Find(bare[:3]); len(hits) == 0 || len(hits) > findMaxResults {
		t.Errorf("prefix search %q: %d hits", bare[:3], len(hits))
	}
	if tiles := p.SettlementTiles(); len(tiles) > 0 {
		name := p.SettlementName(int(tiles[0]))
		hits := p.Find(name)
		if len(hits) == 0 || hits[0].Kind != FeatSettlement || hits[0].Tile != int(tiles[0]) || hits[0].Feature != -1 {
			t.Errorf("settlement %q not found first: %v", name, hits)
		}
	}
	if p.Find("  ") != nil || p.FeatureAt(-1) != -1 || p.FeatureAt(p.TileCount()) != -1 {
		t.Error("blank query or bad tile")
	}
	if p.Name() == "" || p.Name() != worldV1(t, "amber-hollow", 100).Name() || p.Name() == worldV1(t, "a", 100).Name() {
		t.Errorf("planet names %q / %q", p.Name(), worldV1(t, "a", 100).Name())
	}
}

func TestMatchRank(t *testing.T) {
	cases := []struct {
		name, q string
		want    int
	}{{"the vaster ocean", "vaster ocean", 0}, {"lake miro", "lake", 1}, {"the vaster ocean", "vas", 1},
		{"lake miro", "mir", 2}, {"lake miro", "ake", 3}, {"lake miro", "zz", -1}}
	for _, c := range cases {
		if got := matchRank(c.name, c.q); got != c.want {
			t.Errorf("matchRank(%q, %q) = %d, want %d", c.name, c.q, got, c.want)
		}
	}
}

// WG-11: every filter field narrows the pick; the zero filter is
// RandomLanding; an impossible filter gives -1.
func TestRandomLandingHonoursFilter(t *testing.T) {
	p := worldV1(t, "a", 100)
	for n := 0; n < 20; n++ {
		if p.RandomLandingFiltered(Filter{}, n) != p.RandomLanding(n) {
			t.Fatalf("roll %d: zero filter differs from RandomLanding", n)
		}
	}
	var stone geology.Stone
	for i := 0; i < p.TileCount() && stone == geology.None; i++ {
		if ok, _ := p.ValidLanding(i); ok {
			stone = p.Stones[i][0]
		}
	}
	filters := map[string]Filter{
		"biomes":   {Biomes: biome.Of(biome.Desert, biome.Tundra)},
		"temp":     {TempMin: 10, TempMax: 20},
		"hills":    {Hills: 1 << site.Mountainous},
		"river":    {River: Yes},
		"no river": {River: No},
		"coast":    {Coast: Yes},
		"road":     {Road: Yes},
		"stone":    {Stone: stone},
		"caves":    {Caves: Yes},
	}
	for name, f := range filters {
		for n := 0; n < 15; n++ {
			tile := p.RandomLandingFiltered(f, n)
			if tile < 0 {
				t.Fatalf("%s: no tile", name)
			}
			if ok, why := p.ValidLanding(tile); !ok {
				t.Fatalf("%s: invalid tile %d: %s", name, tile, why)
			}
			if tile != p.RandomLandingFiltered(f, n) {
				t.Fatalf("%s: not deterministic", name)
			}
			coast := false
			for _, nb := range p.Neighbours(tile) {
				coast = coast || p.Water[nb] == Ocean
			}
			temp := float64(p.Temp[tile]) / 10
			ok := map[string]bool{
				"biomes": p.Biome[tile] == biome.Desert || p.Biome[tile] == biome.Tundra,
				"temp":   temp >= 10 && temp <= 20, "hills": p.Hill[tile] == site.Mountainous,
				"river": p.River[tile] != site.NoRiver, "no river": p.River[tile] == site.NoRiver,
				"coast": coast, "road": p.ext.roadAt[tile], "caves": p.Caves[tile],
				"stone": p.Stones[tile][0] == stone || p.Stones[tile][1] == stone || p.Stones[tile][2] == stone,
			}[name]
			if !ok {
				t.Fatalf("%s: tile %d does not pass", name, tile)
			}
		}
	}
	if p.RandomLandingFiltered(Filter{Biomes: biome.Of(biome.SeaIce), TempMin: 30, TempMax: 40}, 0) != -1 {
		t.Error("impossible filter must give -1")
	}
	if (&Planet{}).RandomLandingFiltered(Filter{}, 0) != -1 {
		t.Error("empty planet")
	}
}

// WG-03: biome base + hills, winter ramp and roads (wiki: Caravan).
func TestMoveDifficultyWiki(t *testing.T) {
	p := Build(Params{SeedText: "move", Coverage: 5, Version: CurrentVersion})
	i := -1
	for k := 0; k < p.TileCount() && i < 0; k++ {
		if p.Land(k) && !p.ext.roadAt[k] {
			i = k
		}
	}
	if i < 0 {
		t.Skip("no land tile")
	}
	p.Temp[i] = 300 // never winter
	rows := []struct {
		b    biome.ID
		h    site.Hills
		want float64
	}{
		{biome.TemperateForest, site.Flat, 1}, {biome.BorealForest, site.SmallHills, 1.5}, {biome.Tundra, site.LargeHills, 2.5},
		{biome.Desert, site.Mountainous, 4}, {biome.ExtremeDesert, site.Flat, 1}, {biome.AridShrubland, site.Flat, 1},
		{biome.IceSheet, site.Flat, 1.5}, {biome.SeaIce, site.SmallHills, 2}, {biome.TropicalRainforest, site.Flat, 2},
		{biome.TemperateSwamp, site.Flat, 4}, {biome.TropicalSwamp, site.LargeHills, 5.5}, {biome.ColdBog, site.Mountainous, 7},
	}
	for _, r := range rows {
		p.Biome[i], p.Hill[i] = r.b, r.h
		for _, day := range []int{0, 17, 45, -3, 61} {
			if got := p.MoveDifficulty(i, day); math.Abs(got-r.want) > 1e-9 {
				t.Errorf("%s %s day %d: %v, want %v", r.b, r.h.Label(), day, got, r.want)
			}
		}
	}
	// Deep winter: the full +2 on the coldest day; part of it on the ramp.
	p.Biome[i], p.Hill[i] = biome.TemperateForest, site.Flat
	p.Temp[i] = -400
	cl := p.climate(i)
	coldest := (cl.PeakDay + clock.DaysPerYear/2) % clock.DaysPerYear
	if got := p.MoveDifficulty(i, coldest); math.Abs(got-3) > 1e-9 {
		t.Errorf("deep winter: %v, want 3", got)
	}
	ramp := false
	for tm := -250; tm <= -150 && !ramp; tm += 5 {
		p.Temp[i] = int16(tm)
		for d := 0; d < clock.DaysPerYear; d++ {
			mean := float64(p.climate(i).DailyMean(clock.Tick(d*clock.TicksPerDay))) / 10
			if mean < winterStart && mean > winterFull {
				want := 1 + 2*(winterStart-mean)/(winterStart-winterFull)
				if got := p.MoveDifficulty(i, d); math.Abs(got-want) > 1e-9 {
					t.Fatalf("ramp at %.1f °C: %v, want %v", mean, got, want)
				}
				ramp = true
			}
		}
	}
	if !ramp {
		t.Error("no day on the winter ramp")
	}
	// A road halves it.
	p.Temp[i] = 300
	p.Hill[i] = site.LargeHills
	p.ext.roadAt[i] = true
	if got := p.MoveDifficulty(i, 0); got != 1.25 {
		t.Errorf("road: %v, want 1.25", got)
	}
	if p.MoveDifficulty(-1, 0) != 0 {
		t.Error("invalid tile is impassable")
	}
	for k := 0; k < p.TileCount(); k++ {
		if !p.Land(k) {
			if p.MoveDifficulty(k, 0) != 0 {
				t.Error("water is impassable")
			}
			break
		}
	}
}

func TestTimeZoneAndQuadrums(t *testing.T) {
	p := worldV1(t, "a", 100)
	seen := map[int]bool{}
	for i := 0; i < p.TileCount(); i++ {
		tz := p.TimeZone(i)
		if tz < -12 || tz > 12 || tz != int(math.Round(float64(p.Lon[i])/150)) {
			t.Fatalf("tile %d lon %d: zone %d", i, p.Lon[i], tz)
		}
		seen[tz] = true
		if i%97 == 0 {
			q := p.QuadrumTemps(i)
			mean := (q[0] + q[1] + q[2] + q[3]) / 4
			if math.Abs(mean-float64(p.Temp[i])/10) > 1.5 {
				t.Fatalf("tile %d quadrums %v average %.1f, yearly mean %.1f", i, q, mean, float64(p.Temp[i])/10)
			}
		}
	}
	if len(seen) < 24 {
		t.Errorf("%d time zones on a full globe", len(seen))
	}
	if p.TimeZone(-1) != 0 || p.QuadrumTemps(-1) != [4]float64{} {
		t.Error("invalid tile")
	}
}

// WG-12: nearby bases are sorted by hops, agree with Hops, and the
// 4-hop warning fires exactly when one is that close.
func TestNearbyBasesAndWarning(t *testing.T) {
	p := worldV1(t, "a", 100)
	tiles := p.SettlementTiles()
	if len(tiles) < 3 {
		t.Skip("too few settlements")
	}
	var s HopScratch
	for i := 0; i < p.TileCount(); i += 211 {
		bases := p.NearbyBasesWith(&s, i, 3)
		for k, b := range bases {
			f, r := p.SettlementFaction(b.Tile)
			if b.Faction != f || b.Rank != r || b.Hops != p.HopsWith(&s, i, b.Tile, nearbyMaxHops+1) {
				t.Fatalf("tile %d base %+v (faction %d rank %d)", i, b, f, r)
			}
			if k > 0 && bases[k-1].Hops > b.Hops {
				t.Fatalf("tile %d: bases not sorted", i)
			}
		}
		near, ok := p.NearBase(i)
		closest := -1
		for _, st := range tiles {
			if d := p.HopsWith(&s, i, int(st), nearBaseHops+1); d <= nearBaseHops && (closest < 0 || d < closest) {
				closest = d
			}
		}
		if ok != (closest >= 0) || ok && near.Hops != closest {
			t.Fatalf("tile %d: NearBase %+v %v, closest %d", i, near, ok, closest)
		}
	}
	at := int(tiles[0])
	if b := p.NearbyBases(at, 1); len(b) != 1 || b[0].Tile != at || b[0].Hops != 0 {
		t.Errorf("a base's own tile: %v", b)
	}
	if p.NearbyBases(-1, 3) != nil || p.NearbyBases(0, 0) != nil {
		t.Error("bad input")
	}
}

// HopsWith with per-goroutine scratch is race-free and agrees with Hops.
func TestHopsWithConcurrent(t *testing.T) {
	p := worldV1(t, "a", 30)
	n := p.TileCount()
	want := make([]int, 40)
	for k := range want {
		want[k] = p.Hops(k*37%n, k*101%n, 25)
	}
	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var s HopScratch
			for k := range want {
				if got := p.HopsWith(&s, k*37%n, k*101%n, 25); got != want[k] {
					t.Errorf("pair %d: %d, want %d", k, got, want[k])
				}
			}
		}()
	}
	wg.Wait()
}

func BenchmarkPlanetBuild100V1(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Build(Params{SeedText: fmt.Sprintf("bench-%d", i), Coverage: 100, Rain: 2, Temp: 2, Pop: 1, Version: CurrentVersion,
			Factions: faction.DefaultSlots()})
	}
}
