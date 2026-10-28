package ebiten

import (
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// penSim is a stub colony with one pen marker (id 70) and two penned
// species plus a pet, a graze-short pen and a plant of each kind inside.
func penSim() *stubSim {
	bits := make([]uint64, (24*24+63)/64)
	for _, c := range [][2]int{{5, 5}, {6, 5}, {5, 6}, {6, 6}} {
		n := c[1]*24 + c[0]
		bits[n/64] |= 1 << (n % 64)
	}
	return &stubSim{
		pawns: []game.PawnView{{ID: 1, Name: "Ada", X: 2, Y: 2}, {ID: 2, Name: "Bo", X: 3, Y: 2}},
		world: game.WorldView{
			TerrainW: 24,
			Buildings: []game.BuildingView{
				{ID: 70, Kind: "pen marker", X: 5, Y: 5, W: 1, H: 1},
				{ID: 71, Kind: "wall", X: 9, Y: 9, W: 1, H: 1},
			},
			Pens: []game.PenView{{
				ID: 70, X: 5, Y: 5, Name: "Pen 1", Enclosed: true, Bits: bits, Animals: 2,
				GrazePerDay: 30, DemandPerDay: 45, TroughStock: 12,
				Settings: game.PenSettings{
					Species: []game.PenSpecies{{Key: "cow", Allow: true, Male: true, Female: false}},
					AutoCut: []string{"bramble"},
				},
			}},
			Plants: []game.PlantView{
				{ID: 80, X: 5, Y: 5, Species: "cactus"},
				{ID: 81, X: 6, Y: 6, Species: "shrub"},
				{ID: 82, X: 6, Y: 6, Species: "potato", Zone: "potato"},
				{ID: 83, X: 20, Y: 20, Species: "moss"}, // outside the pen
			},
			Animals: []game.AnimalView{
				{ID: 50, Species: "cow", Label: "cow", X: 5, Y: 5, Tame: true, Penned: true, Name: "Bess", Pen: "Pen 1"},
				{ID: 51, Species: "chicken", Label: "chicken", X: 6, Y: 5, Tame: true, Penned: true, Name: "Hen", Pen: "Roaming"},
				{ID: 52, Species: "labrador", Label: "labrador", X: 7, Y: 7, Tame: true, Name: "Rook"},
			},
		},
	}
}

func penApp(t *testing.T, s *stubSim) *App {
	t.Helper()
	a, _ := colonyApp(t, s)
	a.selBuilding, a.hasBuilding = 70, true
	return a
}

// penTabApp is penApp with the Pen tab declared into its expanded panel.
func penTabApp(t *testing.T, s *stubSim) (*App, *fakeInput, rect) {
	t.Helper()
	a, f := colonyApp(t, s)
	a.selBuilding, a.hasBuilding = 70, true
	tab, ok := bldTab(a.buildingPane(70), "inspector.pen")
	if !ok {
		t.Fatal("a pen marker has a Pen tab")
	}
	panel := rect{X: 0, Y: 180, W: tab.WantW, H: tab.WantH}.Inset(paneInset) // what the pane hands Declare
	setHook(a.uiSt(), func(a *App) { tab.Declare(panel) })
	f.Frame(t, a)
	return a, f, panel
}

// penBody is the Pen tab's laid-out content for the app's pen.
func penBody(t *testing.T, a *App) tabBody {
	t.Helper()
	b, ok := a.buildingView(70)
	if !ok {
		t.Fatal("no marker")
	}
	pv, ok := a.penOf(b)
	if !ok {
		t.Fatal("no pen")
	}
	return penTabBody(a.renderer.world(), pv, penTabW-2*paneInset-scrollBarW)
}

func penBodyButtons(body tabBody) map[string]button {
	ids := map[string]button{}
	for _, b := range body.buttons {
		ids[b.ID] = b
	}
	return ids
}

func penSettingsOf(t *testing.T, c command.Command) game.PenSettings {
	t.Helper()
	if c.Kind != command.SetPenSettings || c.Target != 70 {
		t.Fatalf("not a SetPenSettings for the marker: %+v", c)
	}
	ps, ok := game.DecodePenSettings(c.Name)
	if !ok {
		t.Fatalf("payload does not decode: %q", c.Name)
	}
	return ps
}

func penRow(ps game.PenSettings, key string) (game.PenSpecies, bool) {
	for _, sp := range ps.Species {
		if sp.Key == key {
			return sp, true
		}
	}
	return game.PenSpecies{}, false
}

func TestPenInspectorListsPennedSpeciesOnly(t *testing.T) {
	t.Parallel()
	a := penApp(t, penSim())
	body := penBody(t, a)
	ids := penBodyButtons(body)
	for _, id := range []string{"bldg.pen.sp.cow", "bldg.pen.sp.cow.m", "bldg.pen.sp.cow.f",
		"bldg.pen.sp.chicken", "bldg.pen.sp.chicken.m", "bldg.pen.sp.chicken.f", "bldg.pen.all", "bldg.pen.none"} {
		if _, ok := ids[id]; !ok {
			t.Errorf("missing button %s", id)
		}
	}
	if a.buildingPane(70).Close != "bldg.close" {
		t.Error("the pane has a close button")
	}
	if _, ok := ids["bldg.pen.sp.labrador"]; ok {
		t.Error("a pet is not a penned species")
	}
	// cow: allowed, male only; chicken has no entry yet: default all on
	if !ids["bldg.pen.sp.cow"].Selected || !ids["bldg.pen.sp.cow.m"].Selected || ids["bldg.pen.sp.cow.f"].Selected {
		t.Errorf("cow checkboxes %+v %+v %+v", ids["bldg.pen.sp.cow"], ids["bldg.pen.sp.cow.m"], ids["bldg.pen.sp.cow.f"])
	}
	if !ids["bldg.pen.sp.chicken"].Selected || !ids["bldg.pen.sp.chicken.m"].Selected || !ids["bldg.pen.sp.chicken.f"].Selected {
		t.Error("an unlisted species defaults to every sex allowed")
	}
	viewW := float32(penTabW - 2*paneInset - scrollBarW)
	for _, b := range ids {
		if b.Y+b.H > body.h || b.X+b.W > viewW || b.X < 0 || b.Y < 0 {
			t.Errorf("button %s falls out of the tab body", b.ID)
		}
	}
	// the live widgets sit inside the expanded panel too
	a2, _, panel := penTabApp(t, penSim())
	for _, id := range []string{"bldg.pen.sp.cow", "bldg.pen.all", "bldg.pen.cut.cactus"} {
		w, ok := a2.uiSt().Find(id)
		if !ok {
			t.Errorf("no widget %s", id)
			continue
		}
		if w.R.X < panel.X || w.R.Right() > panel.Right() || w.R.Y < panel.Y || w.R.Bottom() > panel.Bottom() {
			t.Errorf("widget %s at %+v falls out of the panel %+v", id, w.R, panel)
		}
	}
}

func TestPenInspectorEmitsSettings(t *testing.T) {
	t.Parallel()
	s := penSim()
	a, f, _ := penTabApp(t, s)
	press(t, a, f, "bldg.pen.sp.cow.f") // cow females on
	ps := penSettingsOf(t, s.cmds[len(s.cmds)-1])
	if sp, ok := penRow(ps, "cow"); !ok || !sp.Allow || !sp.Male || !sp.Female {
		t.Fatalf("cow after female toggle: %+v", ps)
	}
	if sp, ok := penRow(ps, "chicken"); !ok || !sp.Allow || !sp.Male || !sp.Female {
		t.Fatalf("every listed species is written explicitly: %+v", ps)
	}
	if len(ps.AutoCut) != 1 || ps.AutoCut[0] != "bramble" {
		t.Fatalf("auto-cut is kept: %v", ps.AutoCut)
	}
	press(t, a, f, "bldg.pen.sp.chicken") // chicken allow off
	ps = penSettingsOf(t, s.cmds[len(s.cmds)-1])
	if sp, _ := penRow(ps, "chicken"); sp.Allow {
		t.Fatalf("chicken allow should toggle off: %+v", sp)
	}
	press(t, a, f, "bldg.pen.sp.cow.m") // cow males off
	ps = penSettingsOf(t, s.cmds[len(s.cmds)-1])
	if sp, _ := penRow(ps, "cow"); sp.Male {
		t.Fatalf("cow male should toggle off: %+v", sp)
	}
}

func TestPenInspectorAllNone(t *testing.T) {
	t.Parallel()
	s := penSim()
	a, f, _ := penTabApp(t, s)
	press(t, a, f, "bldg.pen.none")
	ps := penSettingsOf(t, s.cmds[len(s.cmds)-1])
	for _, k := range []string{"cow", "chicken"} {
		if sp, ok := penRow(ps, k); !ok || sp.Allow || sp.Male || sp.Female {
			t.Errorf("None clears %s: %+v", k, sp)
		}
	}
	press(t, a, f, "bldg.pen.all")
	ps = penSettingsOf(t, s.cmds[len(s.cmds)-1])
	for _, k := range []string{"cow", "chicken"} {
		if sp, ok := penRow(ps, k); !ok || !sp.Allow || !sp.Male || !sp.Female {
			t.Errorf("All sets %s: %+v", k, sp)
		}
	}
}

func TestPenFoodVerdict(t *testing.T) {
	t.Parallel()
	pv := game.PenView{Enclosed: true, GrazePerDay: 30, DemandPerDay: 45, TroughStock: 12}
	got := strings.Join(penFoodLines(pv), "|")
	for _, want := range []string{"30", "45", "12", "Short by 15 per day: add hay or expand"} {
		if !strings.Contains(got, want) {
			t.Errorf("food lines %q lack %q", got, want)
		}
	}
	pv.GrazePerDay = 45
	if got := strings.Join(penFoodLines(pv), "|"); !strings.Contains(got, "Enough grazing") || strings.Contains(got, "Short") {
		t.Errorf("enough grazing: %q", got)
	}
	pv.GrazePerDay, pv.DemandPerDay = 0, 0
	if got := strings.Join(penFoodLines(pv), "|"); !strings.Contains(got, "Enough grazing") {
		t.Errorf("an empty herd needs nothing: %q", got)
	}
}

func TestPenNotEnclosedWarning(t *testing.T) {
	t.Parallel()
	s := penSim()
	s.world.Pens[0].Enclosed, s.world.Pens[0].Bits = false, nil
	a := penApp(t, s)
	if got := penStatusLine(s.world.Pens[0]); !strings.Contains(got, "Not enclosed") {
		t.Fatalf("status %q", got)
	}
	if got := penStatusLine(penSim().world.Pens[0]); strings.Contains(got, "Not enclosed") {
		t.Fatalf("an enclosed pen is not warned: %q", got)
	}
	screen := ebiten.NewImage(1280, 720)
	a.drawBuildingMark(screen)
	a.drawPenOutline(screen) // an open pen draws a red dashed box: must not panic
}

func TestPenAutoCutChecklist(t *testing.T) {
	t.Parallel()
	s := penSim()
	a, f, _ := penTabApp(t, s)
	ids := penBodyButtons(penBody(t, a))
	for _, k := range []string{"tree", "pine", "jungle_tree", "cactus", "bramble", "shrub"} {
		if _, ok := ids["bldg.pen.cut."+k]; !ok {
			t.Errorf("auto-cut lacks %s (defaults plus plants inside the pen)", k)
		}
	}
	if _, ok := ids["bldg.pen.cut.potato"]; ok {
		t.Error("sown crops are not auto-cut candidates")
	}
	if _, ok := ids["bldg.pen.cut.moss"]; ok {
		t.Error("a plant outside the pen is not listed")
	}
	if !ids["bldg.pen.cut.bramble"].Selected || ids["bldg.pen.cut.cactus"].Selected {
		t.Error("checked state follows Settings.AutoCut")
	}
	press(t, a, f, "bldg.pen.cut.cactus")
	ps := penSettingsOf(t, s.cmds[len(s.cmds)-1])
	if len(ps.AutoCut) != 2 || ps.AutoCut[0] != "bramble" || ps.AutoCut[1] != "cactus" {
		t.Fatalf("cactus joins the list: %v", ps.AutoCut)
	}
	press(t, a, f, "bldg.pen.cut.bramble")
	ps = penSettingsOf(t, s.cmds[len(s.cmds)-1])
	if len(ps.AutoCut) != 0 {
		t.Fatalf("bramble leaves the list: %v", ps.AutoCut)
	}
	if sp, ok := penRow(ps, "cow"); !ok || !sp.Male || sp.Female {
		t.Fatalf("species settings survive an auto-cut edit: %+v", ps)
	}
}

func TestPenMarkerHasNoGenericBuildingExtras(t *testing.T) {
	t.Parallel()
	a := penApp(t, penSim())
	for _, c := range a.buildingCommands(70) {
		if strings.HasPrefix(c.ID, "bill.") {
			t.Errorf("a marker has no bills: %s", c.ID)
		}
	}
	for _, tab := range a.buildingPane(70).Tabs {
		if tab.ID == "inspector.bills" {
			t.Error("a marker has no Bills tab")
		}
	}
	screen := ebiten.NewImage(1280, 720)
	a.drawBuildingMark(screen)
	a.drawPenOutline(screen)
	// the Pen tab asks for a taller panel than the plain pane's header
	tab, _ := bldTab(a.buildingPane(70), "inspector.pen")
	if tab.WantH <= 2*paneInset {
		t.Errorf("the Pen tab wants %v", tab.WantH)
	}
	// other buildings have no Pen tab
	a.selBuilding = 71
	if got := a.buildingPane(71).Tabs; len(got) != 0 {
		t.Errorf("a wall has tabs: %+v", got)
	}
}

func TestPenOutlineEdgesAndToolGate(t *testing.T) {
	t.Parallel()
	pv := penSim().world.Pens[0]
	edges := penEdges(pv.Bits, 24, 24)
	if len(edges) != 8 { // a 2x2 block has 8 unit edges
		t.Fatalf("2x2 pen has %d outline edges", len(edges))
	}
	if len(penEdges(nil, 24, 24)) != 0 {
		t.Fatal("no bits, no edges")
	}
	a := penApp(t, penSim())
	if _, ok := a.penOutlined(); !ok {
		t.Fatal("the selected marker's pen is outlined")
	}
	a.hasBuilding = false
	if _, ok := a.penOutlined(); ok {
		t.Fatal("nothing selected, no pen tool: no outline")
	}
	click(t, a, "tab.architect")
	click(t, a, "arch.cat.misc")
	click(t, a, "arch.tool.pen_marker")
	if all := a.penOutlines(); len(all) != 1 {
		t.Fatalf("holding the pen marker tool shows every pen: %d", len(all))
	}
}

func TestArchitectPenMarkerToolReplacesPenZone(t *testing.T) {
	t.Parallel()
	if _, ok := toolByID("pen"); ok {
		t.Fatal("the old Animal pen zone tool is gone")
	}
	for _, tl := range toolsIn("zones") {
		if tl.cmd == command.DesignatePen {
			t.Fatalf("zones still paint pens: %s", tl.id)
		}
	}
	tl, ok := toolByID("pen_marker")
	if !ok {
		t.Fatal("Architect has a Pen marker tool")
	}
	if tl.category != "misc" || tl.icon != "pen_marker" || tl.build != "pen marker" || tl.mode != placeSingle {
		t.Fatalf("pen marker tool %+v", tl)
	}
}
