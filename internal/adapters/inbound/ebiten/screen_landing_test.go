package ebiten

import (
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/planet"
	"rimworld-go/internal/domain/world/site"
	"rimworld-go/internal/domain/world/temperature"
	"rimworld-go/internal/platform/input"
)

// landingApp is a menu App on Select Landing Site with a tiny world.
func landingApp(t *testing.T) (*App, *fakeInput) {
	t.Helper()
	a, f, _ := ngTest(t, nil)
	toLanding(t, a)
	f.Frame(t, a)
	return a, f
}

// nextState is the Next button's disabled reason ("" enabled).
func nextState(t *testing.T, a *App) string {
	t.Helper()
	w, ok := a.uiSt().Find("wizard.next")
	if !ok {
		t.Fatal("no Next button")
	}
	if !w.Disabled {
		return ""
	}
	return w.Reason
}

// selectTile selects tile i on the globe as a click would.
func selectTile(t *testing.T, a *App, f *fakeInput, i int) {
	t.Helper()
	a.ng.globe.Select(i)
	f.Frame(t, a)
}

func TestLandingInvalidTilesBlockNext(t *testing.T) {
	t.Parallel()
	a, f := landingApp(t)
	p := a.ng.planet
	if r := nextState(t, a); r != "Choose a landing site first." {
		t.Fatalf("no tile: reason %q", r)
	}
	var water, settled, adjacent, valid = -1, -1, -1, -1
	for i := 0; i < p.TileCount(); i++ {
		ok, reason := p.ValidLanding(i)
		switch {
		case ok && valid < 0:
			valid = i
		case !ok && reason == planet.ReasonWater && water < 0:
			water = i
		case !ok && strings.HasPrefix(reason, "Occupied by") && settled < 0:
			settled = i
		case !ok && strings.HasPrefix(reason, "Too close to") && adjacent < 0:
			adjacent = i
		}
	}
	for _, c := range []struct {
		name string
		tile int
		want string
	}{
		{"water", water, "Can't land on water."},
		{"settlement", settled, "Occupied by "},
		{"adjacent", adjacent, "Too close to "},
	} {
		if c.tile < 0 {
			t.Fatalf("the tiny world has no %s tile", c.name)
		}
		selectTile(t, a, f, c.tile)
		if r := nextState(t, a); !strings.HasPrefix(r, c.want) {
			t.Errorf("%s: Next reason %q, want %q...", c.name, r, c.want)
		}
		before := a.nav.screen()
		click(t, a, "wizard.next")
		if a.nav.screen() != before {
			t.Errorf("%s: a disabled Next must not navigate", c.name)
		}
	}
	// Outside the coverage: a click on the uncovered sphere selects nothing.
	a.ng.outside = true
	selectTile(t, a, f, -1)
	if r := nextState(t, a); r != planet.ReasonOutside {
		t.Errorf("uncovered: reason %q, want %q", r, planet.ReasonOutside)
	}
	selectTile(t, a, f, valid)
	if r := nextState(t, a); r != "" {
		t.Fatalf("a valid tile enables Next, got %q", r)
	}
	click(t, a, "wizard.next")
	if a.nav.screen() != screenPawns {
		t.Fatalf("screen = %s, want pawns", a.nav.screen())
	}
}

func TestRandomSiteValidAndDeterministic(t *testing.T) {
	t.Parallel()
	run := func() []int {
		a, _ := landingApp(t)
		var seq []int
		for i := 0; i < 6; i++ {
			click(t, a, "land.random")
			tile := a.ng.tile
			if ok, why := a.ng.planet.ValidLanding(tile); !ok {
				t.Fatalf("press %d chose tile %d: %s", i, tile, why)
			}
			if a.ng.globe.Selected() != tile {
				t.Fatalf("the globe must select the tile: %d vs %d", a.ng.globe.Selected(), tile)
			}
			seq = append(seq, tile)
		}
		return seq
	}
	a, b := run(), run()
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("same planet, same presses must pick the same sites: %v vs %v", a, b)
		}
	}
	distinct := map[int]bool{}
	for _, v := range a {
		distinct[v] = true
	}
	if len(distinct) < 2 {
		t.Fatalf("pressing again should offer other sites: %v", a)
	}
}

func TestAdvancedSeasonAuto(t *testing.T) {
	t.Parallel()
	temperate := temperature.Temperate
	if got := autoSeason(temperate); got != game.SeasonSpring {
		t.Errorf("temperate: %v, want Spring (9 C and rising)", got)
	}
	south := temperate
	south.PeakDay = 45
	if got := autoSeason(south); got != game.SeasonSpring {
		t.Errorf("southern: %v, want Spring (seasons are local)", got)
	}
	cold := temperature.Climate{YearMean: -100, Amplitude: 150, PeakDay: 15, DaySwing: 50}
	if got := autoSeason(cold); got != game.SeasonSummer {
		t.Errorf("cold: %v, want the warmest season (Summer)", got)
	}
	chilly := temperature.Climate{YearMean: 40, Amplitude: 150, PeakDay: 15, DaySwing: 50}
	if got := autoSeason(chilly); got != game.SeasonSummer {
		t.Errorf("chilly spring (4 C): %v, want Summer, the warmest", got)
	}
	if seasonMid(temperate, game.SeasonSummer) != 15 || seasonMid(south, game.SeasonSummer) != 45 {
		t.Error("summer is centred on the peak day")
	}
	if dayMean(temperate, seasonMid(temperate, game.SeasonWinter)) >= dayMean(temperate, seasonMid(temperate, game.SeasonSummer)) {
		t.Error("winter is colder than summer")
	}

	// The dialog: Auto resolves at the chosen tile, a pick overrides it.
	a, f := landingApp(t)
	click(t, a, "land.random")
	ng := a.ng
	if ng.season != -1 {
		t.Fatalf("a fresh draft is Auto, got %d", ng.season)
	}
	click(t, a, "land.advanced")
	f.Frame(t, a)
	if a.uiModalTop() != "land.advanced" {
		t.Fatalf("modal = %q", a.uiModalTop())
	}
	if ng.resolvedSeason() != autoSeason(ng.planet.Site(ng.tile).Climate) {
		t.Fatal("Auto resolves from the tile's climate")
	}
	press(t, a, f, "land.season.4") // Winter
	if ng.season != 3 || ng.resolvedSeason() != game.SeasonWinter {
		t.Fatalf("season = %d resolved %v, want Winter", ng.season, ng.resolvedSeason())
	}
	press(t, a, f, "land.season.0") // Auto again
	if ng.season != -1 {
		t.Fatalf("season = %d, want Auto", ng.season)
	}
	click(t, a, "modal.land.advanced.reset")
	if ng.season != -1 || ng.mapSize != defaultMapSideEnabled() {
		t.Fatal("Reset puts the advanced options back")
	}
}

func TestMapSizeOptionsGated(t *testing.T) {
	t.Parallel()
	opts := mapSizeOptions()
	if len(opts) < 9 {
		t.Fatalf("%d options", len(opts))
	}
	enabled := 0
	for _, o := range opts {
		if o.Enabled != (o.Side <= game.MaxEnabledSide) {
			t.Errorf("%d: enabled=%v with the gate at %d", o.Side, o.Enabled, game.MaxEnabledSide)
		}
		if !o.Enabled && o.Reason != "Not available yet" {
			t.Errorf("%d: reason %q", o.Side, o.Reason)
		}
		if o.Enabled {
			enabled++
		}
		if o.Large != (o.Side >= 350) && o.Side != preReleaseSide {
			t.Errorf("%d: large=%v", o.Side, o.Large)
		}
	}
	if enabled == 0 {
		t.Fatal("at least one size is playable")
	}
	if d := defaultMapSideEnabled(); d > game.MaxEnabledSide {
		t.Fatalf("default %d is over the gate %d", d, game.MaxEnabledSide)
	}

	a, f := landingApp(t)
	click(t, a, "land.advanced")
	f.Frame(t, a)
	before := ng(a).mapSize
	for i, o := range opts {
		if !o.Enabled {
			click(t, a, optID("land.mapsize", i))
			if ng(a).mapSize != before {
				t.Fatalf("a disabled size (%d) must not be selectable", o.Side)
			}
			if !strings.Contains(a.hint, "Not available yet") {
				t.Fatalf("hint %q should say why", a.hint)
			}
			break
		}
	}
	for i, o := range opts {
		if o.Enabled && o.Side != before {
			press(t, a, f, optID("land.mapsize", i))
			if ng(a).mapSize != o.Side {
				t.Fatalf("map size = %d, want %d", ng(a).mapSize, o.Side)
			}
		}
	}
	// The size label is just the size; what a size needs is mapSizeNote's
	// one rule (ram_estimate.go), shown beside it.
	if mapSizeNote(400, 0) != "Large map: needs a capable computer" {
		t.Error("large sizes with unknown hardware say they need a capable computer")
	}
}

func ng(a *App) *newGameState { return a.draft() }

func TestTileInfoFields(t *testing.T) {
	t.Parallel()
	a, _ := landingApp(t)
	p := a.ng.planet
	land, ocean := -1, -1
	for i := 0; i < p.TileCount(); i++ {
		if ok, _ := p.ValidLanding(i); ok && land < 0 {
			land = i
		}
		if p.Water[i] == planet.Ocean && ocean < 0 {
			ocean = i
		}
	}
	ti := buildTileInfo(p, land, 120, 0)
	want := []string{"Terrain", "Coordinates", "Elevation", "Avg temperature", "Summer / winter", "Rainfall",
		"Growing period", "Stone types", "Water", "Caves", "Roads", "Nearest settlement", "Movement difficulty", "Forage", "Wildlife"}
	got := map[string]string{}
	for _, r := range ti.rows {
		got[r.label] = r.value
		if r.value == "" {
			t.Errorf("%s has no value", r.label)
		}
	}
	for _, w := range want {
		if _, ok := got[w]; !ok {
			t.Errorf("the panel lacks %q", w)
		}
	}
	if ti.name != biome.DefOf(p.Biome[land]).Name || ti.blurb == "" || ti.biomeTip == "" {
		t.Errorf("header: %q %q", ti.name, ti.blurb)
	}
	if !strings.Contains(got["Coordinates"], "°") || !strings.HasSuffix(got["Rainfall"], "mm/year") || !strings.HasPrefix(got["Movement difficulty"], "x") {
		t.Errorf("formats: %v", got)
	}
	if !strings.Contains(got["Growing period"], "of 60 days") && got["Growing period"] != "Year-round" {
		t.Errorf("growing period %q", got["Growing period"])
	}
	if ti.invalid != "" {
		t.Errorf("a valid tile has no invalid reason: %q", ti.invalid)
	}
	if o := buildTileInfo(p, ocean, 120, 0); o.name != "Ocean" || o.invalid != planet.ReasonWater || len(o.rows) >= len(want) {
		t.Errorf("ocean info: %+v", o)
	}
	if !buildTileInfo(p, -5, 120, 0).outside {
		t.Error("a missing tile is outside the world")
	}
	// Warnings. With unknown hardware (total 0) a 350+ map gets the gentle
	// note and smaller ones none; the RAM rule is in ram_estimate_test.go.
	if w := buildTileInfo(p, land, 400, 0).warnings; !containsStr(w, "Large map: needs a capable computer") {
		t.Errorf("warnings at 400: %v", w)
	}
	if w := buildTileInfo(p, land, 300, 0).warnings; containsStr(w, "Large map: needs a capable computer") {
		t.Errorf("warnings at 300: %v", w)
	}
	if coordText(-342, -1217) != "34.2°S 121.7°W" || coordText(342, 1217) != "34.2°N 121.7°E" {
		t.Errorf("coords %q", coordText(-342, -1217))
	}
	if movementDifficulty(site.Mountainous, biome.IceSheet) > 3 || movementDifficulty(site.Flat, biome.TemperateForest) != 1 {
		t.Error("movement difficulty runs from x1.0 to x3.0")
	}
	if forageLevel(biome.TemperateForest) != "High" || forageLevel(biome.IceSheet) != "Low" || wildlifeLevel(biome.ExtremeDesert) != "Sparse" || wildlifeLevel(biome.TemperateForest) != "Abundant" {
		t.Error("forage and wildlife levels")
	}
	// The panel tracks the selection and the hover.
	f := fakeOn(a)
	selectTile(t, a, f, land)
	if c := a.ng.info; c.tile != land || c.planet != p {
		t.Errorf("info cache %+v, want tile %d", c.tile, land)
	}
}

func containsStr(l []string, s string) bool {
	for _, v := range l {
		if v == s {
			return true
		}
	}
	return false
}

func TestLandingGlobeClickSelectsTile(t *testing.T) {
	t.Parallel()
	a, f := landingApp(t)
	tile := a.ng.planet.RandomLanding(3)
	a.ng.globe.Center(tile)
	for i := 0; i < 60; i++ {
		f.Frame(t, a)
	}
	geo := landRects(a.sw, a.sh)
	cx, cy := geo.globe.Center()
	f.Cur.Mx, f.Cur.My = int(cx), int(cy)
	f.Frame(t, a)
	want := a.ng.globe.PickTile(int(cx), int(cy))
	f.Click(t, a, mbLeft, 0)
	if want < 0 || a.ng.tile != want {
		t.Fatalf("clicked tile %d, the globe picked %d", a.ng.tile, want)
	}
	// A drag turns the globe and does not select.
	a.ng.globe.Select(-1)
	f.Frame(t, a)
	f.Cur.Press[mbLeft], f.Cur.Down[mbLeft] = true, true
	f.Frame(t, a)
	f.Cur.Mx += 80
	f.Frame(t, a)
	f.Cur.Down[mbLeft], f.Cur.Release[mbLeft] = false, true
	f.Frame(t, a)
	if a.ng.tile != -1 {
		t.Fatalf("a drag selected tile %d", a.ng.tile)
	}
	// The panel and the bar take the pointer, not the globe.
	f.Cur.Mx, f.Cur.My = int(geo.panel.X)+20, int(geo.panel.Y)+200
	f.Click(t, a, mbLeft, 0)
	if a.ng.tile != -1 {
		t.Fatal("a click on the info panel must not select a tile")
	}
}

func TestLandingBackKeepsPlanetAndTile(t *testing.T) {
	t.Parallel()
	a, f := landingApp(t)
	click(t, a, "land.random")
	tile, p := a.ng.tile, a.ng.planet
	click(t, a, "wizard.back")
	f.Frame(t, a)
	if a.nav.screen() != screenWorldGen {
		t.Fatalf("screen = %s", a.nav.screen())
	}
	click(t, a, "world.generate") // Next: the planet is unchanged
	f.Frame(t, a)
	f.Frame(t, a)
	if a.nav.screen() != screenLanding || a.ng.planet != p || a.ng.tile != tile || a.ng.globe.Selected() != tile {
		t.Fatalf("screen=%s tile=%d want %d", a.nav.screen(), a.ng.tile, tile)
	}
}

func TestLandingLegendModesAndDrawing(t *testing.T) {
	t.Parallel()
	a, f := landingApp(t)
	click(t, a, "land.random")
	img := ebiten.NewImage(a.sw, a.sh)
	seen := map[globeMode]bool{}
	for i := 0; i < int(numGlobeModes)+1; i++ {
		seen[a.ng.globe.Mode()] = true
		f.Frame(t, a)
		a.Draw(img)
		click(t, a, "land.mode")
	}
	if len(seen) != int(numGlobeModes) || a.ng.globe.Mode() != globeTemperature {
		t.Fatalf("modes seen %v, now %v", seen, a.ng.globe.Mode())
	}
	click(t, a, "land.legend")
	if !a.ng.legend {
		t.Fatal("the legend opens")
	}
	for m := globeMode(0); m < numGlobeModes; m++ {
		a.ng.globe.SetMode(m)
		f.Frame(t, a)
		a.Draw(img)
	}
	// Both window sizes draw.
	for _, size := range [][2]int{{1280, 720}, {1920, 1080}} {
		sizeTo(t, a, f, size[0], size[1])
		a.Draw(ebiten.NewImage(size[0], size[1]))
	}
	// Enter presses Next when it is enabled.
	f.Key(t, a, "Enter", 0)
	if a.nav.screen() != screenPawns {
		t.Fatalf("Enter on a valid site goes on, screen=%s", a.nav.screen())
	}
	_ = input.Accept
	_ = clock.TicksPerDay
}

func TestFactionsTableSortAndCentre(t *testing.T) {
	t.Parallel()
	a, f := landingApp(t)
	press(t, a, f, "land.factions")
	f.Frame(t, a)
	if a.uiModalTop() != facModalID {
		t.Fatalf("modal = %q", a.uiModalTop())
	}
	rows := factionRows(a.ng.planet.Factions)
	if len(rows) != len(a.ng.planet.Factions)+1 || !rows[len(rows)-1].Swarm {
		t.Fatalf("rows = %d, want the factions and the Swarm", len(rows))
	}
	if rows[len(rows)-1].Leader != "Hive swarms: always hostile, no settlements" {
		t.Fatalf("swarm row %q", rows[len(rows)-1].Leader)
	}
	first := rows[0]
	if !strings.Contains(first.Leader, " ") || first.Settlements != len(first.Tiles) {
		t.Fatalf("row %+v", first)
	}
	// Sorting by Name: the first row of the display is the alphabetical first.
	press(t, a, f, facTableID+".col.name")
	f.Frame(t, a)
	col, desc := a.uiTableSort(facTableID)
	if col != 1 || desc {
		t.Fatalf("sort = %d/%v, want the Name column ascending", col, desc)
	}
	order := a.tableOrder(facTableID, a.factionColumns(rows), len(rows))
	for i := 1; i < len(order); i++ {
		if strings.ToLower(rows[order[i-1]].Name) > strings.ToLower(rows[order[i]].Name) {
			t.Fatalf("names are not sorted: %v", order)
		}
	}
	press(t, a, f, facTableID+".col.name")
	f.Frame(t, a)
	if _, desc := a.uiTableSort(facTableID); !desc {
		t.Fatal("a second click reverses")
	}
	// A click on a row closes the dialog and shows the faction's capital.
	target := rows[order[len(order)-1]]
	_ = target
	d := a.tableOrder(facTableID, a.factionColumns(rows), len(rows))
	press(t, a, f, optID(facTableID+".body", 0))
	f.Frame(t, a)
	if a.uiModalOpen() {
		t.Fatal("picking a faction closes the dialog")
	}
	picked := rows[d[0]]
	if picked.Swarm {
		t.Skip("the Swarm row sorted first; it has no capital")
	}
	g := a.ng.globe
	if len(g.Hl) != len(picked.Tiles) || g.HlLeft <= 0 || g.HlLeft > facHighlight {
		t.Fatalf("highlight %v for %d settlements, left %d", g.Hl, len(picked.Tiles), g.HlLeft)
	}
	if picked.Capital < 0 || g.Hl[0] != picked.Capital {
		t.Fatalf("capital %d, highlight %v", picked.Capital, g.Hl)
	}
}

func TestFactionRowsLabelsAndRelations(t *testing.T) {
	t.Parallel()
	if relationLabel(1) != "Hostile" || relationLabel(2) != "Friendly" || relationLabel(0) != "Neutral" {
		t.Error("relation labels")
	}
	if relationColor(1) != uiDanger || relationColor(2) != uiGood || relationColor(0) != uiTextDim {
		t.Error("relation colours: Hostile danger, Friendly good, Neutral dim")
	}
	if goodwillText(12) != "+12" || goodwillText(-40) != "-40" {
		t.Error("goodwill text")
	}
}
