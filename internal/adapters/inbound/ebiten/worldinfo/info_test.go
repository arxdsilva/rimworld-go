package worldinfo

import (
	"strings"
	"sync"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/globe"
	"rimworld-go/internal/adapters/inbound/ebiten/units"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/planet"
)

var (
	once sync.Once
	pl   *planet.Planet
)

// world is a tiny shared v1 world.
func world(t *testing.T) *planet.Planet {
	t.Helper()
	once.Do(func() {
		b := planet.NewBuilder(planet.Params{SeedText: "worldinfo", Coverage: 5, Rain: 2, Temp: 2, Pop: 1,
			Factions: faction.DefaultSlots(), Version: planet.CurrentVersion})
		for {
			if _, _, done := b.Step(50000); done {
				pl = b.Planet()
				return
			}
		}
	})
	return pl
}

func landTile(t *testing.T, p *planet.Planet) int {
	t.Helper()
	for i := 0; i < p.TileCount(); i++ {
		if ok, _ := p.ValidLanding(i); ok {
			return i
		}
	}
	t.Fatal("no landing tile")
	return -1
}

func rows(ti TileInfo) map[string]string {
	m := map[string]string{}
	for _, r := range ti.Rows {
		if r.Label != "" {
			m[r.Label] = r.Value
		}
	}
	return m
}

func TestTileInfoUsesTheDomainNumbers(t *testing.T) {
	p := world(t)
	i := landTile(t, p)
	ti := BuildTileInfoWith(p, i, Opts{Day: 20})
	got := rows(ti)
	for _, l := range []string{"Terrain", "Coordinates", "Time zone", "Elevation", "Avg temperature", "Summer / winter", "Rainfall", "Growing period",
		"Stone types", "Water", "Caves", "Roads", "Nearby bases", "Movement difficulty", "Caravan per tile", "Forage", "Wildlife", "Disease"} {
		if got[l] == "" {
			t.Errorf("no %q row", l)
		}
	}
	d := biome.DefOf(p.Biome[i])
	if want := TimeZoneText(p.TimeZone(i)); got["Time zone"] != want || !strings.HasPrefix(want, "UTC") {
		t.Errorf("time zone %q", got["Time zone"])
	}
	if got["Forage"] != ForageText(p.Biome[i]) || !strings.HasSuffix(got["Forage"], "%") {
		t.Errorf("forage %q for %d", got["Forage"], d.Forage)
	}
	if want := DiseaseText(p.Biome[i]); got["Disease"] != want || !strings.Contains(want, "every ~") {
		t.Errorf("disease %q", got["Disease"])
	}
	if mv := p.MoveDifficulty(i, 20); !strings.HasPrefix(got["Movement difficulty"], "x") && mv > 0 {
		t.Errorf("movement %q for %v", got["Movement difficulty"], mv)
	}
	if len(ti.Quadrums) != 4 {
		t.Fatalf("quadrum table %v", ti.Quadrums)
	}
	seen := map[string]bool{}
	for _, q := range ti.Quadrums {
		seen[q.Season] = true
		if !strings.HasSuffix(q.Temp, "°C") {
			t.Errorf("quadrum temp %q", q.Temp)
		}
	}
	if len(seen) != 4 {
		t.Errorf("each quadrum names a different season: %v", ti.Quadrums)
	}
}

func TestTileInfoFahrenheit(t *testing.T) {
	p := world(t)
	i := landTile(t, p)
	ti := BuildTileInfoWith(p, i, Opts{Units: units.FromUnit(units.UnitF)})
	got := rows(ti)
	if !strings.HasSuffix(got["Avg temperature"], "°F") || !strings.Contains(got["Summer / winter"], "°F") || strings.Contains(got["Summer / winter"], "°C") {
		t.Errorf("rows %v", got)
	}
	for _, q := range ti.Quadrums {
		if !strings.HasSuffix(q.Temp, "°F") {
			t.Errorf("quadrum %q", q.Temp)
		}
	}
}

func TestWaterAndOutsideTiles(t *testing.T) {
	p := world(t)
	ocean := -1
	for i := 0; i < p.TileCount(); i++ {
		if p.Water[i] == planet.Ocean {
			ocean = i
			break
		}
	}
	if o := BuildTileInfo(p, ocean, ""); o.Name != "Ocean" || len(o.Rows) != 3 || len(o.Quadrums) != 0 || o.Invalid == "" {
		t.Errorf("ocean %+v", o)
	}
	if !BuildTileInfo(p, -1, "").Outside {
		t.Error("outside")
	}
}

func TestNearBaseWarning(t *testing.T) {
	p := world(t)
	hit, miss := false, false
	for i := 0; i < p.TileCount() && !(hit && miss); i++ {
		if !p.Land(i) {
			continue
		}
		_, near := p.NearBase(i)
		warned := false
		for _, w := range BuildTileInfo(p, i, "").Warnings {
			warned = warned || strings.HasPrefix(w, "Near a faction base")
		}
		if warned != near {
			t.Fatalf("tile %d: near %v warned %v", i, near, warned)
		}
		hit, miss = hit || near, miss || !near
	}
	if !hit {
		t.Skip("no tile near a base on this world")
	}
}

func TestMoveHours(t *testing.T) {
	if got := TravelHours(2); got < 2.87 || got > 2.89 { // 0.06 days x 24 x 2
		t.Errorf("hours %v", got)
	}
	if hoursText(0) != "-" || moveText(0) != "Impassable" {
		t.Error("impassable water")
	}
}

func TestFilterPanelMapsToThePlanetFilter(t *testing.T) {
	var f FilterPanel
	if f.Active() || f.Filter() != (planet.Filter{}) {
		t.Fatal("the zero panel filters nothing")
	}
	f.Biomes = biome.Of(biome.Desert)
	f.TempOn, f.TempMin, f.TempMax = true, 10, 10
	f.Hills = 1 << 2
	f.River, f.Stone = planet.Yes, geology.Granite
	got := f.Filter()
	if got.Biomes != f.Biomes || got.TempMin != 10 || got.TempMax <= got.TempMin || got.Hills != 4 || got.River != planet.Yes || got.Stone != geology.Granite {
		t.Errorf("filter %+v", got)
	}
	if !f.Active() {
		t.Error("active")
	}
	f.Reset()
	if f.Active() {
		t.Error("reset")
	}
	// Cycling visits every stone once and returns to any.
	s, n := geology.None, 0
	for {
		s = nextStone(s)
		n++
		if s == geology.None {
			break
		}
	}
	if n != len(geology.Stones())+1 {
		t.Errorf("stone cycle length %d", n)
	}
	if nextTri(planet.No) != planet.Any {
		t.Error("tri cycle")
	}
}

func TestFilteredRandomMatches(t *testing.T) {
	p := world(t)
	f := FilterPanel{Biomes: biome.Of(biome.TemperateForest)}
	for roll := 0; roll < 20; roll++ {
		if i := p.RandomLandingFiltered(f.Filter(), roll); i >= 0 && p.Biome[i] != biome.TemperateForest {
			t.Fatalf("roll %d picked %v", roll, p.Biome[i])
		}
	}
	f.Biomes = 0
	f.TempOn, f.TempMin, f.TempMax = true, 200, 300 // hotter than any tile
	if p.RandomLandingFiltered(f.Filter(), 0) != -1 {
		t.Error("impossible criteria match nothing")
	}
}

type countFinder struct{ n int }

func (c *countFinder) Find(q string) []planet.Hit {
	c.n++
	return []planet.Hit{{Name: q, Kind: planet.FeatSea, Feature: 0, Tile: 1}}
}

func TestSearchBoxRefreshesOnlyOnAChangedQuery(t *testing.T) {
	var s SearchBox
	f := &countFinder{}
	s.Query = "al"
	s.refresh(f)
	s.refresh(f)
	if f.n != 1 || len(s.Hits()) != 1 {
		t.Errorf("finds %d hits %d", f.n, len(s.Hits()))
	}
	s.Query = "alp"
	s.refresh(f)
	if f.n != 2 {
		t.Errorf("finds %d", f.n)
	}
	if s.Height() <= float32(2*searchPad+searchFieldH) {
		t.Error("results grow the panel")
	}
	s.Reset()
	if s.Query != "" || len(s.Hits()) != 0 {
		t.Error("reset")
	}
}

func TestRealSearchFindsFeatures(t *testing.T) {
	p := world(t)
	fs := p.Features()
	if len(fs) == 0 {
		t.Skip("no features")
	}
	hits := p.Find(fs[0].Name)
	if len(hits) == 0 || hits[0].Name != fs[0].Name {
		t.Errorf("hits %v for %q", hits, fs[0].Name)
	}
}

func TestLegendHeights(t *testing.T) {
	// Every mode has a positive height, and the biome list is the tallest.
	for m := 0; m < int(globe.NumModes); m++ {
		if LegendHeight(globe.Mode(m)) <= 0 {
			t.Errorf("mode %d", m)
		}
	}
}

// A base the running game has razed stays on the view's planet copy but
// leaves the card (MAP-05).
func TestNearbyBasesSkipRazedOnes(t *testing.T) {
	p := world(t)
	i := landTile(t, p)
	near := p.NearbyBases(i, 1)
	if len(near) == 0 {
		t.Skip("no base within reach of this tile")
	}
	razed := baseName(p, near[0])
	info := BuildTileInfoWith(p, i, Opts{Live: func(tile int) bool { return tile != near[0].Tile }})
	for _, r := range info.Rows {
		if strings.Contains(r.Value, razed) {
			t.Fatalf("razed base still listed: %q", r.Value)
		}
	}
}
