package ebiten

import (
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

func wallFrame() game.FrameView {
	return game.FrameView{ID: 7, Kind: "wall", X: 3, Y: 3, W: 1, H: 1, Need: 10, Wood: 6, Incoming: 4,
		WorkTotal: 40, WorkLeft: 40, InStock: 45, Reserved: 12, Stuff: "wood", Facing: "south",
		Haulers: []string{"Ada (4 wood)"}, Status: "Materials on the way"}
}

func TestFrameSelectionCycle(t *testing.T) {
	t.Parallel()
	w := game.WorldView{
		Items:     []game.ItemView{{ID: 1, Kind: "wood", Qty: 3, X: 3, Y: 3}},
		Buildings: []game.BuildingView{{ID: 2, Kind: "bed", X: 3, Y: 3, W: 1, H: 1}, {ID: 3, Kind: "floor", Floor: true, X: 3, Y: 3, W: 1, H: 1}},
		Frames: []game.FrameView{
			{ID: 4, Kind: "floor", Floor: true, X: 3, Y: 3, W: 1, H: 1},
			{ID: 5, Kind: "wall", X: 3, Y: 3, W: 1, H: 1},
		},
	}
	got := selectablesAt(w, nil, 3, 3)
	want := []selCand{{selItem, 1}, {selFrame, 5}, {selBuilding, 2}, {selBuilding, 3}, {selFrame, 4}}
	if len(got) != len(want) {
		t.Fatalf("cands = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("cands = %v, want %v", got, want)
		}
	}
	// footprint: a 2x1 frame covers its second tile
	w = game.WorldView{Frames: []game.FrameView{{ID: 9, Kind: "workbench", X: 2, Y: 2, W: 2, H: 1}}}
	if c := selectablesAt(w, nil, 3, 2); len(c) != 1 || c[0].kind != selFrame {
		t.Fatalf("2x1 frame second tile = %v", c)
	}
}

func frameApp(t *testing.T, f game.FrameView) (*App, *stubSim) {
	a, s := stubColony(t)
	s.world = game.WorldView{Frames: []game.FrameView{f}}
	a.selectCand(selCand{selFrame, f.ID})
	return a, s
}

func TestFrameInspectorLines(t *testing.T) {
	t.Parallel()
	in := buildInfo(wallFrame(), game.WorldView{})
	if in.Title != "Wall (frame)" || in.Status != "Materials on the way" {
		t.Fatalf("%+v", in)
	}
	if in.Material != "wood 6 / 10 (+4 on the way)" || in.Stock != "45 wood (12 reserved)" {
		t.Fatalf("%+v", in)
	}
	if !strings.Contains(in.Work, "starts once materials are in") || !strings.Contains(in.People, "Hauling: Ada (4 wood)") {
		t.Fatalf("%+v", in)
	}
	f := wallFrame()
	f.Wood, f.Incoming, f.WorkLeft, f.Status = 0, 0, 40, "No wood in stock"
	if in := buildInfo(f, game.WorldView{}); in.Title != "Wall (blueprint)" || in.Status != "No wood in stock" {
		t.Fatalf("%+v", in)
	}
	f = wallFrame()
	f.Wood, f.WorkLeft, f.Builder = 10, 20, "Bo"
	f.Status = "Being built by Bo"
	in = buildInfo(f, game.WorldView{})
	if !strings.HasPrefix(in.Work, "20 / 40  ~ ") || !strings.Contains(in.Work, "in-game min") || !strings.HasSuffix(in.People, "Building: Bo") || in.Stock != "" {
		t.Fatalf("%+v", in)
	}
	// reinstall
	mv := game.FrameView{ID: 8, Kind: "workbench", Moving: true, MovingFrom: 5, W: 2, H: 1, WorkTotal: 30, WorkLeft: 30}
	in = buildInfo(mv, game.WorldView{Buildings: []game.BuildingView{{ID: 5, Kind: "workbench"}}})
	if in.Status != "Moving workbench here: needs no materials" || in.Material != "" {
		t.Fatalf("%+v", in)
	}
	// empty status falls back
	f = wallFrame()
	f.Status = ""
	f.Wood = 0
	if in := buildInfo(f, game.WorldView{}); in.Status != "Waiting for materials" {
		t.Fatalf("%+v", in)
	}
}

func TestFrameCancelEmitsCancelFrame(t *testing.T) {
	t.Parallel()
	a, s := frameApp(t, wallFrame())
	if _, ok := hasButton(a.frameButtons(), "frame.cancel"); !ok {
		t.Fatal("no cancel button")
	}
	click(t, a, "frame.cancel")
	c := lastCmd(t, s)
	if c.Kind != command.CancelFrame || c.Target != 7 {
		t.Fatalf("cmd = %+v", c)
	}
	if a.hasFrame {
		t.Fatal("panel stays open after cancel")
	}
}

func TestFrameEscClosesAndVanishedClears(t *testing.T) {
	t.Parallel()
	a, s := frameApp(t, wallFrame())
	a.pressEscape()
	if a.hasFrame {
		t.Fatal("Esc should close the frame panel")
	}
	a.selectCand(selCand{selFrame, 7})
	s.world = game.WorldView{}
	if _, ok := a.selectedFrame(); ok || a.hasFrame {
		t.Fatal("a finished frame must deselect")
	}
}

func TestFrameInspectorDrawsWithoutPanic(t *testing.T) {
	t.Parallel()
	a, _ := frameApp(t, wallFrame())
	a.drawFrameInspector(ebiten.NewImage(a.sw, a.sh))
}

func TestCarpetCardStockLines(t *testing.T) {
	t.Parallel()
	tool, _ := toolByID("carpet")
	lines := archTooltipLines(tool, "straw carpet", 0, 0)
	var got string
	for _, l := range lines {
		if strings.HasPrefix(l, "In stock:") {
			got = l
		}
	}
	if !strings.HasPrefix(got, "In stock: 0 straw carpet: sew one at a ") {
		t.Fatalf("stock line = %q in %q", got, lines)
	}
	lines = archTooltipLines(tool, "straw carpet", 0, 2)
	if !contains(lines, "In stock: 2 straw carpet") {
		t.Fatalf("lines = %q", lines)
	}
}

func TestCarpetMaterialButtonsShowCounts(t *testing.T) {
	t.Parallel()
	a, s := stubColony(t)
	s.world = game.WorldView{Items: []game.ItemView{
		{ID: 1, Kind: "cloth carpet", Qty: 3, X: 1, Y: 1},
		{ID: 2, Kind: "cloth carpet", Qty: 5, X: 2, Y: 1, Forbidden: true},
	}}
	click(t, a, "tab.architect")
	click(t, a, "arch.cat.furniture")
	click(t, a, "arch.tool.carpet")
	bs := a.architectButtons()
	if b, _ := hasButton(bs, "arch.stuff.straw carpet"); b.label != "Straw (0)" {
		t.Fatalf("label = %q", b.label)
	}
	if b, _ := hasButton(bs, "arch.stuff.cloth carpet"); b.label != "Cloth (3)" {
		t.Fatalf("label = %q", b.label)
	}
	r := a.architectRect()
	for _, b := range bs {
		if strings.HasPrefix(b.id, "arch.stuff.") && b.y+b.h > r.y+r.h {
			t.Fatalf("%s overflows the panel", b.id)
		}
	}
}

func TestWaitingBlueprintsAlert(t *testing.T) {
	t.Parallel()
	f1, f2, f3 := wallFrame(), wallFrame(), wallFrame()
	f1.Status, f2.Status, f2.X = "No wood in stock", "No straw carpet in stock: sew one at a tailoring bench", 5
	w := game.WorldView{Frames: []game.FrameView{f1, f2, f3}}
	al, ok := waitingFramesAlert(w)
	if !ok || al.label != "Blueprints waiting for materials (2)" || len(al.targets) != 2 {
		t.Fatalf("alert = %+v ok=%v", al, ok)
	}
	if _, ok := waitingFramesAlert(game.WorldView{Frames: []game.FrameView{f3}}); ok {
		t.Fatal("no alert when materials are in stock")
	}
}

func TestFenceAndForbidToolsEmit(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"fence", "fence_gate"} {
		tool, ok := toolByID(id)
		if !ok || tool.category != "structure" || !strings.Contains(tool.desc, "Animals can't cross; colonists climb over (slow)") {
			t.Fatalf("tool %s = %+v", id, tool)
		}
	}
	if g, _ := toolByID("fence_gate"); g.mode != placeSingle {
		t.Fatal("gate is a single placement")
	}
	if f, _ := toolByID("fence"); f.mode != placeLine {
		t.Fatal("fence draws lines")
	}
	a, s := stubColony(t)
	click(t, a, "tab.architect")
	click(t, a, "arch.cat.structure")
	click(t, a, "arch.tool.fence")
	a.archX0, a.archY0, a.archX1, a.archY1 = 2, 2, 4, 2
	a.archDrag = true
	a.submitArchitect()
	id, _ := kindID("fence")
	n := 0
	for _, c := range s.cmds {
		if c.Kind == command.DesignateBuild && c.Build == id {
			n++
		}
	}
	if n == 0 {
		t.Fatalf("no DesignateBuild for fence: %+v", s.cmds)
	}
	click(t, a, "arch.cat.orders")
	for _, tc := range []struct {
		id string
		on bool
	}{{"forbid", true}, {"allow", false}} {
		s.cmds = nil
		click(t, a, "arch.tool."+tc.id)
		a.archX0, a.archY0, a.archX1, a.archY1 = 1, 2, 3, 4
		a.submitArchitect()
		c := lastCmd(t, s)
		if c.Kind != command.DesignateForbid || c.Enabled != tc.on || c.X != 1 || c.Y2 != 4 {
			t.Fatalf("%s cmd = %+v", tc.id, c)
		}
	}
}

func TestItemForbidToggle(t *testing.T) {
	t.Parallel()
	a, s := stubColony(t)
	s.world = game.WorldView{Items: []game.ItemView{{ID: 11, Kind: "wood", Qty: 5, X: 2, Y: 2}}}
	a.selectCand(selCand{selItem, 11})
	click(t, a, "item.forbid")
	if c := lastCmd(t, s); c.Kind != command.SetForbidden || c.Target != 11 || !c.Enabled {
		t.Fatalf("cmd = %+v", c)
	}
	s.world.Items[0].Forbidden = true
	if b, _ := hasButton(a.itemButtons(), "item.forbid"); b.label != "Allow" {
		t.Fatalf("label = %q", b.label)
	}
	click(t, a, "item.forbid")
	if c := lastCmd(t, s); c.Enabled {
		t.Fatalf("cmd = %+v, want allow", c)
	}
	if ls := itemLines(s.world.Items[0], false); !contains(ls, "Forbidden: colonists leave it alone") {
		t.Fatalf("lines = %q", ls)
	}
}

func TestFenceSpriteKeys(t *testing.T) {
	t.Parallel()
	if got := buildSpriteStuff("fence", "wood", "", 5, false, false); got != "structure/fence_wood_05" {
		t.Fatal(got)
	}
	if got := buildSpriteStuff("fence", "stone", "", 15, false, false); got != "structure/fence_stone_15" {
		t.Fatal(got)
	}
	if got := buildSpriteStuff("fence gate", "stone", "east", 0, false, true); got != "structure/fence_gate_open_east" {
		t.Fatal(got)
	}
	if got := buildSpriteStuff("fence gate", "wood", "", 0, false, false); got != "structure/fence_gate_south" {
		t.Fatal(got)
	}
	w := game.WorldView{
		Buildings: []game.BuildingView{{Kind: "fence", X: 1, Y: 1}, {Kind: "fence gate", X: 2, Y: 1}, {Kind: "wall", X: 1, Y: 0}, {Kind: "bed", X: 0, Y: 1}},
	}
	if m := wallMask(fenceSet(w, false), 1, 1); m != wallN|wallE {
		t.Fatalf("mask = %d, want N|E (wall above, gate right, bed ignored)", m)
	}
	if lbl := fenceLabelAt(w, 1, 1); !strings.HasPrefix(lbl, "Fence") {
		t.Fatal(lbl)
	}
}

func TestFenceFallbacksDoNotPanic(t *testing.T) {
	t.Parallel()
	img := ebiten.NewImage(128, 128)
	drawFenceVector(img, 0, 0, 64, "fence", wallN|wallE, false, false, 255)
	drawFenceVector(img, 0, 0, 64, "fence gate", 0, true, true, 140)
	drawFenceVector(img, 0, 0, 64, "fence gate", 0, false, false, 255)
	a, _ := stubColony(t)
	drawIconFallback(img, archTool{label: "Fence"}, 0, 0)
	a.renderer.art = nil
	a.renderer.drawForbiddenMark(img, 1, 1, 64, 128, 128) // nil catalog: vector ring
}
