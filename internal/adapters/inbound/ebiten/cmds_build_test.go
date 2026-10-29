package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	selectionpkg "rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"strconv"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/input"
)

// PLAY-018: a building's and a blueprint's pane, commands and expanded
// tabs (cmds_build.go, building_inspector.go, pen_ui.go).

// bldCmd is the command button with id.
func bldCmd(cs []inspector.Button, id string) (inspector.Button, bool) {
	for _, c := range cs {
		if c.ID == id {
			return c, true
		}
	}
	return inspector.Button{}, false
}

func bldCmdIDs(cs []inspector.Button) []string {
	var out []string
	for _, c := range cs {
		out = append(out, c.ID)
	}
	return out
}

func bldTab(p inspector.Spec, id string) (inspector.Tab, bool) {
	for _, t := range p.Tabs {
		if t.ID == id {
			return t, true
		}
	}
	return inspector.Tab{}, false
}

func bldLines(p inspector.Spec) string { return strings.Join(p.Lines, "\n") }

// bldApp is a paused colony over s with building id selected.
func bldApp(t *testing.T, s *stubSim, id entity.EntityID) (*App, *fakeInput) {
	t.Helper()
	a, f := colonyApp(t, s)
	a.sel.Select(selectionpkg.Cand{Kind: selectionpkg.Building, ID: id})
	if !a.sel.HasBuilding {
		t.Fatalf("building %d is not selectable", id)
	}
	return a, f
}

func bldStub(bs ...game.BuildingView) *stubSim {
	s := &stubSim{w: 24, h: 24, pawns: []game.PawnView{{ID: 1, Name: "Ada", Status: "alive"}}}
	s.world.Buildings = bs
	return s
}

func TestBuildingPaneHeaderHealthAndClose(t *testing.T) {
	t.Parallel()
	b := game.BuildingView{ID: 9, Kind: "wall", Stuff: "stone", X: 4, Y: 5, W: 1, H: 1, HPPct: 64}
	a, _ := bldApp(t, bldStub(b), 9)
	p := a.insp.Building().BuildingPane(9)
	if p.Title != "Stone wall" || !strings.Contains(p.Subtitle, "(4, 5)") {
		t.Fatalf("header = %q / %q", p.Title, p.Subtitle)
	}
	if len(p.Bars) != 1 || p.Bars[0].Label != "Health" || p.Bars[0].Pct != 64 {
		t.Fatalf("bars = %+v", p.Bars)
	}
	if p.Close != "bldg.close" {
		t.Fatalf("close = %q", p.Close)
	}
	if len(p.Tabs) != 0 {
		t.Fatalf("a wall has no tabs: %+v", p.Tabs)
	}
	// an unknown building has no pane (the legacy panel stays gone)
	if a.insp.Building().BuildingPane(404).Title != "" {
		t.Fatal("a missing building must not produce a pane")
	}
	// no health reading yet: no bar
	b.HPPct = 0
	a2, _ := bldApp(t, bldStub(b), 9)
	if bars := a2.insp.Building().BuildingPane(9).Bars; len(bars) != 0 {
		t.Fatalf("no reading, no bar: %+v", bars)
	}
}

func TestBuildingPaneStatusLines(t *testing.T) {
	t.Parallel()
	moving := [2]int{7, 8}
	room := game.WorldView{TerrainW: 24, RoomGrid: make([]int, 24*24)}
	for i := range room.RoomGrid {
		room.RoomGrid[i] = -1
	}
	cases := []struct {
		name string
		b    game.BuildingView
		want []string
		bar  string
	}{
		{"deconstruct", game.BuildingView{Kind: "wall", Deconstruct: true}, []string{"Marked for deconstruction"}, ""},
		{"burning", game.BuildingView{Kind: "campfire", Fuel: true, FuelPct: 40, Burning: true}, []string{"Burning"}, "Fuel"},
		{"out of fuel", game.BuildingView{Kind: "campfire", Fuel: true}, []string{"Out of fuel"}, "Fuel"},
		{"banked", game.BuildingView{Kind: "campfire", Fuel: true, FuelPct: 70}, []string{"Banked (warm enough)"}, "Fuel"},
		{"drying", game.BuildingView{Kind: "drying rack", DryQty: 3, DryKind: "meat", DryPct: 50}, []string{"Drying 3 meat, 50%"}, ""},
		{"grave", game.BuildingView{Kind: "grave", Occupant: "Ada", Epitaph: "Loved."}, []string{"Here lies Ada", "Loved."}, ""},
		{"empty grave", game.BuildingView{Kind: "grave"}, []string{"Empty grave"}, ""},
		{"vent", game.BuildingView{Kind: env.SteamVentKind}, []string{env.VentLine(game.BuildingView{Kind: env.SteamVentKind})}, ""},
		{"carpet", game.BuildingView{Kind: "carpet", Stuff: "straw carpet", Catch: 35}, []string{"Catches 35% of tracked dirt"}, ""},
		{"bridge", game.BuildingView{Kind: overlay.BridgeKind}, []string{"Holds light things: no stone walls or floors"}, ""},
		{"swap", game.BuildingView{Kind: "wall", SwapTo: "steel", SwapPct: 40}, []string{"Changing to steel (40%)"}, ""},
		{"moving", game.BuildingView{Kind: "butcher table", MovingTo: &moving}, []string{"Moving to (7, 8)"}, ""},
		{"room", game.BuildingView{Kind: "wall"}, []string{"Outdoors"}, ""},
	}
	for _, c := range cases {
		c.b.ID, c.b.X, c.b.Y, c.b.W, c.b.H = 5, 3, 3, 1, 1
		s := bldStub(c.b)
		s.world.TerrainW, s.world.RoomGrid = room.TerrainW, room.RoomGrid
		a, _ := bldApp(t, s, 5)
		p := a.insp.Building().BuildingPane(5)
		for _, w := range c.want {
			if !strings.Contains(bldLines(p), w) {
				t.Errorf("%s: lines %q lack %q", c.name, p.Lines, w)
			}
		}
		if c.bar != "" {
			found := false
			for _, br := range p.Bars {
				found = found || br.Label == c.bar
			}
			if !found {
				t.Errorf("%s: no %s bar in %+v", c.name, c.bar, p.Bars)
			}
		}
	}
}

func TestBedPaneOwnersAndCommands(t *testing.T) {
	t.Parallel()
	s := bedStub()
	a, _ := bldApp(t, s.stubSim, 40)
	a.renderer.game = s
	p := a.insp.Building().BuildingPane(40)
	if !strings.Contains(bldLines(p), "Owner 1: Ada") || !strings.Contains(bldLines(p), "Owner 2: nobody") {
		t.Fatalf("owner lines = %q", p.Lines)
	}
	cs := a.insp.Building().BuildingCommands(40)
	for _, id := range []string{inspector.BedOwnerID(0), inspector.BedOwnerID(1), "bldg.bed.prisoners"} {
		c, ok := bldCmd(cs, id)
		if !ok {
			t.Fatalf("no %s command in %v", id, bldCmdIDs(cs))
		}
		if c.Run != nil || len(c.Cmds) != 0 {
			t.Errorf("%s is a legacy id: it keeps its handler, so no Run/Cmds", id)
		}
	}
	pr, _ := bldCmd(cs, "bldg.bed.prisoners")
	if !pr.Toggle || pr.On || pr.Disabled {
		t.Fatalf("prisoner toggle %+v", pr)
	}
	s.prison[40] = true
	if pr, _ = bldCmd(a.insp.Building().BuildingCommands(40), "bldg.bed.prisoners"); !pr.On {
		t.Fatal("the toggle does not read the mark back")
	}
	if !strings.Contains(bldLines(a.insp.Building().BuildingPane(40)), "For prisoners") {
		t.Fatalf("a prisoner bed says so: %q", a.insp.Building().BuildingPane(40).Lines)
	}
	// without the read model the toggle is disabled and says why
	a.renderer.game = s.stubSim
	if pr, _ = bldCmd(a.insp.Building().BuildingCommands(40), "bldg.bed.prisoners"); !pr.Disabled || pr.Reason == "" {
		t.Fatalf("toggle without the read model: %+v", pr)
	}
	// a single bed has one owner command
	one := bldStub(game.BuildingView{ID: 41, Kind: "bed", X: 2, Y: 2, W: 1, H: 2})
	a1, _ := bldApp(t, one, 41)
	c1 := a1.insp.Building().BuildingCommands(41)
	if _, ok := bldCmd(c1, inspector.BedOwnerID(0)); !ok {
		t.Fatalf("single bed: %v", bldCmdIDs(c1))
	}
	if _, ok := bldCmd(c1, inspector.BedOwnerID(1)); ok {
		t.Fatal("a single bed has no second owner")
	}
	if !strings.Contains(bldLines(a1.insp.Building().BuildingPane(41)), "Owner: nobody") {
		t.Fatalf("single bed lines: %q", a1.insp.Building().BuildingPane(41).Lines)
	}
}

func TestTroughPaneAndFilterOptions(t *testing.T) {
	t.Parallel()
	b := game.BuildingView{ID: 77, Kind: inspector.TroughKind, X: 4, Y: 4, W: 1, H: 2, TroughFill: 12, TroughCap: 60, TroughFilter: "plants"}
	a, _ := bldApp(t, bldStub(b), 77)
	p := a.insp.Building().BuildingPane(77)
	var feed *inspector.Bar
	for i := range p.Bars {
		if p.Bars[i].Label == "Feed" {
			feed = &p.Bars[i]
		}
	}
	if feed == nil || feed.Pct != 20 {
		t.Fatalf("feed bar = %+v in %+v", feed, p.Bars)
	}
	if !strings.Contains(bldLines(p), "Accepts: plants") || !strings.Contains(bldLines(p), "Pen animals (0)") {
		t.Fatalf("lines = %q", p.Lines)
	}
	c, ok := bldCmd(a.insp.Building().BuildingCommands(77), "cmd.bldg.trough")
	if !ok || c.Options == nil {
		t.Fatalf("trough filter command = %+v", c)
	}
	opts := c.Options()
	if len(opts) != 3 || opts[0].Label != "Plants" || opts[1].Label != "Meat" || opts[2].Label != "Any" {
		t.Fatalf("options = %+v", opts)
	}
	if !opts[0].Disabled || opts[0].Reason == "" || opts[1].Disabled || opts[2].Disabled {
		t.Fatalf("the current filter is the only disabled option: %+v", opts)
	}
	// left-click runs the first enabled option: Meat
	var modes []uint8
	for _, o := range opts {
		if o.Cmd == nil || o.Cmd.Kind != command.SetTroughFilter || o.Cmd.Target != 77 {
			t.Fatalf("option %q = %+v", o.Label, o.Cmd)
		}
		modes = append(modes, o.Cmd.Mode)
	}
	if modes[0] != 0 || modes[1] != 1 || modes[2] != 2 {
		t.Fatalf("modes = %v, want plants 0, meat 1, any 2", modes)
	}
}

func TestTroughPaneListsThePenHerd(t *testing.T) {
	t.Parallel()
	cell := func(x, y int) uint64 { return 1 << (y*10 + x) }
	s := bldStub(game.BuildingView{ID: 77, Kind: inspector.TroughKind, X: 2, Y: 2, W: 1, H: 1, TroughFilter: "any"})
	s.world.TerrainW = 10
	s.world.Pens = []game.PenView{{Enclosed: true, Bits: []uint64{cell(1, 1) | cell(2, 1)}}}
	for i := 0; i < inspector.MaxTroughAnimals+2; i++ {
		s.world.Animals = append(s.world.Animals, game.AnimalView{ID: entity.EntityID(100 + i), Name: "Moo", Label: "cow", Tame: true, X: 1, Y: 1})
	}
	a, _ := bldApp(t, s, 77)
	got := bldLines(a.insp.Building().BuildingPane(77))
	if !strings.Contains(got, "Pen animals (8)") || !strings.Contains(got, "... and 2 more") || !strings.Contains(got, "Moo (") {
		t.Fatalf("lines = %q", got)
	}
}

func TestBuildingCommandsGeneric(t *testing.T) {
	t.Parallel()
	a, _ := bldApp(t, bldStub(game.BuildingView{ID: 9, Kind: "wall", Stuff: "stone", X: 4, Y: 5, W: 1, H: 1}), 9)
	cs := a.insp.Building().BuildingCommands(9)
	if got := bldCmdIDs(cs); strings.Join(got, ",") != "bldg.move,bldg.copy,bldg.deconstruct" {
		t.Fatalf("a wall's commands = %v", got)
	}
	mv, _ := bldCmd(cs, "bldg.move")
	if !mv.Disabled || mv.Reason != "can't be moved" || mv.Icon != "move" {
		t.Fatalf("move = %+v", mv)
	}
	cp, _ := bldCmd(cs, "bldg.copy")
	if cp.Disabled || cp.Icon != "copy" {
		t.Fatalf("copy = %+v", cp)
	}
	dc, _ := bldCmd(cs, "bldg.deconstruct")
	if dc.Disabled || dc.Icon != "order_deconstruct" || dc.Label != "Deconstruct" {
		t.Fatalf("deconstruct = %+v", dc)
	}
	for _, c := range cs {
		if c.Run != nil || len(c.Cmds) != 0 || c.Options != nil {
			t.Errorf("%s is a legacy id: its click goes to applyClick", c.ID)
		}
		if c.Desc == "" {
			t.Errorf("%s has no tooltip", c.ID)
		}
	}
	// a building already marked has no Deconstruct
	a2, _ := bldApp(t, bldStub(game.BuildingView{ID: 9, Kind: "wall", Deconstruct: true, W: 1, H: 1}), 9)
	if _, ok := bldCmd(a2.insp.Building().BuildingCommands(9), "bldg.deconstruct"); ok {
		t.Fatal("a marked building offers no second Deconstruct")
	}
	// a steam vent is natural: nothing to do
	a3, _ := bldApp(t, bldStub(game.BuildingView{ID: 9, Kind: env.SteamVentKind, W: 1, H: 1}), 9)
	if cs := a3.insp.Building().BuildingCommands(9); len(cs) != 0 {
		t.Fatalf("a steam vent has commands: %v", bldCmdIDs(cs))
	}
	if a3.insp.Building().BuildingCommands(404) != nil {
		t.Fatal("a missing building has no commands")
	}
}

func TestBuildingCommandsCancelMoveAndLockedCopy(t *testing.T) {
	t.Parallel()
	at := [2]int{4, 5}
	s := bldStub(game.BuildingView{ID: 9, Kind: "butcher table", X: 2, Y: 2, W: 2, H: 1, MovingTo: &at})
	s.world.Locked = map[string]string{"build:butcher table": "Smithing"}
	a, _ := bldApp(t, s, 9)
	cs := a.insp.Building().BuildingCommands(9)
	if got := strings.Join(bldCmdIDs(cs), ","); got != "bldg.move,bldg.copy,bldg.cancelmove,bldg.deconstruct" {
		t.Fatalf("commands = %s", got)
	}
	if c, _ := bldCmd(cs, "bldg.copy"); !c.Disabled || c.Reason != "needs research: Smithing" {
		t.Fatalf("locked copy = %+v", c)
	}
	if c, _ := bldCmd(cs, "bldg.cancelmove"); c.Icon != "order_cancel" || c.Disabled {
		t.Fatalf("cancel move = %+v", c)
	}
	if mv, _ := bldCmd(cs, "bldg.move"); mv.Disabled {
		t.Fatalf("a butcher table moves: %+v", mv)
	}
}

func TestPenMarkerOffersOnlyDeconstruct(t *testing.T) {
	t.Parallel()
	a := penApp(t, penSim())
	if got := strings.Join(bldCmdIDs(a.insp.Building().BuildingCommands(70)), ","); got != "bldg.deconstruct" {
		t.Fatalf("pen marker commands = %s", got)
	}
}

func TestPenMarkerPaneStatusAndTab(t *testing.T) {
	t.Parallel()
	a := penApp(t, penSim())
	p := a.insp.Building().BuildingPane(70)
	if p.Title != "Pen marker" || p.Close != "bldg.close" {
		t.Fatalf("header = %q close %q", p.Title, p.Close)
	}
	got := bldLines(p)
	for _, want := range []string{"Pen 1   2 animals", "Grazing: 30 per day", "Herd needs: 45 per day", "Trough stock: 12",
		"Short by 15 per day: add hay or expand"} {
		if !strings.Contains(got, want) {
			t.Errorf("lines %q lack %q", got, want)
		}
	}
	tab, ok := bldTab(p, "inspector.pen")
	if !ok || tab.Label != "Pen" || tab.Declare == nil || tab.WantH <= 0 || tab.WantW < hudlayout.ExpandMinW {
		t.Fatalf("pen tab = %+v ok=%v", tab, ok)
	}
	if _, ok := bldTab(p, "inspector.bills"); ok {
		t.Error("a marker has no bills tab")
	}
	// an open pen warns
	s := penSim()
	s.world.Pens[0].Enclosed, s.world.Pens[0].Bits = false, nil
	open := penApp(t, s)
	if !strings.Contains(bldLines(open.insp.Building().BuildingPane(70)), "Not enclosed") {
		t.Fatalf("an open pen warns: %q", open.insp.Building().BuildingPane(70).Lines)
	}
}

func TestBenchPaneHasBillsTab(t *testing.T) {
	t.Parallel()
	kind, _ := benchWithMostRecipes()
	b := game.BuildingView{ID: 77, Kind: kind, X: 8, Y: 8, W: 2, H: 1, Bench: true, Bills: []game.BillView{{Label: "a"}, {Label: "b"}}}
	a, _ := bldApp(t, bldStub(b), 77)
	p := a.insp.Building().BuildingPane(77)
	tab, ok := bldTab(p, "inspector.bills")
	if !ok || tab.Label != "Bills" || tab.Declare == nil || tab.Draw != nil {
		t.Fatalf("bills tab = %+v ok=%v", tab, ok)
	}
	if tab.WantW < hudlayout.ExpandMinW || tab.WantW > hudlayout.ExpandMaxW || tab.WantH <= 0 {
		t.Fatalf("bills tab wants %vx%v", tab.WantW, tab.WantH)
	}
	// more bills ask for a taller panel
	b.Bills = append(b.Bills, game.BillView{Label: "c"}, game.BillView{Label: "d"})
	a2, _ := bldApp(t, bldStub(b), 77)
	if t2, _ := bldTab(a2.insp.Building().BuildingPane(77), "inspector.bills"); t2.WantH <= tab.WantH {
		t.Fatalf("4 bills want %v, 2 want %v", t2.WantH, tab.WantH)
	}
	// a plain building has no bills tab
	a3, _ := bldApp(t, bldStub(game.BuildingView{ID: 3, Kind: "wall", W: 1, H: 1}), 3)
	if len(a3.insp.Building().BuildingPane(3).Tabs) != 0 {
		t.Fatal("a wall has no tabs")
	}
}

func TestCasketCommandInPane(t *testing.T) {
	t.Parallel()
	b := game.BuildingView{ID: 5, Kind: overlay.CasketKind, X: 2, Y: 2, W: 1, H: 2, Sealed: true, OpenReason: "Nobody can reach it."}
	a, _ := bldApp(t, bldStub(b), 5)
	if got := a.insp.Building().BuildingPane(5).Title; got != "Sleeper casket (sealed)" {
		t.Fatalf("title = %q", got)
	}
	c, ok := bldCmd(a.insp.Building().BuildingCommands(5), inspector.CasketBtnID)
	if !ok || !c.Disabled || c.Reason != "Nobody can reach it." || c.Icon != "open_casket" {
		t.Fatalf("casket command = %+v ok=%v", c, ok)
	}
	b.OpenReason = ""
	a2, _ := bldApp(t, bldStub(b), 5)
	if c, _ = bldCmd(a2.insp.Building().BuildingCommands(5), inspector.CasketBtnID); c.Disabled {
		t.Fatalf("an accepted casket: %+v", c)
	}
}

// ---- the blueprint -------------------------------------------------------

func TestFramePaneLinesAndBars(t *testing.T) {
	t.Parallel()
	a, _ := frameApp(t, wallFrame())
	p := a.insp.Thing().FramePane(7)
	if p.Title != "Wall (frame)" || p.Close != "frame.close" {
		t.Fatalf("header = %q close %q", p.Title, p.Close)
	}
	if !strings.Contains(p.Subtitle, "Footprint 1x1") || !strings.Contains(p.Subtitle, "facing south") || !strings.Contains(p.Subtitle, "(3, 3)") {
		t.Fatalf("subtitle = %q", p.Subtitle)
	}
	got := bldLines(p)
	for _, want := range []string{"Status: Materials on the way", "Materials  wood 6 / 10 (+4 on the way)",
		"In stock  45 wood (12 reserved)", "Work  0 / 40  (starts once materials are in)", "Hauling: Ada (4 wood)   Building: -"} {
		if !strings.Contains(got, want) {
			t.Errorf("lines %q lack %q", got, want)
		}
	}
	bars := map[string]int{}
	for _, b := range p.Bars {
		bars[b.Label] = b.Pct
	}
	if bars["Materials"] != 60 || bars["Work"] != 0 || len(p.Bars) != 2 {
		t.Fatalf("bars = %+v", p.Bars)
	}
	if len(p.Tabs) != 0 {
		t.Fatalf("a blueprint has no tabs: %+v", p.Tabs)
	}
	if a.insp.Thing().FramePane(404).Title != "" {
		t.Fatal("a missing frame must not produce a pane")
	}
}

func TestFramePaneReinstallHasNoMaterials(t *testing.T) {
	t.Parallel()
	mv := game.FrameView{ID: 8, Kind: "workbench", Moving: true, MovingFrom: 5, X: 2, Y: 2, W: 2, H: 1, WorkTotal: 30, WorkLeft: 30}
	a, _ := frameApp(t, mv)
	p := a.insp.Thing().FramePane(8)
	if !strings.Contains(bldLines(p), "Moving workbench here: needs no materials") {
		t.Fatalf("lines = %q", p.Lines)
	}
	for _, b := range p.Bars {
		if b.Label == "Materials" {
			t.Fatalf("a reinstall needs no materials: %+v", p.Bars)
		}
	}
}

func TestFrameCommandsCancelBlueprint(t *testing.T) {
	t.Parallel()
	a, s := frameApp(t, wallFrame())
	cs := a.insp.Thing().FrameCommands(7)
	if len(cs) != 1 {
		t.Fatalf("commands = %v", bldCmdIDs(cs))
	}
	c := cs[0]
	if c.ID != "frame.cancel" || c.Icon != "order_cancel" || c.Desc == "" || c.Disabled {
		t.Fatalf("cancel = %+v", c)
	}
	if c.Run != nil || len(c.Cmds) != 0 {
		t.Fatal("frame.cancel is a legacy id: its handler sends CancelFrame")
	}
	a.applyClick(c.ID)
	if got := lastCmd(t, s); got.Kind != command.CancelFrame || got.Target != 7 {
		t.Fatalf("cmd = %+v", got)
	}
	if a.insp.Thing().FrameCommands(404) != nil {
		t.Fatal("a missing frame has no commands")
	}
}

// ---- the legacy panels are gone ------------------------------------------

func TestLegacyBuildingAndFramePanelsDrawNothing(t *testing.T) {
	t.Parallel()
	a, _ := bldApp(t, bldStub(game.BuildingView{ID: 9, Kind: "wall", W: 1, H: 1}), 9)
	assertChromeIsTheLayout(t, a)                   // no legacy building panel blocks the map
	assertButtonsInRegions(t, a)                    // and no legacy button floats outside the pane and grid
	a.drawBuildingMark(ebiten.NewImage(a.sw, a.sh)) // only the map outline is left

	fa, _ := frameApp(t, wallFrame())
	assertChromeIsTheLayout(t, fa)
	assertButtonsInRegions(t, fa)
	fa.drawFrameMark(ebiten.NewImage(fa.sw, fa.sh))
}

// ---- the bills tab -------------------------------------------------------

// billsTabApp is a paused colony with a bench (id 77) holding bills, and
// the bills tab declared into panel (the pane's expanded rectangle).
func billsTabApp(t *testing.T, bills []game.BillView) (*App, *stubSim, *fakeInput, gfx.Rect) {
	t.Helper()
	kind, _ := benchWithMostRecipes()
	s := bldStub(game.BuildingView{ID: 77, Kind: kind, X: 8, Y: 8, W: 2, H: 1, Bench: true, Bills: bills})
	a, f := bldApp(t, s, 77)
	panel := gfx.Rect{X: 0, Y: 180, W: 520, H: 300}
	setHook(a.uiSt(), func(a *App) { a.insp.Building().BillsTabUI(77, panel) })
	f.Frame(t, a)
	return a, s, f, panel
}

func threeBills() []game.BillView {
	return []game.BillView{
		{Label: "Cook meal", ModeID: inspector.BillModeDo, Target: 5, Done: 2},
		{Label: "Make bandage", ModeID: inspector.BillModeForever, Target: 3, Status: "no cloth"},
		{Label: "Make pemmican", ModeID: inspector.BillModeUntil, Target: 10, Suspended: true},
	}
}

func TestBillsTabDeclaresEveryRowControlInsideThePanel(t *testing.T) {
	t.Parallel()
	a, _, _, panel := billsTabApp(t, threeBills())
	var ids []string
	for i := range threeBills() {
		for _, k := range []string{"minus", "plus", "mode", "pause", "del"} {
			ids = append(ids, "bill."+k+"."+strconv.Itoa(i))
		}
	}
	ids = append(ids, inspector.BillPickBtn)
	for _, id := range ids {
		w, ok := a.uiSt().Find(id)
		if !ok {
			t.Errorf("no widget %s", id)
			continue
		}
		if w.R.X < panel.X || w.R.Y < panel.Y || w.R.Right() > panel.Right() || w.R.Bottom() > panel.Bottom() {
			t.Errorf("%s at %+v falls out of the panel %+v", id, w.R, panel)
		}
	}
	// no two controls overlap (the scroll view's own widget aside)
	cur := a.uiSt().Cur
	for i, p := range cur {
		for _, q := range cur[i+1:] {
			if p.ID == "bill.rows" || q.ID == "bill.rows" {
				continue
			}
			if p.R.X < q.R.Right() && q.R.X < p.R.Right() && p.R.Y < q.R.Bottom() && q.R.Y < p.R.Bottom() {
				t.Errorf("%s overlaps %s", p.ID, q.ID)
			}
		}
	}
	for _, id := range ids {
		if strings.HasPrefix(id, "bill.add.") {
			t.Errorf("per-recipe button %s would overflow the panel", id)
		}
	}
}

func TestBillsTabButtonsWorkByID(t *testing.T) {
	t.Parallel()
	a, s, f, _ := billsTabApp(t, threeBills())

	press(t, a, f, "bill.plus.0") // 5 -> 6
	c := lastCmd(t, s)
	if c.Kind != command.SetBill || c.Index != 0 || c.Count != 6 || c.Mode != inspector.BillModeDo || c.X != 8 || c.Y != 8 {
		t.Fatalf("plus: %+v", c)
	}
	press(t, a, f, "bill.minus.0") // 5 -> 4
	if c = lastCmd(t, s); c.Kind != command.SetBill || c.Count != 4 {
		t.Fatalf("minus: %+v", c)
	}
	press(t, a, f, "bill.mode.1") // forever -> until
	if c = lastCmd(t, s); c.Kind != command.SetBill || c.Index != 1 || c.Mode != inspector.BillModeUntil {
		t.Fatalf("mode: %+v", c)
	}
	press(t, a, f, "bill.pause.2") // suspended -> resumed
	if c = lastCmd(t, s); c.Kind != command.SetBill || c.Index != 2 || c.Enabled {
		t.Fatalf("pause of a suspended bill must resume it: %+v", c)
	}
	press(t, a, f, "bill.pause.0")
	if c = lastCmd(t, s); c.Kind != command.SetBill || c.Index != 0 || !c.Enabled {
		t.Fatalf("pause of a running bill must suspend it: %+v", c)
	}
	press(t, a, f, "bill.del.1")
	if c = lastCmd(t, s); c.Kind != command.RemoveBill || c.Index != 1 {
		t.Fatalf("del: %+v", c)
	}
}

func TestBillsTabAddBillOpensPickerAndAdds(t *testing.T) {
	t.Parallel()
	a, s, f, _ := billsTabApp(t, nil)
	press(t, a, f, inspector.BillPickBtn)
	f.Frame(t, a)
	if a.uiModalTop() != inspector.BillPickID {
		t.Fatalf("top modal %q, want the recipe picker", a.uiModalTop())
	}
	if !hasWidget(a, uikit.OptID(inspector.BillPickList, 0)) {
		t.Fatal("recipes are listed")
	}
	press(t, a, f, uikit.OptID(inspector.BillPickList, 0))
	press(t, a, f, "modal."+inspector.BillPickID+".bill.picker.ok")
	if len(cmdsOf(s, command.AddBill)) != 1 {
		t.Fatalf("AddBill not sent: %+v", s.cmds)
	}
}

func TestBillsTabScrollsLongLists(t *testing.T) {
	t.Parallel()
	var many []game.BillView
	for i := 0; i < 30; i++ {
		many = append(many, game.BillView{Label: "Cook meal", Target: i + 1})
	}
	a, s, f, panel := billsTabApp(t, many)
	if hasWidget(a, "bill.plus.29") {
		t.Fatal("a row below the view must not be live")
	}
	if !hasWidget(a, "bill.plus.0") || !hasWidget(a, inspector.BillPickBtn) {
		t.Fatal("the first row and the Add button stay reachable")
	}
	for _, id := range []string{"bill.plus.0", "bill.plus.1"} {
		w, _ := a.uiSt().Find(id)
		if w.R.Bottom() > panel.Bottom() {
			t.Fatalf("%s spills out of the panel: %+v", id, w.R)
		}
	}
	a.uiSt().Scroll["bill.rows"] = 1e6 // the wheel to the end
	f.Frame(t, a)
	if !hasWidget(a, "bill.plus.29") || hasWidget(a, "bill.plus.0") {
		t.Fatal("scrolled to the end shows the last rows")
	}
	press(t, a, f, "bill.del.29")
	if c := lastCmd(t, s); c.Kind != command.RemoveBill || c.Index != 29 {
		t.Fatalf("del on a scrolled row: %+v", c)
	}
}

// Ctrl, Shift and both scale a bill's quantity step x10, x100 and x1000
// (UI-016), pressed on the tab's own buttons.
func TestBillsTabQuantityModifiers(t *testing.T) {
	t.Parallel()
	a, s, f, _ := billsTabApp(t, []game.BillView{{Label: "Cook meal", Target: 5}})
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
		f.Click(t, a, inputframe.Left, c.mods)
		if set := cmdsOf(s, command.SetBill); len(set) != 1 || set[0].Count != c.want {
			t.Fatalf("%s with mods %v: %+v, want %d", c.id, c.mods, set, c.want)
		}
	}
}

// The pane's close button is the spec's Close id: its click clears the
// selection through the existing handlers.
func TestPaneCloseIDsClearTheSelection(t *testing.T) {
	t.Parallel()
	a, _ := bldApp(t, bldStub(game.BuildingView{ID: 9, Kind: "wall", W: 1, H: 1}), 9)
	a.applyClick(a.insp.Building().BuildingPane(9).Close)
	if a.sel.HasBuilding {
		t.Fatal("closing the building pane keeps the building selected")
	}
	fa, _ := frameApp(t, wallFrame())
	fa.applyClick(fa.insp.Thing().FramePane(7).Close)
	if fa.sel.HasFrame {
		t.Fatal("closing the blueprint pane keeps the frame selected")
	}
}

// A tab declared into the inner rectangle of the panel it asked for shows
// every row and the Add button with no scrolling: the wanted size is
// right.
func TestBillsTabFitsItsWantedPanel(t *testing.T) {
	t.Parallel()
	kind, _ := benchWithMostRecipes()
	for _, n := range []int{0, 1, 3, 8} {
		var bills []game.BillView
		for i := 0; i < n; i++ {
			bills = append(bills, game.BillView{Label: "Cook meal", Target: i + 1})
		}
		s := bldStub(game.BuildingView{ID: 77, Kind: kind, X: 8, Y: 8, W: 2, H: 1, Bench: true, Bills: bills})
		a, f := bldApp(t, s, 77)
		tab, _ := bldTab(a.insp.Building().BuildingPane(77), "inspector.bills")
		inner := gfx.Rect{X: 0, Y: 120, W: tab.WantW, H: tab.WantH}.Inset(inspector.PaneInset)
		setHook(a.uiSt(), func(a *App) { tab.Declare(inner) })
		f.Frame(t, a)
		for i := 0; i < n; i++ {
			if !hasWidget(a, "bill.del."+strconv.Itoa(i)) {
				t.Errorf("%d bills: row %d is not live in its wanted panel", n, i)
			}
		}
		w, ok := a.uiSt().Find(inspector.BillPickBtn)
		if !ok || w.R.Bottom() > inner.Bottom()+0.01 || w.R.Y < inner.Y {
			t.Errorf("%d bills: Add bill at %+v, inner %+v", n, w.R, inner)
		}
	}
}

// The Pen tab fits its wanted panel the same way.
func TestPenTabFitsItsWantedPanel(t *testing.T) {
	t.Parallel()
	a := penApp(t, penSim())
	tab, _ := bldTab(a.insp.Building().BuildingPane(70), "inspector.pen")
	inner := gfx.Rect{X: 0, Y: 120, W: tab.WantW, H: tab.WantH}.Inset(inspector.PaneInset)
	body := inspector.PenTabBody(a.renderer.world(), penSim().world.Pens[0], inner.W-uikit.ScrollBarW)
	if body.H > inner.H+0.01 {
		t.Fatalf("pen body %v tall in an inner panel %v tall", body.H, inner.H)
	}
}

// A bench selected through the real HUD: the pane replaces the legacy
// top-left panel, its commands fill the grid, and the Bills tab opens above
// the pane with live, in-panel controls.
func TestBenchSelectionThroughTheHUD(t *testing.T) {
	t.Parallel()
	kind, _ := benchWithMostRecipes()
	s := bldStub(game.BuildingView{ID: 77, Kind: kind, X: 8, Y: 8, W: 2, H: 1, Bench: true, HPPct: 90,
		Bills: []game.BillView{{Label: "Cook meal", Target: 5}, {Label: "Make bandage", Target: 2}}})
	a, f := bldApp(t, s, 77)
	f.Frame(t, a)
	l := a.hud()
	if l.Pane.Empty() || l.PaneTabs.Empty() {
		t.Fatalf("no pane for a selected bench: %+v", l)
	}
	if got := len(a.insp.Building().BuildingCommands(77)); got == 0 || len(l.GridCells) != got {
		t.Fatalf("grid cells %d, commands %d", len(l.GridCells), got)
	}
	assertChromeIsTheLayout(t, a) // the legacy panel's area is not reserved
	if !l.Expanded.Empty() {
		t.Fatal("the Bills tab starts closed")
	}

	a.applyClick("inspector.bills")
	f.Frame(t, a)
	l = a.hud()
	if l.Expanded.Empty() {
		t.Fatal("clicking the Bills tab opens its panel")
	}
	for _, id := range []string{"bill.minus.0", "bill.plus.1", "bill.del.1", inspector.BillPickBtn} {
		w, ok := a.uiSt().Find(id)
		if !ok {
			t.Fatalf("no widget %s with the Bills tab open", id)
		}
		if w.R.X < l.Expanded.X || w.R.Right() > l.Expanded.Right() || w.R.Y < l.Expanded.Y || w.R.Bottom() > l.Expanded.Bottom() {
			t.Errorf("%s at %+v falls out of the expanded panel %+v", id, w.R, l.Expanded)
		}
	}
	press(t, a, f, "bill.plus.1")
	if c := lastCmd(t, s); c.Kind != command.SetBill || c.Index != 1 || c.Count != 3 {
		t.Fatalf("plus on the second bill: %+v", c)
	}

	// the grid: Deconstruct is a legacy id, clicked like the old button was
	click(t, a, "bldg.deconstruct")
	if c := lastCmd(t, s); c.Kind != command.DesignateDeconstruct || c.X != 8 || c.X2 != 9 {
		t.Fatalf("deconstruct: %+v", c)
	}
}

// The trough's filter, run from the grid: a click switches to the first
// other filter.
func TestTroughFilterCommandRunsFromTheGrid(t *testing.T) {
	t.Parallel()
	b := game.BuildingView{ID: 77, Kind: inspector.TroughKind, X: 4, Y: 4, W: 1, H: 2, TroughFill: 12, TroughCap: 60, TroughFilter: "plants"}
	s := bldStub(b)
	a, f := bldApp(t, s, 77)
	f.Frame(t, a)
	a.insp.Commands().ApplyCommandClick("cmd.bldg.trough")
	if c := lastCmd(t, s); c.Kind != command.SetTroughFilter || c.Target != 77 || c.Mode != 1 {
		t.Fatalf("a click on the filter: %+v, want meat (1)", c)
	}
}

// A blueprint selected through the real HUD: its pane, its Cancel command in
// the grid, and Cancel doing what the old button did.
func TestBlueprintSelectionThroughTheHUD(t *testing.T) {
	t.Parallel()
	s := bldStub()
	s.world.Frames = []game.FrameView{wallFrame()}
	a, f := colonyApp(t, s)
	a.sel.Select(selectionpkg.Cand{Kind: selectionpkg.Frame, ID: 7})
	f.Frame(t, a)
	l := a.hud()
	if l.Pane.Empty() || len(l.GridCells) != 1 {
		t.Fatalf("blueprint layout: pane %+v cells %d", l.Pane, len(l.GridCells))
	}
	assertChromeIsTheLayout(t, a) // the legacy panel's area is not reserved
	click(t, a, "frame.cancel")
	if c := lastCmd(t, s); c.Kind != command.CancelFrame || c.Target != 7 {
		t.Fatalf("cancel: %+v", c)
	}
	if a.sel.HasFrame {
		t.Fatal("the pane stays up after Cancel")
	}
}
