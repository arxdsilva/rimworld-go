package ebiten

import (
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// areaStub is a colony with two areas, Ada in "Indoors" (id 3) and Bo
// unrestricted.
func areaStub() *globeTabSim {
	s := worldStub()
	s.areas = []game.AreaView{
		{Index: 0, ID: 3, Name: "Indoors", Cells: 12, Color: [4]uint8{200, 60, 60, 255}},
		{Index: 1, ID: 5, Name: "Yard", Cells: 30, Color: [4]uint8{60, 160, 220, 255}},
	}
	s.area = map[entity.EntityID]int{1: 3}
	return s
}

func areaApp(t *testing.T, s *globeTabSim) (*App, *fakeInput) {
	t.Helper()
	a, f := colonyApp(t, s.stubSim)
	a.renderer.game = s
	return a, f
}

func lastAreaCmd(s *globeTabSim) command.Command {
	if len(s.cmds) == 0 {
		return command.Command{}
	}
	return s.cmds[len(s.cmds)-1]
}

func TestAreaToolSubmenuListsAreasAndArms(t *testing.T) {
	t.Parallel()
	s := areaStub()
	a, f := areaApp(t, s)
	a.openArchitect()
	a.selectArchTool("allowed_area")
	f.Frame(t, a)
	if a.uiModalTop() != areaPickID {
		t.Fatalf("top modal %q, want the area submenu", a.uiModalTop())
	}
	for _, id := range []string{"areapick.row.3", "areapick.row.5", "areapick.manage"} {
		if !hasWidget(a, id) {
			t.Errorf("submenu lacks %s", id)
		}
	}
	if hasWidget(a, "areapick.row.-1") || hasWidget(a, "areapick.row.-2") {
		t.Error("the Architect submenu lists Unrestricted/Home")
	}
	press(t, a, f, "areapick.row.5")
	f.Frame(t, a)
	if a.archTool != "allowed_area" || a.areaPaintIndex() != 5 || a.uiModalTop() != "" {
		t.Fatalf("armed tool %q area %d modal %q", a.archTool, a.areaPaintIndex(), a.uiModalTop())
	}
	// A left-drag paints a rectangle on area id 5.
	a.archX0, a.archY0, a.archX1, a.archY1 = 2, 3, 6, 7
	a.submitArchitect()
	if c := lastAreaCmd(s); c.Kind != command.PaintArea || c.Index != 5 || !c.Enabled || c.X != 2 || c.Y2 != 7 {
		t.Fatalf("paint command %+v", c)
	}
	// Escape-style cancel: picking the armed card again toggles it off.
	a.selectArchTool("allowed_area")
	if a.archTool != "" {
		t.Fatalf("armed card did not toggle off: %q", a.archTool)
	}
}

func TestAreaClearToolEmitsClear(t *testing.T) {
	t.Parallel()
	s := areaStub()
	a, f := areaApp(t, s)
	a.openArchitect()
	a.selectArchTool("allowed_area_clear")
	press(t, a, f, "areapick.row.3")
	f.Frame(t, a)
	a.archX0, a.archY0, a.archX1, a.archY1 = 1, 1, 2, 2
	a.submitArchitect()
	if c := lastAreaCmd(s); c.Kind != command.PaintArea || c.Index != 3 || c.Enabled {
		t.Fatalf("clear command %+v", c)
	}
}

func TestAreaSubmenuWithoutAreasOnlyManages(t *testing.T) {
	t.Parallel()
	s := worldStub()
	a, f := colonyApp(t, s.stubSim)
	a.renderer.game = s
	a.openArchitect()
	a.selectArchTool("allowed_area")
	f.Frame(t, a)
	if !hasWidget(a, "areapick.manage") {
		t.Fatal("no Manage row")
	}
	for _, w := range a.uiSt().Cur {
		if strings.HasPrefix(w.ID, "areapick.row.") {
			t.Fatalf("unexpected row %s", w.ID)
		}
	}
	press(t, a, f, "areapick.manage")
	f.Frame(t, a)
	if a.uiModalTop() != areasModalID {
		t.Fatalf("Manage opened %q", a.uiModalTop())
	}
}

func TestAreaManagerActions(t *testing.T) {
	t.Parallel()
	s := areaStub()
	a, f := areaApp(t, s)
	a.openAreaManager()
	f.Frame(t, a)
	press(t, a, f, "areas.invert.5")
	if c := lastAreaCmd(s); c.Kind != command.InvertArea || c.Index != 5 {
		t.Fatalf("invert %+v", c)
	}
	// Area 5 is unused: Delete goes out at once.
	press(t, a, f, "areas.delete.5")
	if c := lastAreaCmd(s); c.Kind != command.DeleteArea || c.Index != 5 {
		t.Fatalf("delete %+v", c)
	}
	press(t, a, f, "modal."+areasModalID+".areas.new")
	if c := lastAreaCmd(s); c.Kind != command.CreateArea || c.Name != "Area 3" {
		t.Fatalf("new area %+v", c)
	}
}

func TestAreaManagerRenames(t *testing.T) {
	t.Parallel()
	s := areaStub()
	a, f := areaApp(t, s)
	a.openAreaManager()
	f.Frame(t, a)
	press(t, a, f, "areas.name.5") // focus the field
	f.TypeText(t, a, "s")          // "Yard" -> "Yards"
	f.Key(t, a, "Enter", 0)
	f.Frame(t, a)
	if c := lastAreaCmd(s); c.Kind != command.RenameArea || c.Index != 5 || c.Name != "Yards" {
		t.Fatalf("rename %+v", c)
	}
}

func TestAreaDeleteConfirmsWhenUsed(t *testing.T) {
	t.Parallel()
	s := areaStub()
	s.world.Animals = []game.AnimalView{{Tame: true, AreaRef: 3}}
	a, f := areaApp(t, s)
	a.openAreaManager()
	f.Frame(t, a)
	if n := a.areaUsers(3); n != 2 { // Ada and one animal
		t.Fatalf("users of area 3 = %d, want 2", n)
	}
	press(t, a, f, "areas.delete.3")
	f.Frame(t, a)
	if a.uiModalTop() != areasConfirm {
		t.Fatalf("top modal %q, want the delete confirm", a.uiModalTop())
	}
	for _, c := range s.cmds {
		if c.Kind == command.DeleteArea {
			t.Fatal("deleted before the confirm")
		}
	}
	press(t, a, f, "modal."+areasConfirm+".ok")
	if c := lastAreaCmd(s); c.Kind != command.DeleteArea || c.Index != 3 {
		t.Fatalf("confirmed delete %+v", c)
	}
}

func TestAreaNewDisabledAtCap(t *testing.T) {
	t.Parallel()
	s := worldStub()
	for i := 1; i <= areasMax; i++ {
		s.areas = append(s.areas, game.AreaView{Index: i - 1, ID: uint16(i), Name: "A"})
	}
	a, f := areaApp(t, s)
	a.openAreaManager()
	f.Frame(t, a)
	w, ok := a.uiSt().Find("modal." + areasModalID + ".areas.new")
	if !ok || !w.Disabled {
		t.Fatalf("New area at the cap: found=%v disabled=%v", ok, w.Disabled)
	}
	press(t, a, f, "modal."+areasModalID+".areas.new")
	for _, c := range s.cmds {
		if c.Kind == command.CreateArea {
			t.Fatal("created past the cap")
		}
	}
}

func TestScheduleAllowedAreaDropdown(t *testing.T) {
	t.Parallel()
	s := areaStub()
	a, f := areaApp(t, s)
	a.workOpen, a.panelMode = true, "schedule"
	f.Frame(t, a)
	w, ok := a.uiSt().Find("sched.area.1")
	if !ok || !strings.HasPrefix(w.Label, "Indoors") {
		t.Fatalf("Ada's dropdown: found=%v label %q", ok, w.Label)
	}
	if w2, _ := a.uiSt().Find("sched.area.2"); !strings.HasPrefix(w2.Label, "Unrestricted") {
		t.Fatalf("Bo's dropdown label %q", w2.Label)
	}
	if hasWidget(a, "sched.area.0") {
		t.Fatal("the Everyone row has an area")
	}
	press(t, a, f, "sched.area.1")
	f.Frame(t, a)
	for _, id := range []string{"areapick.row.-1", "areapick.row.-2", "areapick.row.3", "areapick.row.5", "areapick.manage"} {
		if !hasWidget(a, id) {
			t.Errorf("dropdown lacks %s", id)
		}
	}
	for _, tc := range []struct {
		row  string
		want int
	}{
		{"areapick.row.5", 5}, {"areapick.row.-1", command.AreaUnrestricted}, {"areapick.row.-2", command.AreaHome},
	} {
		if !hasWidget(a, tc.row) {
			press(t, a, f, "sched.area.1")
			f.Frame(t, a)
		}
		press(t, a, f, tc.row)
		f.Frame(t, a)
		if c := lastAreaCmd(s); c.Kind != command.AssignArea || c.Pawn != 1 || c.Index != tc.want {
			t.Fatalf("%s: command %+v, want AssignArea Pawn 1 Index %d", tc.row, c, tc.want)
		}
	}
}

func TestInspectorAreaLine(t *testing.T) {
	t.Parallel()
	s := areaStub()
	a, f := areaApp(t, s)
	a.renderer.Select(1)
	f.Frame(t, a)
	w, ok := a.uiSt().Find("inspector.area")
	if !ok || w.Label != "Area: Indoors" {
		t.Fatalf("inspector area line: found=%v label %q", ok, w.Label)
	}
	click(t, a, "inspector.area") // the pane's chip answers through its widget
	f.Frame(t, a)
	press(t, a, f, "areapick.row.-2")
	if c := lastAreaCmd(s); c.Kind != command.AssignArea || c.Pawn != 1 || c.Index != command.AreaHome {
		t.Fatalf("inspector pick %+v", c)
	}
	// PLAY-018: the chip is on the pane itself, so it shows whatever tab is open
	a.inspectTab = "needs"
	f.Frame(t, a)
	if !hasWidget(a, "inspector.area") {
		t.Fatal("the Area chip shows on every tab")
	}
}

func TestAreaRefLabels(t *testing.T) {
	t.Parallel()
	areas := []game.AreaView{{ID: 3, Name: "Indoors"}}
	for ref, want := range map[int]string{0: "Unrestricted", -1: "Unrestricted", -2: "Home", 3: "Indoors", 9: "Unrestricted"} {
		if got := areaRefLabel(areas, ref); got != want {
			t.Errorf("ref %d = %q, want %q", ref, got, want)
		}
	}
	if len(areaPickOpts(areas, true)) != 3 || len(areaPickOpts(areas, false)) != 1 {
		t.Fatal("picker options")
	}
	if c := areaTint(game.AreaView{}); c.A != 255 || c.R == 0 {
		t.Fatalf("uncoloured area tint %v", c)
	}
}

// areaBits is a 10-wide bitset with cells (x, y) set.
func areaBits(w, h int, cells ...[2]int) []uint64 {
	b := make([]uint64, (w*h+63)/64)
	for _, c := range cells {
		i := c[1]*w + c[0]
		b[i>>6] |= 1 << (uint(i) & 63)
	}
	return b
}

func TestAreaOverlayBakesOnVerChangeAndUsesColour(t *testing.T) {
	t.Parallel()
	r := stainRenderer()
	tile := r.tilePx()
	screen := ebiten.NewImage(320, 320)
	w := game.WorldView{TerrainW: 10}
	ar := game.AreaView{ID: 3, Name: "Indoors", Color: [4]uint8{200, 60, 20, 255}, Ver: 1,
		Bits: areaBits(10, 10, [2]int{1, 1}, [2]int{2, 1}, [2]int{3, 1}, [2]int{5, 4})}
	// nothing armed: nothing drawn or baked
	r.drawAreas(screen, w, []game.AreaView{ar}, 320, 320, tile)
	if r.areaOv.verts != 0 || r.areaOv.builds != 0 {
		t.Fatalf("unarmed overlay: %d verts, %d builds", r.areaOv.verts, r.areaOv.builds)
	}
	r.areaOv.armed = 3
	r.drawAreas(screen, w, []game.AreaView{ar}, 320, 320, tile)
	if r.areaOv.builds != 1 || r.areaOv.verts != 4*2 { // two row runs
		t.Fatalf("armed: %d builds %d verts, want 1 and 8", r.areaOv.builds, r.areaOv.verts)
	}
	if c := r.areaOv.tint; c.R != 200 || c.G != 60 || c.B != 20 {
		t.Fatalf("tint colour %v", c)
	}
	r.drawAreas(screen, w, []game.AreaView{ar}, 320, 320, tile)
	if r.areaOv.builds != 1 {
		t.Fatalf("redraw at the same Ver rebuilt: %d", r.areaOv.builds)
	}
	ar.Ver = 2
	ar.Bits = areaBits(10, 10, [2]int{1, 1})
	r.drawAreas(screen, w, []game.AreaView{ar}, 320, 320, tile)
	if r.areaOv.builds != 2 || r.areaOv.verts != 4 {
		t.Fatalf("after Ver change: %d builds %d verts", r.areaOv.builds, r.areaOv.verts)
	}
	// a hovered (not armed) area shows too; no map width draws nothing
	r.areaOv.armed, r.areaOv.hover = 0, 3
	r.drawAreas(screen, w, []game.AreaView{ar}, 320, 320, tile)
	if r.areaOv.verts == 0 {
		t.Fatal("hovered area not drawn")
	}
	r.drawAreas(screen, game.WorldView{}, []game.AreaView{ar}, 320, 320, tile)
	if r.areaOv.verts != 0 {
		t.Fatal("no map drew")
	}
	r.areaOv.hover = 0
	r.drawAreaOverlay(screen, w, 320, 320, tile)
}

func TestAreaOverlayFollowsArmedTool(t *testing.T) {
	t.Parallel()
	s := areaStub()
	a, f := areaApp(t, s)
	a.openArchitect()
	a.selectArchTool("allowed_area")
	press(t, a, f, "areapick.row.5")
	f.Frame(t, a)
	if a.renderer.areaOv.armed != 5 {
		t.Fatalf("overlay armed on %d, want 5", a.renderer.areaOv.armed)
	}
	a.archTool = ""
	f.Frame(t, a)
	if a.renderer.areaOv.armed != 0 {
		t.Fatal("overlay stays armed after the tool is dropped")
	}
}
