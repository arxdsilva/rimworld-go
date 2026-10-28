package ebiten

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// PLAY-020 ZONEUI: selecting a zone and its pane (zone_select.go).

// zoneSim is a stub colony whose air temperature and zone rule the test
// sets.
type zoneSim struct {
	*stubSim
	temp int                 // TemperatureAt, tenths of a degree C
	deny func(x, y int) bool // CanZoneAt is false where it returns true
}

func (s *zoneSim) TemperatureAt(x, y int) int { return s.temp }
func (s *zoneSim) CanZoneAt(kind string, x, y int) bool {
	return s.stubSim.CanZoneAt(kind, x, y) && (s.deny == nil || !s.deny(x, y))
}

// zoneApp is a paused colony over a stub holding zones, with the camera
// placed so tile (x, y) sits at screen (64x, 64y+32). Tiles in columns 8..16
// and rows 1..5 are open map, clear of the pane and the command grid.
func zoneApp(t *testing.T, zones ...game.ZoneView) (*App, *zoneSim, *fakeInput) {
	t.Helper()
	st := &stubSim{w: 24, h: 24, world: game.WorldView{Zones: zones}}
	s := &zoneSim{stubSim: st, temp: 150}
	a, f := colonyApp(t, st)
	a.renderer.game = s
	a.renderer.cam.offsetX, a.renderer.cam.offsetY = 0, -32
	f.frame(t, a)
	return a, s, f
}

// growZone is a growing zone of crop over the rectangle.
func growZone(id uint16, crop string, minX, minY, maxX, maxY int) game.ZoneView {
	z := rectZone(id, game.ZoneGrow, minX, minY, maxX, maxY)
	z.Crop = crop
	return z
}

func TestZoneIsTheLastSelectionCandidate(t *testing.T) {
	t.Parallel()
	w := game.WorldView{
		Zones:  []game.ZoneView{growZone(3, "potato", 4, 4, 7, 7)},
		Plants: []game.PlantView{{ID: 41, X: 5, Y: 5, Species: "potato", Zone: "potato"}},
		Items:  []game.ItemView{{ID: 90, X: 5, Y: 5, Kind: "rifle", Qty: 1, FreshDays: -1}},
	}
	cands := selectablesAt(w, nil, 5, 5)
	if len(cands) != 3 || cands[0].kind != selItem || cands[1].kind != selPlant || cands[2] != (selCand{selZone, 3}) {
		t.Fatalf("candidates at a zone cell with an item and a crop = %+v, want item, plant, zone 3", cands)
	}
	if cands := selectablesAt(w, nil, 6, 6); len(cands) != 1 || cands[0] != (selCand{selZone, 3}) {
		t.Fatalf("an empty zone cell offers only the zone, got %+v", cands)
	}
	if cands := selectablesAt(w, nil, 9, 9); len(cands) != 0 {
		t.Fatalf("a cell outside the zone offers %+v", cands)
	}
}

func TestClickOnAnEmptyZoneCellSelectsTheZone(t *testing.T) {
	t.Parallel()
	a, _, _ := zoneApp(t, growZone(3, "potato", 4, 4, 7, 7))
	clickTile(a, 6, 6)
	if !a.hasZone || a.zoneSel != 3 {
		t.Fatalf("selected zone = %v/%d, want zone 3", a.hasZone, a.zoneSel)
	}
	if !a.paneShown() {
		t.Fatal("a selected zone has an inspect pane")
	}
	clickTile(a, 12, 12) // empty ground
	if a.hasZone {
		t.Fatal("clicking empty ground clears the zone")
	}
}

func TestRepeatClicksCycleToTheZoneAndBack(t *testing.T) {
	t.Parallel()
	a, s, _ := zoneApp(t, growZone(3, "potato", 4, 4, 7, 7))
	s.world.Plants = []game.PlantView{{ID: 41, X: 5, Y: 5, Species: "potato", Zone: "potato"}}
	clickTile(a, 5, 5)
	if _, ok := a.selectedPlant(); !ok || a.hasZone {
		t.Fatal("the crop comes before the zone under it")
	}
	clickTile(a, 5, 5)
	if !a.hasZone || a.hasPlant {
		t.Fatal("a second click cycles to the zone and clears the plant")
	}
	clickTile(a, 5, 5)
	if _, ok := a.selectedPlant(); !ok || a.hasZone {
		t.Fatal("the cycle wraps back to the crop")
	}
}

func TestPawnAndItemStillComeBeforeTheZone(t *testing.T) {
	t.Parallel()
	a, g, _ := liveColony(t, 0)
	if _, err := g.SpawnStack(item.Rifle, 1, 6, 6); err != nil {
		t.Fatal(err)
	}
	g.Enqueue(command.Command{Kind: command.DesignateStockpile, X: 6, Y: 6, X2: 10, Y2: 10})
	g.ApplyPending()
	if len(g.World().Zones) != 1 {
		t.Fatalf("zones = %d, want 1 stockpile", len(g.World().Zones))
	}
	clickTile(a, 6, 6) // a corner, clear of the zone's name plate in the middle
	if _, ok := a.selectedItem(); !ok || a.hasZone {
		t.Fatal("a stack in a stockpile selects the stack first")
	}
	clickTile(a, 6, 6)
	if !a.hasZone {
		t.Fatal("the next click selects the stockpile")
	}
	if got := a.zonePane(a.zoneSel).Title; got != "Stockpile 1" {
		t.Fatalf("title %q, want the default name", got)
	}
}

func TestClickOnAZoneLabelSelectsItDirectly(t *testing.T) {
	t.Parallel()
	z := growZone(3, "potato", 2, 2, 11, 9)
	a, s, _ := zoneApp(t, z)
	// Plants round the label's middle: a plain click there would pick one.
	for i, c := range [][2]int{{6, 5}, {7, 5}, {6, 6}, {7, 6}} {
		s.world.Plants = append(s.world.Plants, game.PlantView{ID: entity.EntityID(41 + i), X: c[0], Y: c[1], Species: "potato", Zone: "potato"})
	}
	r := a.renderer
	plan, _, _, _, _ := r.zoneLabelPlanFor(z, r.zoneGeomOf(z), r.tilePx())
	if !plan.ok {
		t.Fatal("the zone's label should show")
	}
	cx, cy := plan.plate.Center()
	a.cursor = &[2]int{int(cx), int(cy)}
	a.clickSelect()
	if !a.hasZone || a.zoneSel != 3 || a.hasPlant {
		t.Fatalf("a click on the label selected zone=%v/%d plant=%v, want zone 3", a.hasZone, a.zoneSel, a.hasPlant)
	}
	// Just left of the plate it is an ordinary click: the plant under it.
	a.clearSelections(selNone)
	a.cursor = &[2]int{int(plan.plate.X) - 2, int(cy)}
	a.clickSelect()
	if !a.hasPlant || a.hasZone {
		t.Fatalf("a click beside the plate selected plant=%v zone=%v, want the plant", a.hasPlant, a.hasZone)
	}
}

func TestSelectingAnotherKindClearsTheZone(t *testing.T) {
	t.Parallel()
	a, s, _ := zoneApp(t, growZone(3, "potato", 4, 4, 7, 7))
	s.world.Plants = []game.PlantView{{ID: 41, X: 14, Y: 14, Species: "potato"}}
	a.selectCand(selCand{selZone, 3})
	if !a.hasZone {
		t.Fatal("zone not selected")
	}
	a.selectCand(selCand{selPlant, 41})
	if a.hasZone {
		t.Fatal("selecting a plant clears the zone")
	}
	a.selectCand(selCand{selZone, 3})
	a.clearSelections(selNone)
	if a.hasZone {
		t.Fatal("clearSelections clears the zone")
	}
	a.selectCand(selCand{selZone, 3})
	a.renderer.Select(1)
	a.clearSelections(selPawn)
	if a.hasZone {
		t.Fatal("selecting a colonist clears the zone")
	}
}

func TestSelectedZoneFollowsTheSplitPieceWithTheLastClick(t *testing.T) {
	t.Parallel()
	whole := growZone(3, "potato", 4, 4, 9, 5) // two rows of six
	a, s, _ := zoneApp(t, whole)
	clickTile(a, 8, 5) // the right half
	if a.zoneSel != 3 {
		t.Fatalf("selected %d, want 3", a.zoneSel)
	}
	// The sim split it: the left half (the larger piece) keeps the id, the
	// right half is a new zone.
	left := growZone(3, "potato", 4, 4, 7, 5)
	right := growZone(9, "potato", 8, 4, 9, 5)
	s.world.Zones = []game.ZoneView{left, right}
	if z, ok := a.selectedZone(); !ok || z.ID != 3 {
		t.Fatalf("the id still exists, the selection stays: %+v/%v", z.ID, ok)
	}
	// The piece with the id is gone instead (merged away): the pane, asked
	// for the old id, follows the cell.
	s.world.Zones = []game.ZoneView{growZone(5, "potato", 8, 4, 9, 5)}
	if sp := a.selectionPane(); sp.Title != "Growing zone 5" || a.zoneSel != 5 {
		t.Fatalf("followed to %q (sel %d), want zone 5", sp.Title, a.zoneSel)
	}
}

func TestSelectedZoneClearsWhenItIsDeleted(t *testing.T) {
	t.Parallel()
	a, s, _ := zoneApp(t, growZone(3, "potato", 4, 4, 7, 7))
	clickTile(a, 5, 5)
	if !a.hasZone {
		t.Fatal("not selected")
	}
	s.world.Zones = nil
	if _, ok := a.selectedZone(); ok || a.hasZone {
		t.Fatal("a deleted zone clears the selection")
	}
	if sp := a.zonePane(3); sp.Title != "" {
		t.Fatalf("the pane of a missing zone must be empty, got %q", sp.Title)
	}
	if cs := a.zoneCommands(3); cs != nil {
		t.Fatal("a missing zone has no commands")
	}
}

func TestHiddenZoneCanStillBeSelected(t *testing.T) {
	t.Parallel()
	z := growZone(3, "potato", 4, 4, 7, 7)
	z.Hidden = true
	a, _, _ := zoneApp(t, z)
	clickTile(a, 5, 5)
	if !a.hasZone {
		t.Fatal("a hidden zone is still selected by its tiles")
	}
}

func TestZoneCloseButtonAndEscape(t *testing.T) {
	t.Parallel()
	a, _, f := zoneApp(t, growZone(3, "potato", 4, 4, 7, 7))
	a.selectCand(selCand{selZone, 3})
	f.frame(t, a)
	click(t, a, "zone.close")
	if a.hasZone {
		t.Fatal("the pane's close button clears the zone")
	}
	a.selectCand(selCand{selZone, 3})
	f.key(t, a, "Escape", 0)
	if a.hasZone {
		t.Fatal("Escape clears the zone")
	}
}

// ---- the pane ----

func TestZonePaneTitles(t *testing.T) {
	t.Parallel()
	named := growZone(4, "wheat", 4, 4, 5, 5)
	named.Name = "North field"
	a, _, _ := zoneApp(t,
		growZone(1, "potato", 0, 0, 1, 1),
		rectZone(2, game.ZoneStockpile, 3, 0, 4, 1),
		rectZone(3, game.ZonePen, 6, 0, 7, 1),
		named)
	for _, c := range []struct {
		id              uint16
		title, subtitle string
	}{
		{1, "Growing zone 1", ""},
		{2, "Stockpile 2", ""},
		{3, "Pen 3", ""},
		{4, "North field", "Growing zone 4"},
	} {
		a.selectCand(selCand{selZone, entity.EntityID(c.id)})
		sp := a.selectionPane()
		if sp.Title != c.title || sp.Subtitle != c.subtitle || sp.Close != "zone.close" {
			t.Errorf("zone %d pane = %q/%q close %q, want %q/%q", c.id, sp.Title, sp.Subtitle, sp.Close, c.title, c.subtitle)
		}
	}
}

func TestZonePaneLinesForAGrowingZone(t *testing.T) {
	t.Parallel()
	a, s, _ := zoneApp(t, growZone(1, "potato", 4, 4, 5, 5)) // four tiles
	s.world.Plants = []game.PlantView{
		{ID: 1, X: 4, Y: 4, Species: "potato", Zone: "potato", Growth: 50},
		{ID: 2, X: 5, Y: 4, Species: "potato", Zone: "potato", Growth: 60},
		{ID: 3, X: 4, Y: 5, Species: "potato", Zone: "potato", Growth: 100, Harvestable: true},
		{ID: 4, X: 5, Y: 5, Species: "grass", Zone: "potato", Growth: 10},  // wild: not sown
		{ID: 5, X: 9, Y: 9, Species: "potato", Zone: "potato", Growth: 90}, // another field
	}
	s.temp = 180
	got := a.zoneLines(zoneByIDOrFail(t, a, 1))
	want := []string{
		"4 tiles",
		"Plant: Potato",
		"Growing now: 18C (grows between 0C and 45C)",
		"Sown 3 of 4 · ripe 1 · growth 70%",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("lines:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	// Nothing sown yet.
	s.world.Plants = nil
	if got := a.zoneLines(zoneByIDOrFail(t, a, 1)); got[3] != "Sown 0 of 4" {
		t.Fatalf("empty field line %q", got[3])
	}
}

func zoneByIDOrFail(t *testing.T, a *App, id uint16) game.ZoneView {
	t.Helper()
	z, ok := zoneByID(a.renderer.world(), id)
	if !ok {
		t.Fatalf("no zone %d", id)
	}
	return z
}

func TestZonePaneSaysWhenItIsTooColdOrTooHot(t *testing.T) {
	t.Parallel()
	a, s, _ := zoneApp(t, growZone(1, "corn", 4, 4, 5, 5))
	z := zoneByIDOrFail(t, a, 1)
	for _, c := range []struct {
		temp int
		want string
	}{
		{-30, "Too cold to grow: -3C (needs above 0C)"},
		{0, "Too cold to grow: 0C (needs above 0C)"},
		{150, "Growing now: 15C"},
		{450, "Too hot to grow: 45C (needs below 45C)"},
		{500, "Too hot to grow: 50C (needs below 45C)"},
	} {
		s.temp = c.temp
		if got := a.zoneLines(z)[2]; !strings.HasPrefix(got, c.want) {
			t.Errorf("at %d tenths: %q, want %q", c.temp, got, c.want)
		}
	}
}

func TestZonePaneShowsSowingAndCuttingOff(t *testing.T) {
	t.Parallel()
	z := growZone(1, "potato", 4, 4, 5, 5)
	z.NoSow, z.NoCut = true, true
	a, _, _ := zoneApp(t, z)
	got := a.zoneLines(z)
	if got[len(got)-2] != "Sowing off" || got[len(got)-1] != "Cutting off" {
		t.Fatalf("lines %q, want Sowing off and Cutting off last", got)
	}
	z.NoCut = false
	if got := a.zoneLines(z); got[len(got)-1] != "Sowing off" || len(got) != 5 {
		t.Fatalf("lines %q, want only Sowing off", got)
	}
}

func TestZonePaneLinesForStockpilesAndPens(t *testing.T) {
	t.Parallel()
	food := rectZone(2, game.ZoneStockpile, 0, 0, 2, 0)
	food.Food, food.Stone = true, true
	plain := rectZone(3, game.ZoneStockpile, 5, 0, 5, 0)
	pen := rectZone(4, game.ZonePen, 8, 0, 9, 1)
	dump := rectZone(5, game.ZoneDump, 11, 0, 13, 1) // PLAY-023
	dump.Corpses, dump.Chunks, dump.Priority = true, true, "Low"
	a, _, _ := zoneApp(t, food, plain, pen, dump)
	for _, c := range []struct {
		z    game.ZoneView
		want []string
	}{
		{food, []string{"3 tiles", "Priority: Normal", "Accepts: Food, Stone, other goods"}},
		{plain, []string{"1 tile", "Priority: Normal", "Accepts: other goods"}},
		{pen, []string{"4 tiles"}},
		{dump, []string{"6 tiles", "Priority: Low", "Accepts: Animal corpses, Stone chunks"}},
	} {
		if got := a.zoneLines(c.z); strings.Join(got, "|") != strings.Join(c.want, "|") {
			t.Errorf("%s lines = %q, want %q", zoneKindName(c.z), got, c.want)
		}
	}
}
