package ebiten

import (
	"fmt"
	"image/color"
	"rimworld-go/internal/adapters/inbound/ebiten/architect"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	inspectorpkg "rimworld-go/internal/adapters/inbound/ebiten/inspector"
	orderspkg "rimworld-go/internal/adapters/inbound/ebiten/orders"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"slices"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/input"
)

// The colonist's inspect pane and command buttons (PLAY-018,
// cmds_pawn.go): what pawnPane and pawnCommands describe, and that every
// tab body draws inside the expanded panel the layout gives it.

// paneAreaSim is a stub colony that reports each colonist's allowed area
// (the game does, via AreaOf).
type paneAreaSim struct {
	*stubSim
	area map[entity.EntityID]int
}

func (s *paneAreaSim) AreaOf(id entity.EntityID) int {
	if a, ok := s.area[id]; ok {
		return a
	}
	return -1
}

// paneSim is Ada (1) with a bio, a job, a rifle, two garments and a full
// set of needs, a queued-orders-free schedule slot of Work, and the room
// to draw every tab.
func paneSim() *stubSim {
	s := bioSim()
	p := &s.pawns[0]
	p.Job, p.Weapon, p.Activity = "haul wood", "rifle", schedule.Work
	p.Health, p.Mood, p.Hunger, p.Rest, p.Recreation = 82, 64, 45, 70, 50
	p.MoodLabel = "content"
	p.Apparel = []string{"parka", "tuque"}
	p.ComfortMin, p.ComfortMax = -50, 280
	p.Thoughts = []game.ThoughtView{{Label: "Ate a fine meal", Value: 6}, {Label: "Slept on the ground", Value: -3}}
	p.Temperament = "social"
	p.Log = logs(6)
	s.w, s.h = 24, 24
	return s
}

// paneApp is a paused stub colony with Ada selected.
func paneApp(t *testing.T, s *stubSim) (*App, *fakeInput) {
	t.Helper()
	a, f := colonyApp(t, s)
	a.renderer.Select(1)
	f.Frame(t, a)
	return a, f
}

func paneTabByID(spec inspectorpkg.Spec, id string) (inspectorpkg.Tab, bool) {
	for _, tb := range spec.Tabs {
		if tb.ID == id {
			return tb, true
		}
	}
	return inspectorpkg.Tab{}, false
}

func cmdByID(cmds []inspectorpkg.Button, id string) (inspectorpkg.Button, bool) {
	for _, c := range cmds {
		if c.ID == id {
			return c, true
		}
	}
	return inspectorpkg.Button{}, false
}

// pressCmd presses a command button the way the command grid does: a
// disabled one shows its reason; else Run, else Cmds, else the legacy click
// id (hud_commands.go).
func pressCmd(a *App, c inspectorpkg.Button) {
	switch {
	case c.Disabled:
		a.hint = c.Reason
	case c.Run != nil:
		c.Run()
	case len(c.Cmds) > 0:
		for _, cmd := range c.Cmds {
			a.renderer.enqueue(cmd)
		}
	default:
		a.applyClick(c.ID)
	}
}

func TestPawnPaneSpec(t *testing.T) {
	t.Parallel()
	s := paneSim()
	s.pawns[0].QueuedOrders = []game.QueuedOrderView{{Label: "Haul 5 wood to stockpile", X: 3, Y: 3}, {Label: "Equip rifle", X: 4, Y: 4}}
	s.pawns[0].Postponed = []string{"build wall: blocked, retry in 12 s"}
	s.pawns[0].Dirt = 30
	a, _ := paneApp(t, s)

	spec := a.insp.Pawn().PawnPane(1)
	if spec.Title != "Ada" {
		t.Fatalf("title %q, want the name", spec.Title)
	}
	if spec.Subtitle != "Age 31, Convoy mechanic" {
		t.Fatalf("subtitle %q", spec.Subtitle)
	}
	if spec.Close != "inspector.close" {
		t.Fatalf("close id %q", spec.Close)
	}

	// bars: Health, Mood, Food, Rest, in that order, with the read model's percent
	want := []struct {
		label string
		pct   int
	}{{"Health", 82}, {"Mood", 64}, {"Food", 45}, {"Rest", 70}}
	if len(spec.Bars) != len(want) {
		t.Fatalf("bars %+v", spec.Bars)
	}
	for i, w := range want {
		if spec.Bars[i].Label != w.label || spec.Bars[i].Pct != w.pct {
			t.Errorf("bar %d = %+v, want %s %d", i, spec.Bars[i], w.label, w.pct)
		}
	}
	if spec.Bars[1].Color != uikit.MoodGood {
		t.Errorf("a 64%% mood is the good mood colour, got %v", spec.Bars[1].Color)
	}

	// chips: the schedule slot now
	var sched inspectorpkg.Chip
	for _, c := range spec.Chips {
		if c.ID == "tab.schedule" {
			sched = c
		}
	}
	if sched.Label != "Schedule: work" || sched.Tip == "" {
		t.Fatalf("schedule chip %+v in %+v", sched, spec.Chips)
	}

	// lines: job, queued orders (count and each), weapon, wearing, the rest
	lines := strings.Join(spec.Lines, "\n")
	for _, w := range []string{"Job: haul wood", "Queued orders 2/3", "1. Haul 5 wood to stockpile", "2. Equip rifle",
		"Weapon: rifle", "Wearing: parka, tuque", "Dirty boots 30%", "put off: build wall", "Temperament: social"} {
		if !strings.Contains(lines, w) {
			t.Errorf("lines lack %q:\n%s", w, lines)
		}
	}

	// tabs: today's inspector tabs, minus Character (the pane itself)
	var ids []string
	for _, tb := range spec.Tabs {
		ids = append(ids, tb.ID)
		if tb.WantW <= 0 || tb.WantH <= 0 || tb.Label == "" {
			t.Errorf("tab %+v has no size or label", tb)
		}
		if (tb.Draw == nil) == (tb.Declare == nil) {
			t.Errorf("tab %s needs exactly one of Draw (legacy) and Declare (kit)", tb.ID)
		}
	}
	wantIDs := []string{"inspector.needs", "inspector.bio", "inspector.health", "inspector.social", "inspector.gear", "inspector.log"}
	if !slices.Equal(ids, wantIDs) {
		t.Fatalf("tabs %v, want %v", ids, wantIDs)
	}
	for _, id := range []string{"inspector.needs", "inspector.gear", "inspector.log"} {
		if tb, _ := paneTabByID(spec, id); tb.Draw == nil {
			t.Errorf("%s is a legacy draw", id)
		}
	}
	for _, id := range []string{"inspector.bio", "inspector.health", "inspector.social"} {
		if tb, _ := paneTabByID(spec, id); tb.Declare == nil {
			t.Errorf("%s is a widget-kit declare", id)
		}
	}
	if tb, _ := paneTabByID(spec, "inspector.bio"); tb.Disabled {
		t.Errorf("Ada has a bio: %+v", tb)
	}

	// a stale or missing colonist has no pane
	if got := a.insp.Pawn().PawnPane(99); got.Title != "" || len(got.Tabs) != 0 {
		t.Fatalf("pane for nobody: %+v", got)
	}
}

func TestPawnPaneBioTabDisabledWithoutABio(t *testing.T) {
	t.Parallel()
	s := paneSim()
	s.pawns[0].Sex = ""
	a, _ := paneApp(t, s)
	tb, ok := paneTabByID(a.insp.Pawn().PawnPane(1), "inspector.bio")
	if !ok || !tb.Disabled || tb.Reason != "No biography for this colonist" {
		t.Fatalf("Bio tab %+v ok=%v, want disabled with a reason", tb, ok)
	}
	if a.insp.Tabs().BioTabOn() {
		t.Fatal("the Bio tab can't be on for a colonist without one")
	}
}

func TestPawnPaneStatusLines(t *testing.T) {
	t.Parallel()
	s := paneSim()
	a, _ := paneApp(t, s)
	has := func(want string) bool {
		return strings.Contains(strings.Join(a.insp.Pawn().PawnPane(1).Lines, "\n"), want)
	}
	if has("Drafted") || has("MENTAL") || has("Downed") {
		t.Fatalf("a calm colonist has no status line: %q", a.insp.Pawn().PawnPane(1).Lines)
	}
	s.pawns[0].Drafted = true
	s.pawns[0].BlockedBy = 7
	s.pawns[0].MentalBreak = "berserk"
	s.pawns[0].Hypothermia = 250
	s.pawns[0].Infection = 40
	s.pawns[0].Wounded = true
	s.pawns[0].Status, s.pawns[0].Reason = "downed", "bleeding"
	for _, w := range []string{"Drafted", "MENTAL BREAK: berserk", "Hypothermia 25%", "Infection: winning (40%)", "Downed (bleeding)", "Waiting for"} {
		if !has(w) {
			t.Errorf("status lines lack %q: %q", w, a.insp.Pawn().PawnPane(1).Lines)
		}
	}
}

func TestPawnPaneAreaAndScheduleChips(t *testing.T) {
	t.Parallel()
	s := &paneAreaSim{stubSim: paneSim(), area: map[entity.EntityID]int{1: 3}}
	s.areas = []game.AreaView{{Index: 0, ID: 3, Name: "Indoors", Cells: 12}}
	a, _ := colonyApp(t, s.stubSim)
	a.renderer.game = s
	a.renderer.Select(1)
	spec := a.insp.Pawn().PawnPane(1)
	var area inspectorpkg.Chip
	for _, c := range spec.Chips {
		if c.ID == "inspector.area" {
			area = c
		}
	}
	if area.Label != "Area: Indoors" || !strings.Contains(area.Tip, "Allowed area") {
		t.Fatalf("area chip %+v in %+v", area, spec.Chips)
	}
	// the stub without AreaOf reports no areas: no chip, the rest stays
	a2, _ := paneApp(t, paneSim())
	for _, c := range a2.insp.Pawn().PawnPane(1).Chips {
		if c.ID == "inspector.area" {
			t.Fatalf("a game that reports no areas has no area chip: %+v", c)
		}
	}
}

// Clicking the area chip opens the area picker, and a row emits AssignArea
// (PLAY-015 §1.3, now on the pane).
func TestAreaChipOpensPicker(t *testing.T) {
	t.Parallel()
	s := &paneAreaSim{stubSim: paneSim(), area: map[entity.EntityID]int{1: 3}}
	s.areas = []game.AreaView{{Index: 0, ID: 3, Name: "Indoors", Cells: 12}, {Index: 1, ID: 5, Name: "Yard", Cells: 30}}
	a, f := colonyApp(t, s.stubSim)
	a.renderer.game = s
	a.renderer.Select(1)
	f.Frame(t, a)
	if !hasWidget(a, "inspector.area") {
		t.Fatal("the pane's area chip answers its click through a widget")
	}
	click(t, a, "inspector.area")
	f.Frame(t, a)
	if a.uiModalTop() != architect.AreaPickID {
		t.Fatalf("top modal %q, want the area picker", a.uiModalTop())
	}
	press(t, a, f, "areapick.row.5")
	if c := s.cmds[len(s.cmds)-1]; c.Kind != command.AssignArea || c.Pawn != 1 || c.Index != 5 {
		t.Fatalf("pick command %+v, want AssignArea Pawn 1 Index 5", c)
	}
	// the chip is on the pane whatever tab is open
	a.insp.Tab = "needs"
	f.Frame(t, a)
	if !hasWidget(a, "inspector.area") {
		t.Fatal("the area chip shows on every tab")
	}
}

// With the pane live, the chip is a real button on the pane and a mouse
// press on it opens the picker.
func TestAreaChipIsAButtonOnThePane(t *testing.T) {
	t.Parallel()
	s := &paneAreaSim{stubSim: paneSim(), area: map[entity.EntityID]int{1: 3}}
	s.areas = []game.AreaView{{Index: 0, ID: 3, Name: "Indoors", Cells: 12}}
	a, f := colonyApp(t, s.stubSim)
	a.renderer.game = s
	a.renderer.Select(1)
	f.Frame(t, a)
	var chip uikit.Button
	for _, b := range a.insp.PaneButtons() {
		if b.ID == architect.AreaChipID {
			chip = b
		}
	}
	if chip.W == 0 || chip.Label != "Area: Indoors" {
		t.Fatalf("the pane has no area chip button: %+v", chip)
	}
	if !a.hud().Pane.Contains(chip.X+chip.W/2, chip.Y+chip.H/2) {
		t.Fatalf("chip %+v lies outside the pane %+v", chip, a.hud().Pane)
	}
	w, _ := a.uiSt().Find(architect.AreaChipID)
	if w.R != (gfx.Rect{X: chip.X, Y: chip.Y, W: chip.W, H: chip.H}) {
		t.Fatalf("the click widget %+v does not sit on the chip %+v", w.R, chip)
	}
	press(t, a, f, architect.AreaChipID)
	f.Frame(t, a)
	if a.uiModalTop() != architect.AreaPickID {
		t.Fatalf("top modal %q, want the area picker", a.uiModalTop())
	}
}

func TestPawnPanePrisonerSwapsGearForPrisoner(t *testing.T) {
	t.Parallel()
	s := prisonerSim()
	a, f := colonyApp(t, s)
	a.renderer.Select(2)
	f.Frame(t, a)
	spec := a.insp.Pawn().PawnPane(2)
	var ids []string
	for _, tb := range spec.Tabs {
		ids = append(ids, tb.ID)
	}
	if slices.Contains(ids, "inspector.gear") || !slices.Contains(ids, "inspector.prisoner") {
		t.Fatalf("a prisoner's Gear tab gives way to its Prisoner tab: %v", ids)
	}
	if tb, _ := paneTabByID(spec, inspectorpkg.PrisonerTabID); tb.Declare == nil || tb.WantW < hudlayout.ExpandMinW {
		t.Fatalf("Prisoner tab %+v is a widget-kit tab", tb)
	}
	// a colonist has no Prisoner tab
	spec = a.insp.Pawn().PawnPane(1)
	if _, ok := paneTabByID(spec, inspectorpkg.PrisonerTabID); ok {
		t.Fatal("no Prisoner tab for a colonist")
	}
	if _, ok := paneTabByID(spec, "inspector.gear"); !ok {
		t.Fatal("a colonist keeps Gear")
	}
}

// ---- commands ---------------------------------------------------------------

func TestPawnCommandsDeclarationOrderAndHotkeys(t *testing.T) {
	t.Parallel()
	a, _ := paneApp(t, paneSim())
	var ids []string
	for _, c := range a.insp.Pawn().PawnCommands(1) {
		ids = append(ids, c.ID)
		if c.Label == "" || c.Desc == "" {
			t.Errorf("command %s needs a label and a tooltip body", c.ID)
		}
	}
	want := []string{"pawn.draft", "pawn.clearorders", "cmd.pawn.equip", "cmd.pawn.bedrest", inspectorpkg.PrisonerCapture, inspectorpkg.PrisonerArrest, inspectorpkg.DropWeaponID} // Drop weapon (STO-3) comes last, only for an armed colonist, so the hotkeys before it keep their order
	if !slices.Equal(ids, want) {
		t.Fatalf("commands %v, want %v", ids, want)
	}
	if got := a.insp.Pawn().PawnCommands(99); len(got) != 0 {
		t.Fatalf("commands for nobody: %v", got)
	}
}

func TestPawnDraftCommandToggles(t *testing.T) {
	t.Parallel()
	s := paneSim()
	a, _ := paneApp(t, s)
	d, ok := cmdByID(a.insp.Pawn().PawnCommands(1), "pawn.draft")
	if !ok || !d.Toggle || d.On || d.Hotkey != input.Draft || d.Icon != "draft" || d.Disabled {
		t.Fatalf("draft command %+v", d)
	}
	if d.Run != nil || len(d.Cmds) != 0 {
		t.Fatal("a legacy id keeps its app.go handler and carries no Run or Cmds")
	}
	pressCmd(a, d)
	if c := lastCmd(t, s); c.Kind != command.DraftPawn || c.Pawn != 1 || !c.Enabled {
		t.Fatalf("draft click enqueued %+v, want DraftPawn Enabled", c)
	}
	s.pawns[0].Drafted = true
	d, _ = cmdByID(a.insp.Pawn().PawnCommands(1), "pawn.draft")
	if !d.On {
		t.Fatal("a drafted colonist shows the toggle on")
	}
	pressCmd(a, d)
	if c := lastCmd(t, s); c.Kind != command.DraftPawn || c.Enabled {
		t.Fatalf("undraft click enqueued %+v", c)
	}
}

func TestPawnClearOrdersCommand(t *testing.T) {
	t.Parallel()
	s := paneSim()
	a, _ := paneApp(t, s)
	c, _ := cmdByID(a.insp.Pawn().PawnCommands(1), "pawn.clearorders")
	if !c.Disabled || c.Reason != "no queued orders" || c.Icon != "clear_orders" {
		t.Fatalf("empty queue: %+v", c)
	}
	s.pawns[0].QueuedOrders = []game.QueuedOrderView{{Label: "Haul wood", X: 3, Y: 3}}
	c, _ = cmdByID(a.insp.Pawn().PawnCommands(1), "pawn.clearorders")
	if c.Disabled {
		t.Fatalf("a queued order enables Clear orders: %+v", c)
	}
	pressCmd(a, c)
	if k := lastCmd(t, s); k.Kind != command.ClearOrders || k.Pawn != 1 {
		t.Fatalf("clear click enqueued %+v", k)
	}
}

func TestPawnCommandsDisabledWhenNotAlive(t *testing.T) {
	t.Parallel()
	s := paneSim()
	s.pawns[0].Status = "downed"
	s.pawns[0].QueuedOrders = []game.QueuedOrderView{{Label: "Haul wood"}}
	a, _ := paneApp(t, s)
	for _, id := range []string{"pawn.draft", "pawn.clearorders", "cmd.pawn.equip", inspectorpkg.PrisonerCapture} {
		c, ok := cmdByID(a.insp.Pawn().PawnCommands(1), id)
		if !ok || !c.Disabled || c.Reason != "downed" {
			t.Errorf("%s = %+v ok=%v, want disabled for the status", id, c, ok)
		}
	}
	pressCmd(a, mustCmd(t, a.insp.Pawn().PawnCommands(1), "pawn.draft"))
	if a.hint != "downed" {
		t.Fatalf("a disabled command shows its reason, hint %q", a.hint)
	}
	if len(s.cmds) != 0 {
		t.Fatalf("a disabled command enqueues nothing: %v", s.cmds)
	}
}

func mustCmd(t *testing.T, cmds []inspectorpkg.Button, id string) inspectorpkg.Button {
	t.Helper()
	c, ok := cmdByID(cmds, id)
	if !ok {
		t.Fatalf("no command %s", id)
	}
	return c
}

func TestPawnCaptureAndArrestCommands(t *testing.T) {
	t.Parallel()
	s := prisonerSim()
	a, f := colonyApp(t, s)
	a.renderer.Select(1)
	f.Frame(t, a)
	cap := mustCmd(t, a.insp.Pawn().PawnCommands(1), inspectorpkg.PrisonerCapture)
	if cap.Disabled || cap.Icon != "capture" || !strings.Contains(cap.Desc, "Vasha") {
		t.Fatalf("Ada can capture the downed raider: %+v", cap)
	}
	pressCmd(a, cap)
	if c := lastCmd(t, s); c.Kind != command.Capture || c.Pawn != 1 || c.Target != 3 {
		t.Fatalf("capture command %+v", c)
	}
	arr := mustCmd(t, a.insp.Pawn().PawnCommands(1), inspectorpkg.PrisonerArrest)
	if arr.Disabled || arr.Icon != "arrest" {
		t.Fatalf("arrest command %+v", arr)
	}
	pressCmd(a, arr)
	if c := lastCmd(t, s); c.Kind != command.Arrest || c.Target != 1 || c.Pawn != 0 {
		t.Fatalf("arrest command %+v", c)
	}
	// a prisoner is not arrested again and captures no one
	a.renderer.Select(2)
	for _, id := range []string{inspectorpkg.PrisonerArrest, inspectorpkg.PrisonerCapture} {
		if _, ok := cmdByID(a.insp.Pawn().PawnCommands(2), id); ok {
			t.Fatalf("a prisoner has no %s", id)
		}
	}
	// no downed raider: Capture says why
	s.pawns = s.pawns[:2]
	c := mustCmd(t, a.insp.Pawn().PawnCommands(1), inspectorpkg.PrisonerCapture)
	if !c.Disabled || c.Reason != "no downed enemy to capture" {
		t.Fatalf("no one to capture: %+v", c)
	}
}

func TestPawnEquipCommandOptionsEnqueueOrderEquip(t *testing.T) {
	t.Parallel()
	s := paneSim()
	s.world.Items = []game.ItemView{{ID: 50, Kind: "rifle", Qty: 1, X: 8, Y: 5}}
	s.orders = []game.OrderOption{{Label: "Equip rifle", Cmd: command.Command{Kind: command.Prioritize, Order: command.OrderEquip, Pawn: 1, X: 8, Y: 5}}}
	a, _ := paneApp(t, s)
	eq := mustCmd(t, a.insp.Pawn().PawnCommands(1), "cmd.pawn.equip")
	if eq.Disabled || eq.Icon != "equip" || eq.Options == nil || eq.Run != nil || len(eq.Cmds) != 0 {
		t.Fatalf("equip command %+v", eq)
	}
	opts := eq.Options()
	if len(opts) == 0 {
		t.Fatal("a rifle in reach is an option")
	}
	first := opts[0]
	if first.Disabled || first.Cmd == nil || first.Cmd.Order != command.OrderEquip || !strings.Contains(first.Label, "rifle") || !strings.Contains(first.Label, "3 tiles") {
		t.Fatalf("first option %+v", first)
	}
	a.renderer.enqueue(*first.Cmd) // what picking the option does
	if c := lastCmd(t, s); c.Kind != command.Prioritize || c.Order != command.OrderEquip || c.X != 8 || c.Y != 5 {
		t.Fatalf("the option enqueued %+v, want the OrderEquip for the rifle", c)
	}
}

func TestPawnEquipCommandNothingInReach(t *testing.T) {
	t.Parallel()
	a, _ := paneApp(t, paneSim())
	eq := mustCmd(t, a.insp.Pawn().PawnCommands(1), "cmd.pawn.equip")
	if !eq.Disabled || eq.Reason != "nothing to equip in reach" {
		t.Fatalf("empty list: %+v", eq)
	}
}

// With the grid live, the colonist's commands are its cells, a click on
// Draft toggles the draft and a click on Equip runs its first option.
func TestPawnCommandsAreGridCells(t *testing.T) {
	t.Parallel()
	s := paneSim()
	s.world.Items = []game.ItemView{{ID: 50, Kind: "rifle", Qty: 1, X: 8, Y: 5}}
	s.orders = []game.OrderOption{{Label: "Equip rifle", Cmd: command.Command{Kind: command.Prioritize, Order: command.OrderEquip, Pawn: 1, X: 8, Y: 5}}}
	a, _ := paneApp(t, s)
	var ids []string
	for _, b := range a.insp.Commands().CommandGridButtons() {
		ids = append(ids, b.ID)
	}
	for _, want := range []string{"pawn.draft", "pawn.clearorders", "cmd.pawn.equip", inspectorpkg.PrisonerCapture, inspectorpkg.PrisonerArrest} {
		if !slices.Contains(ids, want) {
			t.Errorf("the grid lacks %s: %v", want, ids)
		}
	}
	click(t, a, "pawn.draft")
	if c := lastCmd(t, s); c.Kind != command.DraftPawn || !c.Enabled {
		t.Fatalf("Draft cell enqueued %+v", c)
	}
	click(t, a, "cmd.pawn.equip")
	if c := lastCmd(t, s); c.Kind != command.Prioritize || c.Order != command.OrderEquip || c.X != 8 {
		t.Fatalf("Equip cell enqueued %+v, want the rifle's OrderEquip", c)
	}
	click(t, a, inspectorpkg.PrisonerArrest)
	if c := lastCmd(t, s); c.Kind != command.Arrest || c.Target != 1 {
		t.Fatalf("Arrest cell enqueued %+v", c)
	}
	// the pane's tabs are its tab strip buttons
	var tabs []string
	for _, b := range a.insp.PaneButtons() {
		if strings.HasPrefix(b.ID, "inspector.") {
			tabs = append(tabs, b.ID)
		}
	}
	for _, want := range []string{"inspector.needs", "inspector.bio", "inspector.health", "inspector.social", "inspector.gear", "inspector.log"} {
		if !slices.Contains(tabs, want) {
			t.Errorf("the pane has no %s tab button: %v", want, tabs)
		}
	}
}

// Opening the Bio tab through the pane (its click) declares the body in the
// panel above the pane, and clicking it again collapses it.
func TestPawnBioTabOpensThroughThePane(t *testing.T) {
	t.Parallel()
	a, f := paneApp(t, paneSim())
	click(t, a, "inspector.bio")
	f.Frame(t, a)
	if !a.insp.Tabs().BioTabOn() || !hasWidget(a, "bio.skill.0") {
		t.Fatalf("tab %q: the Bio body is not declared", a.insp.Tab)
	}
	if exp := a.insp.ExpandedRect(); exp.Empty() || !a.hud().PaneTabs.Contains(exp.X+1, exp.Bottom()+1) {
		t.Fatalf("expanded %+v does not open upward from the tab strip %+v", exp, a.hud().PaneTabs)
	}
	click(t, a, "inspector.bio")
	f.Frame(t, a)
	if a.insp.Tabs().BioTabOn() || hasWidget(a, "bio.skill.0") {
		t.Fatal("clicking the open tab collapses it")
	}
}

// ---- tab bodies draw inside the expanded panel --------------------------------

// expandedFor is the Expanded rect a 1280x720 screen gives tab tb.
func expandedFor(sw, sh int, tb inspectorpkg.Tab) gfx.Rect {
	return hudlayout.Compute(sw, sh, hudlayout.Inputs{Colonists: 6, PaneShown: true, ExpandedW: tb.WantW, ExpandedH: tb.WantH}).Expanded
}

// bodyPaints drops what the frame already queued to paint (the pane's own
// scroll bars and the like), so a trace sees only the body declared next.
func bodyPaints(a *App) { a.uiSt().Paints = a.uiSt().Paints[:0] }

// innerFor is the rectangle the pane hands tab tb's body: the panel the
// layout gives it, inset by paneInset.
func innerFor(sw, sh int, tb inspectorpkg.Tab) gfx.Rect {
	return expandedFor(sw, sh, tb).Inset(inspectorpkg.PaneInset)
}

// openPawnTab opens the selected colonist's tab the way a click on the
// pane's tab does. Call it on a closed tab (a second click collapses).
func openPawnTab(t *testing.T, a *App, tab string) {
	t.Helper()
	a.insp.SelectPaneTab("inspector." + tab) // hud_inspect.go
	a.insp.Tab = tab                         // kept in step for the legacy readers
}

// tabFixture is a colony whose colonist 1 has data for every tab.
func tabFixture() *stubSim {
	s := paneSim()
	s.health = map[entity.EntityID]game.HealthView{1: {
		Pain:       12,
		Capacities: []game.CapacityView{{Label: "Moving", Pct: 85}, {Label: "Manipulation", Pct: 100}, {Label: "Sight", Pct: 60}, {Label: "Consciousness", Pct: 100}},
		Parts:      []game.PartView{{Label: "left leg", Status: "missing"}, {Label: "right arm", Status: game.PartWounded}},
		Operations: []game.OperationView{{Label: "Install bionic arm", Slot: 0}},
	}}
	s.social = map[entity.EntityID]game.SocialView{1: socialFixture()}
	return s
}

// summarize lists the first n traced draws.
func summarize(events []gfx.TraceEvent, n int) string {
	var b strings.Builder
	for i, e := range events {
		if i == n {
			b.WriteString(" ...")
			break
		}
		b.WriteString(fmt.Sprintf("\n  %s %q %.0f,%.0f %.0fx%.0f", e.Kind, e.Text, e.X, e.Y, e.W, e.H))
	}
	return b.String()
}

// inside reports every traced draw lies within r (logical) scaled by s,
// give or take a pixel.
func inside(events []gfx.TraceEvent, r gfx.Rect, s float64) (bad []gfx.TraceEvent) {
	const slack = 2
	x0, y0 := float64(r.X)*s-slack, float64(r.Y)*s-slack
	x1, y1 := float64(r.Right())*s+slack, float64(r.Bottom())*s+slack
	for _, e := range events {
		if float64(e.X) < x0 || float64(e.Y) < y0 || float64(e.X+e.W) > x1 || float64(e.Y+e.H) > y1 {
			bad = append(bad, e)
		}
	}
	return bad
}

func TestPawnTabBodiesDrawInsideExpandedPanel(t *testing.T) {
	for _, scale := range []float64{1, 1.5} {
		for _, tab := range []string{"needs", "bio", "health", "social", "gear", "log"} {
			t.Run(tab+"/"+map[float64]string{1: "x1", 1.5: "x1.5"}[scale], func(t *testing.T) {
				a, _ := paneApp(t, tabFixture())
				withUIScale(t, scale) // after the frames: Update resets the scale from the settings
				spec := a.insp.Pawn().PawnPane(1)
				tb, ok := paneTabByID(spec, "inspector."+tab)
				if !ok {
					t.Fatalf("no %s tab", tab)
				}
				if o := expandedFor(1280, 720, tb); o.Empty() || o.W < hudlayout.ExpandMinW || o.W > hudlayout.ExpandMaxW {
					t.Fatalf("expanded rect %+v for want %vx%v", o, tb.WantW, tb.WantH)
				}
				r := innerFor(1280, 720, tb) // what the pane hands the body
				var got []gfx.TraceEvent
				gfx.SetDrawTrace(func(e gfx.TraceEvent) { got = append(got, e) })
				t.Cleanup(func() { gfx.SetDrawTrace(nil); gfx.ForcePass("hud"); gfx.SetVia("") })
				img := ebiten.NewImage(1920, 1080)
				if tb.Draw != nil {
					tb.Draw(img, r)
				} else {
					bodyPaints(a)
					tb.Declare(r)
					a.uiSt().Paint(img)
				}
				if len(got) == 0 {
					t.Fatalf("%s drew nothing", tab)
				}
				if bad := inside(got, r, scale); len(bad) > 0 {
					t.Fatalf("%s draws outside its panel %+v (x%v): %s", tab, r, scale, summarize(bad, 6))
				}
			})
		}
	}
}

// A tab body asked to draw in a smaller panel (a short window) still draws
// and declares without a panic, and its widgets stay inside it.
func TestPawnTabBodiesSurviveASmallPanel(t *testing.T) {
	t.Parallel()
	for _, tab := range []string{"needs", "bio", "health", "social", "gear", "log"} {
		a, _ := paneApp(t, tabFixture())
		tb, _ := paneTabByID(a.insp.Pawn().PawnPane(1), "inspector."+tab)
		r := gfx.Rect{X: 0, Y: 100, W: 400, H: 300}
		img := ebiten.NewImage(800, 600)
		before := len(a.uiSt().Cur) // the HUD's own widgets are not the tab's
		if tb.Draw != nil {
			tb.Draw(img, r)
		} else {
			tb.Declare(r)
			a.uiSt().Paint(img)
		}
		for _, w := range a.uiSt().Cur[before:] {
			if w.R.W > 0 && (w.R.Y < r.Y-1 || w.R.Bottom() > r.Bottom()+1 || w.R.X < r.X-1 || w.R.Right() > r.Right()+1) {
				t.Errorf("%s: widget %s at %+v lies outside the panel %+v", tab, w.ID, w.R, r)
			}
		}
	}
}

// The Log body's wheel scroll stays inside the panel's text area.
func TestPawnLogWheelScrollsInsideTheExpandedPanel(t *testing.T) {
	t.Parallel()
	s := paneSim()
	s.pawns[0].Log = logs(40)
	a, _ := paneApp(t, s)
	r := gfx.Rect{X: 0, Y: 100, W: 400, H: 280}
	p, _ := a.insp.SelectedPawn()
	a.insp.Pawn().DrawPawnLogRows(ebiten.NewImage(800, 600), p, r) // a frame draws before the wheel turns
	x, y, w, h := inspectorpkg.LogBodyRect(r)
	if float32(x) < r.X || float32(y) < r.Y || float32(x+w) > r.Right() || float32(y+h) > r.Bottom() {
		t.Fatalf("log text area %d,%d %dx%d outside the panel %+v", x, y, w, h, r)
	}
	if !a.insp.Pawn().ScrollLogIn(r, x+5, y+5, -1) || a.insp.LogScroll == 0 {
		t.Fatalf("wheel over the log must scroll it (scroll %d)", a.insp.LogScroll)
	}
	if a.insp.Pawn().ScrollLogIn(r, int(r.Right())+40, int(r.Bottom())+40, -1) {
		t.Fatal("wheel outside the panel must not be taken")
	}
}

// Through the pane: with the Log tab open the wheel over its text scrolls
// it, and with another tab open it does not.
func TestPawnLogWheelThroughThePane(t *testing.T) {
	t.Parallel()
	s := paneSim()
	s.pawns[0].Log = logs(40)
	a, f := paneApp(t, s)
	if a.insp.Pawn().ScrollLogAt(100, 300, -1) {
		t.Fatal("no tab open: the wheel is not the log's")
	}
	click(t, a, "inspector.log")
	drawFrame(t, a, f) // the frame draws the Log, which syncs it to the colonist
	x, y, w, h := inspectorpkg.LogBodyRect(a.insp.ExpandedRect().Inset(inspectorpkg.PaneInset))
	if w <= 0 || h <= 0 {
		t.Fatalf("log text area %d,%d %dx%d", x, y, w, h)
	}
	if !a.insp.Pawn().ScrollLogAt(x+5, y+5, -1) || a.insp.LogScroll == 0 {
		t.Fatalf("wheel over the open Log must scroll it (scroll %d)", a.insp.LogScroll)
	}
	click(t, a, "inspector.needs")
	f.Frame(t, a)
	if a.insp.Pawn().ScrollLogAt(x+5, y+5, -1) {
		t.Fatal("the Needs tab is open: the wheel is not the log's")
	}
}

// ---- legacy removal ---------------------------------------------------------

func TestLegacyPawnInspectorDrawsNothing(t *testing.T) {
	t.Parallel()
	a, _ := paneApp(t, paneSim())
	// the legacy inspector and order queue panels have no buttons and no
	// area: every hit target and every blocked point is the layout's
	assertButtonsInRegions(t, a)
	assertChromeIsTheLayout(t, a)
}

// The Prisoner tab is a widget-kit body too, and draws inside the panel.
func TestPrisonerTabDrawsInsideExpandedPanel(t *testing.T) {
	for _, scale := range []float64{1, 1.5} {
		a, f := colonyApp(t, prisonerSim())
		a.renderer.Select(2)
		f.Frame(t, a)
		withUIScale(t, scale) // after the frames: Update resets the scale from the settings
		tb, ok := paneTabByID(a.insp.Pawn().PawnPane(2), inspectorpkg.PrisonerTabID)
		if !ok {
			t.Fatal("no Prisoner tab")
		}
		r := innerFor(1280, 720, tb)
		var got []gfx.TraceEvent
		gfx.SetDrawTrace(func(e gfx.TraceEvent) { got = append(got, e) })
		img := ebiten.NewImage(1920, 1080)
		bodyPaints(a)
		tb.Declare(r)
		a.uiSt().Paint(img)
		gfx.SetDrawTrace(nil)
		gfx.ForcePass("hud")
		gfx.SetVia("")
		if len(got) == 0 {
			t.Fatal("the Prisoner tab drew nothing")
		}
		if bad := inside(got, r, scale); len(bad) > 0 {
			t.Fatalf("the Prisoner tab draws outside its panel %+v (x%v): %s", r, scale, summarize(bad, 6))
		}
		for _, m := range inspectorpkg.PrisonerModes {
			if !hasWidget(a, inspectorpkg.PrisonerModeID(m.Mode)) {
				t.Errorf("mode %s missing", m.Label)
			}
		}
		if !hasWidget(a, inspectorpkg.PrisonerRelease) {
			t.Error("Release now missing")
		}
	}
}

// The order route is drawn in palette tokens, premultiplied for the vector
// fills (no component exceeds its alpha).
func TestOrderRouteTintIsPremultipliedPaletteToken(t *testing.T) {
	t.Parallel()
	for _, c := range []color.RGBA{uikit.Accent, uikit.Btn, uikit.BtnSelected} {
		got := orderspkg.RouteTint(c, 200)
		if got.A != 200 || got.R > got.A || got.G > got.A || got.B > got.A {
			t.Errorf("routeTint(%v, 200) = %v, want a premultiplied colour", c, got)
		}
	}
	if got := orderspkg.RouteTint(uikit.Accent, 255); got != uikit.Accent {
		t.Errorf("opaque tint = %v, want the token %v", got, uikit.Accent)
	}
}
