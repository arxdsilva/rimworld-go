package ebiten

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
)

func TestPlacementCells(t *testing.T) {
	t.Parallel()
	if got := placementCells(placeSingle, 1, 1, 4, 2); len(got) != 1 || got[0] != [2]int{4, 2} {
		t.Fatalf("single = %v", got)
	}
	line := placementCells(placeLine, 2, 2, 6, 3)
	if len(line) != 5 || line[0] != [2]int{2, 2} || line[4] != [2]int{6, 2} {
		t.Fatalf("horizontal line = %v", line)
	}
	if v := placementCells(placeLine, 2, 5, 3, 1); len(v) != 5 || v[0] != [2]int{2, 1} || v[4] != [2]int{2, 5} {
		t.Fatalf("vertical line = %v", v)
	}
	if f := placementCells(placeFill, 3, 3, 1, 2); len(f) != 6 || f[0] != [2]int{1, 2} {
		t.Fatalf("fill = %v", f)
	}
}

func TestRotateFacingWraps(t *testing.T) {
	t.Parallel()
	if rotateFacing(0, -1) != 3 || rotateFacing(3, 1) != 0 || rotateFacing(1, 1) != 2 {
		t.Fatal("rotation should wrap through four facings")
	}
}

func TestArchitectDragPlacesOnlyValidBlueprints(t *testing.T) {
	t.Parallel()
	a, g, _ := liveColony(t, 0)
	if _, err := g.SpawnStack(item.Wood, 5, 6, 4); err != nil {
		t.Fatal(err)
	}
	click(t, a, "tab.architect")
	click(t, a, "arch.cat.structure")
	click(t, a, "arch.tool.wall")
	a.archDrag = true
	a.archX0, a.archY0, a.archX1, a.archY1 = 4, 4, 8, 5 // horizontal line y=4, one cell has wood
	_, cells := a.previewCells()
	bad := 0
	for _, c := range cells {
		if !c.ok {
			bad++
			if c.reason != game.ReasonItem {
				t.Fatalf("reason = %q", c.reason)
			}
		}
	}
	if len(cells) != 5 || bad != 1 {
		t.Fatalf("cells=%d bad=%d", len(cells), bad)
	}
	a.submitArchitect()
	g.Step(1)
	if n := len(g.World().Frames); n != 4 {
		t.Fatalf("frames = %d, want 4 (item cell skipped)", n)
	}
}

func TestArchitectRotationReachesBlueprint(t *testing.T) {
	t.Parallel()
	a, g, _ := liveColony(t, 0)
	click(t, a, "tab.architect")
	click(t, a, "arch.cat.furniture")
	click(t, a, "arch.tool.chair")
	a.rotateTool(1)
	a.archX0, a.archY0, a.archX1, a.archY1 = 6, 6, 6, 6
	a.archDrag = true
	a.submitArchitect()
	g.Step(1)
	f := g.World().Frames
	if len(f) != 1 || f[0].Kind != "chair" || f[0].Facing != "east" {
		t.Fatalf("frames = %+v", f)
	}
}

func TestAreaToolSendsOneRectangleCommand(t *testing.T) {
	t.Parallel()
	sim := &stubSim{w: 10, h: 10}
	a := testApp(sim)
	a.finishBoot()
	a.startGame()
	click(t, a, "tab.architect")
	click(t, a, "arch.cat.zones")
	click(t, a, "arch.tool.stockpile")
	a.archDrag = true
	a.archX0, a.archY0, a.archX1, a.archY1 = 1, 2, 4, 3
	a.submitArchitect()
	if len(sim.cmds) != 1 || sim.cmds[0].Kind != command.DesignateStockpile || sim.cmds[0].X2 != 4 {
		t.Fatalf("cmds = %+v", sim.cmds)
	}
}

func TestDraftButtonAndOrderMenu(t *testing.T) {
	t.Parallel()
	a, g, id := liveColony(t, 0)
	a.renderer.Select(id)
	click(t, a, "pawn.draft")
	g.Step(1)
	if p, _ := a.selectedPawn(); !p.Drafted {
		t.Fatal("draft button should draft the pawn")
	}
	click(t, a, "pawn.draft")
	g.Step(1)
	if p, _ := a.selectedPawn(); p.Drafted {
		t.Fatal("second click should undraft")
	}

	if _, err := g.SpawnStack(item.Food, 3, 6, 2); err != nil {
		t.Fatal(err)
	}
	a.openOrders(6, 2, 300, 200)
	if !a.float.open {
		t.Fatal("order menu should open")
	}
	labels := []string{}
	for _, b := range a.floatButtons() {
		labels = append(labels, b.label)
	}
	joined := strings.Join(labels, "|")
	if !strings.HasPrefix(joined, "Go here") || !strings.Contains(joined, "Eat food") ||
		!strings.Contains(joined, "no stockpile space") {
		t.Fatalf("menu = %q", joined)
	}
	// disabled entry shows its reason and keeps the menu open
	for i, o := range a.float.opts {
		if o.Disabled {
			a.chooseFloat(floatID(i))
			if !a.float.open || a.hint != o.Reason {
				t.Fatalf("disabled click: open=%v hint=%q", a.float.open, a.hint)
			}
		}
	}
	for i, o := range a.float.opts {
		if strings.HasPrefix(o.Label, "Eat") {
			a.chooseFloat(floatID(i))
		}
	}
	if a.float.open {
		t.Fatal("choosing an order closes the menu")
	}
	g.Step(2)
	if p, _ := a.selectedPawn(); !strings.HasPrefix(p.Job, "eat") {
		t.Fatalf("prioritized eat, job = %s", p.Job)
	}
}

func TestEscapeClosesOrderMenuFirst(t *testing.T) {
	t.Parallel()
	a, _, id := liveColony(t, 0)
	a.renderer.Select(id)
	a.openOrders(5, 5, 100, 100)
	a.pressEscape()
	if a.float.open || !a.renderer.HasSelect() {
		t.Fatal("Escape should close the menu and keep the selection")
	}
}

func floatID(i int) string { return "float." + string(rune('0'+i)) }
