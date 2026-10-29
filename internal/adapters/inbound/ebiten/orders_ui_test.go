package ebiten

import (
	"strconv"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/architect"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/adapters/inbound/ebiten/render/world"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
)

func TestPlacementCells(t *testing.T) {
	t.Parallel()
	if got := architect.PlacementCells(architect.PlaceSingle, 1, 1, 4, 2); len(got) != 1 || got[0] != [2]int{4, 2} {
		t.Fatalf("single = %v", got)
	}
	line := architect.PlacementCells(architect.PlaceLine, 2, 2, 6, 3)
	if len(line) != 5 || line[0] != [2]int{2, 2} || line[4] != [2]int{6, 2} {
		t.Fatalf("horizontal line = %v", line)
	}
	if v := architect.PlacementCells(architect.PlaceLine, 2, 5, 3, 1); len(v) != 5 || v[0] != [2]int{2, 1} || v[4] != [2]int{2, 5} {
		t.Fatalf("vertical line = %v", v)
	}
	if f := architect.PlacementCells(architect.PlaceFill, 3, 3, 1, 2); len(f) != 6 || f[0] != [2]int{1, 2} {
		t.Fatalf("fill = %v", f)
	}
}

func TestRotateFacingWraps(t *testing.T) {
	t.Parallel()
	if architect.RotateFacing(0, -1) != 3 || architect.RotateFacing(3, 1) != 0 || architect.RotateFacing(1, 1) != 2 {
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
	a.arch.Drag = true
	a.arch.X0, a.arch.Y0, a.arch.X1, a.arch.Y1 = 4, 4, 8, 5 // horizontal line y=4, one cell has wood
	_, cells := a.previewCells()
	bad := 0
	for _, c := range cells {
		if !c.OK {
			bad++
			if c.Reason != game.ReasonItem {
				t.Fatalf("reason = %q", c.Reason)
			}
		}
	}
	if len(cells) != 5 || bad != 1 {
		t.Fatalf("cells=%d bad=%d", len(cells), bad)
	}
	a.arch.Submit()
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
	a.arch.RotateTool(1)
	a.arch.X0, a.arch.Y0, a.arch.X1, a.arch.Y1 = 6, 6, 6, 6
	a.arch.Drag = true
	a.arch.Submit()
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
	a.arch.Drag = true
	a.arch.X0, a.arch.Y0, a.arch.X1, a.arch.Y1 = 1, 2, 4, 3
	a.arch.Submit()
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
	if p, _ := a.insp.SelectedPawn(); !p.Drafted {
		t.Fatal("draft button should draft the pawn")
	}
	click(t, a, "pawn.draft")
	g.Step(1)
	if p, _ := a.insp.SelectedPawn(); p.Drafted {
		t.Fatal("second click should undraft")
	}

	if _, err := g.SpawnStack(item.Food, 3, 6, 2); err != nil {
		t.Fatal(err)
	}
	a.ord.OpenOrders(6, 2, 300, 200)
	if !a.ord.Float.Open {
		t.Fatal("order menu should open")
	}
	labels := []string{}
	for _, b := range a.ord.FloatButtons() {
		labels = append(labels, b.Label)
	}
	joined := strings.Join(labels, "|")
	if !strings.HasPrefix(joined, "Go here") || !strings.Contains(joined, "Eat food") ||
		!strings.Contains(joined, "no stockpile space") {
		t.Fatalf("menu = %q", joined)
	}
	// disabled entry shows its reason and keeps the menu open
	for i, o := range a.ord.Float.Opts {
		if o.Disabled {
			a.ord.ChooseFloat(floatID(i))
			if !a.ord.Float.Open || a.hint != o.Reason {
				t.Fatalf("disabled click: open=%v hint=%q", a.ord.Float.Open, a.hint)
			}
		}
	}
	for i, o := range a.ord.Float.Opts {
		if strings.HasPrefix(o.Label, "Eat") {
			a.ord.ChooseFloat(floatID(i))
		}
	}
	if a.ord.Float.Open {
		t.Fatal("choosing an order closes the menu")
	}
	g.Step(2)
	if p, _ := a.insp.SelectedPawn(); !strings.HasPrefix(p.Job, "eat") {
		t.Fatalf("prioritized eat, job = %s", p.Job)
	}
}

func TestEscapeClosesOrderMenuFirst(t *testing.T) {
	t.Parallel()
	a, _, id := liveColony(t, 0)
	a.renderer.Select(id)
	a.ord.OpenOrders(5, 5, 100, 100)
	a.pressEscape()
	if a.ord.Float.Open || !a.renderer.HasSelect() {
		t.Fatal("Escape should close the menu and keep the selection")
	}
}

func floatID(i int) string { return "float." + string(rune('0'+i)) }

func TestStoneMaterialReachesBlueprint(t *testing.T) {
	t.Parallel()
	a, g, _ := liveColony(t, 0)
	click(t, a, "tab.architect")
	click(t, a, "arch.cat.structure")
	click(t, a, "arch.tool.wall")
	click(t, a, "arch.stuff.granite blocks")
	if a.toolStuff(architect.ToolDef{Build: "wall"}) != "granite blocks" {
		t.Fatal("granite blocks should be selected for walls")
	}
	if a.toolStuff(architect.ToolDef{Build: "door"}) != "wood" {
		t.Fatal("doors cannot be stone; they fall back to wood")
	}
	a.arch.Drag = true
	a.arch.X0, a.arch.Y0, a.arch.X1, a.arch.Y1 = 6, 6, 7, 6
	a.arch.Submit()
	g.Step(1)
	f := g.World().Frames
	if len(f) != 2 || f[0].Stuff != "granite blocks" || f[1].Stuff != "granite blocks" {
		t.Fatalf("frames = %+v", f)
	}
}

func TestPlantSpriteScalesWithGrowth(t *testing.T) {
	t.Parallel()
	_, young := world.PlantSprite(game.PlantView{Species: "tree", Growth: 10})
	key, grown := world.PlantSprite(game.PlantView{Species: "tree", Growth: 100})
	if !(young < grown) || grown != 1 || !strings.HasPrefix(key, "plant/tree_") {
		t.Fatalf("tree scale young=%v grown=%v key=%s", young, grown, key)
	}
	// crops carry stage and a per-tile variant (ART §9); seasonKey falls
	// back to the older shared sprites (environment_test.go)
	if k, _ := world.PlantSprite(game.PlantView{Species: "potato", Stage: 3}); k != "plant/potato_harvestable_v0" {
		t.Fatalf("ripe crop sprite = %s", k)
	}
	if k, _ := world.PlantSprite(game.PlantView{Species: "corn", Stage: 0}); k != "plant/corn_sown_v0" {
		t.Fatalf("freshly sown crop sprite = %s", k)
	}
}

func TestGrowingZoneToolSendsCrop(t *testing.T) {
	t.Parallel()
	a, g, _ := liveColony(t, 0)
	click(t, a, "tab.architect")
	click(t, a, "arch.cat.zones")
	click(t, a, "arch.tool.grow")
	a.arch.Drag = true
	a.arch.X0, a.arch.Y0, a.arch.X1, a.arch.Y1 = 6, 6, 8, 7
	a.arch.Submit()
	g.Step(1)
	z := zonesOfKind(g.World(), game.ZoneGrow)
	if len(z) != 1 || z[0].Crop != "potato" || z[0].MaxX != 8 {
		t.Fatalf("zones = %+v", z)
	}
	// the crop picker changes what the next zone sows
	click(t, a, "arch.crop.corn")
	a.arch.Drag = true
	a.arch.X0, a.arch.Y0, a.arch.X1, a.arch.Y1 = 10, 6, 11, 7
	a.arch.Submit()
	g.Step(1)
	z = zonesOfKind(g.World(), game.ZoneGrow)
	if len(z) != 2 || z[1].Crop != "corn" {
		t.Fatalf("picked corn: zones = %+v", z)
	}
}

func TestBenchBillsPanel(t *testing.T) {
	t.Parallel()
	a, g, _ := liveColony(t, 0)
	stove, _ := architect.KindID("stove")
	g.Enqueue(command.Command{Kind: command.DesignateBuild, Build: stove, X: 6, Y: 6})
	g.Step(1)
	if len(g.World().Frames) != 1 {
		t.Fatal("stove frame")
	}
	// finish it through the fixture-free path: supply + work
	g.SpawnStack(item.Wood, 10, 5, 5)
	for i := 0; i < 40 && len(g.World().Buildings) == 0; i++ {
		g.Step(game.TicksPerCell * 10)
	}
	if len(g.World().Buildings) != 1 {
		t.Fatalf("stove not built: %+v", g.World().Frames)
	}
	if !a.insp.Building().SelectBuildingAt(7, 6) { // second cell of the 2x1 stove
		t.Fatal("clicking any stove cell selects it")
	}
	// the bills live in the pane's Bills tab (PLAY-018)
	f := fakeOn(a)
	a.renderer.speed = speedPaused
	tab, ok := bldTab(a.insp.Building().BuildingPane(a.sel.SelBuilding), "inspector.bills")
	if !ok {
		t.Fatal("a stove has a Bills tab")
	}
	panel := gfx.Rect{X: 0, Y: 100, W: tab.WantW, H: 400}
	setHook(a.uiSt(), func(a *App) { tab.Declare(panel) })
	f.Frame(t, a)
	if !hasWidget(a, inspector.BillPickBtn) {
		t.Fatal("stove should offer the recipe picker")
	}
	rcs := a.insp.Building().RecipesFor("stove")
	if len(rcs) == 0 {
		t.Fatal("stove should offer a recipe")
	}
	a.insp.Building().ApplyBuildingClick("bill.add." + strconv.Itoa(int(rcs[0].ID)))
	g.Step(1)
	b, _ := a.insp.SelectedBuilding()
	if len(b.Bills) != 1 || b.Bills[0].Target != 1 {
		t.Fatalf("bills = %+v", b.Bills)
	}
	press(t, a, f, "bill.plus.0")
	g.Step(1)
	press(t, a, f, "bill.mode.0")
	g.Step(1)
	b, _ = a.insp.SelectedBuilding()
	if b.Bills[0].Target != 2 || b.Bills[0].Mode != "forever" {
		t.Fatalf("after +/mode: %+v", b.Bills[0])
	}
	if !strings.Contains(inspector.BillLine(b.Bills[0]), "forever") {
		t.Fatalf("line = %q", inspector.BillLine(b.Bills[0]))
	}
	press(t, a, f, "bill.del.0")
	g.Step(1)
	if b, _ = a.insp.SelectedBuilding(); len(b.Bills) != 0 {
		t.Fatal("delete removes the bill")
	}
	dc, ok := bldCmd(a.insp.Building().BuildingCommands(a.sel.SelBuilding), "bldg.deconstruct")
	if !ok {
		t.Fatal("a stove can be deconstructed")
	}
	a.applyClick(dc.ID) // what a click on the command button does
	g.Step(1)
	if b, _ = a.insp.SelectedBuilding(); !b.Deconstruct {
		t.Fatal("deconstruct button marks the building")
	}
	a.pressEscape()
	if a.sel.HasBuilding {
		t.Fatal("Escape clears the building selection")
	}
}
