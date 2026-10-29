package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
)

// scaledColony is a live colony in a 1920x1080 window at a fixed 150%
// UI scale (logical 1280x720), driven by a fake input. Not parallel.
func scaledColony(t *testing.T) (*App, *recSim, *fakeInput) {
	t.Helper()
	g := game.New(game.Config{Seed: 1, Width: 16, Height: 16})
	if _, err := g.SpawnPawn(2, 2); err != nil {
		t.Fatal(err)
	}
	rs := &recSim{Simulation: g, moveOK: true}
	t.Cleanup(func() { gfx.SetScale(gfx.UnitScale) })
	a := testApp(rs)
	a.devFn = func() float64 { return 1 }
	s := a.settings
	s.Accessibility.UIScaleAuto, s.Accessibility.UIScale = false, 150
	a.setSettings(s)
	a.LayoutF(1920, 1080)
	a.finishBoot()
	a.startGame()
	if a.renderer == nil || a.scale.S != 1.5 {
		t.Fatalf("renderer=%v scale=%v", a.renderer != nil, a.scale.S)
	}
	return a, rs, fakeOn(a)
}

// clickAtLogical presses the left button at the logical point's raw
// (physical) position for one Update.
func clickAtLogical(t *testing.T, a *App, f *fakeInput, x, y float32) {
	t.Helper()
	f.Cur.Mx, f.Cur.My = int(float64(x)*a.scale.S), int(float64(y)*a.scale.S)
	f.Cur.Press[inputframe.Left], f.Cur.Down[inputframe.Left] = true, true
	if err := a.Update(); err != nil {
		t.Fatal(err)
	}
	f.Cur.Press[inputframe.Left], f.Cur.Down[inputframe.Left] = false, false
}

func TestArchitectCardClickAtScale(t *testing.T) {
	a, _, f := scaledColony(t)
	a.arch.OpenPanel() // opens on Structure
	card, ok := a.arch.ToolCell("wall")
	if !ok {
		t.Fatalf("no wall cell in the command grid (%d cells for %d tools)", len(a.hud().GridCells), len(a.archToolCommands()))
	}
	// The grid is laid out in logical pixels: the click lands on the
	// cell's logical centre x 1.5 and selects the tool.
	clickAtLogical(t, a, f, card.X+card.W/2, card.Y+card.H/2)
	if a.arch.ToolID != "wall" {
		t.Fatalf("archTool = %q after a tool click at s=1.5, want wall", a.arch.ToolID)
	}
}

func TestArchitectCategoryClickAtScale(t *testing.T) {
	a, _, f := scaledColony(t)
	a.arch.OpenPanel()
	var cat uikit.Button
	for _, b := range a.arch.Buttons() {
		if b.ID == "arch.cat.zones" {
			cat = b
		}
	}
	if cat.ID == "" {
		t.Fatal("no Zones button in the category panel")
	}
	l := a.hud()
	if !l.ArchCats.Contains(cat.X+cat.W/2, cat.Y+cat.H/2) {
		t.Fatalf("Zones button %+v is outside ArchCats %+v", cat, l.ArchCats)
	}
	clickAtLogical(t, a, f, cat.X+cat.W/2, cat.Y+cat.H/2)
	if a.arch.Cat != "zones" {
		t.Fatalf("archCat = %q after a category click at s=1.5, want zones", a.arch.Cat)
	}
}

func TestGhostDragMapsToTilesAtScale(t *testing.T) {
	a, _, _ := scaledColony(t)
	a.arch.OpenPanel()
	a.arch.ToolID = "wall"
	// World mapping uses the raw physical cursor: the UI scale must not
	// change which tile is under the pointer.
	a.cursor = &[2]int{400, 300}
	px, py := a.mapCursor()
	if px != 600 || py != 450 {
		t.Fatalf("mapCursor = (%d,%d), want (600,450)", px, py)
	}
	wantX, wantY := a.renderer.TileAt(px, py)
	x0, y0, x1, y1 := a.arch.DragSpan()
	if x0 != wantX || y0 != wantY || x1 != wantX || y1 != wantY {
		t.Fatalf("hover span (%d,%d)-(%d,%d), want tile (%d,%d)", x0, y0, x1, y1, wantX, wantY)
	}
	if lx, ly := a.renderer.TileAt(400, 300); lx == wantX && ly == wantY {
		t.Fatal("test cursor does not separate logical from physical tiles")
	}
}

func TestWorkCellClickAtScale(t *testing.T) {
	a, rs, f := scaledColony(t)
	a.win.Open, a.win.Mode = true, "work"
	var cell uikit.Button
	for _, b := range a.win.Buttons() {
		if len(b.ID) > 10 && b.ID[:10] == "work.cell." && !b.Disabled {
			cell = b
			break
		}
	}
	if cell.ID == "" {
		t.Fatal("no live work cell")
	}
	clickAtLogical(t, a, f, cell.X+cell.W/2, cell.Y+cell.H/2)
	n := 0
	for _, c := range rs.cmds {
		if c.Kind == command.SetWorkPriority {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("SetWorkPriority commands = %d after a cell click at s=1.5, want 1", n)
	}
}

func TestOrdersMenuEntryClickAtScale(t *testing.T) {
	a, rs, f := scaledColony(t)
	g := rs.Simulation.(*game.Game)
	if _, err := g.SpawnStack(item.Food, 3, 6, 2); err != nil {
		t.Fatal(err)
	}
	p, _ := a.insp.SelectedPawn()
	if p.ID == 0 {
		a.renderer.Select(a.renderer.snapshot()[0].ID)
	}
	a.ord.OpenOrders(6, 2, 300, 200)
	if !a.ord.Float.Open {
		t.Fatal("order menu should open")
	}
	btns := a.ord.FloatButtons()
	if len(btns) == 0 {
		t.Fatal("no menu entries")
	}
	b := btns[0] // "Go here"
	clickAtLogical(t, a, f, b.X+b.W/2, b.Y+b.H/2)
	if a.ord.Float.Open {
		t.Fatal("clicking an order entry at s=1.5 should close the menu")
	}
}
