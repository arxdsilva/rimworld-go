package ebiten

import (
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/platform/input"
)

// journey_architect_test.go: Architect journeys (QA-001). The debts below are what this
// file still owes the guard (TestEveryInputActionHasAJourney); delete a
// name here once a journey in this file covers it.
var _ = owe("journey_architect_test.go",
	[]string{},
	[]string{})

// archJourney is a paused colony with the whole map cleared of plants.
func archJourney(t *testing.T) *journey {
	t.Helper()
	j := newJourney(t)
	j.g.ClearPlants(0, 0, 23, 23)
	return j
}

// openArch opens the Architect with Tab and arms tool id from category cat.
func (j *journey) openArch(cat, tool string) {
	j.t.Helper()
	if !j.a.arch.Open {
		j.key("Tab", 0)
		if !j.a.arch.Open {
			j.t.Fatal("Tab did not open the Architect")
		}
	}
	if j.a.arch.Cat != cat || j.a.arch.OpenedOn() == cat { // the open category again would collapse it
		if !j.clickID("arch.cat." + cat) {
			j.t.Fatalf("category %s is disabled", cat)
		}
	}
	if tool != "" {
		if !j.clickID("arch.tool." + tool) {
			j.t.Fatalf("tool %s is disabled", tool)
		}
		if j.a.arch.ToolID != tool {
			j.t.Fatalf("armed %q, want %q", j.a.arch.ToolID, tool)
		}
	}
}

func TestJourneyArchitectTabOpensStructureWallDrag(t *testing.T) {
	t.Parallel()
	j := archJourney(t)
	j.openArch("structure", "wall")
	j.dragTiles(mbLeft, 8, 2, 11, 2, 0)
	cmds := j.cmdsOf(command.DesignateBuild)
	if len(cmds) != 4 {
		t.Fatalf("%d DesignateBuild, want 4", len(cmds))
	}
	id, _ := kindID("wall")
	for i, c := range cmds {
		if c.Build != id || c.X != 8+i || c.Y != 2 {
			t.Fatalf("cmd %d = %+v, want a wall at (%d, 2)", i, c, 8+i)
		}
	}
	if n := len(j.world().Frames); n != 4 {
		t.Fatalf("%d frames, want 4", n)
	}
}

// TestJourneyArchitectRotateQE: Q/E turn a single-cell tool's facing and
// the facing rides on the placed order; on a drag tool they cycle the
// draw style instead.
func TestJourneyArchitectRotateQE(t *testing.T) {
	t.Parallel()
	j := archJourney(t)
	j.openArch("furniture", "bed")
	row := 0
	face := func(key string, want uint8) {
		t.Helper()
		j.reset()
		if key != "" {
			j.key(key, 0)
		}
		row += 2
		j.clickTile(10, row, mbLeft, 0)
		c, ok := j.lastCmd(command.DesignateBuild)
		if !ok || c.Facing != want {
			t.Fatalf("after %q: %+v ok=%v, want facing %d", key, c, ok, want)
		}
	}
	face("", 0)
	face("E", 1)
	face("Q", 0)
	face("Q", 3)

	j.openArch("structure", "wall")
	if j.a.arch.Style != styleDefault {
		t.Fatalf("fresh wall style %v", j.a.arch.Style)
	}
	before := j.a.arch.Facing
	j.key("E", 0)
	if j.a.curStyle(mustTool(t, "wall")) != styleRect {
		t.Fatalf("E on a wall: style %v, want a filled rectangle", j.a.curStyle(mustTool(t, "wall")))
	}
	j.key("Q", 0)
	j.key("Q", 0)
	if j.a.curStyle(mustTool(t, "wall")) != styleOvalEdge {
		t.Fatalf("Q,Q from the rectangle: style %v, want the empty oval", j.a.curStyle(mustTool(t, "wall")))
	}
	if j.a.arch.Facing != before {
		t.Fatal("Q/E on a drag tool rotated it")
	}
}

func mustTool(t *testing.T, id string) archTool {
	t.Helper()
	tl, ok := toolByID(id)
	if !ok {
		t.Fatalf("no tool %q", id)
	}
	return tl
}

// TestJourneyArchitectCtrlConstrainsDrag: Ctrl squares a rectangle drag.
func TestJourneyArchitectCtrlConstrainsDrag(t *testing.T) {
	t.Parallel()
	j := archJourney(t)
	j.openArch("structure", "floor")
	j.dragTiles(mbLeft, 8, 2, 12, 4, input.ModCtrl)
	cells := map[[2]int]bool{}
	for _, c := range j.cmdsOf(command.DesignateBuild) {
		cells[[2]int{c.X, c.Y}] = true
	}
	if len(cells) != 25 {
		t.Fatalf("Ctrl drag 8,2 to 12,4 placed %d cells, want the 5x5 square", len(cells))
	}
	for _, c := range [][2]int{{8, 2}, {12, 6}} {
		if !cells[c] {
			t.Fatalf("square misses %v: %v", c, cells)
		}
	}
	j.reset()
	j.dragTiles(mbLeft, 8, 7, 10, 8, 0)
	if n := len(j.cmdsOf(command.DesignateBuild)); n != 6 {
		t.Fatalf("a plain drag placed %d cells, want the 3x2 rectangle", n)
	}
}

// TestJourneyArchitectStuffButton: the material buttons choose what the
// wall is built from.
func TestJourneyArchitectStuffButton(t *testing.T) {
	t.Parallel()
	j := archJourney(t)
	j.openArch("structure", "wall")
	bv, _ := buildable("wall")
	if len(bv.Stuffs) < 2 {
		t.Fatalf("wall stuffs %v", bv.Stuffs)
	}
	pick := bv.Stuffs[1]
	if !j.clickID("arch.stuff." + pick) {
		t.Fatalf("stuff %s is disabled", pick)
	}
	j.dragTiles(mbLeft, 8, 2, 9, 2, 0)
	c, ok := j.lastCmd(command.DesignateBuild)
	want, _ := game.StuffID(pick)
	if !ok || c.Stuff != want {
		t.Fatalf("DesignateBuild %+v, want stuff %q (%d)", c, pick, want)
	}
}

// TestJourneyArchitectEscapeAndRightClickLeaveTool: Escape drops the tool
// first and then closes the Architect; right-click drops the tool and
// orders nothing.
func TestJourneyArchitectEscapeAndRightClickLeaveTool(t *testing.T) {
	t.Parallel()
	j := archJourney(t)
	j.openArch("structure", "wall")
	j.key("Escape", 0)
	if j.a.arch.ToolID != "" || !j.a.arch.Open {
		t.Fatalf("first Escape: tool %q open %v, want no tool and the Architect open", j.a.arch.ToolID, j.a.arch.Open)
	}
	j.key("Escape", 0)
	if j.a.arch.Open {
		t.Fatal("second Escape left the Architect open")
	}
	j.openArch("structure", "wall")
	j.f.MoveTo(j.a, 9, 3)
	j.f.Click(j.t, j.a, mbRight, 0)
	if j.a.arch.ToolID != "" {
		t.Fatalf("right-click left tool %q", j.a.arch.ToolID)
	}
	if len(j.cmds) != 0 || len(j.world().Frames) != 0 {
		t.Fatalf("leaving the tool sent %v", j.cmds)
	}
}

// TestJourneyArchitectXDeconstructArea: X arms Deconstruct from the map;
// a drag over a wall marks it.
func TestJourneyArchitectXDeconstructArea(t *testing.T) {
	t.Parallel()
	j := archJourney(t)
	if !j.g.BuildNow("wall", "wood", "south", 9, 3) {
		t.Fatal("no wall")
	}
	j.key("X", 0)
	if j.a.arch.ToolID != "deconstruct" || !j.a.arch.Open {
		t.Fatalf("X armed %q open %v", j.a.arch.ToolID, j.a.arch.Open)
	}
	j.dragTiles(mbLeft, 8, 2, 10, 4, 0)
	c, ok := j.lastCmd(command.DesignateDeconstruct)
	if !ok || c.X != 8 || c.Y != 2 || c.X2 != 10 || c.Y2 != 4 {
		t.Fatalf("DesignateDeconstruct %+v ok=%v", c, ok)
	}
	for _, b := range j.world().Buildings {
		if b.Kind == "wall" && b.X == 9 && b.Y == 3 {
			if !b.Deconstruct {
				t.Fatal("the wall is not marked for deconstruction")
			}
			return
		}
	}
	t.Fatal("wall vanished")
}

// TestJourneyArchitectCCancelArea: C arms Cancel; a drag over blueprints
// removes them.
func TestJourneyArchitectCCancelArea(t *testing.T) {
	t.Parallel()
	j := archJourney(t)
	j.openArch("structure", "wall")
	j.dragTiles(mbLeft, 8, 2, 11, 2, 0)
	if n := len(j.world().Frames); n != 4 {
		t.Fatalf("%d frames, want 4", n)
	}
	j.key("C", 0)
	if j.a.arch.ToolID != "cancel" {
		t.Fatalf("C armed %q", j.a.arch.ToolID)
	}
	j.dragTiles(mbLeft, 7, 1, 12, 3, 0)
	c, ok := j.lastCmd(command.DesignateCancel)
	if !ok || c.X != 7 || c.Y != 1 || c.X2 != 12 || c.Y2 != 3 {
		t.Fatalf("DesignateCancel %+v ok=%v", c, ok)
	}
	if n := len(j.world().Frames); n != 0 {
		t.Fatalf("%d frames left", n)
	}
}

// TestJourneyArchitectOrdersTools: every Orders tool, dragged over a
// rectangle, sends its designation with that rectangle.
func TestJourneyArchitectOrdersTools(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		tool    string
		kind    command.Kind
		enabled bool
	}{
		{"harvest", command.DesignateHarvest, false}, {"cut", command.DesignateCut, false},
		{"mine", command.DesignateMine, false}, {"smooth", command.DesignateSmooth, false},
		{"plan", command.DesignatePlan, true}, {"unplan", command.DesignatePlan, false},
		{"forbid", command.DesignateForbid, true}, {"allow", command.DesignateForbid, false},
		{"haul", command.DesignateHaul, true}, {"hunt", command.DesignateHunt, false},
		{"tame", command.DesignateTame, false},
	} {
		t.Run(c.tool, func(t *testing.T) {
			t.Parallel()
			j := archJourney(t)
			j.openArch("orders", c.tool)
			j.dragTiles(mbLeft, 8, 2, 11, 4, 0)
			got, ok := j.lastCmd(c.kind)
			if !ok || got.X != 8 || got.Y != 2 || got.X2 != 11 || got.Y2 != 4 || got.Enabled != c.enabled {
				t.Fatalf("%s sent %+v ok=%v, want %v rect 8,2-11,4 enabled=%v", c.tool, got, ok, c.kind, c.enabled)
			}
		})
	}
}

// TestJourneyArchitectUndesignate: marking an animal for the hunt from the
// orders tool, then switching the mark off in its pane, sends Undesignate.
func TestJourneyArchitectUndesignate(t *testing.T) {
	t.Parallel()
	j := archJourney(t)
	if _, err := j.g.SpawnAnimal("dustbuck", 10, 5); err != nil {
		t.Fatal(err)
	}
	j.openArch("orders", "hunt")
	j.dragTiles(mbLeft, 10, 5, 10, 5, 0)
	if _, ok := j.lastCmd(command.DesignateHunt); !ok {
		t.Fatal("no DesignateHunt")
	}
	j.key("Escape", 0)
	j.key("Escape", 0)
	j.clickTile(10, 5, mbLeft, 0)
	if !j.clickID("animal.hunt") {
		t.Fatal("animal.hunt disabled")
	}
	c, ok := j.lastCmd(command.Undesignate)
	if !ok || c.Order != command.OrderHunt {
		t.Fatalf("Undesignate %+v ok=%v, want OrderHunt", c, ok)
	}
}

// TestJourneyArchitectBuildingPaneDeconstructCancelFrame: a built wall's
// pane marks it for deconstruction; a blueprint's pane cancels it.
func TestJourneyArchitectBuildingPaneDeconstructCancelFrame(t *testing.T) {
	t.Parallel()
	j := archJourney(t)
	if !j.g.BuildNow("wall", "wood", "south", 9, 3) {
		t.Fatal("no wall")
	}
	j.clickTile(9, 3, mbLeft, 0)
	if !j.clickID("bldg.deconstruct") {
		t.Fatal("bldg.deconstruct disabled")
	}
	c, ok := j.lastCmd(command.DesignateDeconstruct)
	if !ok || c.X != 9 || c.Y != 3 || c.X2 != 9 || c.Y2 != 3 {
		t.Fatalf("DesignateDeconstruct %+v ok=%v", c, ok)
	}

	j.openArch("structure", "wall")
	j.dragTiles(mbLeft, 12, 2, 12, 2, 0)
	if n := len(j.world().Frames); n != 1 {
		t.Fatalf("%d frames, want 1", n)
	}
	j.key("Escape", 0)
	j.key("Escape", 0)
	j.settle()
	j.clickTile(12, 2, mbLeft, 0)
	if !j.clickID("frame.cancel") {
		t.Fatal("frame.cancel disabled")
	}
	if _, ok := j.lastCmd(command.CancelFrame); !ok {
		t.Fatal("no CancelFrame")
	}
	if n := len(j.world().Frames); n != 0 {
		t.Fatalf("%d frames after the cancel", n)
	}
}

// TestJourneyArchitectBuildingMove: Move on a bed's pane, a rotate and a
// click on a free tile reinstall it there.
func TestJourneyArchitectBuildingMove(t *testing.T) {
	t.Parallel()
	j := archJourney(t)
	if !j.g.BuildNow("bed", "wood", "south", 9, 5) {
		t.Fatal("no bed")
	}
	j.clickTile(9, 5, mbLeft, 0)
	if !j.clickID("bldg.move") {
		t.Fatal("bldg.move disabled")
	}
	if !j.a.moveSt().On {
		t.Fatal("Move mode did not start")
	}
	j.key("E", 0)
	j.settle()
	j.clickTile(13, 6, mbLeft, 0)
	c, ok := j.lastCmd(command.MoveBuilding)
	if !ok || c.X != 13 || c.Y != 6 || c.Facing != 1 {
		t.Fatalf("MoveBuilding %+v ok=%v, want (13, 6) facing 1", c, ok)
	}
	moving := false
	for _, f := range j.world().Frames {
		moving = moving || f.Moving
	}
	if !moving {
		t.Fatal("no reinstall frame")
	}
}
