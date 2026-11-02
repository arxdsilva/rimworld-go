package ebiten

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/units"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/planet"
	"rimworld-go/internal/platform/config"
)

// WG-3: the landing screen's grid, search, random-site criteria and
// temperature unit, through the real widgets.

func TestLandingGridButtonTogglesTheGlobeGrid(t *testing.T) {
	t.Parallel()
	a, _ := landingApp(t)
	if a.ng.Globe.Grid() {
		t.Fatal("the grid starts off")
	}
	click(t, a, "land.grid")
	if !a.ng.Globe.Grid() {
		t.Fatal("Grid turns the tile edges on")
	}
	click(t, a, "land.grid")
	if a.ng.Globe.Grid() {
		t.Fatal("Grid turns them off again")
	}
}

func TestLandingSearchSelectsAndCentresAResult(t *testing.T) {
	t.Parallel()
	a, f := landingApp(t)
	fs := a.ng.Planet.Features()
	if len(fs) == 0 {
		t.Skip("the tiny world has no named feature")
	}
	click(t, a, "land.search")
	if !a.ng.Land.Search {
		t.Fatal("Search opens the box")
	}
	click(t, a, "land.search.field")
	f.TypeText(t, a, fs[0].Name)
	f.Frame(t, a)
	f.Frame(t, a)
	click(t, a, "land.search.result.0")
	f.Frame(t, a)
	hit := a.ng.Planet.Find(fs[0].Name)[0]
	if a.ng.Tile != hit.Tile || a.ng.Globe.Selected() != hit.Tile {
		t.Fatalf("selected %d (globe %d), want %d", a.ng.Tile, a.ng.Globe.Selected(), hit.Tile)
	}
	if hit.Feature >= 0 && len(a.ng.Globe.Hl) != len(a.ng.Planet.Features()[hit.Feature].Tiles) {
		t.Errorf("pulsing %d tiles, the feature has %d", len(a.ng.Globe.Hl), len(a.ng.Planet.Features()[hit.Feature].Tiles))
	}
}

func TestLandingFilteredRandomSiteAndNoMatch(t *testing.T) {
	t.Parallel()
	a, f := landingApp(t)
	click(t, a, "land.filter")
	if !a.ng.Land.Filter {
		t.Fatal("the caret opens the criteria")
	}
	// An impossible climate: no tile matches, nothing is selected.
	a.ng.Land.Criteria.TempOn, a.ng.Land.Criteria.TempMin, a.ng.Land.Criteria.TempMax = true, 200, 300
	a.ng.Tile = -1
	a.ng.Globe.Select(-1)
	f.Frame(t, a)
	click(t, a, "land.random")
	if a.hint != "" && !strings.Contains(a.hint, "No tile matches") {
		t.Errorf("hint %q", a.hint)
	}
	if a.ng.Tile >= 0 {
		t.Errorf("tile %d selected although nothing matches", a.ng.Tile)
	}
	// A biome criterion: the pick is in the biome.
	p := a.ng.Planet
	want := p.Biome[landingTileAnyBiome(t, p)]
	a.ng.Land.Criteria.Reset()
	a.ng.Land.Criteria.Biomes = biome.Of(want)
	f.Frame(t, a)
	click(t, a, "land.random")
	if a.ng.Tile < 0 || p.Biome[a.ng.Tile] != want {
		t.Fatalf("tile %d is not in %v", a.ng.Tile, want)
	}
	click(t, a, "land.filter.reset")
	if a.ng.Land.Criteria.Active() {
		t.Error("Reset clears the criteria")
	}
	click(t, a, "land.filter.river")
	if a.ng.Land.Criteria.River != planet.Yes {
		t.Error("the river switch cycles Any, Yes, No")
	}
}

func landingTileAnyBiome(t *testing.T, p *planet.Planet) int {
	t.Helper()
	for i := 0; i < p.TileCount(); i++ {
		if ok, _ := p.ValidLanding(i); ok {
			return i
		}
	}
	t.Fatal("no landing tile")
	return -1
}

func TestLandingTileCardFollowsTheTemperatureUnit(t *testing.T) {
	t.Parallel()
	a, f := landingApp(t)
	click(t, a, "land.random")
	f.Frame(t, a)
	a.settings.Gameplay.TempUnit = units.UnitF
	f.Frame(t, a)
	var temps []string
	for _, r := range a.ng.Info.Card().Rows {
		if r.Label == "Avg temperature" || r.Label == "Summer / winter" {
			temps = append(temps, r.Value)
		}
	}
	if len(temps) != 2 || !strings.Contains(temps[0], "°F") || !strings.Contains(temps[1], "°F") {
		t.Errorf("the tile card reads in Fahrenheit: %v", temps)
	}
	a.settings.Gameplay.TempUnit = ""
	f.Frame(t, a)
	if got := a.ng.Info.Card().Rows[4].Value; strings.Contains(got, "°F") {
		t.Errorf("back to Celsius: %q", got)
	}
}

func TestOptionsTemperatureUnitAndMaxColonies(t *testing.T) {
	t.Parallel()
	store := &fakeSettings{s: config.Defaults()}
	a := optionsApp(store)
	click(t, a, "menu.options")
	click(t, a, "options.tab.gameplay")
	if a.settings.Gameplay.TempUnit != "" {
		t.Fatalf("unit starts as %q", a.settings.Gameplay.TempUnit)
	}
	click(t, a, "options.tempunit")
	if a.settings.Gameplay.TempUnit != units.UnitF || store.s.Gameplay.TempUnit != units.UnitF {
		t.Fatalf("unit after one click: %q (stored %q)", a.settings.Gameplay.TempUnit, store.s.Gameplay.TempUnit)
	}
	click(t, a, "options.tempunit")
	if a.settings.Gameplay.TempUnit != "" {
		t.Fatal("a second click goes back to Celsius")
	}
	n := a.settings.Gameplay.Colonies()
	click(t, a, "options.maxcolonies.inc")
	if got := a.settings.Gameplay.Colonies(); got != min(n+1, config.MaxColoniesLimit) {
		t.Fatalf("colonies %d -> %d", n, got)
	}
	for i := 0; i < 8; i++ {
		click(t, a, "options.maxcolonies.inc")
	}
	if a.settings.Gameplay.Colonies() != config.MaxColoniesLimit {
		t.Fatalf("capped at %d, got %d", config.MaxColoniesLimit, a.settings.Gameplay.Colonies())
	}
	for i := 0; i < 8; i++ {
		click(t, a, "options.maxcolonies.dec")
	}
	if a.settings.Gameplay.Colonies() != 1 {
		t.Fatalf("floor 1, got %d", a.settings.Gameplay.Colonies())
	}
}
