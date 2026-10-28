package ebiten

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/config"
	"rimworld-go/internal/platform/input"
)

// fakeInput is a scripted keyboard and mouse: each poll returns the
// current state, then clears what only lasts one frame (presses,
// releases, typed characters, the wheel).
type fakeInput struct{ inputframe.FakeSource }

func fakeOn(a *App) *fakeInput {
	f := &fakeInput{}
	a.src = f
	return f
}

// Key presses one Key (with mods held) for one Update.
func (f *fakeInput) Key(t *testing.T, a *App, name string, mods input.Mod) {
	t.Helper()
	f.Cur.Pressed, f.Cur.Mods = []string{name}, mods
	if err := a.Update(); err != nil {
		t.Fatal(err)
	}
	f.Cur.Mods = 0
}

// TypeText types characters into whatever has focus (one Update).
func (f *fakeInput) TypeText(t *testing.T, a *App, s string) {
	t.Helper()
	var keys []string
	for _, r := range s {
		keys = append(keys, strings.ToUpper(string(r)))
	}
	f.Cur.Chars, f.Cur.Pressed = []rune(s), keys
	if err := a.Update(); err != nil {
		t.Fatal(err)
	}
}

// MoveTo puts the cursor on the centre of tile (tx, ty).
func (f *fakeInput) MoveTo(a *App, tx, ty int) {
	sx, sy := a.renderer.worldToScreenF(float64(tx)+0.5, float64(ty)+0.5)
	f.Cur.Mx, f.Cur.My = int(sx), int(sy)
}

func (f *fakeInput) Frame(t *testing.T, a *App) {
	t.Helper()
	if err := a.Update(); err != nil {
		t.Fatal(err)
	}
}

// Click presses and releases button b at the cursor (two Updates).
func (f *fakeInput) Click(t *testing.T, a *App, b mouseBtn, mods input.Mod) {
	t.Helper()
	f.Cur.Mods = mods
	f.Cur.Press[b], f.Cur.Down[b] = true, true
	f.Frame(t, a)
	f.Cur.Down[b], f.Cur.Release[b] = false, true
	f.Frame(t, a)
	f.Cur.Mods = 0
}

// Drag holds b from tile a0 to tile a1 and lets go.
func (f *fakeInput) Drag(t *testing.T, a *App, b mouseBtn, x0, y0, x1, y1 int, mods input.Mod) {
	t.Helper()
	f.Cur.Mods = mods
	f.MoveTo(a, x0, y0)
	f.Cur.Press[b], f.Cur.Down[b] = true, true
	f.Frame(t, a)
	f.MoveTo(a, x1, y1)
	f.Frame(t, a)
	f.Cur.Down[b], f.Cur.Release[b] = false, true
	f.Frame(t, a)
	f.Cur.Mods = 0
}

// inputColony is a paused stub colony with colonists at the given tiles.
func inputColony(t *testing.T, at ...[2]int) (*App, *stubSim, *fakeInput) {
	t.Helper()
	s := &stubSim{w: 24, h: 24}
	for i, c := range at {
		s.pawns = append(s.pawns, game.PawnView{ID: entity.EntityID(i + 1), Name: []string{"Ada", "Bo", "Cora", "Dee"}[i%4], X: c[0], Y: c[1], Status: "alive"})
	}
	a := testApp(s)
	a.finishBoot()
	a.startGame()
	f := fakeOn(a)
	f.Frame(t, a) // place the camera
	a.renderer.speed = speedPaused
	// Tile (x, y) at screen (64x, 64y+32): rows 0..8 and, right of the
	// inspector panels, columns 6..19 are open map.
	a.renderer.cam.OffsetX, a.renderer.cam.OffsetY = 0, -32
	return a, s, f
}

func cmdsOf(s *stubSim, k command.Kind) []command.Command {
	var out []command.Command
	for _, c := range s.cmds {
		if c.Kind == k {
			out = append(out, c)
		}
	}
	return out
}

// R drafts (UI-016: the roof overlay moved to Shift+R).
func TestRDraftsAndShiftRTogglesRoof(t *testing.T) {
	t.Parallel()
	a, s, f := inputColony(t, [2]int{5, 5})
	a.renderer.Select(1)
	f.Key(t, a, "R", 0)
	if d := cmdsOf(s, command.DraftPawn); len(d) != 1 || !d[0].Enabled || d[0].Pawn != 1 {
		t.Fatalf("R should draft the selected colonist, got %+v", s.cmds)
	}
	if a.renderer.overlays().Roof.On {
		t.Fatal("R must no longer toggle the roof overlay")
	}
	f.Key(t, a, "R", input.ModShift)
	if !a.renderer.overlays().Roof.On || len(cmdsOf(s, command.DraftPawn)) != 1 {
		t.Fatal("Shift+R toggles the roof overlay and drafts nobody")
	}
	if b, _ := findButton(a, "pawn.draft"); b.Label != "Draft [R]" {
		t.Fatalf("draft button = %q, want the live key", b.Label)
	}
}

func TestFunctionKeysOpenTabs(t *testing.T) {
	t.Parallel()
	a, _, f := inputColony(t, [2]int{5, 5})
	cases := []struct {
		key, mode string
		wild      bool
	}{{"F1", "work", false}, {"F2", "schedule", false}, {"F4", "animals", false}, {"F5", "animals", true}, {"F6", "research", false}}
	for _, c := range cases {
		f.Key(t, a, c.key, 0)
		if !a.win.Open || a.win.Mode != c.mode || (c.mode == "animals" && a.win.Wild != c.wild) {
			t.Fatalf("%s: open=%v mode=%q wild=%v", c.key, a.win.Open, a.win.Mode, a.win.Wild)
		}
	}
	f.Key(t, a, "F5", 0)
	f.Key(t, a, "F5", 0)
	if a.win.Open {
		t.Fatal("F5 again closes the Wild list")
	}
	f.Key(t, a, "Tab", 0)
	if !a.arch.Open {
		t.Fatal("Tab opens the Architect")
	}
	f.Key(t, a, "F3", 0)
	if a.uiModalTop() != assignModalID {
		t.Fatalf("F3 should open Assign, top modal %q", a.uiModalTop())
	}
	f.Key(t, a, "F3", 0)
	f.Key(t, a, "F9", 0)
	if a.uiModalTop() != historyModalID {
		t.Fatalf("F9 should open History, top modal %q", a.uiModalTop())
	}
	f.Key(t, a, "F9", 0)
	if a.uiModalTop() == historyModalID {
		t.Fatal("F9 again closes History")
	}
	f.Key(t, a, "F8", 0)
	if a.uiModalTop() != worldModalID {
		t.Fatalf("F8 should open World, top modal %q", a.uiModalTop())
	}
	f.Key(t, a, "F8", 0)
	if a.uiModalTop() == worldModalID {
		t.Fatal("F8 again closes World")
	}
	for _, k := range []string{"F7", "T"} { // V is live since PLAY-021 (TestVHotkeyFlicksTheSelectedDevice)
		f.Key(t, a, k, 0)
		ts := a.liveToasts()
		if len(ts) == 0 || !strings.Contains(ts[len(ts)-1].text, input.NotYet) {
			t.Fatalf("%s should toast %q, got %+v", k, input.NotYet, ts)
		}
	}
}

func TestSearchTypingDraftsNobody(t *testing.T) {
	t.Parallel()
	a, s, f := inputColony(t, [2]int{5, 5}, [2]int{9, 9})
	a.renderer.Select(1)
	f.Key(t, a, "Z", 0)
	if !a.ord.Search.Open {
		t.Fatal("Z opens search")
	}
	f.TypeText(t, a, "r")
	if len(cmdsOf(s, command.DraftPawn)) != 0 || a.ord.Search.Text != "r" {
		t.Fatalf("typing r drafted someone or was lost: text=%q cmds=%+v", a.ord.Search.Text, s.cmds)
	}
	f.Key(t, a, "Backspace", 0)
	if a.ord.Search.Text != "" {
		t.Fatal("Backspace deletes")
	}
	f.Key(t, a, "Escape", 0)
	if a.ord.Search.Open || !a.renderer.HasSelect() {
		t.Fatal("Esc closes search and keeps the selection")
	}
}

func TestSearchFindsNamedColonist(t *testing.T) {
	t.Parallel()
	a, _, f := inputColony(t, [2]int{3, 3}, [2]int{18, 18})
	f.Key(t, a, "Z", 0)
	f.TypeText(t, a, "bo")
	hits := a.searchHits()
	if len(hits) == 0 || hits[0].label != "Bo" || hits[0].c != (selCand{Kind: selPawn, ID: 2}) {
		t.Fatalf("hits = %+v", hits)
	}
	before := a.renderer.cam
	f.Key(t, a, "Enter", 0)
	if a.ord.Search.Open || !a.renderer.HasSelect() || a.renderer.Selected() != 2 {
		t.Fatal("Enter selects the result and closes search")
	}
	if a.renderer.cam == before {
		t.Fatal("the camera centres on the result")
	}
}

func TestCommaPeriodCycleColonists(t *testing.T) {
	t.Parallel()
	a, _, f := inputColony(t, [2]int{2, 2}, [2]int{12, 12}, [2]int{20, 20})
	f.Key(t, a, "Period", 0)
	if a.renderer.Selected() != 1 {
		t.Fatalf(". first selects bar colonist 1, got %d", a.renderer.Selected())
	}
	f.Key(t, a, "Period", 0)
	f.Key(t, a, "Period", 0)
	if a.renderer.Selected() != 3 {
		t.Fatalf("got %d, want 3", a.renderer.Selected())
	}
	f.Key(t, a, "Period", 0)
	if a.renderer.Selected() != 1 {
		t.Fatal(". wraps to the first colonist")
	}
	f.Key(t, a, "Comma", 0)
	if a.renderer.Selected() != 3 {
		t.Fatal(", goes back (wrapping)")
	}
}

func TestShiftClickBoxAndDoubleClickSelect(t *testing.T) {
	t.Parallel()
	a, _, f := inputColony(t, [2]int{8, 4}, [2]int{10, 4}, [2]int{12, 4})
	f.MoveTo(a, 8, 4)
	f.Click(t, a, mbLeft, 0)
	f.MoveTo(a, 10, 4)
	a.updates += doubleClickFrames // not a double click
	f.Click(t, a, mbLeft, input.ModShift)
	if !a.grouped() || len(a.sel.Group.Ids) != 2 {
		t.Fatalf("Shift+click adds: group = %+v", a.sel.Group)
	}
	a.updates += doubleClickFrames
	f.Click(t, a, mbLeft, input.ModShift)
	if a.grouped() || !a.renderer.HasSelect() || a.renderer.Selected() != 1 {
		t.Fatal("Shift+click again takes it out (one left: the single selection)")
	}
	f.Drag(t, a, mbLeft, 7, 3, 13, 5, 0)
	if !a.grouped() || a.sel.Group.Kind != selPawn || len(a.sel.Group.Ids) != 3 {
		t.Fatalf("box select = %+v", a.sel.Group)
	}
	a.updates += doubleClickFrames
	f.MoveTo(a, 8, 4)
	f.Click(t, a, mbLeft, 0)
	if a.grouped() {
		t.Fatal("a plain click goes back to one")
	}
	f.Click(t, a, mbLeft, 0) // a double click
	if !a.grouped() || len(a.sel.Group.Ids) != 3 {
		t.Fatalf("double click takes every colonist on screen: %+v", a.sel.Group)
	}
	if bs := a.groupButtons(); len(bs) == 0 {
		t.Fatal("the group panel shows")
	}
}

func TestDoubleClickSameSpecies(t *testing.T) {
	t.Parallel()
	a, s, f := inputColony(t)
	s.world.Animals = []game.AnimalView{{ID: 50, Species: "deer", Label: "deer", X: 8, Y: 4}, {ID: 51, Species: "deer", Label: "deer", X: 11, Y: 4},
		{ID: 52, Species: "hare", Label: "hare", X: 13, Y: 4}}
	f.MoveTo(a, 8, 4)
	f.Click(t, a, mbLeft, 0)
	f.Click(t, a, mbLeft, 0)
	if !a.grouped() || a.sel.Group.Kind != selAnimal || len(a.sel.Group.Ids) != 2 {
		t.Fatalf("double click on a deer takes the deer: %+v", a.sel.Group)
	}
}

func TestGroupDraftAndFormation(t *testing.T) {
	t.Parallel()
	a, s, f := inputColony(t, [2]int{8, 2}, [2]int{9, 2}, [2]int{10, 2})
	f.Drag(t, a, mbLeft, 7, 1, 11, 3, 0)
	f.Key(t, a, "R", 0)
	if d := cmdsOf(s, command.DraftPawn); len(d) != 3 {
		t.Fatalf("R drafts the whole group, got %+v", d)
	}
	for i := range s.pawns {
		s.pawns[i].Drafted = true
	}
	click(t, a, "group.undraft")
	if d := cmdsOf(s, command.DraftPawn); len(d) != 6 || d[5].Enabled {
		t.Fatalf("Undraft all = %+v", d)
	}
	s.cmds = nil
	f.MoveTo(a, 14, 5)
	f.Click(t, a, mbRight, 0)
	moves := cmdsOf(s, command.MovePawn)
	if len(moves) != 3 {
		t.Fatalf("one move per drafted colonist, got %+v", moves)
	}
	seen := map[[2]int]bool{}
	for _, m := range moves {
		c := [2]int{m.X, m.Y}
		if seen[c] || max(abs(m.X-14), abs(m.Y-5)) > 1 {
			t.Fatalf("formation cells must be distinct and around the target: %+v", moves)
		}
		seen[c] = true
	}
	s.cmds = nil
	f.Drag(t, a, mbRight, 8, 7, 14, 7, input.ModShift)
	moves = cmdsOf(s, command.MovePawn)
	if len(moves) != 3 || !moves[0].Queue {
		t.Fatalf("a right drag lines them up (Shift queues): %+v", moves)
	}
	xs := map[int]bool{}
	for _, m := range moves {
		if m.Y != 7 {
			t.Fatalf("off the line: %+v", m)
		}
		xs[m.X] = true
	}
	if !xs[8] || !xs[11] || !xs[14] {
		t.Fatalf("evenly spaced along the line, got %+v", moves)
	}
}

func TestFormationCellsDeterministic(t *testing.T) {
	t.Parallel()
	free := func(x, y int) bool { return !(x == 5 && y == 5) && x >= 0 && y >= 0 }
	got := formationCells(5, 5, 4, free)
	want := [][2]int{{5, 4}, {4, 5}, {6, 5}, {5, 6}}
	if len(got) != 4 {
		t.Fatalf("cells = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("cells = %v, want %v (the blocked target skipped, nearest first)", got, want)
		}
	}
	line := lineCells(0, 0, 8, 0, 3, func(x, y int) bool { return true })
	if len(line) != 3 || line[0] != [2]int{0, 0} || line[1] != [2]int{4, 0} || line[2] != [2]int{8, 0} {
		t.Fatalf("line = %v", line)
	}
	ps := []game.PawnView{{ID: 1, X: 9, Y: 0}, {ID: 2, X: 0, Y: 0}}
	m := assignCells(ps, [][2]int{{0, 1}, {9, 1}})
	if m[0].pawn != 2 || m[1].pawn != 1 {
		t.Fatalf("nearest colonist takes each cell: %+v", m)
	}
}

func TestForbidKeyTogglesSelectedItems(t *testing.T) {
	t.Parallel()
	a, s, f := inputColony(t)
	s.world.Items = []game.ItemView{{ID: 70, Kind: "wood", Qty: 20, X: 8, Y: 4}, {ID: 71, Kind: "wood", Qty: 5, X: 10, Y: 4, Forbidden: true}}
	f.MoveTo(a, 8, 4)
	f.Click(t, a, mbLeft, 0)
	f.Key(t, a, "F", 0)
	if c := cmdsOf(s, command.SetForbidden); len(c) != 1 || c[0].Target != 70 || !c[0].Enabled {
		t.Fatalf("F forbids the stack: %+v", c)
	}
	s.cmds = nil
	f.Drag(t, a, mbLeft, 7, 3, 11, 5, 0)
	f.Key(t, a, "F", 0)
	if c := cmdsOf(s, command.SetForbidden); len(c) != 1 || c[0].Target != 70 {
		t.Fatalf("with one forbidden, F forbids the rest: %+v", c)
	}
}

func TestF10WritesScreenshotF11HidesUI(t *testing.T) {
	t.Parallel()
	a, _, f := inputColony(t, [2]int{5, 5})
	dir := t.TempDir()
	a.meter.Shot.Dir = func() (string, error) { return dir, nil }
	f.Key(t, a, "F10", 0)
	if !a.meter.Shot.Pending {
		t.Fatal("F10 asks for a screenshot of the next frame")
	}
	a.meter.Shot.Pending = false
	a.takeScreenshot(func(img *image.RGBA) { img.Pix[0] = 255 }, 8, 6) // what Draw does with ReadPixels
	if a.meter.Shot.Last == "" || filepath.Dir(a.meter.Shot.Last) != dir || !strings.HasPrefix(filepath.Base(a.meter.Shot.Last), "rimworld-go-") {
		t.Fatalf("screenshot path %q", a.meter.Shot.Last)
	}
	if _, err := os.Stat(a.meter.Shot.Last); err != nil {
		t.Fatal(err)
	}
	if ts := a.liveToasts(); len(ts) == 0 || ts[len(ts)-1].text != "Screenshot saved" {
		t.Fatalf("toasts = %+v", ts)
	}
	f.Key(t, a, "F11", 0)
	if !a.hideUI || len(a.colonyButtons(a.size())) != 0 || a.colonyChromeHit(10, 5, 1280, 720) {
		t.Fatal("F11 hides every button and panel")
	}
	f.Key(t, a, "Escape", 0)
	if a.hideUI || a.nav.overlay() != overlayNone {
		t.Fatal("Esc brings the interface back first")
	}
}

func TestRoomStatsOverlayAndCard(t *testing.T) {
	t.Parallel()
	a, s, f := inputColony(t)
	s.world.TerrainW = 24
	s.world.RoomGrid = make([]int, 24*24)
	for i := range s.world.RoomGrid {
		s.world.RoomGrid[i] = -1
	}
	s.world.RoomGrid[5*24+5] = 0
	s.world.Rooms = []game.RoomView{{ID: 0, Role: "bedroom", Score: 14, Label: "nice", Lit: true, Temp: 215, Roofed: 1, Cleanliness: "clean", Cells: 12}}
	f.Key(t, a, "G", 0)
	if !a.rooms {
		t.Fatal("G turns the room overlay on")
	}
	if b, ok := hasButton(a.colonyButtons(a.size()), roomsButtonID); !ok || !b.Selected || !strings.Contains(b.Tip, "[G]") {
		t.Fatalf("HUD rooms button %+v", b)
	}
	lines := roomCardLines(s.world.Rooms[0])
	want := []string{"Bedroom", "Impressiveness 14 (nice)", "Cleanliness: clean", "Temperature 21.5°C", "Lit: yes   Roofed: 100%"}
	for i, w := range want {
		if i == 3 && !strings.HasPrefix(lines[i], "Temperature") {
			t.Fatalf("line %d = %q", i, lines[i])
		}
		if i != 3 && lines[i] != w {
			t.Fatalf("line %d = %q, want %q", i, lines[i], w)
		}
	}
	if lo, hi := roomTint(-5), roomTint(20); lo.R <= lo.G || hi.G <= hi.R {
		t.Fatalf("tints: awful %v impressive %v", lo, hi)
	}
}

func TestAltDetailLines(t *testing.T) {
	t.Parallel()
	w := game.WorldView{TerrainW: 2, Terrain: []uint8{0, 0}, RoomGrid: []int{-1, -1},
		Items: []game.ItemView{{ID: 9, Kind: "wood", Qty: 3, X: 1, Y: 0, Forbidden: true}}}
	lines := strings.Join(detailLines(w, 1, 0), "\n")
	for _, want := range []string{"Walk cost", "Fertility", "Room: outdoors", "#9 wood x 3 [forbidden]"} {
		if !strings.Contains(lines, want) {
			t.Fatalf("detail lines miss %q:\n%s", want, lines)
		}
	}
}

func TestShapeCellsStyles(t *testing.T) {
	t.Parallel()
	a, b := [2]int{0, 0}, [2]int{4, 4}
	count := func(s drawStyle, ctrl bool, b [2]int) int { return len(shapeCells(s, a, b, ctrl)) }
	if n := count(styleLine, false, [2]int{4, 1}); n != 5 {
		t.Fatalf("line = %d", n)
	}
	if n := count(styleRect, false, b); n != 25 {
		t.Fatalf("filled rect = %d", n)
	}
	if n := count(styleRectEdge, false, b); n != 16 {
		t.Fatalf("empty rect = %d", n)
	}
	oval := shapeCells(styleOval, a, b, false)
	in := map[[2]int]bool{}
	for _, c := range oval {
		in[c] = true
	}
	if len(oval) != 21 || in[[2]int{0, 0}] || !in[[2]int{2, 2}] || !in[[2]int{0, 2}] {
		t.Fatalf("filled oval (5x5 disc without corners) = %v", oval)
	}
	edge := shapeCells(styleOvalEdge, a, b, false)
	for _, c := range edge {
		if c == [2]int{2, 2} {
			t.Fatal("an empty oval leaves its middle out")
		}
	}
	if len(edge) != 12 {
		t.Fatalf("empty oval = %d cells: %v", len(edge), edge)
	}
	if n := count(styleRect, true, [2]int{4, 1}); n != 25 {
		t.Fatalf("Ctrl squares the box: %d cells", n)
	}
	if n := count(styleRect, true, [2]int{-2, 1}); n != 9 {
		t.Fatalf("Ctrl keeps the drag direction: %d cells", n)
	}
	runs := rowRuns(oval)
	if len(runs) != 5 || runs[0] != (rowRun{y: 0, x0: 1, x1: 3}) || runs[2] != (rowRun{y: 2, x0: 0, x1: 4}) {
		t.Fatalf("row runs = %+v", runs)
	}
}

func TestQECyclesDrawStyleAndOvalZonesSendRows(t *testing.T) {
	t.Parallel()
	a, s, f := inputColony(t)
	a.openArchitect()
	a.selectArchTool("wall")
	f.Key(t, a, "E", 0)
	f.Key(t, a, "E", 0)
	f.Key(t, a, "E", 0)
	f.Key(t, a, "E", 0)
	if wall, _ := toolByID("wall"); a.curStyle(wall) != styleOvalEdge {
		t.Fatalf("four E presses on a wall: %v", a.curStyle(wall))
	}
	a.selectArchTool("stockpile")
	f.Key(t, a, "Q", 0)
	if sp, _ := toolByID("stockpile"); a.curStyle(sp) != styleOval {
		t.Fatalf("zones offer rectangle and oval: %v", a.curStyle(sp))
	}
	s.cmds = nil
	f.Drag(t, a, mbLeft, 8, 2, 12, 6, 0)
	zs := cmdsOf(s, command.DesignateStockpile)
	if len(zs) != 5 || zs[0].Y != zs[0].Y2 || zs[2].X != 8 || zs[2].X2 != 12 {
		t.Fatalf("an oval zone is one rectangle per row: %+v", zs)
	}
	a.selectArchTool("bed")
	before := a.arch.Facing
	f.Key(t, a, "E", 0)
	if a.arch.Facing == before {
		t.Fatal("single placements keep Q/E rotation")
	}
}

func TestBillQuantityModifiers(t *testing.T) {
	t.Parallel()
	a, s, f := inputColony(t)
	s.world.Buildings = []game.BuildingView{{ID: 30, Kind: "stove", X: 4, Y: 4, W: 2, H: 1, Bench: true,
		Bills: []game.BillView{{Label: "Cook meal", Target: 5}}}}
	a.selectCand(selCand{Kind: selBuilding, ID: 30})
	// the bills live in the pane's Bills tab (PLAY-018): open it with the mouse
	pressButton(t, a, f, "inspector.bills")
	if a.paneOpenTab() != "inspector.bills" {
		t.Fatalf("the Bills tab did not open: %q", a.paneOpenTab())
	}
	for _, c := range []struct {
		mods input.Mod
		id   string
		want int
	}{{0, "bill.plus.0", 6}, {input.ModCtrl, "bill.plus.0", 15}, {input.ModShift, "bill.plus.0", 105},
		{input.ModCtrl | input.ModShift, "bill.plus.0", 1005}, {input.ModShift, "bill.minus.0", 1}} {
		s.cmds = nil
		f.Frame(t, a)
		x, y := widgetAt(t, a, c.id)
		f.Cur.Mx, f.Cur.My = x, y
		f.Click(t, a, mbLeft, c.mods)
		if set := cmdsOf(s, command.SetBill); len(set) != 1 || set[0].Count != c.want {
			t.Fatalf("%s with mods %v: %+v, want %d", c.id, c.mods, set, c.want)
		}
	}
	f.Cur.Mods = 0
}

func TestDevModeTickStepAndUltrafast(t *testing.T) {
	t.Parallel()
	a, s, f := inputColony(t, [2]int{5, 5})
	steps := s.steps
	f.Key(t, a, "NumpadAdd", 0)
	if s.steps != steps {
		t.Fatal("dev keys need developer mode")
	}
	set := a.settings
	set.Gameplay.DevMode = true
	a.setSettings(set)
	f.Key(t, a, "NumpadAdd", 0)
	if s.steps != steps+1 {
		t.Fatalf("one tick while paused: %d -> %d", steps, s.steps)
	}
	f.Frame(t, a)
	if s.steps != steps+1 {
		t.Fatal("and only one")
	}
	f.Key(t, a, "Digit4", 0)
	if a.renderer.speed != speedUltra || a.renderer.speed.ticksPerFrame() != ultraTicks {
		t.Fatalf("4 = ultrafast, got %v", a.renderer.speed)
	}
	f.Key(t, a, "Slash", 0)
	if !a.tools.State.Menu || len(a.devButtons()) == 0 {
		t.Fatal("/ opens the debug actions")
	}
	f.Key(t, a, "R", 0)
	if len(cmdsOf(s, command.DraftPawn)) != 0 {
		t.Fatal("map keys are off under the debug menu")
	}
	f.Key(t, a, "Escape", 0)
	if a.tools.State.Menu {
		t.Fatal("Esc closes the debug menu")
	}
	f.Key(t, a, "Backquote", 0)
	f.Key(t, a, "Quote", 0)
	if !a.tools.State.Log || !a.tools.State.Insp {
		t.Fatal("` and ' toggle the debug log and inspector")
	}
	set.Gameplay.DevMode = false
	a.setSettings(set)
	if a.renderer.speed == speedUltra || a.tools.State.Log {
		t.Fatal("leaving developer mode drops ultrafast and the panels")
	}
}

func TestControlsTabRebindsAndPersists(t *testing.T) {
	t.Parallel()
	store := &fakeSettings{s: config.Defaults()}
	a := optionsApp(store)
	f := fakeOn(a)
	click(t, a, "menu.options")
	click(t, a, "options.tab.controls")
	slot := ctlSlotID(input.Draft, 0)
	for _, b := range a.layoutButtons(1280, 720) {
		if b.ID == slot {
			goto found
		}
	}
	for a.rebind.Page < ctlPages(720)-1 {
		click(t, a, "options.ctl.next")
		for _, b := range a.layoutButtons(1280, 720) {
			if b.ID == slot {
				goto found
			}
		}
	}
	t.Fatal("no Draft row on any page")
found:
	click(t, a, slot)
	if !a.rebind.On {
		t.Fatal("clicking a slot waits for a key")
	}
	f.Key(t, a, "Escape", 0)
	if a.rebind.On || a.nav.screen() != screenOptions || a.bindings().Label(input.Draft) != "R" {
		t.Fatal("Esc cancels the capture (and does not leave Options)")
	}
	click(t, a, slot)
	f.Key(t, a, "Y", input.ModCtrl)
	if a.bindings().Label(input.Draft) != "Ctrl+Y" || store.s.Controls.Bindings["thing.draft"][0] != "Ctrl+Y" {
		t.Fatalf("rebound to %q, stored %v", a.bindings().Label(input.Draft), store.s.Controls.Bindings)
	}
	click(t, a, slot)
	f.Key(t, a, "F", 0) // Forbid's key
	if a.rebind.Pending == nil {
		t.Fatal("a used key asks before swapping")
	}
	click(t, a, "options.ctl.swap")
	if a.bindings().Label(input.Draft) != "F" || a.bindings().Label(input.Forbid) != "Ctrl+Y" {
		t.Fatalf("swap: draft=%q forbid=%q", a.bindings().Label(input.Draft), a.bindings().Label(input.Forbid))
	}
	again := optionsApp(store) // restart
	if again.bindings().Label(input.Draft) != "F" {
		t.Fatal("bindings survive a restart")
	}
	click(t, a, slot)
	f.Key(t, a, "Backspace", 0)
	if len(a.bindings().Of(input.Draft)) != 0 {
		t.Fatal("Backspace clears the slot")
	}
	click(t, a, "options.ctl.reset")
	if a.bindings().Label(input.Draft) != "R" || len(store.s.Controls.Bindings) != 0 {
		t.Fatalf("reset: %q %v", a.bindings().Label(input.Draft), store.s.Controls.Bindings)
	}
}

// The right-click that leaves an Architect tool must not also order the
// selected colonist (UI-016 double handling).
func TestRightClickLeavingToolDoesNotOrder(t *testing.T) {
	t.Parallel()
	a, s, f := inputColony(t, [2]int{5, 5})
	s.orders = []game.OrderOption{{Label: "Go here", Cmd: command.Command{Kind: command.MovePawn, Pawn: 1}}}
	a.renderer.Select(1)
	a.openArchitect()
	a.selectArchTool("wall")
	f.MoveTo(a, 12, 6)
	f.Click(t, a, mbRight, 0)
	if a.arch.ToolID != "" {
		t.Fatal("right-click drops the tool")
	}
	if a.ord.Float.Open || len(s.cmds) != 0 {
		t.Fatalf("and nothing else: menu=%v cmds=%+v", a.ord.Float.Open, s.cmds)
	}
	f.Click(t, a, mbRight, 0)
	if !a.ord.Float.Open {
		t.Fatal("the next right-click opens the order menu")
	}
}

// Esc in Move mode cancels the move and nothing more.
func TestEscInMoveModeHandledOnce(t *testing.T) {
	t.Parallel()
	a, s, f := inputColony(t)
	s.world.Buildings = []game.BuildingView{{ID: 40, Kind: "table", X: 6, Y: 6, W: 1, H: 1}}
	a.selectCand(selCand{Kind: selBuilding, ID: 40})
	a.startMove(s.world.Buildings[0])
	f.Frame(t, a) // the opening frame
	f.Key(t, a, "Escape", 0)
	if a.moveSt().On {
		t.Fatal("Esc leaves Move mode")
	}
	if !a.sel.HasBuilding || a.nav.overlay() != overlayNone {
		t.Fatal("the same Esc must not also deselect or pause")
	}
}

func TestCycleCellKey(t *testing.T) {
	t.Parallel()
	a, s, f := inputColony(t)
	s.world.Items = []game.ItemView{{ID: 80, Kind: "wood", Qty: 1, X: 7, Y: 7}}
	s.world.Plants = []game.PlantView{{ID: 81, Species: "oak", X: 7, Y: 7}}
	a.selectCand(selCand{Kind: selItem, ID: 80})
	f.Key(t, a, "NumpadMultiply", 0)
	if !a.sel.HasPlant || a.sel.SelPlant != 81 {
		t.Fatal("Numpad * goes to the next thing in the cell")
	}
}

func TestGizmoKeysPressInspectorButtons(t *testing.T) {
	t.Parallel()
	a, s, f := inputColony(t)
	s.world.Items = []game.ItemView{{ID: 90, Kind: "wood", Qty: 3, X: 3, Y: 3}}
	a.selectCand(selCand{Kind: selItem, ID: 90})
	cs := a.selectionCommands()
	i := -1
	for j, c := range cs {
		if c.ID == "item.forbid" {
			i = j
		}
	}
	if i < 0 || i >= len(input.Gizmos) {
		t.Fatalf("the Forbid button is a command with a key: %+v", cs)
	}
	f.Key(t, a, a.bindings().Of(input.Gizmos[i])[0].Key, 0)
	if c := cmdsOf(s, command.SetForbidden); len(c) != 1 || c[0].Target != 90 {
		t.Fatalf("gizmo key %d presses Forbid: %+v", i, s.cmds)
	}
}

func TestWheelZoomsUnlessALogTookIt(t *testing.T) {
	t.Parallel()
	a, _, f := inputColony(t)
	a.renderer.smooth.Instant = true
	z := a.renderer.cam.Zoom
	f.Cur.WheelY = 1
	f.Frame(t, a)
	if a.renderer.cam.Zoom <= z {
		t.Fatal("the wheel zooms the map")
	}
	z = a.renderer.cam.Zoom
	f.Key(t, a, "PageUp", 0)
	if a.renderer.cam.Zoom >= z {
		t.Fatal("PgUp zooms out")
	}
	ox := a.renderer.cam.OffsetX
	f.Cur.Mx, f.Cur.My = 400, 300
	f.Cur.Press[mbMiddle], f.Cur.Down[mbMiddle] = true, true
	f.Frame(t, a)
	f.Cur.Mx = 340
	f.Frame(t, a)
	f.Cur.Down[mbMiddle] = false
	if a.renderer.cam.OffsetX == ox {
		t.Fatal("a middle drag pans the map")
	}
}

func TestSpeedHomeEnd(t *testing.T) {
	t.Parallel()
	a, _, f := inputColony(t)
	f.Key(t, a, "Home", 0)
	if a.renderer.speed != speedNormal {
		t.Fatalf("Home from paused = %v", a.renderer.speed)
	}
	f.Key(t, a, "Home", 0)
	f.Key(t, a, "Home", 0)
	f.Key(t, a, "Home", 0)
	if a.renderer.speed != speedVeryFast {
		t.Fatal("Home stops at superfast")
	}
	f.Key(t, a, "End", 0)
	if a.renderer.speed != speedFast {
		t.Fatal("End slows down")
	}
	f.Key(t, a, "Space", 0)
	if a.renderer.speed != speedPaused {
		t.Fatal("Space pauses")
	}
}
