package ebiten

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/input"
)

// journey_select_test.go: selection and inspection journeys (QA-001). The
// debts below are what this file still owes the guard
// (TestEveryInputActionHasAJourney); delete a name here once a journey in
// this file covers it.
//
// dialog.accept (Enter) has no observable effect in the colony outside a
// text field or a wizard: the pause overlay ignores it (see
// TestJourneySelectEscapeChain, which pins that), so it is pressed there.
var _ = owe("journey_select_test.go",
	[]string{},
	[]string{})

// selPawnIDs spawns colonists and returns their ids in spawn order.
func selPawnIDs(j *journey) []entity.EntityID {
	var ids []entity.EntityID
	for _, p := range j.g.Snapshot() {
		ids = append(ids, p.ID)
	}
	return ids
}

// tileOf is where colonist i (spawn order) stands now: the first frame of
// a journey lets a spawned colonist step off its tile, so journeys click
// where the read model says, not where they spawned it.
func tileOf(j *journey, i int) (int, int) {
	p := j.g.Snapshot()[i]
	return p.X, p.Y
}

// TestJourneySelectClickPawnOpensInspector: a left click on a colonist
// selects it and the inspector pane appears.
func TestJourneySelectClickPawnOpensInspector(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	id := selPawnIDs(j)[0]
	if j.a.renderer.HasSelect() || j.a.paneShown() {
		t.Fatal("something is selected before any click")
	}
	px, py := tileOf(j, 0)
	j.clickTile(px, py, mbLeft, 0)
	j.draw(2)
	if !j.a.renderer.HasSelect() || j.a.renderer.Selected() != id {
		t.Fatalf("selected %v (has %v), want pawn %v", j.a.renderer.Selected(), j.a.renderer.HasSelect(), id)
	}
	if !j.a.paneShown() {
		t.Fatal("no inspector pane for the selected colonist")
	}
	j.clickTile(15, 7, mbLeft, 0) // empty ground deselects
	if j.a.renderer.HasSelect() || j.a.paneShown() {
		t.Fatal("a click on bare ground kept the selection")
	}
}

// TestJourneySelectSameTileCycle: a second click on the same tile moves
// the selection to the next thing in it, and Numpad * does the same.
func TestJourneySelectSameTileCycle(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	pid := selPawnIDs(j)[0]
	px, py := tileOf(j, 0)
	sid, err := j.g.SpawnStack(item.Wood, 10, px, py)
	if err != nil {
		t.Fatal(err)
	}
	j.clickTile(px, py, mbLeft, 0)
	if !j.a.renderer.HasSelect() || j.a.renderer.Selected() != pid {
		t.Fatal("the first click did not take the colonist")
	}
	j.settle()
	j.clickTile(px, py, mbLeft, 0)
	if !j.a.hasItem || j.a.selItem != sid || j.a.renderer.HasSelect() {
		t.Fatalf("the second click: item %v/%v pawn %v, want the stack %v", j.a.hasItem, j.a.selItem, j.a.renderer.HasSelect(), sid)
	}
	j.f.MoveTo(j.a, 15, 7) // Numpad * acts on the selection, wherever the mouse is
	j.key("NumpadMultiply", 0)
	if !j.a.renderer.HasSelect() || j.a.renderer.Selected() != pid || j.a.hasItem {
		t.Fatalf("Numpad * did not cycle back to the colonist (pawn %v item %v)", j.a.renderer.HasSelect(), j.a.hasItem)
	}
}

// TestJourneySelectClickBuildingItemPlant: each kind of thing selects into
// its own state.
func TestJourneySelectClickBuildingItemPlant(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	if !j.g.BuildNow("wall", "wood", "south", 10, 2) {
		t.Fatal("no wall")
	}
	sid, _ := j.g.SpawnStack(item.Wood, 10, 12, 2)
	pid, err := j.g.SpawnPlant(14, 2)
	if err != nil {
		t.Fatal(err)
	}
	j.clickTile(10, 2, mbLeft, 0)
	if !j.a.hasBuilding {
		t.Fatal("a wall click did not select a building")
	}
	j.settle()
	j.clickTile(12, 2, mbLeft, 0)
	if !j.a.hasItem || j.a.selItem != sid || j.a.hasBuilding {
		t.Fatalf("a stack click: item %v/%v building %v", j.a.hasItem, j.a.selItem, j.a.hasBuilding)
	}
	j.settle()
	j.clickTile(14, 2, mbLeft, 0)
	if !j.a.hasPlant || j.a.selPlant != pid || j.a.hasItem {
		t.Fatalf("a plant click: plant %v/%v item %v", j.a.hasPlant, j.a.selPlant, j.a.hasItem)
	}
	if !j.a.paneShown() {
		t.Fatal("no pane for the plant")
	}
}

// TestJourneySelectShiftClickAddsAndRemoves: Shift+click toggles a
// colonist in the group.
func TestJourneySelectShiftClickAddsAndRemoves(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}, [2]int{12, 4}))
	ids := selPawnIDs(j)
	ax, ay := tileOf(j, 0)
	bx, by := tileOf(j, 1)
	j.clickTile(ax, ay, mbLeft, 0)
	j.settle()
	j.clickTile(bx, by, mbLeft, input.ModShift)
	if !j.a.grouped() || len(j.a.group.ids) != 2 {
		t.Fatalf("group %v after a Shift click, want both colonists", j.a.group.ids)
	}
	j.settle()
	j.clickTile(bx, by, mbLeft, input.ModShift)
	if j.a.grouped() || !j.a.renderer.HasSelect() || j.a.renderer.Selected() != ids[0] {
		t.Fatalf("after the second Shift click: group %v, selected %v", j.a.group.ids, j.a.renderer.Selected())
	}
}

// TestJourneySelectBoxSelect: dragging a box over three colonists groups
// them.
func TestJourneySelectBoxSelect(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}, [2]int{10, 4}, [2]int{12, 4}))
	j.dragTiles(mbLeft, 7, 3, 13, 5, 0)
	if !j.a.grouped() || len(j.a.group.ids) != 3 || j.a.group.kind != selPawn {
		t.Fatalf("box select gave kind %v ids %v, want 3 colonists", j.a.group.kind, j.a.group.ids)
	}
}

// TestJourneySelectDoubleClickKind: a double click takes every visible
// colonist.
func TestJourneySelectDoubleClickKind(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}, [2]int{10, 4}, [2]int{12, 6}))
	ax, ay := tileOf(j, 0)
	j.clickTile(ax, ay, mbLeft, 0)
	j.clickTile(ax, ay, mbLeft, 0)
	if !j.a.grouped() || len(j.a.group.ids) != 3 {
		t.Fatalf("a double click grouped %v, want all 3 colonists", j.a.group.ids)
	}
}

// TestJourneySelectInspectorTabs: the pane's tab buttons open their tab and
// Escape closes the tab before the selection.
func TestJourneySelectInspectorTabs(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	px, py := tileOf(j, 0)
	j.clickTile(px, py, mbLeft, 0)
	j.draw(2)
	if j.a.paneOpenTab() != "" {
		t.Fatalf("tab %q open before any click", j.a.paneOpenTab())
	}
	for _, id := range []string{"inspector.needs", "inspector.health", "inspector.social", "inspector.log"} {
		if !j.clickID(id) {
			t.Fatalf("%s is disabled", id)
		}
		if j.a.paneOpenTab() != id {
			t.Fatalf("clicked %s: open tab %q", id, j.a.paneOpenTab())
		}
	}
	j.key("Escape", 0)
	if j.a.paneOpenTab() != "" || !j.a.renderer.HasSelect() {
		t.Fatalf("first Escape: tab %q selected %v, want the tab closed and the colonist kept", j.a.paneOpenTab(), j.a.renderer.HasSelect())
	}
	j.key("Escape", 0)
	if j.a.renderer.HasSelect() {
		t.Fatal("second Escape kept the selection")
	}
}

// TestJourneySelectHideUI: F11 hides the interface, so a click where a
// colonist-bar slot was reaches the map; Escape brings the interface back.
func TestJourneySelectHideUI(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	j.update(1)
	slots := j.a.hud().BarSlots
	if len(slots) == 0 {
		t.Fatal("no colonist bar")
	}
	s := float32(j.a.scale.S)
	cx, cy := slots[0].Center()
	j.key("F11", 0)
	if !j.a.hideUI {
		t.Fatal("F11 did not hide the interface")
	}
	j.f.clickAt(j.t, j.a, int(cx*s), int(cy*s))
	if j.a.renderer.HasSelect() {
		t.Fatal("a click on the hidden bar selected a colonist")
	}
	j.key("Escape", 0)
	if j.a.hideUI {
		t.Fatal("Escape left the interface hidden")
	}
	if j.a.nav.overlay() == overlayPause {
		t.Fatal("that Escape also opened the pause menu")
	}
	j.f.clickAt(j.t, j.a, int(cx*s), int(cy*s))
	if !j.a.renderer.HasSelect() {
		t.Fatal("the restored bar slot does not select")
	}
	j.key("F11", 0)
	j.key("F11", 0)
	if j.a.hideUI {
		t.Fatal("F11 twice should show the interface")
	}
}

// TestJourneySelectEscapeChain: Escape peels the stack in the documented
// order and ends on the pause overlay, which Escape resumes.
func TestJourneySelectEscapeChain(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	px, py := tileOf(j, 0)
	j.clickTile(px, py, mbLeft, 0)
	j.key("F1", 0) // the Work window
	if !j.a.workOpen {
		t.Fatal("F1 did not open the Work window")
	}
	j.key("Tab", 0) // the Architect (replaces the window)
	j.clickID("arch.cat.structure")
	j.clickID("arch.tool.wall")
	if !j.a.archOpen || j.a.archTool == "" {
		t.Fatalf("no armed Architect tool (open %v tool %q)", j.a.archOpen, j.a.archTool)
	}
	j.key("F11", 0)
	steps := []struct {
		name string
		done func() bool
	}{
		{"hidden UI returns", func() bool { return !j.a.hideUI && j.a.archTool != "" }},
		{"Architect tool disarms", func() bool { return j.a.archOpen && j.a.archTool == "" }},
		{"Architect closes", func() bool { return !j.a.archOpen && j.a.renderer.HasSelect() }},
		{"selection clears", func() bool { return !j.a.renderer.HasSelect() && j.a.nav.overlay() != overlayPause }},
		{"pause overlay opens", func() bool { return j.a.nav.overlay() == overlayPause }},
	}
	for _, s := range steps {
		j.key("Escape", 0)
		if !s.done() {
			t.Fatalf("Escape step %q: archOpen %v tool %q hideUI %v workOpen %v selected %v overlay %v",
				s.name, j.a.archOpen, j.a.archTool, j.a.hideUI, j.a.workOpen, j.a.renderer.HasSelect(), j.a.nav.overlay())
		}
	}
	j.key("Enter", 0) // dialog.accept: the pause overlay has no default button
	if j.a.nav.overlay() != overlayPause {
		t.Fatal("Enter dismissed the pause overlay")
	}
	j.key("Escape", 0)
	if j.a.nav.overlay() == overlayPause {
		t.Fatal("Escape did not resume from the pause overlay")
	}
}
