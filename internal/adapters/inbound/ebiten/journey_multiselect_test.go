package ebiten

import (
	"slices"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/architect"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	selectionpkg "rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/input"
)

// journey_multiselect_test.go: the mouse's group gestures (CTL-001..009 and
// the lone double click) through the real loop. Each test is one gesture in
// a small scene; helpers buildAll, doubleClick and sameIDs live in
// group_kinds_test.go, zoneJourney and layZone in journey_zones_test.go.

// multiPaneTitle is the title the inspector's pane shows for the selection.
func multiPaneTitle(j *journey) string {
	j.update(1)
	return j.a.insp.SelectionPane().Title
}

// multiHas reports a button with this id is in the layout the mouse hits.
func multiHas(j *journey, id string) bool {
	j.update(1)
	for _, b := range j.a.layoutHits(j.a.sw, j.a.sh) {
		if b.ID == id {
			return true
		}
	}
	return false
}

// multiDoors builds doors in a row of the clear rows (y 2) and double
// clicks the first, so the three of them are the group.
func multiDoors(t *testing.T) (*journey, []entity.EntityID) {
	t.Helper()
	j := newJourney(t, jPawns([2]int{18, 8}))
	doors := buildAll(t, j, "door", [2]int{10, 2}, [2]int{12, 2}, [2]int{14, 2})
	doubleClick(j, 10, 2)
	if g := j.a.sel.Group; g.Kind() != selectionpkg.Building || !sameIDs(g.IDs(), doors) {
		t.Fatalf("a double click on a door gave %v %v, want the three doors %v", g.Kind(), g.IDs(), doors)
	}
	return j, doors
}

// CTL-001: only the doors on screen.
func TestJourneyMultiDoubleClickDoorsOnScreen(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jSize(200, 24), jPawns([2]int{18, 8}))
	doors := buildAll(t, j, "door", [2]int{10, 2}, [2]int{12, 2}, [2]int{150, 2})
	doubleClick(j, 10, 2)
	if g := j.a.sel.Group; g.Kind() != selectionpkg.Building || !sameIDs(g.IDs(), doors[:2]) {
		t.Fatalf("group %v %v, want the two doors in view %v (not the one at x=150)", g.Kind(), g.IDs(), doors[:2])
	}
}

// CTL-001: walls and the hopper do not group.
func TestJourneyMultiDoubleClickWallAndHopperStayPlain(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{18, 8}))
	walls := buildAll(t, j, "wall", [2]int{10, 2}, [2]int{12, 2})
	var hoppers []entity.EntityID
	for _, x := range []int{10, 12} {
		if !j.g.BuildNow("hopper", "steel", "south", x, 5) {
			t.Fatalf("no hopper at %d,5", x)
		}
	}
	j.update(1)
	for _, b := range j.world().Buildings {
		if b.Kind == "hopper" {
			hoppers = append(hoppers, b.ID)
		}
	}
	if len(hoppers) != 2 {
		t.Fatalf("%d hoppers built", len(hoppers))
	}
	doubleClick(j, 10, 2)
	if j.a.sel.Grouped() || !j.a.sel.HasBuilding || j.a.sel.SelBuilding != walls[0] {
		t.Fatalf("a wall double click: group %v building %v, want the one wall %v", j.a.sel.Group.IDs(), j.a.sel.SelBuilding, walls[0])
	}
	j.settle()
	doubleClick(j, 10, 5)
	// the second click cycles the tile (the hopper, then its own zone)
	if j.a.sel.Grouped() || !j.a.sel.HasZone {
		t.Fatalf("a hopper double click: group %v building %v zone %v, want a plain click's cycle to the hopper's zone", j.a.sel.Group.IDs(), j.a.sel.HasBuilding, j.a.sel.HasZone)
	}
}

// CTL-002: same-kind blueprints, a wall's included.
func TestJourneyMultiDoubleClickBlueprints(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{18, 8}))
	for _, c := range []struct {
		kind construction.Kind
		x    int
	}{{construction.Wall, 10}, {construction.Wall, 12}, {construction.Door, 14}} {
		j.g.Enqueue(command.Command{Kind: command.DesignateBuild, Build: uint8(c.kind), Stuff: uint8(item.Wood), X: c.x, Y: 2})
	}
	j.tick(1)
	frames := j.world().Frames
	if len(frames) != 3 {
		t.Fatalf("%d blueprints, want 3", len(frames))
	}
	var walls []entity.EntityID
	for _, f := range frames {
		if f.X != 14 {
			walls = append(walls, f.ID)
		}
	}
	doubleClick(j, 10, 2)
	if g := j.a.sel.Group; g.Kind() != selectionpkg.Frame || !sameIDs(g.IDs(), walls) {
		t.Fatalf("group %v %v, want the two wall blueprints %v and not the door's", g.Kind(), g.IDs(), walls)
	}
}

// CTL-003: a zone takes that kind's zones in view.
func TestJourneyMultiDoubleClickZone(t *testing.T) {
	t.Parallel()
	j := zoneJourney(t)
	a := j.layZone("zones", "grow", 8, 1, 9, 2)
	b := j.layZone("zones", "grow", 12, 1, 13, 2)
	j.layZone("zones", "stockpile", 14, 5, 15, 6)
	j.a.sel.Clear(selectionpkg.None)
	doubleClick(j, 8, 1)
	want := []entity.EntityID{entity.EntityID(a.ID), entity.EntityID(b.ID)}
	if g := j.a.sel.Group; g.Kind() != selectionpkg.Zone || !sameIDs(g.IDs(), want) {
		t.Fatalf("group %v %v, want the two grow zones %v and not the stockpile", g.Kind(), g.IDs(), want)
	}
}

// CTL-004: a crop plant inside a zone takes the zones.
func TestJourneyMultiDoubleClickZonePlant(t *testing.T) {
	t.Parallel()
	j := zoneJourney(t)
	a := j.layZone("zones", "grow", 8, 1, 9, 2)
	b := j.layZone("zones", "grow", 12, 1, 13, 2)
	j.a.sel.Clear(selectionpkg.None)
	if _, err := j.g.SpawnPlantSpecies(j.world().Zones[0].Crop, 9, 2, 50); err != nil {
		t.Fatal(err)
	}
	j.update(1)
	doubleClick(j, 9, 2)
	want := []entity.EntityID{entity.EntityID(a.ID), entity.EntityID(b.ID)}
	if g := j.a.sel.Group; g.Kind() != selectionpkg.Zone || !sameIDs(g.IDs(), want) {
		t.Fatalf("group %v %v, want the zones %v", g.Kind(), g.IDs(), want)
	}
}

// CTL-004: a wild plant is a plain click.
func TestJourneyMultiDoubleClickWildPlantStaysPlain(t *testing.T) {
	t.Parallel()
	j := zoneJourney(t)
	j.layZone("zones", "grow", 8, 1, 9, 2)
	j.layZone("zones", "grow", 12, 1, 13, 2)
	j.a.sel.Clear(selectionpkg.None)
	pid, err := j.g.SpawnPlant(10, 2)
	if err != nil {
		t.Fatal(err)
	}
	j.update(1)
	doubleClick(j, 10, 2)
	if j.a.sel.Grouped() || !j.a.sel.HasPlant || j.a.sel.SelPlant != pid {
		t.Fatalf("group %v plant %v/%v, want the wild plant alone", j.a.sel.Group.IDs(), j.a.sel.HasPlant, j.a.sel.SelPlant)
	}
}

// CTL-005/006: Shift adds a door and then an item: a Mixed group titled
// Various; Shift on one of them takes it out and the group falls to one; a
// pane row picks that member.
func TestJourneyMultiShiftMixedGroup(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{18, 8}))
	door := buildAll(t, j, "door", [2]int{10, 2})[0]
	stack, err := j.g.SpawnStack(item.Wood, 10, 12, 2)
	if err != nil {
		t.Fatal(err)
	}
	j.update(1)
	j.clickTile(10, 2, inputframe.Left, 0)
	j.settle()
	j.clickTile(12, 2, inputframe.Left, input.ModShift)
	want := []selectionpkg.Cand{{Kind: selectionpkg.Building, ID: door}, {Kind: selectionpkg.Item, ID: stack}}
	if g := j.a.sel.Group; g.Kind() != selectionpkg.Mixed || !slices.Equal(g.Members, want) {
		t.Fatalf("group %v %v, want a mixed group %v", g.Kind(), g.Members, want)
	}
	if got := multiPaneTitle(j); got != "Various (2)" {
		t.Fatalf("pane title %q, want Various (2)", got)
	}
	// a pane row selects that member alone
	if !j.clickID(selectionpkg.PickID(want[1])) {
		t.Fatal("the item's pane row is disabled")
	}
	if j.a.sel.Grouped() || !j.a.sel.HasItem || j.a.sel.SelItem != stack {
		t.Fatalf("a pane row left group %v item %v, want the stack %v alone", j.a.sel.Group.IDs(), j.a.sel.SelItem, stack)
	}
	// Shift on one of two removes it
	j.settle()
	j.clickTile(10, 2, inputframe.Left, 0)
	j.settle()
	j.clickTile(12, 2, inputframe.Left, input.ModShift)
	if g := j.a.sel.Group; g.Kind() != selectionpkg.Mixed || len(g.Members) != 2 {
		t.Fatalf("the group was rebuilt as %v %v", g.Kind(), g.Members)
	}
	j.settle()
	j.clickTile(10, 2, inputframe.Left, input.ModShift)
	if j.a.sel.Grouped() || !j.a.sel.HasItem || j.a.sel.SelItem != stack {
		t.Fatalf("Shift on the door left group %v item %v, want the stack alone", j.a.sel.Group.IDs(), j.a.sel.SelItem)
	}
}

// CTL-007: a box over doors and a bed takes furniture; a colonist in the box
// takes only the colonist.
func TestJourneyMultiBox(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{12, 5}))
	furniture := buildAll(t, j, "door", [2]int{8, 2}, [2]int{10, 2})
	furniture = append(furniture, buildAll(t, j, "bed", [2]int{8, 4})...)
	// the box stops short of the colonist's row
	j.dragTiles(inputframe.Left, 7, 1, 11, 5, 0)
	if g := j.a.sel.Group; g.Kind() != selectionpkg.Building || !sameIDs(g.IDs(), furniture) {
		t.Fatalf("a box over doors and a bed: %v %v, want the furniture %v", g.Kind(), g.IDs(), furniture)
	}
	j.settle()
	j.dragTiles(inputframe.Left, 7, 1, 13, 6, 0)
	pawn := selPawnIDs(j)[0]
	if j.a.sel.Grouped() || j.a.sel.HasBuilding || !j.a.renderer.HasSelect() || j.a.renderer.Selected() != pawn {
		t.Fatalf("a box with a colonist in it: group %v building %v selected %v/%v, want the colonist %v only",
			j.a.sel.Group.IDs(), j.a.sel.HasBuilding, j.a.renderer.HasSelect(), j.a.renderer.Selected(), pawn)
	}
}

// CTL-007: Shift+box adds to what is selected.
func TestJourneyMultiShiftBoxAdds(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{18, 8}))
	doors := buildAll(t, j, "door", [2]int{8, 2}, [2]int{10, 2})
	beds := buildAll(t, j, "bed", [2]int{12, 2})
	j.dragTiles(inputframe.Left, 7, 1, 11, 3, 0)
	if g := j.a.sel.Group; g.Kind() != selectionpkg.Building || !sameIDs(g.IDs(), doors) {
		t.Fatalf("a box over doors: %v %v, want %v", g.Kind(), g.IDs(), doors)
	}
	j.settle()
	j.dragTiles(inputframe.Left, 11, 1, 13, 4, input.ModShift) // from bare ground, over the bed
	if g := j.a.sel.Group; g.Kind() != selectionpkg.Building || !sameIDs(g.IDs(), append(slices.Clone(doors), beds...)) {
		t.Fatalf("Shift+box over the bed: %v %v (building %v), want the doors %v and the bed %v", g.Kind(), g.IDs(), j.a.sel.HasBuilding, doors, beds)
	}
}

// CTL-008: the pane for three doors reads the count.
func TestJourneyMultiPaneCount(t *testing.T) {
	t.Parallel()
	j, _ := multiDoors(t)
	j.draw(2)
	if got := multiPaneTitle(j); got != "3 doors" {
		t.Fatalf("pane title %q, want 3 doors", got)
	}
}

// CTL-009: the Forbid button sends SetForbidden for every door; the Move
// button, which names one building, is not offered.
func TestJourneyMultiForbidButtonAndNoMove(t *testing.T) {
	t.Parallel()
	j, doors := multiDoors(t)
	for _, id := range []string{"cmd.group.bldg.move", "bldg.move"} {
		if multiHas(j, id) {
			t.Fatalf("a group of doors offers %s", id)
		}
	}
	if !j.clickID("cmd.group.bldg.forbid") {
		t.Fatal("the group's Forbid is disabled")
	}
	var got []entity.EntityID
	for _, c := range j.cmdsOf(command.SetForbidden) {
		if !c.Enabled {
			t.Fatalf("Forbid sent %+v", c)
		}
		got = append(got, entity.EntityID(c.Target))
	}
	if !sameIDs(got, doors) {
		t.Fatalf("SetForbidden reached %v, want every door %v", got, doors)
	}
}

// CTL-009: Deconstruct marks every door.
func TestJourneyMultiDeconstructButton(t *testing.T) {
	t.Parallel()
	j, doors := multiDoors(t)
	if !j.clickID("cmd.group.bldg.deconstruct") {
		t.Fatal("the group's Deconstruct is disabled")
	}
	j.tick(1)
	marked := 0
	for _, b := range j.world().Buildings {
		if slices.Contains(doors, b.ID) && b.Deconstruct {
			marked++
		}
	}
	if marked != len(doors) {
		t.Fatalf("%d of %d doors are marked for deconstruction", marked, len(doors))
	}
}

// CTL-009: a crop menu on two zones opens once and a pick reaches both.
func TestJourneyMultiCropMenuReachesEveryZone(t *testing.T) {
	t.Parallel()
	j := zoneJourney(t)
	a := j.layZone("zones", "grow", 8, 1, 9, 2)
	b := j.layZone("zones", "grow", 12, 1, 13, 2)
	j.a.sel.Clear(selectionpkg.None)
	doubleClick(j, 8, 1)
	if g := j.a.sel.Group; g.Kind() != selectionpkg.Zone || len(g.IDs()) != 2 {
		t.Fatalf("group %v %v, want both zones", g.Kind(), g.IDs())
	}
	if !j.clickID("cmd.group.zone.crop") {
		t.Fatal("the group's crop button is disabled")
	}
	if n := len(j.a.ord.FloatButtons()); n == 0 {
		t.Fatal("the crop button opened no menu")
	}
	j.reset()
	crop := ""
	for _, c := range architect.ZoneSpecies() {
		if c.Name != a.Crop && c.Name != "tree" && j.a.win.Research().LockedBy("crop:"+c.Name) == "" {
			crop = c.Name
			break
		}
	}
	if crop == "" {
		t.Fatalf("no crop differs from %q", a.Crop)
	}
	j.pick(architect.CropLabel(crop))
	var targets []entity.EntityID
	for _, c := range j.cmdsOf(command.SetZoneCrop) {
		targets = append(targets, entity.EntityID(c.Target))
	}
	if want := []entity.EntityID{entity.EntityID(a.ID), entity.EntityID(b.ID)}; !sameIDs(targets, want) {
		t.Fatalf("SetZoneCrop reached %v, want both zones %v", targets, want)
	}
}

// CTL-009: F on a group of a stack and a door flips both.
func TestJourneyMultiForbidKeyOnMixedGroup(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{18, 8}))
	door := buildAll(t, j, "door", [2]int{10, 2})[0]
	stack, err := j.g.SpawnStack(item.Wood, 10, 12, 2)
	if err != nil {
		t.Fatal(err)
	}
	j.update(1)
	j.clickTile(10, 2, inputframe.Left, 0)
	j.settle()
	j.clickTile(12, 2, inputframe.Left, input.ModShift)
	if j.a.sel.Group.Kind() != selectionpkg.Mixed {
		t.Fatalf("group %v, want a mixed one", j.a.sel.Group.Kind())
	}
	j.reset()
	j.key("F", 0)
	var got []entity.EntityID
	for _, c := range j.cmdsOf(command.SetForbidden) {
		if !c.Enabled {
			t.Fatalf("F sent %+v, want a Forbid", c)
		}
		got = append(got, entity.EntityID(c.Target))
	}
	if !sameIDs(got, []entity.EntityID{door, stack}) {
		t.Fatalf("F reached %v, want the door %v and the stack %v", got, door, stack)
	}
}

// A tile with one lone building and a colonist, nothing alike: a double
// click on the building finds no second one and still cycles the tile to the
// colonist (the fallback of selectSameOnScreen). A door stands for the
// casket: a casket blocks the cell, so a colonist is pushed off it.
func TestJourneyMultiLoneDoubleClickCycles(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{12, 4}))
	px, py := tileOf(j, 0)
	pawn := selPawnIDs(j)[0]
	if !j.g.BuildNow("door", "wood", "south", px, py) {
		t.Fatal("no door under the colonist")
	}
	j.update(1)
	j.clickTile(px, py, inputframe.Left, 0)
	if !j.a.renderer.HasSelect() || j.a.renderer.Selected() != pawn {
		t.Fatal("the first click did not take the colonist")
	}
	j.settle()
	j.clickTile(px, py, inputframe.Left, 0) // a single click: the cycle reaches the door
	if !j.a.sel.HasBuilding || j.a.renderer.HasSelect() {
		t.Fatalf("the second click: building %v pawn %v, want the door", j.a.sel.HasBuilding, j.a.renderer.HasSelect())
	}
	j.clickTile(px, py, inputframe.Left, 0) // a double click: nothing alike
	if j.a.sel.Grouped() || j.a.sel.HasBuilding || !j.a.renderer.HasSelect() || j.a.renderer.Selected() != pawn {
		t.Fatalf("a double click on a lone door: group %v building %v pawn %v, want the cycle to the colonist",
			j.a.sel.Group.IDs(), j.a.sel.HasBuilding, j.a.renderer.HasSelect())
	}
}

// With the Architect open while a group is selected, a tool card in the
// command grid can be clicked.
func TestJourneyMultiArchitectToolWithGroup(t *testing.T) {
	t.Parallel()
	j, _ := multiDoors(t)
	j.key("Tab", 0)
	if !j.a.arch.Open {
		t.Fatal("Tab did not open the Architect")
	}
	if !j.clickID("arch.cat.structure") {
		t.Fatal("Structure is disabled")
	}
	if !j.clickID("arch.tool.wall") {
		t.Fatal("the wall tool is disabled")
	}
	if j.a.arch.ToolID != "wall" {
		t.Fatalf("armed %q with a group selected, want wall", j.a.arch.ToolID)
	}
}
