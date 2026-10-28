package ebiten

import (
	"fmt"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/input"
)

// PLAY-018: the animal, item and plant panes and their command buttons
// (cmds_thing.go). These tests read the producers directly (what the pane
// and the grid are told to show) and press commands the way the grid
// does, so they hold whatever draws the pane.

// thingCmd is the command with id, or a failure.
func thingCmd(t *testing.T, cs []cmdButton, id string) cmdButton {
	t.Helper()
	c, ok := thingFind(cs, id)
	if !ok {
		t.Fatalf("no command %q among %v", id, thingIDs(cs))
	}
	return c
}

func thingFind(cs []cmdButton, id string) (cmdButton, bool) {
	for _, c := range cs {
		if c.ID == id {
			return c, true
		}
	}
	return cmdButton{}, false
}

func thingIDs(cs []cmdButton) []string {
	var out []string
	for _, c := range cs {
		out = append(out, c.ID)
	}
	return out
}

// thingPick picks option o: its command is enqueued, else it runs.
func thingPick(a *App, o cmdOption) {
	switch {
	case o.Cmd != nil:
		a.renderer.enqueue(*o.Cmd)
	case o.Run != nil:
		o.Run(a)
	}
}

// thingPress presses c like the grid: a disabled command only shows its
// reason; else Run, else its Cmds, else (only Options) the first enabled
// option, else the legacy click id.
func thingPress(a *App, c cmdButton) {
	switch {
	case c.Disabled:
		a.hint = c.Reason
	case c.Run != nil:
		c.Run(a)
	case len(c.Cmds) > 0:
		for _, cmd := range c.Cmds {
			a.renderer.enqueue(cmd)
		}
	case c.Options != nil:
		for _, o := range c.Options(a) {
			if !o.Disabled {
				thingPick(a, o)
				return
			}
		}
	default:
		a.applyClick(c.ID)
	}
}

// thingTraceDraw runs fn and returns every draw the canvas made (physical
// pixels). The trace is global: callers must not be parallel.
func thingTraceDraw(t *testing.T, fn func()) []traceEvent {
	t.Helper()
	var ev []traceEvent
	gfx.SetDrawTrace(func(e traceEvent) { ev = append(ev, e) })
	t.Cleanup(func() { gfx.SetDrawTrace(nil) })
	fn()
	gfx.SetDrawTrace(nil)
	return ev
}

// thingLog is n log lines, newest first.
func thingLog(n int) []game.LogLineView {
	var out []game.LogLineView
	for i := 0; i < n; i++ {
		out = append(out, game.LogLineView{Day: 3, Hour: 20 - i%20, Text: fmt.Sprintf("something happened to it, number %d", i)})
	}
	return out
}

func thingLines(sp paneSpec) string { return strings.Join(sp.Lines, "|") }

func thingBar(sp paneSpec, label string) (paneBar, bool) {
	for _, b := range sp.Bars {
		if b.Label == label {
			return b, true
		}
	}
	return paneBar{}, false
}

func thingTab(sp paneSpec, id string) (paneTab, bool) {
	for _, tb := range sp.Tabs {
		if tb.ID == id {
			return tb, true
		}
	}
	return paneTab{}, false
}

// thingDeerSim is a stub colony with a wild doe at (4, 4) and two colonists.
func thingDeerSim() *stubSim {
	deer := game.AnimalView{ID: 9, Species: "deer", Label: "deer", X: 4, Y: 4, Mode: "wandering", State: "wandering",
		Health: 90, Food: 70, Rest: 60, Diet: "plants", Female: true, Adult: true, Stage: "adult", AgeDays: 34}
	return &stubSim{
		pawns: []game.PawnView{{ID: 1, Name: "Ada", X: 2, Y: 2, Status: "alive"}, {ID: 2, Name: "Bo", X: 3, Y: 2, Status: "alive"}},
		wild:  []game.WildlifeRow{{Species: "deer", Label: "deer", TameChance: 25}},
		world: game.WorldView{Animals: []game.AnimalView{deer}},
	}
}

// ---- animal pane ----

func TestAnimalPaneWild(t *testing.T) {
	t.Parallel()
	s := thingDeerSim()
	a, _ := colonyApp(t, s)
	a.selectCand(selCand{selAnimal, 9})
	sp := a.animalPane(9)
	if sp.Title != "Deer" || sp.Close != "animal.close" {
		t.Fatalf("title %q close %q", sp.Title, sp.Close)
	}
	for _, want := range []string{"Female", "adult", "wild"} {
		if !strings.Contains(sp.Subtitle, want) {
			t.Errorf("subtitle %q lacks %q", sp.Subtitle, want)
		}
	}
	for label, pct := range map[string]int{"Health": 90, "Food": 70, "Rest": 60} {
		if b, ok := thingBar(sp, label); !ok || b.Pct != pct || b.Color.A == 0 {
			t.Errorf("bar %s = %+v (%v), want %d%%", label, b, ok, pct)
		}
	}
	lines := thingLines(sp)
	for _, want := range []string{"Wild | Wandering", "Diet: plants", "Tame chance 25%", "34 days old"} {
		if !strings.Contains(lines, want) {
			t.Errorf("lines %q lack %q", lines, want)
		}
	}
	if strings.Contains(lines, "Master") || len(sp.Chips) != 0 {
		t.Errorf("a wild animal has no master row or area chip: %q %+v", lines, sp.Chips)
	}
	// the hunt and tame marks are lines of their own
	s.world.Animals[0].Designated, s.world.Animals[0].TameMark = true, true
	if l := thingLines(a.animalPane(9)); !strings.Contains(l, "Marked for hunting") || !strings.Contains(l, "Marked for taming") {
		t.Errorf("marks missing from %q", l)
	}
	// a vanished animal has no pane and clears the selection
	if sp := a.animalPane(404); sp.Title != "" {
		t.Fatalf("pane for a missing animal = %+v", sp)
	}
	a.selAnimal, a.hasAnimal = 404, true
	a.animalPane(404)
	if a.hasAnimal {
		t.Fatal("a vanished animal is deselected")
	}
}

func TestAnimalPaneManhunterAndHostileStates(t *testing.T) {
	t.Parallel()
	s := thingDeerSim()
	a, _ := colonyApp(t, s)
	a.selectCand(selCand{selAnimal, 9})
	s.world.Animals[0].Manhunter = true
	if l := thingLines(a.animalPane(9)); !strings.Contains(l, "MANHUNTER: hunting colonists!") {
		t.Errorf("manhunter lines %q", l)
	}
	s.world.Animals[0].Manhunter, s.world.Animals[0].Hostile = false, true
	if l := thingLines(a.animalPane(9)); !strings.Contains(l, "attacking a colonist!") {
		t.Errorf("hostile lines %q", l)
	}
	// no needs in the view (older game): only the health bar
	s.world.Animals[0].State, s.world.Animals[0].Hostile = "", false
	sp := a.animalPane(9)
	if _, ok := thingBar(sp, "Food"); ok {
		t.Errorf("an animal without needs shows a Food bar: %+v", sp.Bars)
	}
	if _, ok := thingBar(sp, "Health"); !ok {
		t.Error("health bar missing")
	}
}

func TestAnimalPaneTameShowsMasterBondAndAreaChip(t *testing.T) {
	t.Parallel()
	s := bondSim()
	s.world.Animals[0].Product, s.world.Animals[0].ProductPct = "milk", 40
	a, _ := colonyApp(t, s)
	a.selectCand(selCand{selAnimal, 50})
	sp := a.animalPane(50)
	if sp.Title != "Rook" {
		t.Fatalf("a named tame animal is titled by its name: %q", sp.Title)
	}
	for _, want := range []string{"Labrador", "tame"} {
		if !strings.Contains(sp.Subtitle, want) {
			t.Errorf("subtitle %q lacks %q", sp.Subtitle, want)
		}
	}
	lines := thingLines(sp)
	for _, want := range []string{"Master: Ada", "Bond: Bo", "Mother: Clover", "Young: Ash, Moss", "Milk: 40%"} {
		if !strings.Contains(lines, want) {
			t.Errorf("lines %q lack %q", lines, want)
		}
	}
	if len(sp.Chips) != 1 || sp.Chips[0].ID != "animal.area" || sp.Chips[0].Label != "Area: Unrestricted" || sp.Chips[0].Tip == "" {
		t.Fatalf("a roaming tame animal has the allowed-area chip: %+v", sp.Chips)
	}
	if strings.Contains(thingLines(sp), "Area:") {
		t.Fatalf("the area is on the chip, not repeated as a line: %q", sp.Lines)
	}
	// a penned animal shows its pen as a line and has no area chip
	s.world.Animals[0].Penned, s.world.Animals[0].Pen = true, "Pen 1"
	sp = a.animalPane(50)
	if !strings.Contains(thingLines(sp), "Pen: Pen 1") || len(sp.Chips) != 0 {
		t.Fatalf("penned: lines %q chips %+v", thingLines(sp), sp.Chips)
	}
	// the area chip opens the picker and assigns the pick
	s.world.Animals[0].Penned = false
	a.applyClick("animal.area")
	if a.uiModalTop() != areaPickID {
		t.Fatal("the area chip opens the allowed-area picker")
	}
}

func TestAnimalPaneTabs(t *testing.T) {
	t.Parallel()
	s := bondSim()
	a, _ := colonyApp(t, s)
	a.selectCand(selCand{selAnimal, 50})
	sp := a.animalPane(50)
	logTab, ok := thingTab(sp, "inspector.log")
	if !ok || logTab.Label != "Log" || logTab.Draw == nil || logTab.WantW < expandMinW || logTab.WantH <= 0 {
		t.Fatalf("log tab = %+v (%v)", logTab, ok)
	}
	train, ok := thingTab(sp, "inspector.training")
	if !ok || train.Label != "Training" || train.Draw == nil || train.WantH <= 0 {
		t.Fatalf("a tame trainable animal has the Training tab: %+v (%v)", train, ok)
	}
	// both bodies draw (no art, no panic) inside their panel
	img := ebiten.NewImage(1280, 720)
	r := rect{X: 0, Y: 300, W: logTab.WantW, H: logTab.WantH}
	logTab.Draw(img, r)
	train.Draw(img, rect{X: 0, Y: 300, W: train.WantW, H: train.WantH})

	// a wild animal has the Log but no Training tab
	w := thingDeerSim()
	b, _ := colonyApp(t, w)
	b.selectCand(selCand{selAnimal, 9})
	wp := b.animalPane(9)
	if _, ok := thingTab(wp, "inspector.log"); !ok {
		t.Error("a wild animal still has a Log")
	}
	if _, ok := thingTab(wp, "inspector.training"); ok {
		t.Error("a wild animal cannot be trained")
	}
	// a tame animal that can learn nothing has none either
	s.world.Animals[0].Training = nil
	if _, ok := thingTab(a.animalPane(50), "inspector.training"); ok {
		t.Error("nothing to train: no Training tab")
	}
}

// ---- animal commands ----

func TestAnimalCommandsWild(t *testing.T) {
	t.Parallel()
	s := thingDeerSim()
	a, _ := colonyApp(t, s)
	a.selectCand(selCand{selAnimal, 9})
	cs := a.animalCommands(9)
	if got := strings.Join(thingIDs(cs), ","); got != "animal.hunt,animal.tame" {
		t.Fatalf("wild animal commands = %s", got)
	}
	hunt := thingCmd(t, cs, "animal.hunt")
	if hunt.Label != "Hunt" || hunt.Icon != "order_hunt" || !hunt.Toggle || hunt.On || hunt.Disabled || hunt.Desc == "" {
		t.Fatalf("hunt = %+v", hunt)
	}
	tame := thingCmd(t, cs, "animal.tame")
	if tame.Label != "Tame" || tame.Icon != "order_tame" || !tame.Toggle || tame.On || tame.Disabled {
		t.Fatalf("tame = %+v", tame)
	}
	if hunt.Run != nil || hunt.Cmds != nil || tame.Run != nil || tame.Cmds != nil {
		t.Fatal("legacy ids keep their app.go handlers: no Run or Cmds")
	}

	// marked: the label flips and the toggle is on
	s.world.Animals[0].Designated = true
	hunt = thingCmd(t, a.animalCommands(9), "animal.hunt")
	if hunt.Label != "Cancel hunt" || !hunt.On || hunt.Disabled {
		t.Fatalf("marked hunt = %+v", hunt)
	}
	// while it is marked for hunting, taming it is blocked, and the reverse
	tame = thingCmd(t, a.animalCommands(9), "animal.tame")
	if !tame.Disabled || tame.Reason != "marked for hunting" {
		t.Fatalf("tame while hunted = %+v", tame)
	}
	s.world.Animals[0].Designated, s.world.Animals[0].TameMark = false, true
	cs = a.animalCommands(9)
	if tame = thingCmd(t, cs, "animal.tame"); tame.Label != "Cancel tame" || !tame.On || tame.Disabled {
		t.Fatalf("marked tame = %+v", tame)
	}
	if hunt = thingCmd(t, cs, "animal.hunt"); !hunt.Disabled || hunt.Reason != "marked for taming" {
		t.Fatalf("hunt while marked for taming = %+v", hunt)
	}
}

func TestAnimalTameDisabledWhenUntameable(t *testing.T) {
	t.Parallel()
	s := thingDeerSim()
	s.wild[0].TameChance = 0 // hive insects are never tamed
	a, _ := colonyApp(t, s)
	a.selectCand(selCand{selAnimal, 9})
	tame := thingCmd(t, a.animalCommands(9), "animal.tame")
	if !tame.Disabled || tame.Reason != "cannot be tamed" {
		t.Fatalf("tame = %+v", tame)
	}
	// the click shows the reason and sends nothing
	thingPress(a, tame)
	if a.hint != "cannot be tamed" || len(s.cmds) != 0 {
		t.Fatalf("hint %q cmds %+v", a.hint, s.cmds)
	}
}

func TestManhunterCannotBeTamed(t *testing.T) {
	t.Parallel()
	s := thingDeerSim()
	s.world.Animals[0].Manhunter = true
	a, _ := colonyApp(t, s)
	a.selectCand(selCand{selAnimal, 9})
	cs := a.animalCommands(9)
	if _, ok := thingFind(cs, "animal.tame"); ok {
		t.Fatalf("an enraged animal has no Tame command: %v", thingIDs(cs))
	}
	if _, ok := thingFind(cs, "animal.hunt"); !ok {
		t.Fatalf("it can still be hunted: %v", thingIDs(cs))
	}
}

func TestAnimalCommandsEmitHuntAndTameOrders(t *testing.T) {
	t.Parallel()
	s := thingDeerSim()
	a, _ := colonyApp(t, s)
	a.selectCand(selCand{selAnimal, 9})
	thingPress(a, thingCmd(t, a.animalCommands(9), "animal.hunt"))
	if c := lastCmd(t, s); c.Kind != command.DesignateHunt || c.X != 4 || c.Y != 4 || c.X2 != 4 || c.Y2 != 4 {
		t.Fatalf("hunt = %+v", c)
	}
	s.world.Animals[0].Designated = true
	thingPress(a, thingCmd(t, a.animalCommands(9), "animal.hunt"))
	if c := lastCmd(t, s); c.Kind != command.Undesignate || c.Order != command.OrderHunt || c.X != 4 {
		t.Fatalf("cancel hunt clears only the mark: %+v", c)
	}
	s.world.Animals[0].Designated = false
	thingPress(a, thingCmd(t, a.animalCommands(9), "animal.tame"))
	if c := lastCmd(t, s); c.Kind != command.DesignateTame || c.X != 4 || c.Y != 4 {
		t.Fatalf("tame = %+v", c)
	}
	s.world.Animals[0].TameMark = true
	thingPress(a, thingCmd(t, a.animalCommands(9), "animal.tame"))
	if c := lastCmd(t, s); c.Kind != command.Undesignate || c.Order != command.OrderTame {
		t.Fatalf("cancel tame = %+v", c)
	}
}

func TestAnimalCommandsTame(t *testing.T) {
	t.Parallel()
	s := bondSim()
	a, _ := colonyApp(t, s)
	a.selectCand(selCand{selAnimal, 50})
	cs := a.animalCommands(50)
	if got := strings.Join(thingIDs(cs), ","); got != "animal.hunt,animal.release,cmd.animal.master,cmd.animal.care,animal.train.0,animal.train.1" {
		t.Fatalf("tame commands = %s", got)
	}
	slaughter := thingCmd(t, cs, "animal.hunt")
	if slaughter.Label != "Slaughter" || slaughter.Icon != "slaughter" || slaughter.Desc == "" {
		t.Fatalf("slaughter = %+v", slaughter)
	}
	s.world.Animals[0].Designated = true
	if c := thingCmd(t, a.animalCommands(50), "animal.hunt"); c.Label != "Cancel hunt" || !c.On {
		t.Fatalf("marked slaughter = %+v", c)
	}
	s.world.Animals[0].Designated = false
	release := thingCmd(t, cs, "animal.release")
	if release.Label != "Release" || release.Icon != "release" || release.Disabled {
		t.Fatalf("release = %+v", release)
	}
	thingPress(a, release)
	if c := lastCmd(t, s); c.Kind != command.ReleaseAnimal || c.Pawn != 50 {
		t.Fatalf("release = %+v", c)
	}
	if _, ok := thingFind(cs, "animal.tame"); ok {
		t.Fatal("a tame animal has no Tame command")
	}
}

func TestAnimalTrainingToggles(t *testing.T) {
	t.Parallel()
	s := bondSim()
	a, _ := colonyApp(t, s)
	a.selectCand(selCand{selAnimal, 50})
	cs := a.animalCommands(50)
	if _, ok := thingFind(cs, "animal.train.2"); ok {
		t.Fatal("a skill the species can't learn has no command")
	}
	obey, guard := thingCmd(t, cs, "animal.train.0"), thingCmd(t, cs, "animal.train.1")
	if !obey.On || !obey.Toggle || guard.On || obey.Label != "Obedience 2/3" {
		t.Fatalf("obedience %+v guard %+v", obey, guard)
	}
	for _, c := range []cmdButton{obey, guard} {
		if c.Desc == "" || c.Run != nil || c.Cmds != nil {
			t.Errorf("%s: desc %q (legacy id: no Run or Cmds)", c.ID, c.Desc)
		}
		for _, w := range strings.Fields(c.Label) {
			if tw := textW(w, tsSmall); tw > cmdCell-8 {
				t.Errorf("%s: %q (%.0f px) does not fit a %d px cell", c.ID, w, tw, cmdCell)
			}
		}
	}
	thingPress(a, guard)
	thingPress(a, obey)
	if len(s.cmds) != 2 {
		t.Fatalf("cmds %+v", s.cmds)
	}
	if c := s.cmds[0]; c.Kind != command.SetTraining || c.Target != 50 || c.Index != 1 || !c.Enabled {
		t.Fatalf("guard on: %+v", c)
	}
	if c := s.cmds[1]; c.Kind != command.SetTraining || c.Index != 0 || c.Enabled {
		t.Fatalf("obedience off: %+v", c)
	}
}

func TestAnimalMasterOptions(t *testing.T) {
	t.Parallel()
	s := bondSim()
	a, _ := colonyApp(t, s)
	a.selectCand(selCand{selAnimal, 50})
	master := thingCmd(t, a.animalCommands(50), "cmd.animal.master")
	if master.Label != "Master" || master.Options == nil || master.Disabled || master.Desc == "" {
		t.Fatalf("master = %+v", master)
	}
	opts := master.Options(a)
	if len(opts) != 3 || opts[0].Label != "Ada (master)" || opts[1].Label != "Bo" || opts[2].Label != "No master" {
		t.Fatalf("options = %+v", opts)
	}
	if opts[2].Cmd == nil || opts[2].Cmd.Pawn != 0 || opts[2].Cmd.Target != 50 || opts[2].Cmd.Kind != command.SetAnimalMaster {
		t.Fatalf("no master option = %+v", opts[2])
	}
	thingPick(a, opts[1])
	if c := lastCmd(t, s); c.Kind != command.SetAnimalMaster || c.Target != 50 || c.Pawn != 2 {
		t.Fatalf("pick Bo: %+v", c)
	}
	// a plain click opens the same list instead of picking the first one
	thingPress(a, master)
	if !a.float.open || len(a.float.opts) != 3 || a.float.opts[0].Label != "Ada (master)" {
		t.Fatalf("a click lists the colonists: %+v", a.float)
	}
	if len(s.cmds) != 1 {
		t.Fatalf("opening the list sent %+v", s.cmds)
	}
}

func TestWildAnimalHasNoMasterOrTrainingCommands(t *testing.T) {
	t.Parallel()
	a, _ := colonyApp(t, thingDeerSim())
	a.selectCand(selCand{selAnimal, 9})
	for _, c := range a.animalCommands(9) {
		if strings.Contains(c.ID, "master") || strings.Contains(c.ID, "train") || c.ID == "animal.release" {
			t.Errorf("wild animal has %s", c.ID)
		}
	}
	if cs := a.animalCommands(404); cs != nil {
		t.Fatalf("commands for a missing animal: %v", thingIDs(cs))
	}
}

// ---- item pane and commands ----

func thingItemSim(items ...game.ItemView) *stubSim {
	s := &stubSim{pawns: []game.PawnView{
		{ID: 1, Name: "Ada", X: 5, Y: 5, Status: "alive"}, {ID: 2, Name: "Bo", X: 12, Y: 5, Status: "alive"}}}
	s.world = game.WorldView{Items: items}
	return s
}

func TestItemPaneContents(t *testing.T) {
	t.Parallel()
	s := thingItemSim(
		game.ItemView{ID: 21, Kind: "wood", Qty: 25, X: 2, Y: 2, FreshDays: -1, Unclaimed: true},
		game.ItemView{ID: 22, Kind: "meal", Qty: 2, X: 3, Y: 2, FreshDays: 4, RotRate: 500, Forbidden: true},
		game.ItemView{ID: 23, Kind: "cloth carpet", Qty: 1, X: 4, Y: 2, FreshDays: -1, Catch: 40},
		game.ItemView{ID: 24, Kind: "meat", Qty: 3, X: 5, Y: 2, Dried: true, FreshDays: 9})
	a, _ := colonyApp(t, s)

	a.selectCand(selCand{selItem, 21})
	sp := a.itemPane(21)
	if sp.Title != "Wood ×25" || sp.Subtitle != "material" || sp.Close != "item.close" || len(sp.Tabs) != 0 {
		t.Fatalf("pane = %+v", sp)
	}
	if want := "Found here. Colonists won't haul it until you mark it."; !contains(sp.Lines, want) {
		t.Fatalf("lines %q lack the found line", sp.Lines)
	}
	if last := sp.Lines[len(sp.Lines)-1]; last != "Not stored" {
		t.Fatalf("storage stays last: %q", sp.Lines)
	}

	a.selectCand(selCand{selItem, 22})
	l := thingLines(a.itemPane(22))
	for _, want := range []string{"Fresh for 4 days", "Rots at 50% of the normal rate", "Forbidden: colonists leave it alone"} {
		if !strings.Contains(l, want) {
			t.Errorf("meal lines %q lack %q", l, want)
		}
	}
	s.world.Items[1].RotRate = 0
	if l := thingLines(a.itemPane(22)); !strings.Contains(l, "Frozen: it does not rot") {
		t.Errorf("frozen meal lines %q", l)
	}
	s.world.Items[1].RotRate = 1000
	if l := thingLines(a.itemPane(22)); strings.Contains(l, "Rots") || strings.Contains(l, "Frozen") {
		t.Errorf("a normal rot rate adds no line: %q", l)
	}

	a.selectCand(selCand{selItem, 23})
	if l := thingLines(a.itemPane(23)); !strings.Contains(l, "Catches 40% of tracked dirt") {
		t.Errorf("carpet lines %q", l)
	}
	a.selectCand(selCand{selItem, 24})
	l = thingLines(a.itemPane(24))
	if !strings.Contains(l, "Dried") || strings.Contains(l, "Rots") {
		t.Errorf("dried lines %q", l)
	}
}

func TestItemPaneGoneItem(t *testing.T) {
	t.Parallel()
	a, _ := colonyApp(t, thingItemSim())
	a.selItem, a.hasItem = 9999, true
	if sp := a.itemPane(9999); sp.Title != "" {
		t.Fatalf("pane = %+v", sp)
	}
	if cs := a.itemCommands(9999); len(cs) != 0 || a.hasItem {
		t.Fatalf("a vanished stack has no commands and closes the selection (%v, hasItem %v)", thingIDs(cs), a.hasItem)
	}
	a.applyItemClick("item.close") // nothing selected: no-op
}

func TestItemForbidCommand(t *testing.T) {
	t.Parallel()
	s := thingItemSim(game.ItemView{ID: 11, Kind: "wood", Qty: 5, X: 2, Y: 2, FreshDays: -1})
	a, _ := colonyApp(t, s)
	a.selectCand(selCand{selItem, 11})
	cs := a.itemCommands(11)
	forbid := thingCmd(t, cs, "item.forbid")
	if forbid.Label != "Forbid" || forbid.Icon != "order_forbid" || !forbid.Toggle || forbid.On || forbid.Hotkey != input.Forbid || forbid.Desc == "" {
		t.Fatalf("forbid = %+v", forbid)
	}
	if forbid.Run != nil || forbid.Cmds != nil {
		t.Fatal("legacy id: no Run or Cmds")
	}
	if cs[0].ID != "item.forbid" {
		t.Fatalf("Forbid is declared first: %v", thingIDs(cs))
	}
	thingPress(a, forbid)
	if c := lastCmd(t, s); c.Kind != command.SetForbidden || c.Target != 11 || !c.Enabled {
		t.Fatalf("forbid cmd = %+v", c)
	}
	s.world.Items[0].Forbidden = true
	allow := thingCmd(t, a.itemCommands(11), "item.forbid")
	if allow.Label != "Allow" || allow.Icon != "order_allow" || !allow.On {
		t.Fatalf("allow = %+v", allow)
	}
	thingPress(a, allow)
	if c := lastCmd(t, s); c.Kind != command.SetForbidden || c.Enabled {
		t.Fatalf("allow cmd = %+v", c)
	}
}

// F toggles the selected stack's forbid flag, and the Forbid command is
// the one that shows F.
func TestForbidKeyTogglesSelectedItem(t *testing.T) {
	t.Parallel()
	a, s, f := inputColony(t)
	s.world.Items = []game.ItemView{{ID: 90, Kind: "wood", Qty: 3, X: 3, Y: 3, FreshDays: -1}}
	a.selectCand(selCand{selItem, 90})
	f.Key(t, a, a.bindings().Of(input.Forbid)[0].Key, 0)
	if c := cmdsOf(s, command.SetForbidden); len(c) != 1 || c[0].Target != 90 || !c[0].Enabled {
		t.Fatalf("F forbids the selected stack: %+v", s.cmds)
	}
	s.world.Items[0].Forbidden = true
	f.Key(t, a, a.bindings().Of(input.Forbid)[0].Key, 0)
	if c := cmdsOf(s, command.SetForbidden); len(c) != 2 || c[1].Enabled {
		t.Fatalf("F again allows it: %+v", s.cmds)
	}
	if got := thingCmd(t, a.itemCommands(90), "item.forbid").Hotkey; got != input.Forbid {
		t.Fatalf("the Forbid command carries the Forbid key, got %q", got)
	}
}

func TestItemHaulMarkCommand(t *testing.T) {
	t.Parallel()
	s := thingItemSim(foundItems()...)
	a, _ := colonyApp(t, s)
	a.selectCand(selCand{selItem, 21})
	mark := thingCmd(t, a.itemCommands(21), "item.haulmark")
	if mark.Label != "Mark for hauling" || mark.Icon != "order_haul" || !mark.Toggle || mark.On || mark.Desc == "" {
		t.Fatalf("mark = %+v", mark)
	}
	// a marked stack offers Unmark
	a.selectCand(selCand{selItem, 22})
	if c := thingCmd(t, a.itemCommands(22), "item.haulmark"); c.Label != "Unmark" || !c.On {
		t.Fatalf("marked = %+v", c)
	}
	// a forbidden stack would not be hauled: the mark says so, but Unmark stays on
	s.world.Items[0].Forbidden = true
	if c := thingCmd(t, a.itemCommands(21), "item.haulmark"); !c.Disabled || c.Reason != "forbidden: allow it first" {
		t.Fatalf("forbidden stack mark = %+v", c)
	}
	s.world.Items[1].Forbidden = true // already marked
	if c := thingCmd(t, a.itemCommands(22), "item.haulmark"); c.Disabled || c.Label != "Unmark" {
		t.Fatalf("a marked stack can always be unmarked: %+v", c)
	}
	// a colony stack needs no mark
	a.selectCand(selCand{selItem, 23})
	if _, ok := thingFind(a.itemCommands(23), "item.haulmark"); ok {
		t.Fatal("a claimed stack has no mark command")
	}
}

// thingItemVerbSim is a stub colony with a rifle (equip), a parka (wear), a
// meal (eat) and wood (haul only) and two colonists who can do any order.
func thingItemVerbSim(order command.Order) *stubSim {
	s := thingItemSim(
		game.ItemView{ID: 31, Kind: "rifle", Qty: 1, X: 6, Y: 5, FreshDays: -1},
		game.ItemView{ID: 32, Kind: "parka", Qty: 1, X: 6, Y: 6, FreshDays: -1},
		game.ItemView{ID: 33, Kind: "meal", Qty: 2, X: 7, Y: 6, FreshDays: 3},
		game.ItemView{ID: 34, Kind: "wood", Qty: 9, X: 8, Y: 6, FreshDays: -1})
	s.orders = []game.OrderOption{{Label: "x", Cmd: command.Command{Kind: command.Prioritize, Order: order}}}
	return s
}

func TestItemVerbCommands(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		id, kind, label, icon string
		order                 command.Order
		pawn                  uint64
	}{
		{"cmd.item.equip", "rifle", "Equip", "equip", command.OrderEquip, 31},
		{"cmd.item.wear", "parka", "Wear", "wear", command.OrderEquip, 32},
		{"cmd.item.eat", "meal", "Eat", "eat", command.OrderEat, 33},
		{"cmd.item.haul", "wood", "Haul", "order_haul", command.OrderHaul, 34},
	} {
		s := thingItemVerbSim(c.order)
		a, _ := colonyApp(t, s)
		a.selectCand(selCand{selItem, entity.EntityID(c.pawn)})
		cs := a.itemCommands(entity.EntityID(c.pawn))
		cmd := thingCmd(t, cs, c.id)
		if cmd.Label != c.label || cmd.Icon != c.icon || cmd.Options == nil || cmd.Disabled || cmd.Desc == "" {
			t.Errorf("%s: %+v", c.id, cmd)
			continue
		}
		if cmd.Run != nil || cmd.Cmds != nil {
			t.Errorf("%s: a new id lists options only", c.id)
		}
		opts := cmd.Options(a)
		if len(opts) != 2 || opts[0].Label != "Ada" || opts[1].Label != "Bo" {
			t.Errorf("%s: options = %+v (nearest first)", c.id, opts)
			continue
		}
		// a plain click is the first (nearest) colonist; a pick of the other
		// sends their order
		thingPress(a, cmd)
		if got := lastCmd(t, s); got.Kind != command.Prioritize || got.Order != c.order {
			t.Errorf("%s: click sent %+v", c.id, got)
		}
		thingPick(a, opts[1])
		if got := lastCmd(t, s); got.Kind != command.Prioritize || got.Order != c.order {
			t.Errorf("%s: pick sent %+v", c.id, got)
		}
		// only the verbs of its category: a rifle is not eaten, a meal not worn
		for _, other := range []string{"cmd.item.equip", "cmd.item.wear", "cmd.item.eat", "cmd.item.haul"} {
			if other == c.id || other == "cmd.item.haul" {
				continue
			}
			if _, ok := thingFind(cs, other); ok {
				t.Errorf("%s offers %s", c.kind, other)
			}
		}
	}
}

func TestItemVerbOptionsCarryDisabledReasons(t *testing.T) {
	t.Parallel()
	opts := colonistOptions([]colonistAction{
		{Pawn: 1, Name: "Ada", Cmd: command.Command{Kind: command.Prioritize, Order: command.OrderEquip, Pawn: 1}},
		{Pawn: 2, Name: "Bo", Disabled: true, Reason: "cannot reach"},
	})
	if len(opts) != 2 || opts[0].Disabled || opts[0].Cmd == nil || opts[0].Cmd.Pawn != 1 {
		t.Fatalf("options = %+v", opts)
	}
	if !opts[1].Disabled || opts[1].Reason != "cannot reach" || opts[1].Label != "Bo" {
		t.Fatalf("the unreachable colonist's option = %+v", opts[1])
	}
	if got := colonistOptions(nil); len(got) != 0 {
		t.Fatalf("no colonists: %+v", got)
	}
}

func TestItemVerbDisabledWhenNobodyCan(t *testing.T) {
	t.Parallel()
	s := thingItemVerbSim(command.OrderEquip)
	s.orders = []game.OrderOption{{Label: "Equip", Disabled: true, Reason: "cannot reach",
		Cmd: command.Command{Kind: command.Prioritize, Order: command.OrderEquip}}}
	a, _ := colonyApp(t, s)
	a.selectCand(selCand{selItem, 31})
	equip := thingCmd(t, a.itemCommands(31), "cmd.item.equip")
	if !equip.Disabled || equip.Reason != "cannot reach" {
		t.Fatalf("equip = %+v", equip)
	}
	for _, o := range equip.Options(a) {
		if !o.Disabled || o.Reason != "cannot reach" {
			t.Errorf("option %+v should be disabled with the reason", o)
		}
	}
	thingPress(a, equip)
	if len(s.cmds) != 0 || a.hint != "cannot reach" {
		t.Fatalf("a disabled verb sends nothing and shows why: %+v / %q", s.cmds, a.hint)
	}

	// no colonist at all: still listed, with a reason
	s.pawns = nil
	if equip = thingCmd(t, a.itemCommands(31), "cmd.item.equip"); !equip.Disabled || equip.Reason != "no colonist can do that" {
		t.Fatalf("no colonists: %+v", equip)
	}
}

func TestItemHaulCommandOnlyWhenUnstored(t *testing.T) {
	t.Parallel()
	s := thingItemVerbSim(command.OrderHaul)
	s.world.Zones = []game.ZoneView{rectZone(1, game.ZoneStockpile, 8, 5, 9, 7)}
	a, _ := colonyApp(t, s)
	a.selectCand(selCand{selItem, 34}) // wood at (8, 6): in the stockpile
	if _, ok := thingFind(a.itemCommands(34), "cmd.item.haul"); ok {
		t.Fatalf("stored wood has no Haul: %v", thingIDs(a.itemCommands(34)))
	}
	if l := a.itemPane(34).Lines; l[len(l)-1] != "In a stockpile" {
		t.Fatalf("lines = %q", l)
	}
	s.world.Items[3].X = 12 // moved out of the stockpile
	if _, ok := thingFind(a.itemCommands(34), "cmd.item.haul"); !ok {
		t.Fatal("loose wood can be hauled")
	}
	if l := a.itemPane(34).Lines; l[len(l)-1] != "Not stored" {
		t.Fatalf("lines = %q", l)
	}
}

// ---- plant pane and commands ----

func thingPlantSim(ps ...game.PlantView) *stubSim {
	s := &stubSim{pawns: []game.PawnView{
		{ID: 1, Name: "Ada", X: 5, Y: 5, Status: "alive"}, {ID: 2, Name: "Bo", X: 9, Y: 5, Status: "alive"}}}
	s.world = game.WorldView{Plants: ps}
	return s
}

func TestPlantPaneContents(t *testing.T) {
	t.Parallel()
	d := clock.TicksPerDay
	s := thingPlantSim(
		game.PlantView{ID: 41, X: 3, Y: 3, Species: "potato", Growth: 42, Stage: 2, Zone: "potato"},
		game.PlantView{ID: 42, X: 4, Y: 3, Species: "potato", Growth: 42, Stage: 2, Damage: 30, DamageKind: "cold"},
		game.PlantView{ID: 43, X: 5, Y: 3, Species: "tree", Growth: 90, Stage: 3},
		game.PlantView{ID: 44, X: 6, Y: 3, Species: "corn", Growth: 100, Stage: 3, Harvestable: true, Designated: true})
	s.growth = map[entity.EntityID]game.PlantGrowthView{ // PLAY-017: asked per plant (Simulation.PlantGrowth)
		41: {State: "growing", RipeIn: d * 3}, 42: {State: "dark", RipeIn: -1},
		43: {State: "growing", RipeIn: d * 12}, 44: {State: "ripe", RipeIn: -1}}
	a, _ := colonyApp(t, s)

	a.selectCand(selCand{selPlant, 41})
	sp := a.plantPane(41)
	if sp.Title != "Potato" || sp.Close != "plant.close" || len(sp.Tabs) != 0 {
		t.Fatalf("pane = %+v", sp)
	}
	for _, want := range []string{"Crop", "young"} {
		if !strings.Contains(sp.Subtitle, want) {
			t.Errorf("subtitle %q lacks %q", sp.Subtitle, want)
		}
	}
	b, ok := thingBar(sp, "Growth")
	if !ok || b.Pct != 42 || b.Color != uiGood {
		t.Fatalf("growth bar = %+v (%v)", b, ok)
	}
	// the PLAY-017 caption leads, then the old lines in their order
	want := []string{"42% · growing", "Not ready (42%)", "Ripe in about 3 days", "In a growing zone (Potato)"}
	if strings.Join(sp.Lines, "|") != strings.Join(want, "|") {
		t.Fatalf("lines = %q, want %q", sp.Lines, want)
	}

	// a state that needs attention paints the bar in the warning colour
	a.selectCand(selCand{selPlant, 42})
	sp = a.plantPane(42)
	if b, _ := thingBar(sp, "Growth"); b.Color != uiWarning {
		t.Fatalf("a dark crop's bar = %+v", b)
	}
	if sp.Lines[0] != "too dark: needs sun or a lamp nearby" {
		t.Fatalf("lines = %q", sp.Lines)
	}
	if l := thingLines(sp); !strings.Contains(l, plantDamageLine(s.world.Plants[1])) || plantDamageLine(s.world.Plants[1]) == "" {
		t.Fatalf("the damage line is kept: %q", l)
	}

	a.selectCand(selCand{selPlant, 43})
	if sp = a.plantPane(43); sp.Subtitle == "" || !strings.HasPrefix(sp.Subtitle, "Tree") {
		t.Fatalf("tree subtitle = %q", sp.Subtitle)
	}
	a.selectCand(selCand{selPlant, 44})
	sp = a.plantPane(44)
	if !strings.Contains(sp.Subtitle, "ready to harvest") || !contains(sp.Lines, "Marked to harvest") || !contains(sp.Lines, "Ready to harvest") {
		t.Fatalf("ripe pane = %+v", sp)
	}
}

func TestPlantPaneGoneAndMissing(t *testing.T) {
	t.Parallel()
	a, _ := colonyApp(t, thingPlantSim())
	a.selPlant, a.hasPlant = 9999, true
	if sp := a.plantPane(9999); sp.Title != "" {
		t.Fatalf("pane = %+v", sp)
	}
	if cs := a.plantCommands(9999); len(cs) != 0 || a.hasPlant {
		t.Fatalf("a vanished plant has no commands and closes (%v, hasPlant %v)", thingIDs(cs), a.hasPlant)
	}
	a.applyPlantClick("plant.harvest") // nothing selected: no-op
	a.applyPlantClick("plant.close")
}

func TestPlantHarvestAndCutCommands(t *testing.T) {
	t.Parallel()
	s := thingPlantSim(
		game.PlantView{ID: 41, X: 3, Y: 3, Species: "potato", Growth: 100, Stage: 3, Harvestable: true},
		game.PlantView{ID: 42, X: 4, Y: 3, Species: "corn", Growth: 30, Stage: 1},
		game.PlantView{ID: 43, X: 5, Y: 3, Species: "tree", Growth: 100, Stage: 3})
	a, _ := colonyApp(t, s)

	a.selectCand(selCand{selPlant, 41})
	cs := a.plantCommands(41)
	harvest := thingCmd(t, cs, "plant.harvest")
	if harvest.Label != "Harvest" || harvest.Icon != "order_harvest" || harvest.Disabled || !harvest.Toggle || harvest.On || harvest.Desc == "" {
		t.Fatalf("harvest = %+v", harvest)
	}
	cut := thingCmd(t, cs, "plant.cut")
	if cut.Label != "Cut" || cut.Icon != "order_cut" || !cut.Toggle || cut.On || cut.Desc == "" {
		t.Fatalf("cut = %+v", cut)
	}
	thingPress(a, harvest)
	if c := lastCmd(t, s); c.Kind != command.DesignateHarvest || c.X != 3 || c.Y != 3 || c.X2 != 3 {
		t.Fatalf("harvest cmd = %+v", c)
	}
	s.world.Plants[0].Designated = true
	harvest = thingCmd(t, a.plantCommands(41), "plant.harvest")
	if harvest.Label != "Cancel harvest" || !harvest.On {
		t.Fatalf("marked harvest = %+v", harvest)
	}
	thingPress(a, harvest)
	if c := lastCmd(t, s); c.Kind != command.Undesignate || c.Order != command.OrderHarvest {
		t.Fatalf("cancel harvest = %+v", c)
	}
	thingPress(a, cut)
	if c := lastCmd(t, s); c.Kind != command.DesignateCut {
		t.Fatalf("cut cmd = %+v", c)
	}
	s.world.Plants[0].Cut = true
	cut = thingCmd(t, a.plantCommands(41), "plant.cut")
	if cut.Label != "Cancel cut" || !cut.On {
		t.Fatalf("marked cut = %+v", cut)
	}

	// an unripe crop cannot be harvested yet: disabled with the reason
	a.selectCand(selCand{selPlant, 42})
	harvest = thingCmd(t, a.plantCommands(42), "plant.harvest")
	if !harvest.Disabled || harvest.Reason != "not ready" {
		t.Fatalf("unripe harvest = %+v", harvest)
	}
	thingPress(a, harvest)
	if a.hint != "not ready" {
		t.Fatalf("hint = %q", a.hint)
	}

	// trees are chopped, never harvested
	a.selectCand(selCand{selPlant, 43})
	cs = a.plantCommands(43)
	if c := thingCmd(t, cs, "plant.cut"); c.Label != "Chop" {
		t.Fatalf("tree cut = %+v", c)
	}
	if _, ok := thingFind(cs, "plant.harvest"); ok {
		t.Fatal("trees have no Harvest")
	}
}

func TestPlantNowByOptions(t *testing.T) {
	t.Parallel()
	s := thingPlantSim(
		game.PlantView{ID: 41, X: 3, Y: 3, Species: "potato", Growth: 100, Stage: 3, Harvestable: true},
		game.PlantView{ID: 42, X: 4, Y: 3, Species: "corn", Growth: 30, Stage: 1})
	s.orders = []game.OrderOption{{Label: "Harvest", Cmd: command.Command{Kind: command.Prioritize, Order: command.OrderHarvest}}}
	a, _ := colonyApp(t, s)
	a.selectCand(selCand{selPlant, 41})
	now := thingCmd(t, a.plantCommands(41), "cmd.plant.now")
	if now.Label != "Now by..." || now.Options == nil || now.Disabled || now.Run != nil || now.Cmds != nil || now.Desc == "" {
		t.Fatalf("now = %+v", now)
	}
	opts := now.Options(a)
	if len(opts) != 2 || opts[0].Label != "Ada" || opts[1].Label != "Bo" || opts[0].Cmd == nil {
		t.Fatalf("options = %+v", opts)
	}
	thingPick(a, opts[1])
	if c := lastCmd(t, s); c.Kind != command.Prioritize || c.Order != command.OrderHarvest {
		t.Fatalf("pick sends the existing order: %+v", c)
	}

	// an unripe crop's "now" is a cut order
	a.selectCand(selCand{selPlant, 42})
	s.orders = []game.OrderOption{{Label: "Cut", Cmd: command.Command{Kind: command.Prioritize, Order: command.OrderCut}}}
	now = thingCmd(t, a.plantCommands(42), "cmd.plant.now")
	thingPress(a, now)
	if c := lastCmd(t, s); c.Order != command.OrderCut {
		t.Fatalf("unripe now = %+v", c)
	}

	// every colonist blocked: the command says why; none at all too
	s.orders = []game.OrderOption{{Label: "Cut", Disabled: true, Reason: "cannot reach",
		Cmd: command.Command{Kind: command.Prioritize, Order: command.OrderCut}}}
	if now = thingCmd(t, a.plantCommands(42), "cmd.plant.now"); !now.Disabled || now.Reason != "cannot reach" {
		t.Fatalf("blocked now = %+v", now)
	}
	s.pawns = nil
	if now = thingCmd(t, a.plantCommands(42), "cmd.plant.now"); !now.Disabled || now.Reason != "no colonist can do that" {
		t.Fatalf("nobody now = %+v", now)
	}
}

// ---- legacy panels ----

// The old top-left panels draw nothing but the selection mark on the map,
// and have no buttons: the pane and the grid replaced them.
func TestLegacyThingPanelsAreGone(t *testing.T) { // not parallel: the canvas trace is global
	s := bondSim()
	s.world.Items = []game.ItemView{{ID: 90, Kind: "wood", Qty: 3, X: 3, Y: 3, FreshDays: -1}}
	s.world.Plants = []game.PlantView{{ID: 41, X: 3, Y: 4, Species: "potato", Growth: 100, Stage: 3, Harvestable: true}}
	a, _ := colonyApp(t, s)
	img := ebiten.NewImage(1280, 720)
	frames := traceCanvas(t)
	for _, c := range []selCand{{selAnimal, 50}, {selItem, 90}, {selPlant, 41}} {
		a.selectCand(c)
		assertButtonsInRegions(t, a) // legacy buttons are gone: every hit target is in a layout region
		*frames = nil
		a.drawAnimalMark(img)
		a.drawItemMark(img)
		a.drawPlantMark(img)
		for _, f := range *frames {
			if strings.Contains(f, "340x") || strings.Contains(f, "300x") {
				t.Errorf("%v: a legacy panel frame was drawn: %q", c, f)
			}
		}
	}
}

// Everything a kind offers fits the command grid next to its pane.
func TestThingCommandsFitTheGrid(t *testing.T) {
	t.Parallel()
	s := bondSim()
	s.world.Items = []game.ItemView{{ID: 90, Kind: "rifle", Qty: 1, X: 3, Y: 3, FreshDays: -1, Unclaimed: true}}
	s.world.Plants = []game.PlantView{{ID: 41, X: 3, Y: 4, Species: "potato", Growth: 100, Stage: 3, Harvestable: true}}
	s.orders = []game.OrderOption{{Label: "x", Cmd: command.Command{Kind: command.Prioritize, Order: command.OrderEquip}}}
	s.pawns[0].Status, s.pawns[1].Status = "alive", "alive"
	a, _ := colonyApp(t, s)
	for _, c := range []struct {
		sel selCand
		cmd func() []cmdButton
	}{
		{selCand{selAnimal, 50}, func() []cmdButton { return a.animalCommands(50) }},
		{selCand{selItem, 90}, func() []cmdButton { return a.itemCommands(90) }},
		{selCand{selPlant, 41}, func() []cmdButton { return a.plantCommands(41) }},
	} {
		a.selectCand(c.sel)
		cs := c.cmd()
		if len(cs) == 0 {
			t.Fatalf("%v: no commands", c.sel)
		}
		for _, sw := range []struct{ w, h int }{{1280, 720}, {1113, 626}, {1920, 1080}} {
			l := computeHUDLayout(sw.w, sw.h, hudState{PaneShown: true, Commands: len(cs), GlobalH: 200})
			if len(l.GridCells) != len(cs) {
				t.Errorf("%v at %dx%d: %d commands got %d cells", c.sel, sw.w, sw.h, len(cs), len(l.GridCells))
			}
			for _, cell := range l.GridCells {
				if cell.X < l.Pane.Right() || cell.Right() > l.Global.X {
					t.Errorf("%v at %dx%d: cell %+v outside the grid band", c.sel, sw.w, sw.h, cell)
				}
			}
		}
		for _, cm := range cs {
			if cm.Label == "" || cm.ID == "" {
				t.Errorf("%v: command without label or id: %+v", c.sel, cm)
			}
			if strings.HasPrefix(cm.ID, "cmd.") && cm.Run == nil && cm.Cmds == nil && cm.Options == nil {
				t.Errorf("%s: a new command id needs Run, Cmds or Options", cm.ID)
			}
			if !strings.HasPrefix(cm.ID, "cmd.") && (cm.Run != nil || cm.Cmds != nil) {
				t.Errorf("%s: a legacy id must not carry Run or Cmds", cm.ID)
			}
		}
	}
}

// Through the real layout: each kind's pane is up (the old top-left panel
// is not reserved), every command sits in its own grid cell and answers a
// click by its id, the pane's close button closes, and a click on the pane
// never reaches the map.
func TestThingPanesAndGridThroughTheLayout(t *testing.T) {
	t.Parallel()
	s := bondSim()
	s.world.Items = []game.ItemView{{ID: 90, Kind: "rifle", Qty: 1, X: 3, Y: 3, FreshDays: -1, Unclaimed: true}}
	s.world.Plants = []game.PlantView{{ID: 41, X: 3, Y: 4, Species: "potato", Growth: 100, Stage: 3, Harvestable: true}}
	s.pawns[0].Status, s.pawns[1].Status = "alive", "alive"
	s.orders = []game.OrderOption{{Label: "x", Cmd: command.Command{Kind: command.Prioritize, Order: command.OrderEquip}}}
	a, f := colonyApp(t, s)
	for _, c := range []struct {
		sel   selCand
		close string
	}{{selCand{selAnimal, 50}, "animal.close"}, {selCand{selItem, 90}, "item.close"}, {selCand{selPlant, 41}, "plant.close"}} {
		a.selectCand(c.sel)
		f.Frame(t, a)
		l := a.hud()
		if l.Pane.Empty() || !a.paneShown() {
			t.Fatalf("%v: no pane", c.sel)
		}
		assertChromeIsTheLayout(t, a) // the legacy panel area is not reserved
		cs := a.selectionCommands()
		if len(l.GridCells) != len(cs) {
			t.Fatalf("%v: %d commands, %d cells", c.sel, len(cs), len(l.GridCells))
		}
		hits := a.layoutHits(a.sw, a.sh)
		hits = append(hits, a.colonyButtons(a.sw, a.sh)...)
		for i, cm := range cs {
			b, ok := hasButton(hits, cm.ID)
			if !ok {
				t.Errorf("%v: command %s has no hit target", c.sel, cm.ID)
				continue
			}
			if got := (rect{X: b.x, Y: b.y, W: b.w, H: b.h}); got != l.GridCells[i] {
				t.Errorf("%v: %s is at %+v, its cell is %+v", c.sel, cm.ID, got, l.GridCells[i])
			}
		}
		px, py := l.Pane.Center()
		if !a.colonyChromeHit(int(px), int(py), a.sw, a.sh) {
			t.Errorf("%v: a click on the pane would reach the map", c.sel)
		}
		click(t, a, c.close)
		if a.hasAnimal || a.hasItem || a.hasPlant {
			t.Errorf("%v: the pane's close button leaves the selection", c.sel)
		}
	}
}

// The tame animal's area chip is a real button on the pane and opens the
// picker; a pick assigns the animal's area.
func TestAnimalAreaChipOnThePane(t *testing.T) {
	t.Parallel()
	s := bondSim()
	s.areas = []game.AreaView{{Index: 0, ID: 3, Name: "Yard", Cells: 20}}
	a, f := colonyApp(t, s)
	a.selectCand(selCand{selAnimal, 50})
	f.Frame(t, a)
	click(t, a, "animal.area")
	if a.uiModalTop() != areaPickID {
		t.Fatalf("the chip opens the area picker, top modal %q", a.uiModalTop())
	}
	f.Frame(t, a)
	if sp := a.animalPane(50); sp.Chips[0].Label != "Area: Unrestricted" {
		t.Fatalf("chip %+v", sp.Chips)
	}
	press(t, a, f, "areapick.row.3")
	if c := lastCmd(t, s); c.Kind != command.AssignArea || c.Pawn != 50 || c.Index != 3 {
		t.Fatalf("pick = %+v, want AssignArea for animal 50, area 3", c)
	}
}

// The headers' icons (the pane takes them from these helpers): they draw
// with and without art and never panic on a missing sprite.
func TestThingIconsDrawWithoutPanic(t *testing.T) {
	t.Parallel()
	s := bondSim()
	s.world.Items = []game.ItemView{{ID: 90, Kind: "granite chunk", Qty: 1, X: 3, Y: 3, FreshDays: -1}, {ID: 91, Kind: "rifle", Qty: 1, X: 4, Y: 3, FreshDays: -1}}
	s.world.Plants = []game.PlantView{{ID: 41, X: 3, Y: 4, Species: "potato", Growth: 100, Stage: 3}}
	a, _ := colonyApp(t, s)
	img := ebiten.NewImage(200, 200)
	for _, it := range s.world.Items {
		a.drawItemIcon(img, it, 10, 10, 40)
	}
	a.drawPlantIcon(img, s.world.Plants[0], 10, 10, 40)
	drawAnimalIcon(img, a.renderer.art, s.world.Animals[0], 10, 10, 40)
	a.renderer.art = nil // no art: nothing drawn, no panic
	a.drawItemIcon(img, s.world.Items[0], 10, 10, 40)
	a.drawPlantIcon(img, s.world.Plants[0], 10, 10, 40)
}
