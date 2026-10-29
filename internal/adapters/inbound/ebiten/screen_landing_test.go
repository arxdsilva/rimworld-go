package ebiten

import (
	globepkg "rimworld-go/internal/adapters/inbound/ebiten/globe"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	optionspkg "rimworld-go/internal/adapters/inbound/ebiten/options"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/adapters/inbound/ebiten/wizard"
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
	a.ng.Globe.Select(i)
	f.Frame(t, a)
}

func TestLandingInvalidTilesBlockNext(t *testing.T) {
	t.Parallel()
	a, f := landingApp(t)
	p := a.ng.Planet
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
	a.ng.Outside = true
	selectTile(t, a, f, -1)
	if r := nextState(t, a); r != planet.ReasonOutside {
		t.Errorf("uncovered: reason %q, want %q", r, planet.ReasonOutside)
	}
	selectTile(t, a, f, valid)
	if r := nextState(t, a); r != "" {
		t.Fatalf("a valid tile enables Next, got %q", r)
	}
	click(t, a, "wizard.next")
	if a.nav.screen() != ports.ScreenPawns {
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
			tile := a.ng.Tile
			if ok, why := a.ng.Planet.ValidLanding(tile); !ok {
				t.Fatalf("press %d chose tile %d: %s", i, tile, why)
			}
			if a.ng.Globe.Selected() != tile {
				t.Fatalf("the globe must select the tile: %d vs %d", a.ng.Globe.Selected(), tile)
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
	if got := wizard.AutoSeason(temperate); got != game.SeasonSpring {
		t.Errorf("temperate: %v, want Spring (9 C and rising)", got)
	}
	south := temperate
	south.PeakDay = 45
	if got := wizard.AutoSeason(south); got != game.SeasonSpring {
		t.Errorf("southern: %v, want Spring (seasons are local)", got)
	}
	cold := temperature.Climate{YearMean: -100, Amplitude: 150, PeakDay: 15, DaySwing: 50}
	if got := wizard.AutoSeason(cold); got != game.SeasonSummer {
		t.Errorf("cold: %v, want the warmest season (Summer)", got)
	}
	chilly := temperature.Climate{YearMean: 40, Amplitude: 150, PeakDay: 15, DaySwing: 50}
	if got := wizard.AutoSeason(chilly); got != game.SeasonSummer {
		t.Errorf("chilly spring (4 C): %v, want Summer, the warmest", got)
	}
	if wizard.SeasonMid(temperate, game.SeasonSummer) != 15 || wizard.SeasonMid(south, game.SeasonSummer) != 45 {
		t.Error("summer is centred on the peak day")
	}
	if wizard.DayMean(temperate, wizard.SeasonMid(temperate, game.SeasonWinter)) >= wizard.DayMean(temperate, wizard.SeasonMid(temperate, game.SeasonSummer)) {
		t.Error("winter is colder than summer")
	}

	// The dialog: Auto resolves at the chosen tile, a pick overrides it.
	a, f := landingApp(t)
	click(t, a, "land.random")
	ng := a.ng
	if ng.Season != -1 {
		t.Fatalf("a fresh draft is Auto, got %d", ng.Season)
	}
	click(t, a, "land.advanced")
	f.Frame(t, a)
	if a.uiModalTop() != "land.advanced" {
		t.Fatalf("modal = %q", a.uiModalTop())
	}
	if ng.ResolvedSeason() != wizard.AutoSeason(ng.Planet.Site(ng.Tile).Climate) {
		t.Fatal("Auto resolves from the tile's climate")
	}
	press(t, a, f, "land.season.4") // Winter
	if ng.Season != 3 || ng.ResolvedSeason() != game.SeasonWinter {
		t.Fatalf("season = %d resolved %v, want Winter", ng.Season, ng.ResolvedSeason())
	}
	press(t, a, f, "land.season.0") // Auto again
	if ng.Season != -1 {
		t.Fatalf("season = %d, want Auto", ng.Season)
	}
	click(t, a, "modal.land.advanced.reset")
	if ng.Season != -1 || ng.MapSize != wizard.DefaultMapSideEnabled() {
		t.Fatal("Reset puts the advanced options back")
	}
}

func TestMapSizeOptionsGated(t *testing.T) {
	t.Parallel()
	opts := wizard.MapSizeOptions()
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
		if o.Large != (o.Side >= 350) && o.Side != wizard.PreReleaseSide {
			t.Errorf("%d: large=%v", o.Side, o.Large)
		}
	}
	if enabled == 0 {
		t.Fatal("at least one size is playable")
	}
	if d := wizard.DefaultMapSideEnabled(); d > game.MaxEnabledSide {
		t.Fatalf("default %d is over the gate %d", d, game.MaxEnabledSide)
	}

	a, f := landingApp(t)
	click(t, a, "land.advanced")
	f.Frame(t, a)
	before := ng(a).MapSize
	for i, o := range opts {
		if !o.Enabled {
			click(t, a, uikit.OptID("land.mapsize", i))
			if ng(a).MapSize != before {
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
			press(t, a, f, uikit.OptID("land.mapsize", i))
			if ng(a).MapSize != o.Side {
				t.Fatalf("map size = %d, want %d", ng(a).MapSize, o.Side)
			}
		}
	}
	// The size label is just the size; what a size needs is mapSizeNote's
	// one rule (ram_estimate.go), shown beside it.
	if optionspkg.MapSizeNote(400, 0) != "Large map: needs a capable computer" {
		t.Error("large sizes with unknown hardware say they need a capable computer")
	}
}

func ng(a *App) *wizard.Draft { return a.wiz.Draft() }

func TestTileInfoFields(t *testing.T) {
	t.Parallel()
	a, _ := landingApp(t)
	p := a.ng.Planet
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
	for _, r := range ti.Rows {
		got[r.Label] = r.Value
		if r.Value == "" {
			t.Errorf("%s has no value", r.Label)
		}
	}
	for _, w := range want {
		if _, ok := got[w]; !ok {
			t.Errorf("the panel lacks %q", w)
		}
	}
	if ti.Name != biome.DefOf(p.Biome[land]).Name || ti.Blurb == "" || ti.BiomeTip == "" {
		t.Errorf("header: %q %q", ti.Name, ti.Blurb)
	}
	if !strings.Contains(got["Coordinates"], "°") || !strings.HasSuffix(got["Rainfall"], "mm/year") || !strings.HasPrefix(got["Movement difficulty"], "x") {
		t.Errorf("formats: %v", got)
	}
	if !strings.Contains(got["Growing period"], "of 60 days") && got["Growing period"] != "Year-round" {
		t.Errorf("growing period %q", got["Growing period"])
	}
	if ti.Invalid != "" {
		t.Errorf("a valid tile has no invalid reason: %q", ti.Invalid)
	}
	if o := buildTileInfo(p, ocean, 120, 0); o.Name != "Ocean" || o.Invalid != planet.ReasonWater || len(o.Rows) >= len(want) {
		t.Errorf("ocean info: %+v", o)
	}
	if !buildTileInfo(p, -5, 120, 0).Outside {
		t.Error("a missing tile is outside the world")
	}
	// Warnings. With unknown hardware (total 0) a 350+ map gets the gentle
	// note and smaller ones none; the RAM rule is in ram_estimate_test.go.
	if w := buildTileInfo(p, land, 400, 0).Warnings; !containsStr(w, "Large map: needs a capable computer") {
		t.Errorf("warnings at 400: %v", w)
	}
	if w := buildTileInfo(p, land, 300, 0).Warnings; containsStr(w, "Large map: needs a capable computer") {
		t.Errorf("warnings at 300: %v", w)
	}
	if wizard.CoordText(-342, -1217) != "34.2°S 121.7°W" || wizard.CoordText(342, 1217) != "34.2°N 121.7°E" {
		t.Errorf("coords %q", wizard.CoordText(-342, -1217))
	}
	if wizard.MovementDifficulty(site.Mountainous, biome.IceSheet) > 3 || wizard.MovementDifficulty(site.Flat, biome.TemperateForest) != 1 {
		t.Error("movement difficulty runs from x1.0 to x3.0")
	}
	if wizard.ForageLevel(biome.TemperateForest) != "High" || wizard.ForageLevel(biome.IceSheet) != "Low" || wizard.WildlifeLevel(biome.ExtremeDesert) != "Sparse" || wizard.WildlifeLevel(biome.TemperateForest) != "Abundant" {
		t.Error("forage and wildlife levels")
	}
	// The panel tracks the selection and the hover.
	f := fakeOn(a)
	selectTile(t, a, f, land)
	if c := a.ng.Info; c.Tile != land || c.Planet != p {
		t.Errorf("info cache %+v, want tile %d", c.Tile, land)
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
	tile := a.ng.Planet.RandomLanding(3)
	a.ng.Globe.Center(tile)
	for i := 0; i < 60; i++ {
		f.Frame(t, a)
	}
	geo := wizard.LandRects(a.sw, a.sh)
	cx, cy := geo.Globe.Center()
	f.Cur.Mx, f.Cur.My = int(cx), int(cy)
	f.Frame(t, a)
	want := a.ng.Globe.PickTile(int(cx), int(cy))
	f.Click(t, a, inputframe.Left, 0)
	if want < 0 || a.ng.Tile != want {
		t.Fatalf("clicked tile %d, the globe picked %d", a.ng.Tile, want)
	}
	// A drag turns the globe and does not select.
	a.ng.Globe.Select(-1)
	f.Frame(t, a)
	f.Cur.Press[inputframe.Left], f.Cur.Down[inputframe.Left] = true, true
	f.Frame(t, a)
	f.Cur.Mx += 80
	f.Frame(t, a)
	f.Cur.Down[inputframe.Left], f.Cur.Release[inputframe.Left] = false, true
	f.Frame(t, a)
	if a.ng.Tile != -1 {
		t.Fatalf("a drag selected tile %d", a.ng.Tile)
	}
	// The panel and the bar take the pointer, not the globe.
	f.Cur.Mx, f.Cur.My = int(geo.Panel.X)+20, int(geo.Panel.Y)+200
	f.Click(t, a, inputframe.Left, 0)
	if a.ng.Tile != -1 {
		t.Fatal("a click on the info panel must not select a tile")
	}
}

func TestLandingBackKeepsPlanetAndTile(t *testing.T) {
	t.Parallel()
	a, f := landingApp(t)
	click(t, a, "land.random")
	tile, p := a.ng.Tile, a.ng.Planet
	click(t, a, "wizard.back")
	f.Frame(t, a)
	if a.nav.screen() != ports.ScreenWorldGen {
		t.Fatalf("screen = %s", a.nav.screen())
	}
	click(t, a, "world.generate") // Next: the planet is unchanged
	f.Frame(t, a)
	f.Frame(t, a)
	if a.nav.screen() != ports.ScreenLanding || a.ng.Planet != p || a.ng.Tile != tile || a.ng.Globe.Selected() != tile {
		t.Fatalf("screen=%s tile=%d want %d", a.nav.screen(), a.ng.Tile, tile)
	}
}

func TestLandingLegendModesAndDrawing(t *testing.T) {
	t.Parallel()
	a, f := landingApp(t)
	click(t, a, "land.random")
	img := ebiten.NewImage(a.sw, a.sh)
	seen := map[globepkg.Mode]bool{}
	for i := 0; i < int(globepkg.NumModes)+1; i++ {
		seen[a.ng.Globe.Mode()] = true
		f.Frame(t, a)
		a.Draw(img)
		click(t, a, "land.mode")
	}
	if len(seen) != int(globepkg.NumModes) || a.ng.Globe.Mode() != globepkg.ModeTemperature {
		t.Fatalf("modes seen %v, now %v", seen, a.ng.Globe.Mode())
	}
	click(t, a, "land.legend")
	if !a.ng.Legend {
		t.Fatal("the legend opens")
	}
	for m := globepkg.Mode(0); m < globepkg.NumModes; m++ {
		a.ng.Globe.SetMode(m)
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
	if a.nav.screen() != ports.ScreenPawns {
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
	if a.uiModalTop() != wizard.FacModalID {
		t.Fatalf("modal = %q", a.uiModalTop())
	}
	rows := wizard.FactionRows(a.ng.Planet.Factions)
	if len(rows) != len(a.ng.Planet.Factions)+1 || !rows[len(rows)-1].Swarm {
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
	press(t, a, f, wizard.FacTableID+".col.name")
	f.Frame(t, a)
	col, desc := a.uiSt().TableSort(wizard.FacTableID)
	if col != 1 || desc {
		t.Fatalf("sort = %d/%v, want the Name column ascending", col, desc)
	}
	order := a.uiSt().TableOrder(wizard.FacTableID, a.wiz.FactionColumns(rows), len(rows))
	for i := 1; i < len(order); i++ {
		if strings.ToLower(rows[order[i-1]].Name) > strings.ToLower(rows[order[i]].Name) {
			t.Fatalf("names are not sorted: %v", order)
		}
	}
	press(t, a, f, wizard.FacTableID+".col.name")
	f.Frame(t, a)
	if _, desc := a.uiSt().TableSort(wizard.FacTableID); !desc {
		t.Fatal("a second click reverses")
	}
	// A click on a row closes the dialog and shows the faction's capital.
	target := rows[order[len(order)-1]]
	_ = target
	d := a.uiSt().TableOrder(wizard.FacTableID, a.wiz.FactionColumns(rows), len(rows))
	press(t, a, f, uikit.OptID(wizard.FacTableID+".body", 0))
	f.Frame(t, a)
	if a.uiSt().ModalOpen() {
		t.Fatal("picking a faction closes the dialog")
	}
	picked := rows[d[0]]
	if picked.Swarm {
		t.Skip("the Swarm row sorted first; it has no capital")
	}
	g := a.ng.Globe
	if len(g.Hl) != len(picked.Tiles) || g.HlLeft <= 0 || g.HlLeft > wizard.FacHighlight {
		t.Fatalf("highlight %v for %d settlements, left %d", g.Hl, len(picked.Tiles), g.HlLeft)
	}
	if picked.Capital < 0 || g.Hl[0] != picked.Capital {
		t.Fatalf("capital %d, highlight %v", picked.Capital, g.Hl)
	}
}

func TestFactionRowsLabelsAndRelations(t *testing.T) {
	t.Parallel()
	if wizard.RelationLabel(1) != "Hostile" || wizard.RelationLabel(2) != "Friendly" || wizard.RelationLabel(0) != "Neutral" {
		t.Error("relation labels")
	}
	if wizard.RelationColor(1) != uikit.Danger || wizard.RelationColor(2) != uikit.Good || wizard.RelationColor(0) != uikit.TextDim {
		t.Error("relation colours: Hostile danger, Friendly good, Neutral dim")
	}
	if wizard.GoodwillText(12) != "+12" || wizard.GoodwillText(-40) != "-40" {
		t.Error("goodwill text")
	}
}
