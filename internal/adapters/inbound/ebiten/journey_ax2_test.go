package ebiten

// journey_ax2_test.go: the AX-2 journeys: the structure, furniture and
// misc cards A2 made real (ornate door, column, animal flap, the table
// family, couch, sleeping spots and boxes, plant pot, small shelf, the
// hung lights, flood light, urn, egg box and steles) and the plant pot's
// sow buttons, all through the real input path.

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/architect"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/command"
)

// ax2Place arms tool from category cat, clicks (x, y) and returns the
// DesignateBuild that left, checking it builds kind.
func ax2Place(j *journey, cat, tool, kind string, x, y int) command.Command {
	j.t.Helper()
	j.openArch(cat, tool)
	j.reset()
	j.clickTile(x, y, inputframe.Left, 0)
	c, ok := j.lastCmd(command.DesignateBuild)
	id, _ := architect.KindID(kind)
	if !ok || c.Build != id || c.X != x || c.Y != y {
		j.t.Fatalf("%s at (%d, %d): %+v ok=%v, want a %s", tool, x, y, c, ok, kind)
	}
	return c
}

// ax2Frame reports a blueprint of kind with its top-left at (x, y), and
// its width.
func ax2Frame(j *journey, kind string, x, y int) (int, bool) {
	for _, f := range j.world().Frames {
		if f.Kind == kind && f.X == x && f.Y == y {
			return f.W, true
		}
	}
	return 0, false
}

// TestJourneyAX2StructureCards (AX-2): the Architect lays a 2-wide ornate
// door, a column and an animal flap.
func TestJourneyAX2StructureCards(t *testing.T) {
	t.Parallel()
	j := archJourney(t)
	j.g.CompleteResearch() // the ornate door needs Upholstery
	ax2Place(j, "structure", "ornate_door", "ornate door", 10, 2)
	if w, ok := ax2Frame(j, "ornate door", 10, 2); !ok || w != 2 {
		t.Fatalf("ornate door blueprint %v, width %d, want 2", ok, w)
	}
	ax2Place(j, "structure", "column", "column", 13, 2)
	ax2Place(j, "structure", "animal_flap", "animal flap", 15, 2)
	for _, k := range []string{"column", "animal flap"} {
		x := map[string]int{"column": 13, "animal flap": 15}[k]
		if _, ok := ax2Frame(j, k, x, 2); !ok {
			t.Fatalf("no %s blueprint", k)
		}
	}
}

// TestJourneyAX2TableFamily (AX-2): the table card stands for the four
// sizes: each click on it arms the next (1x2, 2x2, 2x4, 3x3), the last
// click puts the tables away, and a placed big table builds that size.
func TestJourneyAX2TableFamily(t *testing.T) {
	t.Parallel()
	j := archJourney(t)
	j.openArch("furniture", "table")
	for _, want := range []string{"table_2x2", "table_2x4", "table_3x3", ""} {
		if !j.clickID("arch.tool.table") {
			t.Fatal("the table card is disabled")
		}
		if j.a.arch.ToolID != want {
			t.Fatalf("armed %q, want %q", j.a.arch.ToolID, want)
		}
	}
	j.openArch("furniture", "table")
	j.clickID("arch.tool.table") // the 2x2
	j.reset()
	j.clickTile(11, 2, inputframe.Left, 0)
	c, ok := j.lastCmd(command.DesignateBuild)
	if id, _ := architect.KindID("table 2x2"); !ok || c.Build != id {
		t.Fatalf("%+v ok=%v, want a 2x2 table", c, ok)
	}
	if w, ok := ax2Frame(j, "table 2x2", 11, 2); !ok || w != 2 {
		t.Fatalf("2x2 table blueprint %v width %d", ok, w)
	}
}

// TestJourneyAX2FurnitureCards (AX-2): the couch, double sleeping spot
// (placed at once), animal sleeping box, small shelf and flood light
// cards, and a wall torch hung on a wall faces away from it by itself.
func TestJourneyAX2FurnitureCards(t *testing.T) {
	t.Parallel()
	j := archJourney(t)
	j.g.CompleteResearch() // couch and small shelf: Upholstery; flood light: its own project
	ax2Place(j, "furniture", "couch", "couch", 10, 1)
	ax2Place(j, "furniture", "double_sleeping_spot", "double sleeping spot", 13, 1)
	if _, ok := t1Building(j, "double sleeping spot", 13, 1); !ok {
		t.Fatal("a double sleeping spot is placed at once")
	}
	ax2Place(j, "furniture", "animal_sleeping_box", "animal sleeping box", 12, 3)
	ax2Place(j, "storage", "small_shelf", "small shelf", 14, 3)
	ax2Place(j, "furniture", "flood_light", "flood light", 10, 3)
	// rows 4-5: clear of the command grid's 75 px cells (UIS-050), whose
	// third row reaches row 6 at 1280x720
	if !j.g.BuildNow("wall", "wood", "south", 11, 4) {
		t.Fatal("no wall")
	}
	c := ax2Place(j, "furniture", "wall_torch", "wall torch", 11, 5)
	if c.Facing != uint8(construction.South) {
		t.Fatalf("wall torch facing %d, want south (away from the wall north of it)", c.Facing)
	}
	if _, ok := ax2Frame(j, "wall torch", 11, 5); !ok {
		t.Fatal("no wall torch blueprint")
	}
	j.reset()
	j.clickTile(14, 5, inputframe.Left, 0) // no wall behind: refused
	if _, ok := ax2Frame(j, "wall torch", 14, 5); ok {
		t.Fatal("a wall torch hung on nothing")
	}
}

// TestJourneyAX2UrnLockedAndMisc (AX-2): the urn card is locked (found,
// never built); the egg box builds from Storage and both steles from Misc.
func TestJourneyAX2UrnLockedAndMisc(t *testing.T) {
	t.Parallel()
	j := archJourney(t)
	j.g.CompleteResearch()
	j.openArch("furniture", "")
	if j.clickID("arch.tool.urn") {
		t.Fatal("the urn card can be armed")
	}
	if len(j.cmdsOf(command.DesignateBuild)) != 0 {
		t.Fatal("the urn card sent a blueprint")
	}
	ax2Place(j, "storage", "egg_box", "egg box", 10, 2)
	ax2Place(j, "misc", "large_stele", "large stele", 12, 2)
	ax2Place(j, "misc", "grand_stele", "grand stele", 14, 1)
}

// TestJourneyAX2PlantPotSow (AX-2): a plant pot's pane picks a rose
// (SetPotPlant), the colonist (a grower) sows it, and the rose's button
// empties the pot again.
func TestJourneyAX2PlantPotSow(t *testing.T) {
	t.Parallel()
	j := archJourney(t)
	if !j.g.BuildNow("plant pot", "wood", "south", 12, 4) {
		t.Fatal("no plant pot")
	}
	pot, _ := t1Building(j, "plant pot", 12, 4)
	j.clickTile(12, 4, inputframe.Left, 0)
	if !j.clickID("cmd.pot.rose") {
		t.Fatal("cmd.pot.rose disabled")
	}
	c, ok := j.lastCmd(command.SetPotPlant)
	if !ok || c.Target != uint64(pot.ID) || c.Build != uint8(construction.PotRose) {
		t.Fatalf("SetPotPlant %+v ok=%v, want a rose in pot %d", c, ok, pot.ID)
	}
	j.tick(1)
	if pot, _ = t1Building(j, "plant pot", 12, 4); pot.AX.Plant != "rose" || pot.AX.PlantPct != 0 {
		t.Fatalf("the pot holds %q at %d%%, want a rose waiting to be sown", pot.AX.Plant, pot.AX.PlantPct)
	}
	j.key("Digit3", 0) // the colonist, a grower, walks over and sows it
	j.until(func() bool { p, _ := t1Building(j, "plant pot", 12, 4); return p.AX.PlantPct > 0 }, 3000)
	j.key("Space", 0)
	j.reset()
	if !j.clickID("cmd.pot.rose") {
		t.Fatal("cmd.pot.rose disabled once sown")
	}
	if c, ok := j.lastCmd(command.SetPotPlant); !ok || c.Build != uint8(construction.PotNone) {
		t.Fatalf("the sown plant's button sent %+v ok=%v, want an empty pot", c, ok)
	}
}
