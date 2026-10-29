package ebiten

import (
	"fmt"
	architectpkg "rimworld-go/internal/adapters/inbound/ebiten/architect"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hud"
	inspectorpkg "rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/adapters/inbound/ebiten/options"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	worldpkg "rimworld-go/internal/adapters/inbound/ebiten/render/world"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/adapters/inbound/ebiten/windows"
	"rimworld-go/internal/adapters/inbound/ebiten/wizard"
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// colonyApp starts a paused colony over the stub and runs a frame, so the
// colony's widget hook has declared its widgets.
func colonyApp(t *testing.T, s *stubSim) (*App, *fakeInput) {
	t.Helper()
	if s.w == 0 {
		s.w, s.h = 24, 24
	}
	a := testApp(s)
	a.finishBoot()
	a.startGame()
	if a.nav.screen() != ports.ScreenColony {
		t.Fatalf("screen = %s, want colony", a.nav.screen())
	}
	f := fakeOn(a)
	f.Cur.Mx, f.Cur.My = -50, -50
	f.Frame(t, a)
	a.renderer.speed = speedPaused
	return a, f
}

// tipOf is the hover card of the widget id declared this frame.
func tipOf(t *testing.T, a *App, id string) string {
	t.Helper()
	w, ok := a.uiSt().Find(id)
	if !ok {
		t.Fatalf("no widget %q", id)
	}
	return w.Tip
}

// ---- Factions window ----------------------------------------------------

func factionViews() []game.FactionView {
	return []game.FactionView{
		{ID: 1, Name: "Lynx Reavers", Type: "Freebooters", Leader: "Brand Eastwood", LeaderTitle: "Ringleader", Goodwill: -60,
			Relation: "hostile", Tech: "Industrial", Emblem: "ui_icon/faction_freebooters", Accent: "#d2564a",
			History: []int16{-40, -45, -50, -60}, Events: []string{"Day 2: raid repelled", "Day 3: raid repelled", "Day 4: raid repelled"}},
		{ID: 2, Name: "The Colony", Type: "Colony", Hidden: true, Relation: "friendly"},
		{ID: 3, Name: "Oakwood Commonwealth", Type: "Settlers", Leader: "Perrin Kettering", LeaderTitle: "Steward", Goodwill: 30,
			Relation: "friendly", Settlements: 2, Tech: "Industrial", Accent: "#5b9bd5"},
	}
}

func TestColonyFactionsWindowReadsReadModel(t *testing.T) {
	t.Parallel()
	s := &stubSim{factions: factionViews()}
	a, f := colonyApp(t, s)

	var tab uikit.Button
	for _, b := range a.chrome.Tabs().Buttons(a.sw, a.sh) {
		if b.ID == "tab.factions" {
			tab = b
		}
	}
	if tab.Disabled || tab.Label != "Factions" {
		t.Fatalf("Factions tab = %+v, want enabled", tab)
	}
	press(t, a, f, "tab.factions")
	f.Frame(t, a)
	if a.uiModalTop() != windows.ColFacModalID {
		t.Fatalf("top modal = %q, want %q", a.uiModalTop(), windows.ColFacModalID)
	}
	if !hasWidget(a, wizard.FacTableID+".body") {
		t.Fatal("the window has no faction table")
	}

	rows := colonyFactionRows(a.renderer.game.Factions())
	if len(rows) != 3 { // two visible factions and the Swarm; the colony itself is hidden
		t.Fatalf("rows = %d, want 3", len(rows))
	}
	r := rows[0]
	if r.Name != "Lynx Reavers" || r.Leader != "Ringleader Brand Eastwood" || r.Relation != faction.Hostile || r.Goodwill != -60 {
		t.Fatalf("first row = %+v", r.FacRow)
	}
	if got := fmt.Sprint(r.History); got != "[-40 -45 -50 -60]" || len(r.Events) != 3 {
		t.Fatalf("detail = %v %v", r.History, r.Events)
	}
	if rows[1].Name != "Oakwood Commonwealth" || rows[1].Settlements != 2 || rows[1].Relation != faction.Friendly {
		t.Fatalf("second row = %+v", rows[1].FacRow)
	}
	if last := rows[2]; !last.Swarm || last.Relation != faction.Hostile {
		t.Fatalf("last row = %+v, want the Swarm", last.FacRow)
	}

	// The window follows the read model: a goodwill change shows next frame.
	s.factions[0].Goodwill = -10
	if got := colonyFactionRows(a.renderer.game.Factions())[0].Goodwill; got != -10 {
		t.Fatalf("goodwill = %d, want the live -10", got)
	}
	f.Frame(t, a)

	// Close ends it.
	press(t, a, f, "modal."+windows.ColFacModalID+".close")
	if a.uiSt().ModalOpen() {
		t.Fatal("the window should close")
	}
}

func TestFactionsTabDisabledWithoutFactions(t *testing.T) {
	t.Parallel()
	a, _ := colonyApp(t, &stubSim{})
	for _, b := range a.chrome.Tabs().Buttons(a.sw, a.sh) {
		if b.ID == "tab.factions" && (!b.Disabled || b.Reason == "") {
			t.Fatalf("Factions tab = %+v, want disabled with a reason", b)
		}
	}
}

func TestGoodwillChartPoints(t *testing.T) {
	t.Parallel()
	plot := gfx.Rect{X: 100, Y: 100, W: 590, H: 200}
	pts := windows.ChartPoints([]int{-100, 0, 100}, plot)
	if len(pts) != 3 {
		t.Fatalf("points = %d", len(pts))
	}
	// The last value sits on the right edge, +100 on top, -100 at the bottom.
	if pts[2][0] != plot.Right() || pts[2][1] != plot.Y {
		t.Fatalf("last point = %v, want the top-right corner", pts[2])
	}
	if pts[0][1] != plot.Bottom() || pts[1][1] != plot.Y+plot.H/2 {
		t.Fatalf("points = %v", pts)
	}
	if pts[0][0] >= pts[1][0] || pts[0][0] <= plot.X {
		t.Fatalf("a short history should leave the left empty: %v", pts)
	}
	long := make([]int, 90)
	if p := windows.ChartPoints(long, plot); len(p) != faction.HistoryDays || p[0][0] != plot.X {
		t.Fatalf("a long history keeps the last %d days", faction.HistoryDays)
	}
}

// ---- Bio tab ---------------------------------------------------------------

func bioSim() *stubSim {
	var skills []game.SkillView
	for i := skill.ID(0); i < skill.NumSkills; i++ {
		skills = append(skills, game.SkillView{Key: i.Key(), Label: i.Label(), Level: int(i) + 1, LevelLabel: "Skilled", XPPct: 40, SpeedPct: 110, Affects: "Something", Passion: int(i) % 3})
	}
	skills[skill.Melee] = game.SkillView{Key: "melee", Label: "Melee", LevelLabel: "—", Incapable: true}
	bio := game.BioView{
		First: "Ada", Nick: "Sparks", Last: "Voss", Sex: "Female", Build: "Heavy", AgeYears: 31,
		Childhood: "Herd-watcher", ChildhoodDesc: "Sparks spent her youth with the herds.",
		Adulthood: "Convoy mechanic", AdulthoodDesc: "Sparks kept the convoy running.",
		Traits:     []game.TraitView{{Key: "sunny", Label: "Sunny", Desc: "Sparks is cheerful.", Effects: "Mood +4"}},
		Skills:     skills,
		Incapable:  []string{"Violence (backstory: Convoy mechanic)"},
		Conditions: []string{"Bad back: slower hauling"},
	}
	s := &stubSim{
		pawns: []game.PawnView{{ID: 1, Name: "Ada", X: 5, Y: 5, Status: "alive", Sex: "Female", AgeYears: 31, Title: "Convoy mechanic"}},
		bios:  map[entity.EntityID]game.BioView{1: bio},
	}
	s.learned = map[entity.EntityID][skill.NumSkills]int{1: {skill.Plants: 120}}
	return s
}

func TestBioTabFields(t *testing.T) {
	t.Parallel()
	s := bioSim()
	a, f := colonyApp(t, s)
	a.renderer.Select(1)
	f.Frame(t, a)

	if tab, ok := paneTabByID(a.insp.Pawn().PawnPane(1), "inspector.bio"); !ok || tab.Disabled {
		t.Fatalf("the inspector has no live Bio tab: %+v %v", tab, ok)
	}
	pressButton(t, a, f, "inspector.bio") // the pane's tab strip, the real mouse path
	f.Frame(t, a)
	if a.insp.Tab != "bio" || a.insp.PaneOpenTab() != "inspector.bio" || !a.insp.Tabs().BioTabOn() {
		t.Fatalf("tab = %q (pane %q), want bio", a.insp.Tab, a.insp.PaneOpenTab())
	}

	b := s.bios[1]
	if got := inspectorpkg.BioName(b); got != `Ada "Sparks" Voss` {
		t.Fatalf("name = %q", got)
	}
	if got := inspectorpkg.BioInfoLine(b); got != "Female, age 31, heavy build" {
		t.Fatalf("info = %q", got)
	}
	// Backstories, traits and the incapable list carry their descriptions.
	for id, want := range map[string]string{
		"bio.child":     "Sparks spent her youth",
		"bio.adult":     "kept the convoy running",
		"pawns.trait.0": "Mood +4",
		"bio.incap":     "Violence (backstory: Convoy mechanic)",
	} {
		if tip := tipOf(t, a, id); !strings.Contains(tip, want) {
			t.Errorf("%s tip = %q, want %q", id, tip, want)
		}
	}
	// Every skill has a row card; the learning shows as this week's gain.
	if got := tipOf(t, a, fmt.Sprintf("bio.skill.%d", skill.Plants)); !strings.Contains(got, "Learned 120 experience") || !strings.Contains(got, "40% of the way") {
		t.Errorf("Plants tip = %q", got)
	}
	if got := tipOf(t, a, fmt.Sprintf("bio.skill.%d", skill.Melee)); !strings.Contains(got, "Incapable") {
		t.Errorf("Melee tip = %q, want incapable", got)
	}
	if got := bioSkillTip(b.Skills[skill.Mining], 0); !strings.Contains(got, "Nothing learned") {
		t.Errorf("idle tip = %q", got)
	}
	if inspectorpkg.WeekText(0) != "·" || inspectorpkg.WeekText(35) != "+35" {
		t.Errorf("weekText = %q %q", inspectorpkg.WeekText(0), inspectorpkg.WeekText(35))
	}
	short, full := inspectorpkg.BioIncapable(b)
	if short != "Violence" || !strings.Contains(full, "Convoy mechanic") {
		t.Errorf("incapable = %q / %q", short, full)
	}
	if s, _ := inspectorpkg.BioIncapable(game.BioView{}); s != "None" {
		t.Errorf("no incapabilities = %q", s)
	}

	// A colonist without a bio has the tab disabled.
	s.pawns[0].Sex = ""
	if tab, ok := paneTabByID(a.insp.Pawn().PawnPane(1), "inspector.bio"); !ok || !tab.Disabled || tab.Reason == "" {
		t.Fatalf("Bio tab = %+v (%v), want disabled with a reason", tab, ok)
	}
	if b, ok := hasButton(a.insp.PaneButtons(), "inspector.bio"); !ok || !b.Disabled || b.Reason == "" {
		t.Fatalf("Bio tab button = %+v (%v), want disabled with a reason", b, ok)
	}
	if a.insp.Tabs().BioTabOn() {
		t.Fatal("the Bio tab can't be on for a colonist without one")
	}
}

func TestColonistBarTooltipAgeAndTitle(t *testing.T) {
	t.Parallel()
	a, _ := colonyApp(t, bioSim())
	var tip string
	for _, b := range a.chrome.Bar().Buttons(a.sw, a.sh) {
		tip = b.Tip
	}
	if tip != "Ada, 31, Convoy mechanic" {
		t.Fatalf("bar tip = %q", tip)
	}
	if got := hud.BarTip(game.PawnView{ID: 4}); got != "Colonist 4" {
		t.Fatalf("legacy bar tip = %q", got)
	}
}

// ---- Work tab --------------------------------------------------------------

func TestWorkTabIncapableCells(t *testing.T) {
	t.Parallel()
	const why = "Incapable: Hauling (backstory: Convoy brat)"
	s := &stubSim{pawns: []game.PawnView{{ID: 1, Name: "Ada", X: 5, Y: 5, Status: "alive",
		Incapable: work.Mask(0).Set(work.Haul, true), Priorities: []int{0, 3, 3, 3, 3, 3, 0, 3, 3, 3, 3, 3}}}}
	s.pawns[0].SkillLv[skill.Plants], s.pawns[0].Passion[skill.Plants] = 12, 2
	s.incapable = map[entity.EntityID]map[work.Type]string{1: {work.Haul: why}}
	a, f := colonyApp(t, s)
	click(t, a, "tab.work")
	f.Frame(t, a)
	if !a.win.Open {
		t.Fatal("the Work tab should open")
	}

	haul, grow := "work.cell.1.6", "work.cell.1.2"
	byID := map[string]uikit.Button{}
	for _, b := range a.win.Buttons() {
		byID[b.ID] = b
	}
	if b := byID[haul]; !b.Disabled || b.Reason != why {
		t.Fatalf("incapable cell = %+v, want disabled with %q", b, why)
	}
	if byID[grow].Disabled {
		t.Fatal("a capable cell must stay clickable")
	}

	// Clicking the incapable cell changes nothing; a capable one cycles.
	click(t, a, haul)
	if len(cmdsOf(s, command.SetWorkPriority)) != 0 {
		t.Fatalf("incapable cell emitted %+v", s.cmds)
	}
	if a.hint != why {
		t.Fatalf("hint = %q, want the reason", a.hint)
	}
	click(t, a, grow)
	if cs := cmdsOf(s, command.SetWorkPriority); len(cs) != 1 || cs[0].Count != 4 {
		t.Fatalf("capable cell emitted %+v, want priority 4", s.cmds)
	}

	// Tooltips: the reason, the skill and passion of a live cell, the header.
	f.Frame(t, a)
	if got := tipOf(t, a, "work.tip.1.6"); got != why {
		t.Errorf("incapable tip = %q", got)
	}
	if got := tipOf(t, a, "work.tip.1.2"); !strings.Contains(got, "Plants level 12, major passion") || !strings.Contains(got, "Priority 3") {
		t.Errorf("live tip = %q", got)
	}
	if got := tipOf(t, a, "work.hdr.4"); !strings.Contains(got, "Skill: Construction") {
		t.Errorf("header tip = %q", got)
	}
	for col := range windows.WorkColumns { // every header names its work
		if windows.WorkHelp[col] == "" {
			t.Errorf("column %d has no header card", col)
		}
	}
}

// ---- Options > Story ---------------------------------------------------------

func storySim() *stubSim {
	s := &stubSim{pawns: []game.PawnView{{ID: 1, Name: "Ada", X: 5, Y: 5, Status: "alive"}}}
	s.story = game.StoryView{Storyteller: "Orla Vane", Title: "the Steady Hand", Preset: "Settled", Difficulty: difficulty.Of(difficulty.Settled)}
	return s
}

func openStory(t *testing.T, s *stubSim) (*App, *fakeInput) {
	t.Helper()
	a, f := colonyApp(t, s)
	a.opt.Tab = "story"
	a.apply(intent{kind: intentPush, screen: ports.ScreenOptions})
	f.Frame(t, a)
	if a.opt.CurrentTab() != "story" {
		t.Fatalf("tab = %q, want story", a.opt.CurrentTab())
	}
	return a, f
}

func TestOptionsStoryTabSendsCommands(t *testing.T) {
	t.Parallel()
	s := storySim()
	a, f := openStory(t, s)

	// A storyteller card sends SetStoryteller with the profile's key.
	press(t, a, f, "opt.story.pick.hearth")
	if cs := cmdsOf(s, command.SetStoryteller); len(cs) != 1 || cs[0].Name != "hearth" {
		t.Fatalf("storyteller commands = %+v, want hearth", cs)
	}
	// A preset sends SetDifficulty with its key.
	press(t, a, f, "opt.diff.grim")
	if cs := cmdsOf(s, command.SetDifficulty); len(cs) != 1 || cs[0].Name != "grim" {
		t.Fatalf("difficulty commands = %+v, want grim", cs)
	}

	// Custom opens the dialog on the current values; Accept sends them.
	press(t, a, f, options.StCustomID)
	f.Frame(t, a)
	if a.uiModalTop() != options.StCustomMod {
		t.Fatalf("top modal = %q, want the custom dialog", a.uiModalTop())
	}
	press(t, a, f, "modal."+options.StCustomMod+".accept")
	cs := cmdsOf(s, command.SetDifficulty)
	if len(cs) != 2 || !strings.HasPrefix(cs[1].Name, "custom:") {
		t.Fatalf("difficulty commands = %+v, want a custom one", cs)
	}
	got, err := difficulty.Decode(strings.TrimPrefix(cs[1].Name, "custom:"))
	if err != nil || got != difficulty.Of(difficulty.Settled) {
		t.Fatalf("custom values = %+v (%v), want the current Settled values", got, err)
	}
	if a.uiSt().ModalOpen() {
		t.Fatal("Accept should close the dialog")
	}
}

func TestOptionsStoryTabSaveModeReadOnly(t *testing.T) {
	t.Parallel()
	s := storySim()
	s.story.Commitment = true
	a, f := openStory(t, s)
	f.Frame(t, a)
	for _, id := range []string{"save.reload", "save.commit"} {
		if hasWidget(a, id) {
			t.Errorf("save mode %q must not be a control in play", id)
		}
	}
	if got := options.StorySummary(s.story); len(got) < 2 || !strings.Contains(got[0], "Orla Vane") {
		t.Fatalf("summary = %v", got)
	}
}

func TestOptionsStoryTabOnlyInColony(t *testing.T) {
	t.Parallel()
	a := testApp(&stubSim{w: 8, h: 8})
	a.finishBoot()
	for _, tab := range a.opt.TabList() {
		if tab == "story" {
			t.Fatal("the menu's Options must not offer Story")
		}
	}
	a.opt.Tab = "story"
	if a.opt.CurrentTab() == "story" {
		t.Fatal("no colony: the story tab can't be current")
	}
	b, _ := colonyApp(t, storySim())
	if tabs := b.opt.TabList(); !strings.Contains(strings.Join(tabs, ","), "story") {
		t.Fatalf("colony tabs = %v, want story among them", tabs)
	}
}

func TestOptionsStoryTabLegacyColonyReadOnly(t *testing.T) {
	t.Parallel()
	a, f := openStory(t, &stubSim{pawns: []game.PawnView{{ID: 1, Name: "Ada", Status: "alive"}}})
	f.Frame(t, a)
	if hasWidget(a, "opt.story.pick.steady") || hasWidget(a, "opt.diff.grim") {
		t.Fatal("a classic-start colony has no storyteller to change")
	}
}

// ---- Letters ----------------------------------------------------------------

// letterSim is a colony whose scenario intro letter (the stack's
// WorldView.Letters, PLAY-018) arrived at tick 5, with its message line.
func letterSim() *stubSim {
	s := &stubSim{pawns: []game.PawnView{{ID: 1, Name: "Ada", Status: "alive"}}, tick: 10}
	s.world.Messages = []game.MessageView{{Tick: 5, Text: "The lifeboats strike the ground.", Letter: true, Title: "Broken Hull"}}
	s.world.Letters = []game.LetterView{{ID: 1, Tick: 5, Kind: "info", Title: "Broken Hull", Text: "The lifeboats strike the ground.", Intro: true, X: -1, Y: -1}}
	return s
}

func TestIntroLetterShownOnce(t *testing.T) {
	t.Parallel()
	s := letterSim()
	a, f := colonyApp(t, s)
	if !strings.HasPrefix(a.uiModalTop(), "letter.") {
		t.Fatalf("top modal = %q, want the intro letter", a.uiModalTop())
	}
	if len(a.uiSt().Modals) != 1 || a.uiSt().Modals[0].Title != "Broken Hull" {
		t.Fatalf("modals = %+v", a.uiSt().Modals)
	}
	press(t, a, f, "modal."+a.uiModalTop()+".continue")
	if a.uiSt().ModalOpen() {
		t.Fatal("Continue should close the letter")
	}
	for i := 0; i < 5; i++ {
		f.Frame(t, a)
	}
	if a.uiSt().ModalOpen() {
		t.Fatal("the letter must not open again")
	}

	// A new letter joins the stack and waits there (only the intro opens by
	// itself, PLAY-018); a colony started later shows its own intro.
	s.world.Messages = append(s.world.Messages, game.MessageView{Tick: 8, Text: "A heat wave has settled.", Letter: true, Title: "Heat wave"})
	s.world.Letters = append(s.world.Letters, game.LetterView{ID: 2, Tick: 8, Kind: "caution", Title: "Heat wave", Text: "A heat wave has settled.", X: -1, Y: -1})
	f.Frame(t, a)
	if a.uiSt().ModalOpen() {
		t.Fatalf("modals = %+v: a new letter must wait in the stack", a.uiSt().Modals)
	}
	if n := len(a.letterRows(a.hud())); n != 2 {
		t.Fatalf("the stack holds %d letters, want the intro and the heat wave", n)
	}
	a.uiCloseTop()
	a.dropSession()
	a.startSession(s)
	f.Frame(t, a)
	if !a.uiSt().ModalOpen() {
		t.Fatal("a new colony should show its own intro letter")
	}
}

func TestOldLettersAreNotReopened(t *testing.T) {
	t.Parallel()
	s := letterSim()
	s.tick = 5 + hud.LetterFresh + 1 // a loaded save's saved letter
	a, f := colonyApp(t, s)
	f.Frame(t, a)
	if a.uiSt().ModalOpen() {
		t.Fatalf("stale letter opened: %+v", a.uiSt().Modals)
	}
	if got := hud.MessageLine(s.world.Messages[0]); got != "Letter: Broken Hull" {
		t.Fatalf("message strip line = %q, want the letter's title", got)
	}
	if got := hud.MessageLine(game.MessageView{Text: "12 meat rotted away"}); got != "12 meat rotted away" {
		t.Fatalf("plain message = %q", got)
	}
}

// ---- Architect ----------------------------------------------------------------

func lockedCatalog(kind string) []game.BuildableView {
	cat := game.Buildables()
	for i := range cat {
		if cat[i].Kind == kind {
			cat[i].Locked = game.ReasonScenario
		}
	}
	return cat
}

func TestArchitectLockedByScenario(t *testing.T) {
	t.Parallel()
	s := &stubSim{buildables: lockedCatalog("campfire")}
	a, _ := colonyApp(t, s)
	a.arch.OpenPanel()
	a.arch.SelectCategory("furniture")

	var fire, lamp uikit.Button
	for _, b := range a.insp.Commands().CommandGridButtons() { // the tools are command buttons
		switch b.ID {
		case "arch.tool.campfire":
			fire = b
		case "arch.tool.lamp":
			lamp = b
		}
	}
	if !fire.Disabled || fire.Reason != game.ReasonScenario {
		t.Fatalf("campfire card = %+v, want disabled: %s", fire, game.ReasonScenario)
	}
	if lamp.Disabled {
		t.Fatalf("lamp card = %+v, want enabled", lamp)
	}
	click(t, a, "arch.tool.campfire")
	if a.arch.ToolID != "" || a.hint != game.ReasonScenario {
		t.Fatalf("tool=%q hint=%q: a locked card must not select and says why", a.arch.ToolID, a.hint)
	}
	click(t, a, "arch.tool.lamp")
	if a.arch.ToolID != "lamp" {
		t.Fatalf("tool = %q, want lamp", a.arch.ToolID)
	}
	if architectpkg.LockWord(fire.Reason) != "locked" || architectpkg.LockWord("Needs research: Fabrication") != "needs research" {
		t.Fatal("lockWord")
	}
}

func TestNewBuildingCards(t *testing.T) {
	t.Parallel()
	cab, ok1 := toolByID("tool_cabinet")
	bed, ok2 := toolByID("hospital_bed")
	if !ok1 || !ok2 {
		t.Fatal("the Architect needs a Tool cabinet and a Hospital bed card")
	}
	if cab.icon != "tool_cabinet" || bed.icon != "hospital_bed" || cab.build != "tool cabinet" || bed.build != "hospital bed" {
		t.Fatalf("cards = %+v %+v", cab, bed)
	}
	if _, ok := architectpkg.KindID("tool cabinet"); !ok {
		t.Fatal("the cabinet is not in the catalog")
	}
	if got := worldpkg.BuildSprite("tool cabinet", "south", 0, false, false); got != "furniture/tool_cabinet" {
		t.Fatalf("cabinet sprite = %q", got)
	}
	if got := worldpkg.BuildSprite("hospital bed", "east", 0, false, false); got != "furniture/hospital_bed_east" {
		t.Fatalf("bed sprite = %q", got)
	}
	// The tooltips carry the component cost beside the material.
	cabTip := strings.Join(archTooltipLines(cab, "steel", 0, 0), "\n")
	if !strings.Contains(cabTip, "Cost: 30 steel + 1 component ") {
		t.Errorf("cabinet tip = %q", cabTip)
	}
	bedTip := strings.Join(archTooltipLines(bed, "wood", 100, 0), "\n")
	if !strings.Contains(bedTip, "Cost: 45 wood + 2 components") {
		t.Errorf("bed tip = %q", bedTip)
	}
	if wall, _ := toolByID("wall"); strings.Contains(strings.Join(archTooltipLines(wall, "wood", 100, 0), ""), "component") {
		t.Error("a wall needs no components")
	}

	// They sit in their categories, each card reachable.
	a, _ := colonyApp(t, &stubSim{})
	a.arch.OpenPanel()
	for cat, id := range map[string]string{"production": "tool_cabinet", "furniture": "hospital_bed"} {
		a.arch.SelectCategory(cat)
		found := false
		for _, b := range a.insp.Commands().CommandGridButtons() { // the tools are command buttons
			found = found || b.ID == "arch.tool."+id
		}
		if !found {
			t.Errorf("no %s card in %s", id, cat)
		}
	}
	// The longest category still fits: every tool has a grid cell, and the
	// Architect (categories, description, grid) ends above the tab strip.
	a.arch.SelectCategory("furniture")
	l := a.hud()
	if n := len(a.archToolCommands()); len(l.GridCells) != n {
		t.Errorf("furniture has %d tools but the grid shows %d", n, len(l.GridCells))
	}
	if r := a.arch.Rect(); r.Y+r.H > l.Tabs.Y {
		t.Errorf("architect panel ends at %v, below the tab strip at %v", r.Y+r.H, l.Tabs.Y)
	}
	if l.Grid.Bottom() > l.Tabs.Y {
		t.Errorf("the tool grid ends at %v, below the tab strip at %v", l.Grid.Bottom(), l.Tabs.Y)
	}
}

func TestNewBuildingArtExists(t *testing.T) {
	c := realArt(t)
	for _, key := range []string{"ui_icon/tool_cabinet", "ui_icon/hospital_bed", "furniture/tool_cabinet", "furniture/hospital_bed_south"} {
		if !c.Has(key) {
			t.Errorf("art %q missing", key)
		}
	}
}

// ---- Status chips --------------------------------------------------------------

func TestStatusChipsFromWorld(t *testing.T) {
	t.Parallel()
	w := game.WorldView{Extreme: "heat_wave", Conditions: []string{"ashfall_sky", "nonsense"}}
	chips := statusChips(w)
	if len(chips) != 2 || chips[0].label != "Heat wave" || chips[1].label != "Ashfall sky" {
		t.Fatalf("chips = %+v", chips)
	}
	if !strings.Contains(chips[1].tip, "-20% light") {
		t.Fatalf("condition tip = %q", chips[1].tip)
	}
	if cold := statusChips(game.WorldView{Extreme: "cold_snap"}); len(cold) != 1 || cold[0].label != "Cold snap" {
		t.Fatalf("cold snap chips = %+v", cold)
	}
	if len(statusChips(game.WorldView{})) != 0 {
		t.Fatal("a calm world has no chips")
	}

	s := &stubSim{pawns: []game.PawnView{{ID: 1, Name: "Ada", Status: "alive"}}, world: w}
	a, f := colonyApp(t, s)
	f.Frame(t, a)
	rs := a.chipRects(chips, a.sw)
	if g := a.hud().Global; len(rs) != 2 || rs[0].Right() != g.Right() || rs[1].Y <= rs[0].Y || rs[0].Y < g.Y || rs[1].Bottom() > g.Bottom() {
		t.Fatalf("chip rects = %+v, want a right-aligned stack inside Global %+v", rs, g)
	}
	if got := tipOf(t, a, "hud.chip.1"); !strings.Contains(got, "Ashfall sky") {
		t.Fatalf("chip tip = %q", got)
	}
}
