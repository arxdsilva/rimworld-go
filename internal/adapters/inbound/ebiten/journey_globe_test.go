package ebiten

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/globe"
	"rimworld-go/internal/adapters/inbound/ebiten/globemath"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/adapters/inbound/ebiten/units"
	"rimworld-go/internal/adapters/inbound/ebiten/wizard"
	"rimworld-go/internal/domain/world/biome"
)

// journey_globe_test.go: the new-game globe (WG-18): the landing screen's
// view buttons, search, random-site criteria, Factions dialog and tile
// panel, the world sliders and the two Options rows, all by mouse and keys.
var _ = owe("journey_globe_test.go",
	[]string{},
	[]string{})

// globeToWorld walks the wizard to Create World with a tiny world's seed.
func globeToWorld(t *testing.T) *journey {
	t.Helper()
	j := newMenuJourney(t)
	j.clickID("menu.newgame")
	j.clickID("wizard.next")
	j.clickID("wizard.next")
	if j.a.nav.screen() != ports.ScreenWorldGen {
		t.Fatalf("screen %s, want Create World", j.a.nav.screen())
	}
	ng := j.a.wiz.Draft()
	ng.World.SeedText, ng.World.Coverage = tinySeed, 5
	return j
}

// globeToLanding generates the tiny world and arrives at Select Landing Site.
func globeToLanding(t *testing.T) *journey {
	t.Helper()
	j := globeToWorld(t)
	j.clickID("world.generate")
	flowUntil(j, "the landing screen", func() bool { return j.a.nav.screen() == ports.ScreenLanding }, 4000)
	j.update(3)
	return j
}

// TestJourneyGlobeClickATile: a click at a land tile's projected screen
// point selects exactly that tile.
func TestJourneyGlobeClickATile(t *testing.T) {
	t.Parallel()
	j := globeToLanding(t)
	ng := j.a.wiz.Draft()
	g := ng.Globe
	geo := wizard.LandRects(j.a.sw, j.a.sh)
	cx, cy := geo.Globe.Center()
	best, bx, by, bd := -1, 0, 0, 1e18
	for i := 0; i < ng.Planet.TileCount(); i++ {
		c := ng.Planet.Center[i]
		sx, sy, z := g.Cam.Project(g.Viewport(), globemath.Vec3{float64(c[0]), float64(c[1]), float64(c[2])})
		if z <= globemath.HorizonZ(g.Cam.Dist)+0.05 || g.PickTile(int(sx), int(sy)) != i {
			continue
		}
		if d := (sx-float64(cx))*(sx-float64(cx)) + (sy-float64(cy))*(sy-float64(cy)); d < bd {
			best, bx, by, bd = i, int(sx), int(sy), d
		}
	}
	if best < 0 {
		t.Fatal("no tile faces the eye")
	}
	j.f.clickAt(t, j.a, bx, by)
	j.f.Cur.Mx, j.f.Cur.My = -50, -50
	j.update(1)
	if ng.Tile != best || g.Selected() != best {
		t.Fatalf("clicking tile %d's centre at (%d, %d) selected %d (globe %d)", best, bx, by, ng.Tile, g.Selected())
	}
}

// TestJourneyGlobeViewButtons: View cycles every mode, Legend and Grid
// toggle.
func TestJourneyGlobeViewButtons(t *testing.T) {
	t.Parallel()
	j := globeToLanding(t)
	ng := j.a.wiz.Draft()
	g := ng.Globe
	start := g.Mode()
	seen := map[globe.Mode]bool{start: true}
	for i := 1; i < int(globe.NumModes); i++ {
		j.clickID("land.mode")
		if want := (start + globe.Mode(i)) % globe.NumModes; g.Mode() != want {
			t.Fatalf("press %d: mode %v, want %v", i, g.Mode(), want)
		}
		seen[g.Mode()] = true
	}
	if len(seen) != int(globe.NumModes) {
		t.Fatalf("saw %d modes, want %d", len(seen), globe.NumModes)
	}
	j.clickID("land.mode")
	if g.Mode() != start {
		t.Fatalf("a full cycle ended on %v, want %v", g.Mode(), start)
	}
	j.clickID("land.legend")
	if !ng.Land.Legend {
		t.Fatal("Legend did not open")
	}
	j.clickID("land.legend")
	if ng.Land.Legend {
		t.Fatal("Legend did not close")
	}
	j.clickID("land.grid")
	if !g.Grid() {
		t.Fatal("Grid did not turn on")
	}
	j.clickID("land.grid")
	if g.Grid() {
		t.Fatal("Grid did not turn off")
	}
}

// TestJourneyGlobeSearchPicksAFeature: typing a feature's name in the
// search box and clicking the result selects the tile planet.Find returns.
func TestJourneyGlobeSearchPicksAFeature(t *testing.T) {
	t.Parallel()
	j := globeToLanding(t)
	ng := j.a.wiz.Draft()
	fs := ng.Planet.Features()
	if len(fs) == 0 {
		t.Skip("the tiny world has no named feature")
	}
	before := ng.Tile
	j.clickID("land.search")
	if !ng.Land.Search {
		t.Fatal("Search did not open the box")
	}
	j.clickID("land.search.field")
	j.typeText(fs[0].Name)
	j.update(2)
	hit := ng.Planet.Find(fs[0].Name)[0]
	if !j.clickID("land.search.result.0") {
		t.Fatal("the result row is disabled")
	}
	j.update(1)
	if ng.Tile != hit.Tile || ng.Globe.Selected() != hit.Tile {
		t.Fatalf("selected %d (globe %d), want %d (was %d)", ng.Tile, ng.Globe.Selected(), hit.Tile, before)
	}
}

// TestJourneyGlobeFilteredRandomSite: a biome ticked in the criteria
// panel is honoured by Select random site, and the panel's other switches
// reach the criteria.
func TestJourneyGlobeFilteredRandomSite(t *testing.T) {
	t.Parallel()
	j := globeToLanding(t)
	ng := j.a.wiz.Draft()
	p := ng.Planet
	want := biome.Of(0)
	var key string
	for i := 0; i < p.TileCount(); i++ {
		if ok, _ := p.ValidLanding(i); ok {
			key = biome.DefOf(p.Biome[i]).Key
			want = biome.Of(p.Biome[i])
			break
		}
	}
	if key == "" {
		t.Fatal("no landing tile")
	}
	j.clickID("land.filter")
	if !ng.Land.Filter {
		t.Fatal("the caret did not open the criteria")
	}
	j.clickID("land.filter.biome." + key)
	if ng.Land.Criteria.Biomes != want {
		t.Fatalf("biomes %v after ticking %s, want %v", ng.Land.Criteria.Biomes, key, want)
	}
	for i := 0; i < 3; i++ { // each press rolls another site; all honour the filter
		j.clickID("land.random")
		j.update(1)
		if ng.Tile < 0 || biome.Of(p.Biome[ng.Tile]) != want {
			t.Fatalf("roll %d: tile %d is %v, want only %s", i, ng.Tile, p.Biome[max(ng.Tile, 0)], key)
		}
	}
	for _, id := range []string{"river", "coast", "road", "caves", "stone"} {
		j.clickID("land.filter." + id)
	}
	j.clickID("land.filter.hills.0")
	j.clickID("land.filter.temp")
	lo := ng.Land.Criteria.TempMin
	j.clickID("land.filter.tempmin.inc")
	if !ng.Land.Criteria.TempOn || ng.Land.Criteria.TempMin <= lo {
		t.Fatalf("temperature on %v, min %d (was %d)", ng.Land.Criteria.TempOn, ng.Land.Criteria.TempMin, lo)
	}
	if !ng.Land.Criteria.Active() {
		t.Fatal("the switches left the criteria inactive")
	}
	j.clickID("land.filter.reset")
	if ng.Land.Criteria.Active() {
		t.Fatal("Reset left the criteria active")
	}
}

// TestJourneyGlobeFactionsRowFindsTheCapital: the Factions dialog lists the
// world's factions; a row click closes it and pulses the settlements.
func TestJourneyGlobeFactionsRowFindsTheCapital(t *testing.T) {
	t.Parallel()
	j := globeToLanding(t)
	ng := j.a.wiz.Draft()
	j.clickID("land.factions")
	if j.a.uiModalTop() != wizard.FacModalID {
		t.Fatalf("modal %q, want the Factions dialog", j.a.uiModalTop())
	}
	rows := wizard.FactionRows(ng.Planet.Factions)
	row := -1
	for i, r := range rows {
		if r.Capital >= 0 {
			row = i
			break
		}
	}
	if row < 0 {
		t.Fatal("no faction has a capital")
	}
	j.update(2)
	w, ok := j.a.uiSt().Find(wizard.FacTableID + ".body")
	if !ok {
		t.Fatal("no faction table")
	}
	s := float32(j.a.scale.S)
	x := (w.R.X + w.R.W/3) * s
	y := (w.R.Y + wizard.FacRowH*(float32(row)+0.5)) * s
	j.f.clickAt(t, j.a, int(x), int(y))
	j.f.Cur.Mx, j.f.Cur.My = -50, -50
	j.update(1)
	if j.a.uiModalTop() == wizard.FacModalID {
		t.Fatal("the row click left the dialog open")
	}
	if len(ng.Globe.Hl) == 0 {
		t.Fatal("the faction's settlements are not pulsing")
	}
}

// TestJourneyGlobeWorldSliders: the three world sliders take a click at
// either end of their track.
func TestJourneyGlobeWorldSliders(t *testing.T) {
	t.Parallel()
	j := globeToWorld(t)
	j.update(2)
	ng := j.a.wiz.Draft()
	s := float32(j.a.scale.S)
	for _, c := range []struct {
		id   string
		v    *int
		last int
	}{
		{"world.rain", &ng.World.Rain, 4},
		{"world.temp", &ng.World.Temp, 4},
		{"world.pop", &ng.World.Pop, 2},
	} {
		w, ok := j.a.uiSt().Find(c.id)
		if !ok {
			t.Fatalf("no slider %s", c.id)
		}
		tr := uikit.SliderTrackRect(w.R)
		cy := int((tr.Y + tr.H/2) * s)
		j.f.clickAt(t, j.a, int((tr.X+1)*s), cy)
		j.f.Cur.Mx, j.f.Cur.My = -50, -50
		if *c.v != 0 {
			t.Fatalf("%s after a click on the left end: %d, want 0", c.id, *c.v)
		}
		j.f.clickAt(t, j.a, int((tr.Right()-1)*s), cy)
		j.f.Cur.Mx, j.f.Cur.My = -50, -50
		if *c.v != c.last {
			t.Fatalf("%s after a click on the right end: %d, want %d", c.id, *c.v, c.last)
		}
	}
	j.clickID("world.generate")
	flowUntil(j, "the landing screen", func() bool { return j.a.nav.screen() == ports.ScreenLanding }, 4000)
	if p := ng.Planet.Params; p.Rain != 4 || p.Temp != 4 || p.Pop != 2 {
		t.Fatalf("the generated world has rain %d temp %d pop %d, want 4 4 2", p.Rain, p.Temp, p.Pop)
	}
}

// TestJourneyGlobeOptionsUnitAndColonies: Options › Gameplay flips the
// temperature unit, which the landing tile card then reads in, and raises
// the colony cap.
func TestJourneyGlobeOptionsUnitAndColonies(t *testing.T) {
	t.Parallel()
	j := newMenuJourney(t)
	j.clickID("menu.options")
	j.clickID("options.tab.gameplay")
	n := j.a.settings.Gameplay.Colonies()
	j.clickID("options.maxcolonies.inc")
	if got := j.a.settings.Gameplay.Colonies(); got != n+1 {
		t.Fatalf("max colonies %d -> %d, want +1", n, got)
	}
	j.clickID("options.tempunit")
	if j.a.settings.Gameplay.TempUnit != units.UnitF {
		t.Fatalf("unit %q after the click, want Fahrenheit", j.a.settings.Gameplay.TempUnit)
	}
	j.key("Escape", 0)
	if j.a.nav.screen() != ports.ScreenMainMenu {
		t.Fatalf("Escape on Options opened %s", j.a.nav.screen())
	}
	j.clickID("menu.newgame")
	j.clickID("wizard.next")
	j.clickID("wizard.next")
	j.a.wiz.Draft().World.SeedText, j.a.wiz.Draft().World.Coverage = tinySeed, 5
	j.clickID("world.generate")
	flowUntil(j, "the landing screen", func() bool { return j.a.nav.screen() == ports.ScreenLanding }, 4000)
	j.clickID("land.random")
	j.update(2)
	var temps []string
	for _, r := range j.a.wiz.Draft().Info.Card().Rows {
		if r.Label == "Avg temperature" || r.Label == "Summer / winter" {
			temps = append(temps, r.Value)
		}
	}
	if len(temps) != 2 || !strings.Contains(temps[0], "°F") || !strings.Contains(temps[1], "°F") {
		t.Fatalf("the tile card reads %v, want degrees Fahrenheit", temps)
	}
}
