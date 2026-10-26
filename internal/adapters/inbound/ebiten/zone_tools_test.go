package ebiten

import (
	"slices"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// PLAY-020 ZONEUI: the Expand and Shrink tools (zone_tools.go).

// toolApp is a colony with the growing zone 1 (columns 8..11, rows 2..4)
// selected and, touching it, the stockpile 2 (columns 12..13, same rows).
// Columns 8..16 and rows 1..5 are open map.
func toolApp(t *testing.T) (*App, *zoneSim, *fakeInput) {
	t.Helper()
	a, s, f := zoneApp(t,
		growZone(1, "potato", 8, 2, 11, 4),
		rectZone(2, game.ZoneStockpile, 12, 2, 13, 4))
	a.selectCand(selCand{selZone, 1})
	f.frame(t, a)
	for _, p := range [][2]int{{8, 2}, {16, 5}, {8, 5}} {
		if mx, my := a.renderer.worldToScreenF(float64(p[0])+0.5, float64(p[1])+0.5); a.colonyChromeHit(int(mx), int(my), a.sw, a.sh) {
			t.Fatalf("tile %v is under the HUD", p)
		}
	}
	return a, s, f
}

// armed starts tool k and lets its opening frame pass.
func armed(t *testing.T, a *App, f *fakeInput, k zoneToolKind) {
	t.Helper()
	a.startZoneTool(k)
	f.frame(t, a)
	if !a.zoneToolOn(k) {
		t.Fatalf("tool %v is not on", k)
	}
}

func cellsOf(gs []ghostCell) [][2]int {
	var out [][2]int
	for _, g := range gs {
		out = append(out, [2]int{g.x, g.y})
	}
	return out
}

func TestZoneToolGatesTheMapMouse(t *testing.T) {
	t.Parallel()
	a, _, f := toolApp(t)
	if a.handleZoneTool() {
		t.Fatal("no tool, no gate")
	}
	armed(t, a, f, zoneToolExpand)
	if !a.handleZoneTool() {
		t.Fatal("an active tool owns the map mouse")
	}
	// A click on empty ground with the tool on must not select anything.
	f.moveTo(a, 15, 5)
	f.click(t, a, mbLeft, 0)
	if a.renderer.HasSelect() || a.hasPlant || !a.hasZone {
		t.Fatal("the tool took the click: the selection must not change")
	}
}

func TestZoneToolSwallowsTheOpeningClick(t *testing.T) {
	t.Parallel()
	a, s, f := toolApp(t)
	a.startZoneTool(zoneToolExpand)
	// The very next frame carries a left press on the map (the click that
	// pressed the button): it starts no drag.
	f.moveTo(a, 15, 3)
	f.cur.press[mbLeft], f.cur.down[mbLeft] = true, true
	f.frame(t, a)
	if a.zoneSt().drag {
		t.Fatal("the opening click started a drag")
	}
	f.cur.down[mbLeft], f.cur.release[mbLeft] = false, true
	f.frame(t, a)
	if len(s.cmds) != 0 {
		t.Fatalf("the opening click sent %+v", s.cmds)
	}
	// The next press does start one.
	f.cur.press[mbLeft], f.cur.down[mbLeft] = true, true
	f.frame(t, a)
	if !a.zoneSt().drag {
		t.Fatal("a later press starts the drag")
	}
}

func TestExpandSendsOneCommandPerDragAndStaysOn(t *testing.T) {
	t.Parallel()
	a, s, f := toolApp(t)
	armed(t, a, f, zoneToolExpand)
	f.drag(t, a, mbLeft, 14, 2, 16, 4, 0) // three columns east of the stockpile
	got := cmdsOf(s.stubSim, command.ExpandZone)
	if len(got) != 1 {
		t.Fatalf("%d ExpandZone commands, want one per drag (all: %+v)", len(got), s.cmds)
	}
	if c := got[0]; c.Target != 1 || c.X != 14 || c.Y != 2 || c.X2 != 16 || c.Y2 != 4 {
		t.Fatalf("expand %+v, want zone 1 rect (14,2)-(16,4)", c)
	}
	if !a.zoneToolOn(zoneToolExpand) {
		t.Fatal("the tool stays on after a drag")
	}
	// A drag the other way round is normalised; a second drag is a second command.
	f.drag(t, a, mbLeft, 16, 5, 15, 5, 0)
	got = cmdsOf(s.stubSim, command.ExpandZone)
	if len(got) != 2 || got[1].X != 15 || got[1].X2 != 16 || got[1].Y != 5 || got[1].Y2 != 5 {
		t.Fatalf("second drag: %+v", got)
	}
	// A single click adds one tile.
	f.moveTo(a, 9, 5)
	f.click(t, a, mbLeft, 0)
	got = cmdsOf(s.stubSim, command.ExpandZone)
	if len(got) != 3 || got[2].X != 9 || got[2].X2 != 9 || got[2].Y != 5 || got[2].Y2 != 5 {
		t.Fatalf("click: %+v", got)
	}
}

func TestExpandGhostIsPerCellGreenOrRedByTheZoneRules(t *testing.T) {
	t.Parallel()
	a, s, f := toolApp(t)
	s.deny = func(x, y int) bool { return x == 15 } // a column the kind cannot claim
	armed(t, a, f, zoneToolExpand)
	z := zoneByIDOrFail(t, a, 1)
	// Hovering shows the tile under the pointer.
	f.moveTo(a, 14, 3)
	f.frame(t, a)
	if got := a.zoneToolCells(z); len(got) != 1 || got[0] != (ghostCell{x: 14, y: 3, ok: true}) {
		t.Fatalf("hover ghost %+v, want the one valid tile", got)
	}
	// Mid-drag the ghost is the rectangle, each cell judged on its own.
	f.cur.press[mbLeft], f.cur.down[mbLeft] = true, true
	f.frame(t, a)
	f.moveTo(a, 16, 4)
	f.frame(t, a)
	got := a.zoneToolCells(z)
	if len(got) != 6 {
		t.Fatalf("drag ghost has %d cells, want 3 columns x 2 rows: %+v", len(got), got)
	}
	for _, g := range got {
		if want := g.x != 15; g.ok != want {
			t.Errorf("cell (%d,%d) ok=%v, want %v", g.x, g.y, g.ok, want)
		}
	}
	// Let go: one command for the whole rectangle (the sim skips the red cell).
	f.cur.down[mbLeft], f.cur.release[mbLeft] = false, true
	f.frame(t, a)
	if got := cmdsOf(s.stubSim, command.ExpandZone); len(got) != 1 || got[0].X != 14 || got[0].X2 != 16 {
		t.Fatalf("expand %+v", got)
	}
	// Nothing valid in the rectangle: no command, and the reason is hinted.
	f.drag(t, a, mbLeft, 15, 2, 15, 4, 0)
	if n := len(cmdsOf(s.stubSim, command.ExpandZone)); n != 1 {
		t.Fatalf("a drag over only invalid tiles sent a command (%d total)", n)
	}
	if a.hint != "nothing to add there" {
		t.Fatalf("hint %q", a.hint)
	}
}

func TestShrinkGhostAndCommandOnlyTouchTheSelectedZone(t *testing.T) {
	t.Parallel()
	a, s, f := toolApp(t)
	armed(t, a, f, zoneToolShrink)
	z := zoneByIDOrFail(t, a, 1)
	// Drag across the grow zone's east half and the stockpile beside it.
	f.cur.mods = 0
	f.moveTo(a, 10, 2)
	f.cur.press[mbLeft], f.cur.down[mbLeft] = true, true
	f.frame(t, a)
	f.moveTo(a, 13, 3)
	f.frame(t, a)
	got := cellsOf(a.zoneToolCells(z))
	want := [][2]int{{10, 2}, {11, 2}, {10, 3}, {11, 3}} // the stockpile's tiles are not tinted
	slices.SortFunc(got, func(p, q [2]int) int {
		if p[1] != q[1] {
			return p[1] - q[1]
		}
		return p[0] - q[0]
	})
	if !slices.Equal(got, want) {
		t.Fatalf("shrink ghost %v, want only the grow zone's tiles %v", got, want)
	}
	for _, g := range a.zoneToolCells(z) {
		if g.ok {
			t.Errorf("cell (%d,%d) is drawn as an addition", g.x, g.y)
		}
	}
	f.cur.down[mbLeft], f.cur.release[mbLeft] = false, true
	f.frame(t, a)
	rm := cmdsOf(s.stubSim, command.RemoveZoneCells)
	if len(rm) != 1 || rm[0].Target != 1 || rm[0].X != 10 || rm[0].Y != 2 || rm[0].X2 != 13 || rm[0].Y2 != 3 {
		t.Fatalf("shrink sent %+v, want one RemoveZoneCells on zone 1 for the whole rectangle", rm)
	}
	if !a.zoneToolOn(zoneToolShrink) {
		t.Fatal("the tool stays on after a drag")
	}
}

func TestShrinkSingleClickRemovesOneCell(t *testing.T) {
	t.Parallel()
	a, s, f := toolApp(t)
	armed(t, a, f, zoneToolShrink)
	f.moveTo(a, 9, 3)
	f.click(t, a, mbLeft, 0)
	rm := cmdsOf(s.stubSim, command.RemoveZoneCells)
	if len(rm) != 1 || rm[0] != (command.Command{Kind: command.RemoveZoneCells, Target: 1, X: 9, Y: 3, X2: 9, Y2: 3}) {
		t.Fatalf("a click sent %+v, want one cell", rm)
	}
	// A click outside the zone changes nothing and says so.
	f.moveTo(a, 15, 5)
	f.click(t, a, mbLeft, 0)
	if n := len(cmdsOf(s.stubSim, command.RemoveZoneCells)); n != 1 {
		t.Fatalf("a click outside the zone sent a command (%d total)", n)
	}
	if !strings.Contains(a.hint, "tiles there") {
		t.Fatalf("hint %q", a.hint)
	}
}

func TestZoneToolRightClickAndEscapeExit(t *testing.T) {
	t.Parallel()
	for _, how := range []string{"right-click", "escape"} {
		a, s, f := toolApp(t)
		armed(t, a, f, zoneToolExpand)
		f.moveTo(a, 14, 3)
		switch how {
		case "right-click":
			f.click(t, a, mbRight, 0)
		case "escape":
			f.key(t, a, "Escape", 0)
		}
		if a.zoneToolOn(zoneToolExpand) {
			t.Fatalf("%s leaves the tool on", how)
		}
		if !a.hasZone {
			t.Fatalf("%s must only end the tool, the zone stays selected", how)
		}
		if a.handleZoneTool() {
			t.Fatalf("%s: the map mouse is free again", how)
		}
		if len(s.cmds) != 0 {
			t.Fatalf("%s sent %+v", how, s.cmds)
		}
		// A second Escape then clears the zone.
		if how == "escape" {
			f.key(t, a, "Escape", 0)
			if a.hasZone {
				t.Fatal("the next Escape clears the selection")
			}
		}
	}
}

func TestRightClickDuringADragOnlyDropsTheDrag(t *testing.T) {
	t.Parallel()
	a, s, f := toolApp(t)
	armed(t, a, f, zoneToolShrink)
	f.moveTo(a, 9, 3)
	f.cur.press[mbLeft], f.cur.down[mbLeft] = true, true
	f.frame(t, a)
	f.cur.press[mbRight] = true
	f.frame(t, a)
	if a.zoneSt().drag || !a.zoneToolOn(zoneToolShrink) {
		t.Fatalf("drag=%v tool on=%v, want the drag dropped and the tool kept", a.zoneSt().drag, a.zoneToolOn(zoneToolShrink))
	}
	f.cur.down[mbLeft], f.cur.release[mbLeft] = false, true
	f.frame(t, a)
	if len(s.cmds) != 0 {
		t.Fatalf("a dropped drag sent %+v", s.cmds)
	}
}

func TestZoneToolEndsWhenTheZoneVanishesOrAnotherPanelOpens(t *testing.T) {
	t.Parallel()
	a, s, f := toolApp(t)
	armed(t, a, f, zoneToolShrink)
	s.world.Zones = s.world.Zones[1:] // zone 1 is gone
	f.frame(t, a)
	if a.hasZone || a.zoneToolOn(zoneToolShrink) {
		t.Fatalf("zone selected=%v tool on=%v, want both gone", a.hasZone, a.zoneToolOn(zoneToolShrink))
	}
	a, _, f = toolApp(t)
	armed(t, a, f, zoneToolExpand)
	a.openArchitect()
	f.frame(t, a)
	if a.zoneToolOn(zoneToolExpand) {
		t.Fatal("opening the Architect ends the zone tool")
	}
}

func TestSelectingSomethingElseEndsTheTool(t *testing.T) {
	t.Parallel()
	a, _, f := toolApp(t)
	armed(t, a, f, zoneToolExpand)
	a.renderer.Select(1)
	a.clearSelections(selPawn)
	if a.zoneToolOn(zoneToolExpand) {
		t.Fatal("deselecting the zone ends its tool")
	}
	// And the tool of one zone is not the next zone's.
	a.selectCand(selCand{selZone, 2})
	if a.zoneToolOn(zoneToolExpand) {
		t.Fatal("selecting another zone starts with no tool")
	}
}

func TestZoneToolHintNamesTheZone(t *testing.T) {
	t.Parallel()
	z := growZone(1, "potato", 8, 2, 11, 4)
	if got := zoneToolHint(zoneToolExpand, z); got != "Expand Growing zone 1: drag to add tiles · right-click or Esc: done" {
		t.Errorf("expand hint %q", got)
	}
	z.Name = "North field"
	if got := zoneToolHint(zoneToolShrink, z); got != "Shrink North field: drag to remove tiles · right-click or Esc: done" {
		t.Errorf("shrink hint %q", got)
	}
}

func TestZoneToolDrawsItsHintAndIcon(t *testing.T) {
	a, _, f := toolApp(t)
	armed(t, a, f, zoneToolShrink)
	f.moveTo(a, 9, 3)
	f.frame(t, a)
	got := wlTrace(t)
	a.drawZoneSelection(ebiten.NewImage(64, 64))
	var texts []string
	for _, e := range wlOfKind(*got, "text") {
		texts = append(texts, e.Text)
	}
	if !slices.ContainsFunc(texts, func(s string) bool { return strings.HasPrefix(s, "Shrink Growing zone 1: drag to remove tiles") }) {
		t.Fatalf("the hint is not drawn through the canvas: %q", texts)
	}
	// Over the HUD (the pane) it draws nothing.
	*got = (*got)[:0]
	f.cur.mx, f.cur.my = 20, a.sh-20
	f.frame(t, a)
	a.drawZoneSelection(ebiten.NewImage(64, 64))
	for _, e := range wlOfKind(*got, "text") {
		if strings.HasPrefix(e.Text, "Shrink") {
			t.Fatalf("the hint follows the pointer onto the HUD: %q", e.Text)
		}
	}
}
