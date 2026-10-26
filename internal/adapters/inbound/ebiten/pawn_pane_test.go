package ebiten

import (
	"fmt"
	"image/color"
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
	f.frame(t, a)
	return a, f
}

func paneTabByID(spec paneSpec, id string) (paneTab, bool) {
	for _, tb := range spec.Tabs {
		if tb.ID == id {
			return tb, true
		}
	}
	return paneTab{}, false
}

func cmdByID(cmds []cmdButton, id string) (cmdButton, bool) {
	for _, c := range cmds {
		if c.ID == id {
			return c, true
		}
	}
	return cmdButton{}, false
}

// pressCmd presses a command button the way the command grid does: a
// disabled one shows its reason; else Run, else Cmds, else the legacy click
// id (hud_commands.go).
func pressCmd(a *App, c cmdButton) {
	switch {
	case c.Disabled:
		a.hint = c.Reason
	case c.Run != nil:
		c.Run(a)
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

	spec := a.pawnPane(1)
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
	if spec.Bars[1].Color != uiMoodGood {
		t.Errorf("a 64%% mood is the good mood colour, got %v", spec.Bars[1].Color)
	}

	// chips: the schedule slot now
	var sched paneChip
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
	if got := a.pawnPane(99); got.Title != "" || len(got.Tabs) != 0 {
		t.Fatalf("pane for nobody: %+v", got)
	}
}

func TestPawnPaneBioTabDisabledWithoutABio(t *testing.T) {
	t.Parallel()
	s := paneSim()
	s.pawns[0].Sex = ""
	a, _ := paneApp(t, s)
	tb, ok := paneTabByID(a.pawnPane(1), "inspector.bio")
	if !ok || !tb.Disabled || tb.Reason != "No biography for this colonist" {
		t.Fatalf("Bio tab %+v ok=%v, want disabled with a reason", tb, ok)
	}
	if a.bioTabOn() {
		t.Fatal("the Bio tab can't be on for a colonist without one")
	}
}

func TestPawnPaneStatusLines(t *testing.T) {
	t.Parallel()
	s := paneSim()
	a, _ := paneApp(t, s)
	has := func(want string) bool {
		return strings.Contains(strings.Join(a.pawnPane(1).Lines, "\n"), want)
	}
	if has("Drafted") || has("MENTAL") || has("Downed") {
		t.Fatalf("a calm colonist has no status line: %q", a.pawnPane(1).Lines)
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
			t.Errorf("status lines lack %q: %q", w, a.pawnPane(1).Lines)
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
	spec := a.pawnPane(1)
	var area paneChip
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
	for _, c := range a2.pawnPane(1).Chips {
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
	f.frame(t, a)
	if !hasWidget(a, "inspector.area") {
		t.Fatal("the pane's area chip answers its click through a widget")
	}
	click(t, a, "inspector.area")
	f.frame(t, a)
	if a.uiModalTop() != areaPickID {
		t.Fatalf("top modal %q, want the area picker", a.uiModalTop())
	}
	press(t, a, f, "areapick.row.5")
	if c := s.cmds[len(s.cmds)-1]; c.Kind != command.AssignArea || c.Pawn != 1 || c.Index != 5 {
		t.Fatalf("pick command %+v, want AssignArea Pawn 1 Index 5", c)
	}
	// the chip is on the pane whatever tab is open
	a.inspectTab = "needs"
	f.frame(t, a)
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
	f.frame(t, a)
	var chip button
	for _, b := range a.paneButtons() {
		if b.id == paneAreaChipID {
			chip = b
		}
	}
	if chip.w == 0 || chip.label != "Area: Indoors" {
		t.Fatalf("the pane has no area chip button: %+v", chip)
	}
	if !a.hud().Pane.contains(chip.x+chip.w/2, chip.y+chip.h/2) {
		t.Fatalf("chip %+v lies outside the pane %+v", chip, a.hud().Pane)
	}
	w, _ := a.uiSt().find(paneAreaChipID)
	if w.r != (rect{chip.x, chip.y, chip.w, chip.h}) {
		t.Fatalf("the click widget %+v does not sit on the chip %+v", w.r, chip)
	}
	press(t, a, f, paneAreaChipID)
	f.frame(t, a)
	if a.uiModalTop() != areaPickID {
		t.Fatalf("top modal %q, want the area picker", a.uiModalTop())
	}
}

func TestPawnPanePrisonerSwapsGearForPrisoner(t *testing.T) {
	t.Parallel()
	s := prisonerSim()
	a, f := colonyApp(t, s)
	a.renderer.Select(2)
	f.frame(t, a)
	spec := a.pawnPane(2)
	var ids []string
	for _, tb := range spec.Tabs {
		ids = append(ids, tb.ID)
	}
	if slices.Contains(ids, "inspector.gear") || !slices.Contains(ids, "inspector.prisoner") {
		t.Fatalf("a prisoner's Gear tab gives way to its Prisoner tab: %v", ids)
	}
	if tb, _ := paneTabByID(spec, prisonerTabID); tb.Declare == nil || tb.WantW < expandMinW {
		t.Fatalf("Prisoner tab %+v is a widget-kit tab", tb)
	}
	// a colonist has no Prisoner tab
	spec = a.pawnPane(1)
	if _, ok := paneTabByID(spec, prisonerTabID); ok {
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
	for _, c := range a.pawnCommands(1) {
		ids = append(ids, c.ID)
		if c.Label == "" || c.Desc == "" {
			t.Errorf("command %s needs a label and a tooltip body", c.ID)
		}
	}
	want := []string{"pawn.draft", "pawn.clearorders", "cmd.pawn.equip", "cmd.pawn.bedrest", prisonerCapture, prisonerArrest}
	if !slices.Equal(ids, want) {
		t.Fatalf("commands %v, want %v", ids, want)
	}
	if got := a.pawnCommands(99); len(got) != 0 {
		t.Fatalf("commands for nobody: %v", got)
	}
}

func TestPawnDraftCommandToggles(t *testing.T) {
	t.Parallel()
	s := paneSim()
	a, _ := paneApp(t, s)
	d, ok := cmdByID(a.pawnCommands(1), "pawn.draft")
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
	d, _ = cmdByID(a.pawnCommands(1), "pawn.draft")
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
	c, _ := cmdByID(a.pawnCommands(1), "pawn.clearorders")
	if !c.Disabled || c.Reason != "no queued orders" || c.Icon != "clear_orders" {
		t.Fatalf("empty queue: %+v", c)
	}
	s.pawns[0].QueuedOrders = []game.QueuedOrderView{{Label: "Haul wood", X: 3, Y: 3}}
	c, _ = cmdByID(a.pawnCommands(1), "pawn.clearorders")
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
	for _, id := range []string{"pawn.draft", "pawn.clearorders", "cmd.pawn.equip", prisonerCapture} {
		c, ok := cmdByID(a.pawnCommands(1), id)
		if !ok || !c.Disabled || c.Reason != "downed" {
			t.Errorf("%s = %+v ok=%v, want disabled for the status", id, c, ok)
		}
	}
	pressCmd(a, mustCmd(t, a.pawnCommands(1), "pawn.draft"))
	if a.hint != "downed" {
		t.Fatalf("a disabled command shows its reason, hint %q", a.hint)
	}
	if len(s.cmds) != 0 {
		t.Fatalf("a disabled command enqueues nothing: %v", s.cmds)
	}
}

func mustCmd(t *testing.T, cmds []cmdButton, id string) cmdButton {
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
	f.frame(t, a)
	cap := mustCmd(t, a.pawnCommands(1), prisonerCapture)
	if cap.Disabled || cap.Icon != "capture" || !strings.Contains(cap.Desc, "Vasha") {
		t.Fatalf("Ada can capture the downed raider: %+v", cap)
	}
	pressCmd(a, cap)
	if c := lastCmd(t, s); c.Kind != command.Capture || c.Pawn != 1 || c.Target != 3 {
		t.Fatalf("capture command %+v", c)
	}
	arr := mustCmd(t, a.pawnCommands(1), prisonerArrest)
	if arr.Disabled || arr.Icon != "arrest" {
		t.Fatalf("arrest command %+v", arr)
	}
	pressCmd(a, arr)
	if c := lastCmd(t, s); c.Kind != command.Arrest || c.Target != 1 || c.Pawn != 0 {
		t.Fatalf("arrest command %+v", c)
	}
	// a prisoner is not arrested again and captures no one
	a.renderer.Select(2)
	for _, id := range []string{prisonerArrest, prisonerCapture} {
		if _, ok := cmdByID(a.pawnCommands(2), id); ok {
			t.Fatalf("a prisoner has no %s", id)
		}
	}
	// no downed raider: Capture says why
	s.pawns = s.pawns[:2]
	c := mustCmd(t, a.pawnCommands(1), prisonerCapture)
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
	eq := mustCmd(t, a.pawnCommands(1), "cmd.pawn.equip")
	if eq.Disabled || eq.Icon != "equip" || eq.Options == nil || eq.Run != nil || len(eq.Cmds) != 0 {
		t.Fatalf("equip command %+v", eq)
	}
	opts := eq.Options(a)
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
	eq := mustCmd(t, a.pawnCommands(1), "cmd.pawn.equip")
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
	for _, b := range a.commandGridButtons() {
		ids = append(ids, b.id)
	}
	for _, want := range []string{"pawn.draft", "pawn.clearorders", "cmd.pawn.equip", prisonerCapture, prisonerArrest} {
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
	click(t, a, prisonerArrest)
	if c := lastCmd(t, s); c.Kind != command.Arrest || c.Target != 1 {
		t.Fatalf("Arrest cell enqueued %+v", c)
	}
	// the pane's tabs are its tab strip buttons
	var tabs []string
	for _, b := range a.paneButtons() {
		if strings.HasPrefix(b.id, "inspector.") {
			tabs = append(tabs, b.id)
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
	f.frame(t, a)
	if !a.bioTabOn() || !hasWidget(a, "bio.skill.0") {
		t.Fatalf("tab %q: the Bio body is not declared", a.inspectTab)
	}
	if exp := a.expandedRect(); exp.empty() || !a.hud().PaneTabs.contains(exp.X+1, exp.bottom()+1) {
		t.Fatalf("expanded %+v does not open upward from the tab strip %+v", exp, a.hud().PaneTabs)
	}
	click(t, a, "inspector.bio")
	f.frame(t, a)
	if a.bioTabOn() || hasWidget(a, "bio.skill.0") {
		t.Fatal("clicking the open tab collapses it")
	}
}

// ---- tab bodies draw inside the expanded panel --------------------------------

// expandedFor is the Expanded rect a 1280x720 screen gives tab tb.
func expandedFor(sw, sh int, tb paneTab) rect {
	return computeHUDLayout(sw, sh, hudState{Colonists: 6, PaneShown: true, ExpandedW: tb.WantW, ExpandedH: tb.WantH}).Expanded
}

// bodyPaints drops what the frame already queued to paint (the pane's own
// scroll bars and the like), so a trace sees only the body declared next.
func bodyPaints(a *App) { a.uiSt().paints = a.uiSt().paints[:0] }

// innerFor is the rectangle the pane hands tab tb's body: the panel the
// layout gives it, inset by paneInset.
func innerFor(sw, sh int, tb paneTab) rect { return expandedFor(sw, sh, tb).inset(paneInset) }

// openPawnTab opens the selected colonist's tab the way a click on the
// pane's tab does. Call it on a closed tab (a second click collapses).
func openPawnTab(t *testing.T, a *App, tab string) {
	t.Helper()
	a.selectPaneTab("inspector." + tab) // hud_inspect.go
	a.inspectTab = tab                  // kept in step for the legacy readers
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
func summarize(events []traceEvent, n int) string {
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
func inside(events []traceEvent, r rect, s float64) (bad []traceEvent) {
	const slack = 2
	x0, y0 := float64(r.X)*s-slack, float64(r.Y)*s-slack
	x1, y1 := float64(r.right())*s+slack, float64(r.bottom())*s+slack
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
				spec := a.pawnPane(1)
				tb, ok := paneTabByID(spec, "inspector."+tab)
				if !ok {
					t.Fatalf("no %s tab", tab)
				}
				if o := expandedFor(1280, 720, tb); o.empty() || o.W < expandMinW || o.W > expandMaxW {
					t.Fatalf("expanded rect %+v for want %vx%v", o, tb.WantW, tb.WantH)
				}
				r := innerFor(1280, 720, tb) // what the pane hands the body
				var got []traceEvent
				drawTrace = func(e traceEvent) { got = append(got, e) }
				t.Cleanup(func() { drawTrace = nil; tracePass = "hud"; traceVia = "" })
				img := ebiten.NewImage(1920, 1080)
				if tb.Draw != nil {
					tb.Draw(img, r)
				} else {
					bodyPaints(a)
					tb.Declare(r)
					a.uiPaint(img)
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
		tb, _ := paneTabByID(a.pawnPane(1), "inspector."+tab)
		r := rect{0, 100, 400, 300}
		img := ebiten.NewImage(800, 600)
		before := len(a.uiSt().cur) // the HUD's own widgets are not the tab's
		if tb.Draw != nil {
			tb.Draw(img, r)
		} else {
			tb.Declare(r)
			a.uiPaint(img)
		}
		for _, w := range a.uiSt().cur[before:] {
			if w.r.W > 0 && (w.r.Y < r.Y-1 || w.r.bottom() > r.bottom()+1 || w.r.X < r.X-1 || w.r.right() > r.right()+1) {
				t.Errorf("%s: widget %s at %+v lies outside the panel %+v", tab, w.id, w.r, r)
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
	r := rect{0, 100, 400, 280}
	p, _ := a.selectedPawn()
	a.drawPawnLogRows(ebiten.NewImage(800, 600), p, r) // a frame draws before the wheel turns
	x, y, w, h := logBodyRect(r)
	if float32(x) < r.X || float32(y) < r.Y || float32(x+w) > r.right() || float32(y+h) > r.bottom() {
		t.Fatalf("log text area %d,%d %dx%d outside the panel %+v", x, y, w, h, r)
	}
	if !a.scrollLogIn(r, x+5, y+5, -1) || a.logScroll == 0 {
		t.Fatalf("wheel over the log must scroll it (scroll %d)", a.logScroll)
	}
	if a.scrollLogIn(r, int(r.right())+40, int(r.bottom())+40, -1) {
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
	if a.scrollLogAt(100, 300, -1) {
		t.Fatal("no tab open: the wheel is not the log's")
	}
	click(t, a, "inspector.log")
	drawFrame(t, a, f) // the frame draws the Log, which syncs it to the colonist
	x, y, w, h := logBodyRect(a.expandedRect().inset(paneInset))
	if w <= 0 || h <= 0 {
		t.Fatalf("log text area %d,%d %dx%d", x, y, w, h)
	}
	if !a.scrollLogAt(x+5, y+5, -1) || a.logScroll == 0 {
		t.Fatalf("wheel over the open Log must scroll it (scroll %d)", a.logScroll)
	}
	click(t, a, "inspector.needs")
	f.frame(t, a)
	if a.scrollLogAt(x+5, y+5, -1) {
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
		f.frame(t, a)
		withUIScale(t, scale) // after the frames: Update resets the scale from the settings
		tb, ok := paneTabByID(a.pawnPane(2), prisonerTabID)
		if !ok {
			t.Fatal("no Prisoner tab")
		}
		r := innerFor(1280, 720, tb)
		var got []traceEvent
		drawTrace = func(e traceEvent) { got = append(got, e) }
		img := ebiten.NewImage(1920, 1080)
		bodyPaints(a)
		tb.Declare(r)
		a.uiPaint(img)
		drawTrace, tracePass, traceVia = nil, "hud", ""
		if len(got) == 0 {
			t.Fatal("the Prisoner tab drew nothing")
		}
		if bad := inside(got, r, scale); len(bad) > 0 {
			t.Fatalf("the Prisoner tab draws outside its panel %+v (x%v): %s", r, scale, summarize(bad, 6))
		}
		for _, m := range prisonerModes {
			if !hasWidget(a, prisonerModeID(m.mode)) {
				t.Errorf("mode %s missing", m.label)
			}
		}
		if !hasWidget(a, prisonerRelease) {
			t.Error("Release now missing")
		}
	}
}

// The order route is drawn in palette tokens, premultiplied for the vector
// fills (no component exceeds its alpha).
func TestOrderRouteTintIsPremultipliedPaletteToken(t *testing.T) {
	t.Parallel()
	for _, c := range []color.RGBA{uiAccent, uiBtn, uiBtnSelected} {
		got := routeTint(c, 200)
		if got.A != 200 || got.R > got.A || got.G > got.A || got.B > got.A {
			t.Errorf("routeTint(%v, 200) = %v, want a premultiplied colour", c, got)
		}
	}
	if got := routeTint(uiAccent, 255); got != uiAccent {
		t.Errorf("opaque tint = %v, want the token %v", got, uiAccent)
	}
}
