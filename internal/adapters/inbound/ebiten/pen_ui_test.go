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
	ids := map[string]button{}
	for _, b := range a.buildingButtons() {
		ids[b.id] = b
	}
	for _, id := range []string{"bldg.pen.sp.cow", "bldg.pen.sp.cow.m", "bldg.pen.sp.cow.f",
		"bldg.pen.sp.chicken", "bldg.pen.sp.chicken.m", "bldg.pen.sp.chicken.f", "bldg.pen.all", "bldg.pen.none", "bldg.close"} {
		if _, ok := ids[id]; !ok {
			t.Errorf("missing button %s", id)
		}
	}
	if _, ok := ids["bldg.pen.sp.labrador"]; ok {
		t.Error("a pet is not a penned species")
	}
	// cow: allowed, male only; chicken has no entry yet: default all on
	if !ids["bldg.pen.sp.cow"].selected || !ids["bldg.pen.sp.cow.m"].selected || ids["bldg.pen.sp.cow.f"].selected {
		t.Errorf("cow checkboxes %+v %+v %+v", ids["bldg.pen.sp.cow"], ids["bldg.pen.sp.cow.m"], ids["bldg.pen.sp.cow.f"])
	}
	if !ids["bldg.pen.sp.chicken"].selected || !ids["bldg.pen.sp.chicken.m"].selected || !ids["bldg.pen.sp.chicken.f"].selected {
		t.Error("an unlisted species defaults to every sex allowed")
	}
	panel := a.buildingPanelRect()
	for _, b := range ids {
		if b.y+b.h > panel.y+panel.h || b.x+b.w > panel.x+panel.w {
			t.Errorf("button %s falls out of the panel", b.id)
		}
	}
}

func TestPenInspectorEmitsSettings(t *testing.T) {
	t.Parallel()
	s := penSim()
	a := penApp(t, s)
	click(t, a, "bldg.pen.sp.cow.f") // cow females on
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
	click(t, a, "bldg.pen.sp.chicken") // chicken allow off
	ps = penSettingsOf(t, s.cmds[len(s.cmds)-1])
	if sp, _ := penRow(ps, "chicken"); sp.Allow {
		t.Fatalf("chicken allow should toggle off: %+v", sp)
	}
	click(t, a, "bldg.pen.sp.cow.m") // cow males off
	ps = penSettingsOf(t, s.cmds[len(s.cmds)-1])
	if sp, _ := penRow(ps, "cow"); sp.Male {
		t.Fatalf("cow male should toggle off: %+v", sp)
	}
}

func TestPenInspectorAllNone(t *testing.T) {
	t.Parallel()
	s := penSim()
	a := penApp(t, s)
	click(t, a, "bldg.pen.none")
	ps := penSettingsOf(t, s.cmds[len(s.cmds)-1])
	for _, k := range []string{"cow", "chicken"} {
		if sp, ok := penRow(ps, k); !ok || sp.Allow || sp.Male || sp.Female {
			t.Errorf("None clears %s: %+v", k, sp)
		}
	}
	click(t, a, "bldg.pen.all")
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
	a.drawBuildingInspector(screen)
	a.drawPenOutline(screen) // an open pen draws a red dashed box: must not panic
}

func TestPenAutoCutChecklist(t *testing.T) {
	t.Parallel()
	s := penSim()
	a := penApp(t, s)
	ids := map[string]button{}
	for _, b := range a.buildingButtons() {
		ids[b.id] = b
	}
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
	if !ids["bldg.pen.cut.bramble"].selected || ids["bldg.pen.cut.cactus"].selected {
		t.Error("checked state follows Settings.AutoCut")
	}
	click(t, a, "bldg.pen.cut.cactus")
	ps := penSettingsOf(t, s.cmds[len(s.cmds)-1])
	if len(ps.AutoCut) != 2 || ps.AutoCut[0] != "bramble" || ps.AutoCut[1] != "cactus" {
		t.Fatalf("cactus joins the list: %v", ps.AutoCut)
	}
	click(t, a, "bldg.pen.cut.bramble")
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
	for _, b := range a.buildingButtons() {
		if strings.HasPrefix(b.id, "bill.") {
			t.Errorf("a marker has no bills: %s", b.id)
		}
	}
	screen := ebiten.NewImage(1280, 720)
	a.drawBuildingInspector(screen)
	a.drawPenOutline(screen)
	if r := a.buildingPanelRect(); r.h <= buildingRect().h {
		t.Errorf("the pen panel is taller than the generic one: %v", r.h)
	}
	a.selBuilding = 71
	if a.buildingPanelRect() != buildingRect() {
		t.Error("other buildings keep the generic rect")
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
