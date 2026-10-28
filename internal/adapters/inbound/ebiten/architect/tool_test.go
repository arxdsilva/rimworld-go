package architect

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// fakeSim is a placement rule: every cell placeable except the blocked
// ones, zones only on free cells; areas and per-pawn refs from fields.
type fakeSim struct {
	Sim
	blocked map[[2]int]string
	areas   []game.AreaView
	areaOf  map[entity.EntityID]int
}

func (s *fakeSim) CanPlaceAt(_, _, _ string, x, y int) (bool, string) {
	why, bad := s.blocked[[2]int{x, y}]
	return !bad, why
}
func (s *fakeSim) ReplacesAt(string, string, int, int) string     { return "" }
func (s *fakeSim) AutoFacingAt(_, facing string, _, _ int) string { return facing }
func (s *fakeSim) CanZoneAt(_ string, x, y int) bool              { return s.blocked[[2]int{x, y}] == "" }
func (s *fakeSim) Areas() []game.AreaView                         { return s.areas }
func (s *fakeSim) Buildables() []game.BuildableView               { return game.Buildables() }
func (s *fakeSim) AreaOf(id entity.EntityID) int                  { return s.areaOf[id] }

func TestOpenPanelConfirmsItsOwnCategoryBeforeCollapsing(t *testing.T) {
	t.Parallel()
	tl, f := newFake(nil)
	tl.OpenPanel()
	if !tl.Open || tl.Cat != "structure" || tl.OpenedOn() != "structure" || f.floats != 1 {
		t.Fatalf("opened on %q (%q), floats %d", tl.Cat, tl.OpenedOn(), f.floats)
	}
	if len(f.intents) != 1 || f.intents[0] != (ports.CloseWindows{}) {
		t.Fatalf("intents %v", f.intents)
	}
	tl.SelectCategory("structure")
	if tl.Cat != "structure" || tl.OpenedOn() != "" {
		t.Fatal("the first click on the default category confirms it")
	}
	tl.SelectCategory("structure")
	if tl.Cat != "" {
		t.Fatal("a second click collapses it")
	}
	tl.SelectTool("wall")
	if tl.Cat != "structure" || tl.ToolID != "wall" {
		t.Fatalf("tool %q in %q", tl.ToolID, tl.Cat)
	}
	tl.SelectCategory("orders")
	if tl.ToolID != "" {
		t.Fatal("another category drops the tool")
	}
	tl.SelectTool("harvest")
	tl.SelectTool("harvest")
	if tl.ToolID != "" {
		t.Fatal("the armed card toggles off")
	}
	tl.SelectTool("no such tool")
	tl.ClosePanel()
	if tl.Open || tl.ToolID != "" || f.syncs == 0 {
		t.Fatalf("closed: open %v tool %q syncs %d", tl.Open, tl.ToolID, f.syncs)
	}
}

func TestPresetPicksMaterialAndFacing(t *testing.T) {
	t.Parallel()
	tl, _ := newFake(nil)
	if !tl.Preset("wall", "granite blocks", "east") || tl.ToolID != "wall" || tl.Stuff != "granite blocks" || tl.Facing != 1 {
		t.Fatalf("preset: tool %q stuff %q facing %d", tl.ToolID, tl.Stuff, tl.Facing)
	}
	wall, _ := ToolByID("wall")
	if tl.ToolStuff(wall) != "granite blocks" {
		t.Fatal("ToolStuff follows the pick")
	}
	tl.Stuff = "unobtainium"
	if tl.ToolStuff(wall) != "wood" || tl.ToolStuff(ToolDef{Build: "door"}) != "wood" {
		t.Fatal("ToolStuff falls back to the first material")
	}
	if tl.Preset("no such kind", "", "") {
		t.Fatal("no tool builds an unknown kind")
	}
}

func TestSubmitSendsPlaceableCellsAndZoneRows(t *testing.T) {
	t.Parallel()
	sim := &fakeSim{blocked: map[[2]int]string{{2, 0}: "Rock"}}
	tl, f := newFake(sim)
	tl.Open, tl.ToolID, tl.Drag = true, "wall", true
	tl.X0, tl.Y0, tl.X1, tl.Y1 = 0, 0, 3, 0
	_, cells := tl.PreviewCells()
	if len(cells) != 4 || cells[2].OK || cells[2].Reason != "Rock" {
		t.Fatalf("preview %+v", cells)
	}
	tl.Submit()
	if len(f.cmds) != 3 || f.cmds[0].Kind != command.DesignateBuild || f.cmds[2].X != 3 {
		t.Fatalf("build commands %+v", f.cmds)
	}
	f.cmds = nil
	tl.ToolID, tl.Style = "stockpile", StyleOval
	tl.X0, tl.Y0, tl.X1, tl.Y1 = 0, 0, 4, 4
	tl.Submit()
	if len(f.cmds) != 5 || f.cmds[0].Kind != command.DesignateStockpile || f.cmds[0].X != 1 || f.cmds[0].X2 != 3 {
		t.Fatalf("an oval zone goes out a row at a time: %+v", f.cmds)
	}
	f.cmds = nil
	tl.Style, tl.Drag = StyleDefault, false
	f.hover = [2]int{7, 8}
	if x0, y0, x1, y1 := tl.DragSpan(); x0 != 7 || y1 != 8 || x1 != 7 || y0 != 8 {
		t.Fatal("not dragging: the hovered cell")
	}
	tl.ToolID = "grow"
	tl.X0, tl.Y0, tl.X1, tl.Y1 = 1, 1, 2, 2
	tl.Submit()
	if len(f.cmds) != 1 || f.cmds[0].Kind != command.DesignateGrowZone || f.cmds[0].Build == 0 {
		t.Fatalf("grow zone %+v", f.cmds)
	}
}

func TestCardsCarryLocksAndTheArmedTool(t *testing.T) {
	t.Parallel()
	tl, f := newFake(&fakeSim{})
	f.locked = map[string]string{"build:stonecutter table": "Stonecutting"}
	tl.Open, tl.Cat, tl.ToolID = true, "production", "workbench"
	cards := tl.Cards()
	if len(cards) != len(ToolsIn("production")) {
		t.Fatalf("%d cards", len(cards))
	}
	var on, locked bool
	for _, c := range cards {
		on = on || c.On && c.ID == ids.PrefixArchTool+"workbench"
		locked = locked || c.Disabled && c.Reason == "Needs research: Stonecutting"
	}
	if !on || !locked {
		t.Fatalf("cards %+v", cards)
	}
	if tl.DescHeight() <= 0 || len(tl.Plan().Lines) == 0 || tl.Plan().Lines[0].Text != "Workbench" {
		t.Fatalf("description %+v", tl.Plan().Lines)
	}
	tl.Open = false
	if tl.Cards() != nil || tl.DescHeight() != 0 || tl.Buttons() != nil {
		t.Fatal("a closed panel has nothing")
	}
}

func TestAreaHelpersAndUsers(t *testing.T) {
	t.Parallel()
	areas := []game.AreaView{{ID: 3, Name: "Indoors"}}
	for ref, want := range map[int]string{0: "Unrestricted", -1: "Unrestricted", -2: "Home", 3: "Indoors", 9: "Unrestricted"} {
		if got := AreaRefLabel(areas, ref); got != want {
			t.Errorf("ref %d = %q, want %q", ref, got, want)
		}
	}
	if len(AreaPickOpts(areas, true)) != 3 || len(AreaPickOpts(areas, false)) != 1 {
		t.Fatal("picker options")
	}
	if c := AreaTint(game.AreaView{}); c.A != 255 || c.R == 0 {
		t.Fatalf("uncoloured area tint %v", c)
	}
	sim := &fakeSim{areas: areas, areaOf: map[entity.EntityID]int{7: 3}}
	tl, f := newFake(sim)
	f.w.Animals = []game.AnimalView{{Tame: true, AreaRef: 3}, {Tame: true, Penned: true, AreaRef: 3}}
	if ref, ok := tl.Areas.ColonistArea(7); !ok || ref != 3 {
		t.Fatalf("colonist area %d %v", ref, ok)
	}
	if c, ok := tl.Areas.Chip(game.PawnView{ID: 7}); !ok || c.Label != "Area: Indoors" || c.ID != AreaChipID {
		t.Fatalf("chip %+v", c)
	}
	if n := tl.Areas.Users(3); n != 1 { // the penned animal does not count; no pawns listed
		t.Fatalf("users %d", n)
	}
	tl.Areas.arm("allowed_area", 3)
	if tl.ToolID != "allowed_area" || tl.Areas.PaintIndex() != 3 || tl.Cat != "zones" {
		t.Fatal("arm")
	}
	tl.Areas.Assign(7, -2)
	if c := f.cmds[len(f.cmds)-1]; c.Kind != command.AssignArea || c.Pawn != 7 || c.Index != -2 {
		t.Fatalf("assign %+v", c)
	}
}

func TestMoverStepsWithACheck(t *testing.T) {
	t.Parallel()
	tl, f := newFake(&fakeSim{})
	f.w.Buildings = []game.BuildingView{{ID: 5, Kind: "bed", Facing: "east"}}
	tl.Mover.Start(game.BuildingView{ID: 5, Kind: "wall"})
	if tl.Mover.State().On {
		t.Fatal("a wall does not move")
	}
	tl.ToolID = "wall"
	tl.Mover.Start(f.w.Buildings[0])
	s := tl.Mover.State()
	if !s.On || s.Facing != 1 || tl.ToolID != "" {
		t.Fatalf("start %+v tool %q", s, tl.ToolID)
	}
	s.Check = moveRule(false)
	if !tl.Mover.Step(2, 2, true, 0, true, false) || !s.On {
		t.Fatal("the opening click is swallowed")
	}
	tl.Mover.Step(2, 2, true, 0, true, false)
	if f.hint != "nope" || len(f.cmds) != 0 {
		t.Fatalf("refused: hint %q cmds %v", f.hint, f.cmds)
	}
	s.Check = moveRule(true)
	tl.Mover.Step(2, 3, true, 0, true, false)
	if s.On || len(f.cmds) != 1 || f.cmds[0].Kind != command.MoveBuilding || f.cmds[0].Y != 3 {
		t.Fatalf("moved: %+v", f.cmds)
	}
	if MovableKind("floor", true) || !MovableKind("bed", false) || MovingLine(game.BuildingView{}) != "" {
		t.Fatal("helpers")
	}
	if f, ok := PendingMoveFrame(game.WorldView{Frames: []game.FrameView{{Moving: true, MovingFrom: 5, X: 1}}}, 5); !ok || MoveCancelRect(f).X2 != 1 {
		t.Fatal("pending move frame")
	}
}

type moveRule bool

func (r moveRule) CanMoveTo(entity.EntityID, string, int, int) (bool, string) {
	if r {
		return true, ""
	}
	return false, "nope"
}

func TestMarksForceTheirOverlays(t *testing.T) {
	t.Parallel()
	tl, f := newFake(nil)
	m := tl.Marks
	for tool, want := range map[string][5]bool{
		"roof": {true}, "home": {false, true}, "plan": {false, false, true},
		"conduit": {false, false, false, true}, "snow_clear": {false, false, false, false, true}, "wall": {},
	} {
		tl.Open, tl.ToolID = true, tool
		if got := [5]bool{m.ForcesRoof(), m.ForcesHome(), m.ForcesPlan(), m.ForcesPower(), m.ForcesSnowClear()}; got != want {
			t.Errorf("%s forces %v, want %v", tool, got, want)
		}
	}
	tl.Open = false
	if m.ForcesRoof() || m.ForcesPower() {
		t.Fatal("a closed Architect forces nothing")
	}
	if !strings.HasPrefix(m.PowerToggleTip(), "Power grid\n") || m.PowerLegend() != "" || m.PenOutlines() != nil {
		t.Fatal("power card without a key, no legend or pens off the colony")
	}
	edges := PenEdges([]uint64{0b11}, 4, 2)
	if len(edges) != 6 { // a 2x1 pen: two tops, two bottoms, the two ends
		t.Fatalf("pen edges %+v", edges)
	}
	tl.ToggleHomeAuto()
	if len(f.cmds) != 0 {
		t.Fatal("no colony: nothing sent")
	}
}
