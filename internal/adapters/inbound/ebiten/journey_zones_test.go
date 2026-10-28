package ebiten

import (
	"fmt"
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// journey_zones_test.go: zones, areas and pens journeys (QA-001). The debts below are what this
// file still owes the guard (TestEveryInputActionHasAJourney); delete a
// name here once a journey in this file covers it.
var _ = owe("journey_zones_test.go",
	// No Architect tool, button or key emits DesignatePen: painted pens are
	// legacy (zones.go), pens are made with the pen marker building.
	[]string{"DesignatePen"},
	[]string{})

// zoneJourney is a colony with the map cleared of plants and all soil, the
// colonist parked away from the open ground.
func zoneJourney(t *testing.T) *journey {
	t.Helper()
	j := newJourney(t, jPawns([2]int{18, 8}))
	j.g.ClearPlants(0, 0, 23, 23)
	if !j.g.SetGround(0, 0, 23, 23, "soil", "", "") {
		t.Fatal("no soil")
	}
	return j
}

// leaveArch drops the tool and closes the Architect with Escape.
func (j *journey) leaveArch() {
	j.t.Helper()
	for i := 0; i < 2 && j.a.arch.Open; i++ {
		j.key("Escape", 0)
	}
	if j.a.arch.Open {
		j.t.Fatal("the Architect stays open")
	}
}

// zoneAt is the zone holding tile (x, y) in the read model.
func (j *journey) zoneAt(x, y int) (game.ZoneView, bool) { return zoneCellAt(j.world(), x, y) }

// layZone drags kind's tool over (x0, y0)-(x1, y1), leaves the Architect
// and clicks the zone's (x0, y0) cell, so its pane is selected.
func (j *journey) layZone(cat, tool string, x0, y0, x1, y1 int) game.ZoneView {
	j.t.Helper()
	j.openArch(cat, tool)
	j.dragTiles(mbLeft, x0, y0, x1, y1, 0)
	z, ok := j.zoneAt(x0, y0)
	if !ok {
		j.t.Fatalf("no zone at (%d, %d) after the %s drag", x0, y0, tool)
	}
	j.leaveArch()
	j.settle()
	j.clickTile(x0, y0, mbLeft, 0)
	if !j.a.sel.HasZone || j.a.sel.ZoneSel != z.ID {
		j.t.Fatalf("clicking the zone selected %v/%d, want zone %d", j.a.sel.HasZone, j.a.sel.ZoneSel, z.ID)
	}
	return z
}

func TestJourneyZonesGrowZoneWithCrop(t *testing.T) {
	t.Parallel()
	j := zoneJourney(t)
	crop := ""
	for _, c := range zoneSpecies() {
		if c.Name != "potato" && c.Name != "tree" && j.a.lockedBy("crop:"+c.Name) == "" {
			crop = c.Name
			break
		}
	}
	if crop == "" {
		t.Fatal("no second crop is unlocked")
	}
	j.openArch("zones", "grow")
	if !j.clickID("arch.crop." + crop) {
		t.Fatalf("crop %s is disabled", crop)
	}
	j.dragTiles(mbLeft, 8, 2, 10, 3, 0)
	c, ok := j.lastCmd(command.DesignateGrowZone)
	want, _ := game.ZoneSpeciesID(crop)
	if !ok || c.Build != want || c.X != 8 || c.Y != 2 || c.X2 != 10 || c.Y2 != 3 {
		t.Fatalf("DesignateGrowZone %+v ok=%v, want crop %s (%d) over 8,2-10,3", c, ok, crop, want)
	}
	z, ok := j.zoneAt(9, 3)
	if !ok || z.Kind != game.ZoneGrow || z.Crop != crop || len(z.Cells) != 6 {
		t.Fatalf("zone %+v ok=%v, want a 6-cell %s grow zone", z, ok, crop)
	}
}

func TestJourneyZonesStockpileAndDump(t *testing.T) {
	t.Parallel()
	j := zoneJourney(t)
	j.openArch("zones", "stockpile")
	j.dragTiles(mbLeft, 8, 1, 9, 2, 0)
	j.openArch("zones", "dump")
	j.dragTiles(mbLeft, 12, 1, 13, 2, 0)
	cs := j.cmdsOf(command.DesignateStockpile)
	if len(cs) != 2 || cs[0].Build != 0 || cs[1].Build != game.StockpileVariantDump {
		t.Fatalf("DesignateStockpile %+v, want a plain one and a dump", cs)
	}
	if z, ok := j.zoneAt(8, 1); !ok || z.Kind != game.ZoneStockpile {
		t.Fatalf("zone at 8,1: %+v ok=%v", z, ok)
	}
	if z, ok := j.zoneAt(12, 1); !ok || z.Kind != game.ZoneDump {
		t.Fatalf("zone at 12,1: %+v ok=%v", z, ok)
	}
}

func homeBit(j *journey, x, y int) bool {
	w := j.world()
	return w.Home[y*w.TerrainW+x] != 0
}

// TestJourneyZonesHomeRoofSnowArea: the Home, roof and snow tools send
// their rectangles; Home and snow clearing change the read model.
func TestJourneyZonesHomeRoofSnowArea(t *testing.T) {
	t.Parallel()
	j := zoneJourney(t)
	j.openArch("zones", "unhome")
	j.dragTiles(mbLeft, 8, 2, 9, 3, 0)
	if c, ok := j.lastCmd(command.DesignateHome); !ok || c.Enabled || homeBit(j, 8, 2) {
		t.Fatalf("unhome sent %+v ok=%v, Home bit %v", c, ok, homeBit(j, 8, 2))
	}
	j.openArch("zones", "home")
	j.dragTiles(mbLeft, 8, 2, 9, 3, 0)
	if c, ok := j.lastCmd(command.DesignateHome); !ok || !c.Enabled || c.X != 8 || c.X2 != 9 || !homeBit(j, 9, 3) {
		t.Fatalf("home sent %+v ok=%v, Home bit %v", c, ok, homeBit(j, 9, 3))
	}
	j.openArch("zones", "roof")
	j.dragTiles(mbLeft, 14, 2, 16, 3, 0)
	if c, ok := j.lastCmd(command.DesignateRoof); !ok || c.X != 14 || c.Y2 != 3 {
		t.Fatalf("roof sent %+v ok=%v", c, ok)
	}
	j.openArch("zones", "noroof")
	j.dragTiles(mbLeft, 14, 2, 16, 3, 0)
	if c, ok := j.lastCmd(command.DesignateRemoveRoof); !ok || c.X != 14 || c.Y2 != 3 {
		t.Fatalf("noroof sent %+v ok=%v", c, ok)
	}
	j.openArch("zones", "snow_clear")
	j.dragTiles(mbLeft, 8, 6, 10, 6, 0)
	if c, ok := j.lastCmd(command.DesignateSnowClear); !ok || !c.Enabled || c.X != 8 || c.X2 != 10 || c.Y != 6 {
		t.Fatalf("snow_clear sent %+v ok=%v", c, ok)
	}
}

// TestJourneyZonesAllowedAreaAndAreas: the whole area life through the UI:
// create, paint with the allowed-area tool, invert, rename, assign to a
// colonist from the Schedule tab, delete.
func TestJourneyZonesAllowedAreaAndAreas(t *testing.T) {
	t.Parallel()
	j := zoneJourney(t)
	pawn := j.g.Snapshot()[0].ID

	// Create: Architect > Zones > Expand allowed area > Manage > New area.
	j.key("Tab", 0)
	j.clickID("arch.cat.zones")
	j.clickID("arch.tool.allowed_area")
	if !j.clickID("areapick.manage") {
		t.Fatal("no Manage row")
	}
	if !j.clickID("modal.colony.areas.areas.new") {
		t.Fatal("no New area button")
	}
	if _, ok := j.lastCmd(command.CreateArea); !ok {
		t.Fatal("no CreateArea")
	}
	areas := j.g.Areas()
	if len(areas) != 1 {
		t.Fatalf("%d areas, want 1", len(areas))
	}
	id := int(areas[0].ID)

	// Invert (empty becomes everything), rename, then close the manager.
	j.clickID(fmt.Sprintf("areas.invert.%d", id))
	if c, ok := j.lastCmd(command.InvertArea); !ok || c.Index != id {
		t.Fatalf("InvertArea %+v ok=%v", c, ok)
	}
	if got := j.g.Areas()[0].Cells; got != 24*24 {
		t.Fatalf("inverted area holds %d cells, want the whole map", got)
	}
	field := fmt.Sprintf("areas.name.%d", id)
	j.clickID(field)
	for i := 0; i < 12; i++ {
		j.key("Backspace", 0)
	}
	j.typeText("Orchard")
	j.key("Enter", 0)
	j.update(1) // the field reports the commit on the next layout
	if c, ok := j.lastCmd(command.RenameArea); !ok || c.Index != id || c.Name != "Orchard" {
		t.Fatalf("RenameArea %+v ok=%v", c, ok)
	}
	j.clickID("modal.colony.areas.close")

	// Paint with the tool: pick the area, drag.
	j.clickID("arch.tool.allowed_area")
	if j.a.arch.ToolID == "allowed_area" {
		j.clickID("arch.tool.allowed_area") // the first click armed nothing: the picker is up
	}
	j.clickID(fmt.Sprintf("areapick.row.%d", id))
	if j.a.arch.ToolID != "allowed_area" {
		t.Fatalf("armed %q, want allowed_area", j.a.arch.ToolID)
	}
	j.dragTiles(mbLeft, 8, 2, 9, 3, 0)
	if c, ok := j.lastCmd(command.PaintArea); !ok || c.Index != id || !c.Enabled || c.X != 8 || c.Y2 != 3 {
		t.Fatalf("PaintArea %+v ok=%v", c, ok)
	}
	j.leaveArch()

	// Assign from the Schedule tab's dropdown.
	j.key("F2", 0)
	dd := fmt.Sprintf("sched.area.%d", pawn)
	if !j.clickID(dd) {
		t.Fatal("dropdown disabled")
	}
	j.clickID(fmt.Sprintf("areapick.row.%d", id))
	if c, ok := j.lastCmd(command.AssignArea); !ok || c.Pawn != uint64(pawn) || c.Index != id {
		t.Fatalf("AssignArea %+v ok=%v", c, ok)
	}
	if got := j.g.AreaOf(pawn); got != id {
		t.Fatalf("colonist's area %d, want %d", got, id)
	}
	// Unrestrict again so deleting needs no confirmation.
	j.clickID(dd)
	j.clickID("areapick.row.-1")
	if got := j.g.AreaOf(pawn); got == id {
		t.Fatal("still assigned")
	}
	j.clickID(dd)
	j.clickID("areapick.manage")
	j.clickID(fmt.Sprintf("areas.delete.%d", id))
	if c, ok := j.lastCmd(command.DeleteArea); !ok || c.Index != id {
		t.Fatalf("DeleteArea %+v ok=%v", c, ok)
	}
	if n := len(j.g.Areas()); n != 0 {
		t.Fatalf("%d areas left", n)
	}
}

// TestJourneyZonesPen: a pen marker's Pen tab sends the pen's settings; a
// feeding trough's button switches its filter.
func TestJourneyZonesPen(t *testing.T) {
	t.Parallel()
	j := zoneJourney(t)
	if !j.g.BuildNow("pen marker", "wood", "south", 9, 3) {
		t.Fatal("no pen marker")
	}
	if !j.g.BuildNow("feeding trough", "wood", "south", 13, 3) {
		t.Fatal("no trough")
	}
	j.clickTile(9, 3, mbLeft, 0)
	if !j.clickID("inspector.pen") {
		t.Fatal("no Pen tab")
	}
	if !j.clickID("bldg.pen.none") {
		t.Fatal("no None button")
	}
	if c, ok := j.lastCmd(command.SetPenSettings); !ok || c.Name == "" {
		t.Fatalf("SetPenSettings %+v ok=%v", c, ok)
	}
	j.reset()
	if !j.clickID("bldg.pen.cut.tree") {
		t.Fatal("no auto-cut row")
	}
	c, ok := j.lastCmd(command.SetPenSettings)
	if !ok || !strings.Contains(c.Name, "tree") {
		t.Fatalf("auto-cut SetPenSettings %+v ok=%v", c, ok)
	}

	j.key("Escape", 0)
	j.settle()
	j.clickTile(13, 3, mbLeft, 0)
	if !j.clickID("cmd.bldg.trough") {
		t.Fatal("trough button disabled")
	}
	tc, ok := j.lastCmd(command.SetTroughFilter)
	if !ok || tc.Mode > 2 {
		t.Fatalf("SetTroughFilter %+v ok=%v", tc, ok)
	}
}

// TestJourneyZonesZonePaneCommands: a zone's pane buttons each send their
// command; Delete removes the zone.
func TestJourneyZonesZonePaneCommands(t *testing.T) {
	t.Parallel()
	j := zoneJourney(t)
	grow := j.layZone("zones", "grow", 8, 1, 10, 2)

	j.clickID("cmd.zone.crop")
	crop := ""
	for _, c := range zoneSpecies() {
		if c.Name != grow.Crop && c.Name != "tree" && j.a.lockedBy("crop:"+c.Name) == "" {
			crop = c.Name
			break
		}
	}
	j.pick(cropLabel(crop))
	want, _ := game.ZoneSpeciesID(crop)
	if c, ok := j.lastCmd(command.SetZoneCrop); !ok || c.Target != uint64(grow.ID) || c.Build != want {
		t.Fatalf("SetZoneCrop %+v ok=%v, want %s (%d)", c, ok, crop, want)
	}
	if z, _ := j.zoneAt(8, 1); z.Crop != crop {
		t.Fatalf("zone crop %q, want %q", z.Crop, crop)
	}

	toggle := func(btn string, kind command.Kind, field func(game.ZoneView) bool) {
		t.Helper()
		before := field(mustZone(t, j, 8, 1))
		j.clickID(btn)
		c, ok := j.lastCmd(kind)
		if !ok || c.Target != uint64(grow.ID) {
			t.Fatalf("%s sent %+v ok=%v", btn, c, ok)
		}
		if field(mustZone(t, j, 8, 1)) == before {
			t.Fatalf("%s changed nothing in the zone", btn)
		}
	}
	toggle("cmd.zone.sow", command.SetZoneSow, func(z game.ZoneView) bool { return z.NoSow })
	toggle("cmd.zone.cut", command.SetZoneCut, func(z game.ZoneView) bool { return z.NoCut })
	toggle("cmd.zone.hide", command.SetZoneHidden, func(z game.ZoneView) bool { return z.Hidden })

	j.clickID("cmd.zone.delete")
	if c, ok := j.lastCmd(command.DeleteZone); !ok || c.Target != uint64(grow.ID) {
		t.Fatalf("DeleteZone %+v ok=%v", c, ok)
	}
	if _, ok := j.zoneAt(8, 1); ok {
		t.Fatal("the zone survived Delete")
	}

	// A stockpile has a priority picker instead of the crop and toggles.
	sp := j.layZone("zones", "stockpile", 8, 4, 9, 5)
	j.clickID("cmd.zone.priority")
	names := game.StoragePriorities()
	idx := 0
	if names[0] == sp.Priority {
		idx = 1
	}
	j.pick(names[idx])
	if c, ok := j.lastCmd(command.SetZonePriority); !ok || c.Target != uint64(sp.ID) || c.Index != idx {
		t.Fatalf("SetZonePriority %+v ok=%v, want level %d", c, ok, idx)
	}
}

func mustZone(t *testing.T, j *journey, x, y int) game.ZoneView {
	t.Helper()
	z, ok := j.zoneAt(x, y)
	if !ok {
		t.Fatalf("no zone at (%d, %d)", x, y)
	}
	return z
}

// TestJourneyZonesZoneExpandShrink: the Expand and Shrink tools take a
// drag each.
func TestJourneyZonesZoneExpandShrink(t *testing.T) {
	t.Parallel()
	j := zoneJourney(t)
	z := j.layZone("zones", "stockpile", 8, 2, 9, 3)
	j.clickID("cmd.zone.expand")
	j.update(1)
	j.dragTiles(mbLeft, 10, 2, 11, 3, 0)
	c, ok := j.lastCmd(command.ExpandZone)
	if !ok || c.Target != uint64(z.ID) || c.X != 10 || c.X2 != 11 || c.Y != 2 || c.Y2 != 3 {
		t.Fatalf("ExpandZone %+v ok=%v", c, ok)
	}
	if got := len(mustZone(t, j, 8, 2).Cells); got != 8 {
		t.Fatalf("zone has %d cells after the expand, want 8", got)
	}
	j.key("Escape", 0) // ends the tool, keeps the selection
	j.clickID("cmd.zone.shrink")
	j.update(1)
	j.dragTiles(mbLeft, 10, 2, 11, 3, 0)
	c, ok = j.lastCmd(command.RemoveZoneCells)
	if !ok || c.Target != uint64(z.ID) || c.X != 10 || c.X2 != 11 {
		t.Fatalf("RemoveZoneCells %+v ok=%v", c, ok)
	}
	if got := len(mustZone(t, j, 8, 2).Cells); got != 4 {
		t.Fatalf("zone has %d cells after the shrink, want 4", got)
	}
}

// TestJourneyZonesZoneRename: the Rename button opens the name field;
// typing and Enter send the name.
func TestJourneyZonesZoneRename(t *testing.T) {
	t.Parallel()
	j := zoneJourney(t)
	z := j.layZone("zones", "grow", 8, 2, 10, 3)
	j.clickID("cmd.zone.rename")
	j.update(1)
	j.typeText("North field")
	j.key("Enter", 0)
	j.update(1) // the field reports the commit on the next layout
	c, ok := j.lastCmd(command.RenameZone)
	if !ok || c.Target != uint64(z.ID) || c.Name != "North field" {
		t.Fatalf("RenameZone %+v ok=%v", c, ok)
	}
	if got := mustZone(t, j, 8, 2).Name; got != "North field" {
		t.Fatalf("zone name %q", got)
	}
}
