package architect

import (
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

func TestZoneToolHintNamesTheZone(t *testing.T) {
	t.Parallel()
	z := game.ZoneView{ID: 1, Kind: game.ZoneGrow, Crop: "potato"}
	if got := ZoneToolHint(ZoneToolExpand, z); got != "Expand Growing zone 1: drag to add tiles · right-click or Esc: done" {
		t.Errorf("expand hint %q", got)
	}
	z.Name = "North field"
	if got := ZoneToolHint(ZoneToolShrink, z); got != "Shrink North field: drag to remove tiles · right-click or Esc: done" {
		t.Errorf("shrink hint %q", got)
	}
}

func TestZoneDragRefusalIsTheCommonestReason(t *testing.T) {
	t.Parallel()
	why := func(x, y int) string {
		switch {
		case x == 2 && y == 2:
			return "Water"
		case x >= 1:
			return "Rock"
		}
		return "Water"
	}
	if r, free := ZoneDragRefusal(0, 0, 2, 2, why); free || r != "Rock" {
		t.Fatalf("refusal %q free %v", r, free)
	}
	if _, free := ZoneDragRefusal(2, 2, 0, 0, func(x, y int) string {
		if x == 1 && y == 1 {
			return ""
		}
		return "Rock"
	}); !free {
		t.Fatal("one free cell frees the drag")
	}
	for why, want := range map[string]string{"": "No free tiles there", ZoneNoReason: "No free tiles there", "Rock": "Can't zone here: Rock"} {
		if got := ZoneDragHint(why); got != want {
			t.Errorf("hint for %q = %q", why, got)
		}
	}
	for why, want := range map[string]string{"": "", ZoneNoReason: "Can't zone here", "Rock": "Can't zone: Rock"} {
		if got := ZoneTipText(why); got != want {
			t.Errorf("tip for %q = %q", why, got)
		}
	}
}

func TestZoneGhostCellsClipToTheView(t *testing.T) {
	t.Parallel()
	cells := ZoneGhostCells(5, 5, -5, -5, [4]int{0, 0, 2, 1}, func(x, y int) bool { return x != 1 })
	if len(cells) != 6 || cells[0] != (GhostCell{X: 0, Y: 0, OK: true}) || cells[1].OK {
		t.Fatalf("cells %+v", cells)
	}
	if AnyZoneCell(0, 0, 3, 3, func(x, y int) bool { return false }) || !AnyZoneCell(3, 3, 0, 0, func(x, y int) bool { return x == 0 && y == 3 }) {
		t.Fatal("AnyZoneCell")
	}
}

func TestZoneKindOfTools(t *testing.T) {
	t.Parallel()
	for id, want := range map[string]string{"stockpile": game.ZoneStockpile, "dump": game.ZoneDump, "grow": game.ZoneGrow, "wall": ""} {
		d, _ := ToolByID(id)
		if got := ZoneKindOf(d); got != want {
			t.Errorf("%s zone kind %q, want %q", id, got, want)
		}
	}
	if ZoneKindName(game.ZoneView{Kind: game.ZonePen}) != "Pen" || ZonePriorityName(game.ZoneView{}) != "Normal" || ZoneCropName("") != "nothing" {
		t.Error("zone names")
	}
}

// TestZoneToolStepsAndSubmits: a drag on the Shrink tool sends one
// command for the zone's own cells; one over none of them only hints.
func TestZoneToolStepsAndSubmits(t *testing.T) {
	t.Parallel()
	tl, f := newFake(&fakeSim{})
	z := tl.Zones
	z.Start(ZoneToolExpand)
	if !z.On(ZoneToolExpand) || !z.State().Fresh || f.floats != 1 {
		t.Fatalf("start: %+v", z.State())
	}
	z.Start(ZoneToolExpand)
	if z.On(ZoneToolExpand) {
		t.Fatal("the armed tool's button turns it off")
	}
	z.Start(ZoneToolExpand)
	zv := game.ZoneView{ID: 4, Kind: game.ZoneGrow}
	z.step(zv, 1, 1, true, true, true, false, false)
	z.step(zv, 3, 2, true, false, true, true, false)
	if len(f.cmds) != 1 || f.cmds[0].Kind != command.ExpandZone || f.cmds[0].X2 != 3 || f.cmds[0].Target != 4 {
		t.Fatalf("expand sent %+v", f.cmds)
	}
	z.step(zv, 0, 0, true, true, true, false, false)
	z.step(zv, 0, 0, true, false, false, false, true)
	if z.State().Drag || !z.On(ZoneToolExpand) {
		t.Fatal("right-click drops the drag, keeps the tool")
	}
	z.step(zv, 0, 0, true, false, false, false, true)
	if z.On(ZoneToolExpand) || z.Escape() {
		t.Fatal("a second right-click ends the tool; Esc has nothing left")
	}
	z.NoteCell([2]int{2, 2})
	if !z.st.hasCell || z.st.cell != [2]int{2, 2} {
		t.Fatal("NoteCell")
	}
}
