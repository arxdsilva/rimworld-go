package ebiten

import (
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
	t.Cleanup(func() { curUIScale = unitUIScale })
	a := testApp(rs)
	a.devFn = func() float64 { return 1 }
	s := a.settings
	s.Accessibility.UIScaleAuto, s.Accessibility.UIScale = false, 150
	a.setSettings(s)
	a.LayoutF(1920, 1080)
	a.finishBoot()
	a.startGame()
	if a.renderer == nil || a.scale.s != 1.5 {
		t.Fatalf("renderer=%v scale=%v", a.renderer != nil, a.scale.s)
	}
	return a, rs, fakeOn(a)
}

// clickAtLogical presses the left button at the logical point's raw
// (physical) position for one Update.
func clickAtLogical(t *testing.T, a *App, f *fakeInput, x, y float32) {
	t.Helper()
	f.cur.mx, f.cur.my = int(float64(x)*a.scale.s), int(float64(y)*a.scale.s)
	f.cur.press[mbLeft], f.cur.down[mbLeft] = true, true
	if err := a.Update(); err != nil {
		t.Fatal(err)
	}
	f.cur.press[mbLeft], f.cur.down[mbLeft] = false, false
}

func TestArchitectCardClickAtScale(t *testing.T) {
	a, _, f := scaledColony(t)
	a.openArchitect()
	var card button
	for _, b := range a.architectButtons() {
		if b.id == "arch.tool.wall" {
			card = b
		}
	}
	if card.id == "" {
		t.Fatal("no wall card in the Architect palette")
	}
	// The palette is laid out in logical pixels: the click lands on the
	// card's logical centre x 1.5 and selects the tool.
	clickAtLogical(t, a, f, card.x+card.w/2, card.y+card.h/2)
	if a.archTool != "wall" {
		t.Fatalf("archTool = %q after a card click at s=1.5, want wall", a.archTool)
	}
}

func TestGhostDragMapsToTilesAtScale(t *testing.T) {
	a, _, _ := scaledColony(t)
	a.openArchitect()
	a.archTool = "wall"
	// World mapping uses the raw physical cursor: the UI scale must not
	// change which tile is under the pointer.
	a.cursor = &[2]int{400, 300}
	px, py := a.mapCursor()
	if px != 600 || py != 450 {
		t.Fatalf("mapCursor = (%d,%d), want (600,450)", px, py)
	}
	wantX, wantY := a.renderer.TileAt(px, py)
	x0, y0, x1, y1 := a.dragSpan()
	if x0 != wantX || y0 != wantY || x1 != wantX || y1 != wantY {
		t.Fatalf("hover span (%d,%d)-(%d,%d), want tile (%d,%d)", x0, y0, x1, y1, wantX, wantY)
	}
	if lx, ly := a.renderer.TileAt(400, 300); lx == wantX && ly == wantY {
		t.Fatal("test cursor does not separate logical from physical tiles")
	}
}

func TestWorkCellClickAtScale(t *testing.T) {
	a, rs, f := scaledColony(t)
	a.workOpen, a.panelMode = true, "work"
	var cell button
	for _, b := range a.workButtons() {
		if len(b.id) > 10 && b.id[:10] == "work.cell." && !b.disabled {
			cell = b
			break
		}
	}
	if cell.id == "" {
		t.Fatal("no live work cell")
	}
	clickAtLogical(t, a, f, cell.x+cell.w/2, cell.y+cell.h/2)
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
	p, _ := a.selectedPawn()
	if p.ID == 0 {
		a.renderer.Select(a.renderer.snapshot()[0].ID)
	}
	a.openOrders(6, 2, 300, 200)
	if !a.float.open {
		t.Fatal("order menu should open")
	}
	btns := a.floatButtons()
	if len(btns) == 0 {
		t.Fatal("no menu entries")
	}
	b := btns[0] // "Go here"
	clickAtLogical(t, a, f, b.x+b.w/2, b.y+b.h/2)
	if a.float.open {
		t.Fatal("clicking an order entry at s=1.5 should close the menu")
	}
}
