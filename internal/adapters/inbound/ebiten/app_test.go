package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/mainmenu"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	wizardpkg "rimworld-go/internal/adapters/inbound/ebiten/wizard"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

type stubSim struct {
	cmds   []command.Command
	pawns  []game.PawnView
	orders []game.OrderOption
	steps  int
	w, h   int
	tick   clock.Tick
	wild   []game.WildlifeRow
	world  game.WorldView
	// NG-1 colony UI read models (nil/zero: none).
	factions   []game.FactionView
	story      game.StoryView
	bios       map[entity.EntityID]game.BioView
	incapable  map[entity.EntityID]map[work.Type]string
	learned    map[entity.EntityID][skill.NumSkills]int
	buildables []game.BuildableView
	// ENG-003 social and host-system read models (nil/zero: none).
	social    map[entity.EntityID]game.SocialView
	health    map[entity.EntityID]game.HealthView
	prisoners map[entity.EntityID]game.PrisonerView
	worldMap  game.WorldMapView
	areas     []game.AreaView
	history   []game.HistoryEntryView
	// growth is each plant's growth state (PlantGrowth; absent: none).
	growth map[entity.EntityID]game.PlantGrowthView
	// onStep, when set, runs after each tick (budget tests advance a fake clock).
	onStep func()
}

func (s *stubSim) PlantGrowth(id entity.EntityID) (game.PlantGrowthView, bool) {
	v, ok := s.growth[id]
	return v, ok
}

func (s *stubSim) Social(id entity.EntityID) (game.SocialView, bool) {
	v, ok := s.social[id]
	return v, ok
}
func (s *stubSim) Health(id entity.EntityID) (game.HealthView, bool) {
	v, ok := s.health[id]
	return v, ok
}
func (s *stubSim) Prisoner(id entity.EntityID) (game.PrisonerView, bool) {
	v, ok := s.prisoners[id]
	return v, ok
}
func (s *stubSim) WorldMap() game.WorldMapView      { return s.worldMap }
func (s *stubSim) Areas() []game.AreaView           { return s.areas }
func (s *stubSim) History() []game.HistoryEntryView { return s.history }

func (s *stubSim) Step(n int) {
	for range n {
		s.steps++
		s.tick++
		if s.onStep != nil {
			s.onStep()
		}
	}
}

func (s *stubSim) Snapshot() []game.PawnView  { return s.pawns }
func (s *stubSim) Width() int                 { return s.w }
func (s *stubSim) TemperatureAt(x, y int) int { return 150 }
func (s *stubSim) Height() int                { return s.h }
func (s *stubSim) Tick() clock.Tick           { return s.tick }
func (s *stubSim) Date() clock.Date           { return clock.DateAt(s.tick) }
func (s *stubSim) Activity() schedule.Activity {
	return schedule.Work
}
func (s *stubSim) World() game.WorldView        { return s.world }
func (s *stubSim) Wildlife() []game.WildlifeRow { return s.wild }
func (s *stubSim) Factions() []game.FactionView { return s.factions }
func (s *stubSim) Story() game.StoryView        { return s.story }
func (s *stubSim) Bio(id entity.EntityID) (game.BioView, bool) {
	b, ok := s.bios[id]
	return b, ok
}
func (s *stubSim) Buildables() []game.BuildableView {
	if s.buildables != nil {
		return s.buildables
	}
	return game.Buildables()
}
func (s *stubSim) IncapableReason(id entity.EntityID, t work.Type) string {
	return s.incapable[id][t]
}
func (s *stubSim) LearnedThisWeek(id entity.EntityID) [skill.NumSkills]int { return s.learned[id] }
func (s *stubSim) Apply(command.Command)                                   {}
func (s *stubSim) Enqueue(c command.Command)                               { s.cmds = append(s.cmds, c) }
func (s *stubSim) CanPlaceAt(kind, stuff, facing string, x, y int) (bool, string) {
	if x < 0 || y < 0 || x >= s.w || y >= s.h {
		return false, "outside the map"
	}
	return true, ""
}
func (s *stubSim) ReplacesAt(kind, stuff string, x, y int) string    { return "" }
func (s *stubSim) AutoFacingAt(kind, facing string, x, y int) string { return facing }
func (s *stubSim) OrdersAt(pawn entity.EntityID, x, y int) []game.OrderOption {
	return s.orders
}
func (s *stubSim) CanZoneAt(kind string, x, y int) bool {
	return x >= 0 && y >= 0 && x < s.w && y < s.h
}

func testApp(sim Simulation) *App {
	return NewApp(AppConfig{
		Version: "test",
		LoadArt: func() (*art.Catalog, error) {
			return nil, nil
		},
		LoadSplash: func() (*ebiten.Image, error) {
			return nil, nil
		},
		LoadMenuScene: func() (*mainmenu.Diorama, error) {
			return nil, nil
		},
		NewSim: func(NewGame) (Simulation, error) {
			return sim, nil
		},
	})
}

// settleUI runs one Update on a menu screen (with a fake input if the test
// has none) so the screen's kit widgets are declared and clickable by id.
func settleUI(t *testing.T, a *App) {
	t.Helper()
	if s := a.nav.screen(); s == ports.ScreenBoot || s == ports.ScreenColony {
		return
	}
	if a.src == nil {
		fakeOn(a)
	}
	if _, ok := a.src.(*fakeInput); !ok {
		return
	}
	if err := a.Update(); err != nil {
		t.Fatal(err)
	}
}

// click presses the HUD button or kit widget id with the mouse, the path a
// player's click takes (QA-001): one Update lays the screen out, then a
// press and a release at the element's centre (two more Updates), then
// the cursor parks off the window. A disabled element only sets its
// reason as the hint, as the dispatcher would. Tests that must not run
// those Updates (a colony that may not step) pause first; a test that
// deliberately asserts on the dispatch alone uses clickDirect.
func click(t *testing.T, a *App, id string) {
	t.Helper()
	f := fakeFor(a)
	defer holdColony(a, f, id)()
	// A test that placed the pointer on a tile by hand (clickTile) leaves
	// the override behind; the press must land on the element.
	a.cursor = nil
	f.Frame(t, a)
	x, y, disabled, reason, ok := elementAt(a, id)
	if !ok {
		t.Fatalf("no button or widget %q on %s", id, a.nav.screen())
	}
	if disabled {
		a.hint = reason
		return
	}
	s := float32(a.scale.S)
	f.Cur.Mx, f.Cur.My = int(x*s), int(y*s)
	f.Cur.Press[inputframe.Left], f.Cur.Down[inputframe.Left] = true, true
	quitFrame(t, a)
	f.Cur.Down[inputframe.Left], f.Cur.Release[inputframe.Left] = false, true
	quitFrame(t, a)
	f.Cur.Mx, f.Cur.My = -50, -50
}

// holdColony keeps a colony from stepping during click's three Updates, so
// a test that reads a tick count or a pawn's tile around a click keeps its
// meaning; the returned func lifts the hold.
//
// On the colony screen the hold is the pause itself, with the learn
// helper's baseline moved along so it never reads the hold as the player's
// own "Control time" (learnWatch) and saves a lesson. The speed comes back
// unless the click changed it; a dialog that opened under the hold keeps
// the game paused and resumes the held speed when it closes, as
// dealingsPause would have done. A speed control is not held at all: its
// click acts on the speed the player sees (pause toggles).
//
// Elsewhere (the wizard's Start, Continue) the colony is born inside the
// press Update, so the window counts as blurred for the click instead:
// background.go then skips the tick (a test with Run in background on
// steps as it would). That hold is not used on the colony screen because
// the "paused (in background)" reason widens the global bar and folds the
// learn helper on small logical screens.
func holdColony(a *App, f *fakeInput, id string) func() {
	r := a.renderer
	if r == nil || a.nav.screen() != ports.ScreenColony {
		blurred := f.Cur.Blurred
		f.Cur.Blurred = true
		return func() {
			f.Cur.Blurred, a.in.Raw.Blurred = blurred, blurred
			a.syncBackground() // a.tick() right after the click sees the focus back
		}
	}
	if r.speed == speedPaused || strings.HasPrefix(id, "speed.") {
		return func() {}
	}
	was := r.speed
	r.speed = speedPaused
	a.lrn.SetBaseSpeed(int(speedPaused))
	return func() {
		if a.renderer != r || r.speed != speedPaused {
			return
		}
		if a.deal.Open() {
			a.deal.HoldPause(int(was))
			return
		}
		r.speed = was
		a.lrn.SetBaseSpeed(int(was))
	}
}

// quitFrame is one Update that may end the game (a Quit button clicked):
// ebiten.Termination is the expected result then, not a failure.
func quitFrame(t *testing.T, a *App) {
	t.Helper()
	if err := a.Update(); err != nil && err != ebiten.Termination {
		t.Fatal(err)
	}
}

// clickDirect dispatches id without the mouse (click's old shortcut): only
// for tests of the dispatcher itself, never to reach a feature.
func clickDirect(t *testing.T, a *App, id string) {
	t.Helper()
	settleUI(t, a)
	for _, b := range a.layoutHits(a.sw, a.sh) {
		if b.ID != id {
			continue
		}
		if b.Disabled {
			a.hint = b.Reason
			return
		}
		a.applyClick(id)
		return
	}
	t.Fatalf("no button %q on %s", id, a.nav.screen())
}

// fakeFor is the App's fake input, attached if the test has none.
func fakeFor(a *App) *fakeInput {
	if f, ok := a.src.(*fakeInput); ok {
		return f
	}
	return fakeOn(a)
}

// elementAt is the logical centre of the HUD button or kit widget id this
// frame, with a button's disabled state.
func elementAt(a *App, id string) (x, y float32, disabled bool, reason string, ok bool) {
	sw, sh := a.size() // the size Update hit-tests with (a test may never Layout)
	for _, b := range a.layoutHits(sw, sh) {
		if b.ID == id {
			return b.X + b.W/2, b.Y + b.H/2, b.Disabled, b.Reason, true
		}
	}
	if w, found := a.uiSt().Find(id); found {
		x, y = w.R.Center()
		return x, y, false, "", true
	}
	return 0, 0, false, "", false
}

func TestBootTicksToMainMenuWithoutSim(t *testing.T) {
	t.Parallel()
	sim := &stubSim{w: 8, h: 8}
	a := testApp(sim)
	if a.nav.screen() != ports.ScreenBoot {
		t.Fatalf("start screen = %s, want boot", a.nav.screen())
	}
	for i := 0; i < bootFramesPerPhase*len(bootPhases)+1; i++ {
		a.tick()
	}
	if a.nav.screen() != ports.ScreenMainMenu {
		t.Fatalf("after boot = %s, want main_menu", a.nav.screen())
	}
	if a.simsStarted != 0 || a.renderer != nil {
		t.Fatalf("boot must not create a simulation")
	}
	if sim.steps != 0 {
		t.Fatalf("menu must not step, got %d", sim.steps)
	}
}

func TestNewGameWizardAndStart(t *testing.T) {
	t.Parallel()
	sim := &stubSim{w: 8, h: 8}
	a := testApp(sim)
	a.finishBoot()

	want := []struct {
		id  string
		scr ports.Screen
	}{
		{"menu.newgame", ports.ScreenScenario},
		{"wizard.next", ports.ScreenStoryteller},
		{"wizard.next", ports.ScreenWorldGen},
	}
	for _, step := range want {
		click(t, a, step.id)
		if a.nav.screen() != step.scr {
			t.Fatalf("after %s = %s, want %s", step.id, a.nav.screen(), step.scr)
		}
	}
	// Create World generates (a tiny world) and arrives at the landing
	// site screen; Next then needs a chosen site.
	ng := a.wiz.Draft()
	ng.World.SeedText, ng.World.Coverage = tinySeed, 5
	click(t, a, "world.generate")
	runUntil(t, a, func() bool { return a.nav.screen() == ports.ScreenLanding })
	click(t, a, "land.random")
	click(t, a, "wizard.next")
	if a.nav.screen() != ports.ScreenPawns {
		t.Fatalf("after landing Next = %s, want pawns", a.nav.screen())
	}
	click(t, a, "wizard.start")
	if a.nav.screen() != ports.ScreenColony {
		t.Fatalf("after start = %s, want colony", a.nav.screen())
	}
	if a.simsStarted != 1 || a.renderer == nil {
		t.Fatalf("start must construct a renderer, sims=%d renderer=%v", a.simsStarted, a.renderer != nil)
	}
	if sim.steps != 0 {
		t.Fatalf("start itself must not step, got %d", sim.steps)
	}
}

func TestDisabledActionsDoNotNavigate(t *testing.T) {
	t.Parallel()
	a := testApp(&stubSim{w: 4, h: 4})
	a.finishBoot()
	click(t, a, "menu.continue")
	if a.nav.screen() != ports.ScreenMainMenu {
		t.Fatalf("continue navigated to %s", a.nav.screen())
	}
	if a.hint != "no saves yet" {
		t.Fatalf("hint = %q, want no saves yet", a.hint)
	}
	click(t, a, "menu.plugins")
	if a.nav.screen() != ports.ScreenMainMenu {
		t.Fatalf("plugins navigated to %s", a.nav.screen())
	}
}

func TestBackAndEscapeUnwind(t *testing.T) {
	t.Parallel()
	a := testApp(&stubSim{w: 4, h: 4})
	a.finishBoot()
	click(t, a, "menu.newgame")
	click(t, a, "wizard.next")
	a.apply(a.nav.escapeIntent())
	if a.nav.screen() != ports.ScreenScenario {
		t.Fatalf("escape = %s, want scenario", a.nav.screen())
	}
	click(t, a, "wizard.back")
	if a.nav.screen() != ports.ScreenMainMenu {
		t.Fatalf("back = %s, want main_menu", a.nav.screen())
	}
}

func TestPauseBlocksSteppingAndResumeContinues(t *testing.T) {
	t.Parallel()
	sim := &stubSim{w: 8, h: 8}
	a := testApp(sim)
	a.finishBoot()
	quickStart(t, a)

	a.tick()
	if sim.steps == 0 {
		t.Fatal("colony tick should step")
	}

	a.apply(a.nav.escapeIntent())
	if a.nav.overlay() != overlayPause {
		t.Fatalf("overlay = %s, want pause", a.nav.overlay())
	}
	pausedAt := sim.steps
	a.tick()
	if sim.steps != pausedAt {
		t.Fatalf("pause stepped: %d -> %d", pausedAt, sim.steps)
	}

	click(t, a, "pause.resume")
	if a.nav.overlay() != overlayNone {
		t.Fatalf("resume overlay = %s", a.nav.overlay())
	}
	a.tick()
	if sim.steps == pausedAt {
		t.Fatal("resume should step")
	}
}

func TestReturnToMenuDropsSim(t *testing.T) {
	t.Parallel()
	sim := &stubSim{w: 8, h: 8}
	a := testApp(sim)
	a.finishBoot()
	quickStart(t, a)
	a.apply(intent{kind: intentPause})
	click(t, a, "pause.menu")
	if a.nav.screen() != ports.ScreenMainMenu {
		t.Fatalf("return = %s, want main_menu", a.nav.screen())
	}
	if a.renderer != nil {
		t.Fatal("return to menu must drop the renderer")
	}
	a.tick()
	if sim.steps != 0 {
		t.Fatalf("headless menu stepped %d", sim.steps)
	}
	quickStart(t, a)
	if a.simsStarted != 2 {
		t.Fatalf("second start simsStarted=%d, want 2", a.simsStarted)
	}
}

func TestOptionsFromPauseReturnsToPause(t *testing.T) {
	t.Parallel()
	a := testApp(&stubSim{w: 4, h: 4})
	a.finishBoot()
	quickStart(t, a)
	a.apply(intent{kind: intentPause})
	click(t, a, "pause.options")
	if a.nav.screen() != ports.ScreenOptions {
		t.Fatalf("options = %s", a.nav.screen())
	}
	click(t, a, "options.back")
	if a.nav.screen() != ports.ScreenColony || a.nav.overlay() != overlayPause {
		t.Fatalf("back from options = %s/%s, want colony/pause", a.nav.screen(), a.nav.overlay())
	}
}

func TestQuitFromMenu(t *testing.T) {
	t.Parallel()
	a := testApp(&stubSim{w: 4, h: 4})
	a.finishBoot()
	click(t, a, "menu.quit")
	if !a.shouldQuit {
		t.Fatal("quit should set shouldQuit")
	}
}

func TestLoadAndCreditsNavigate(t *testing.T) {
	t.Parallel()
	a := testApp(&stubSim{w: 4, h: 4})
	a.finishBoot()
	click(t, a, "menu.load")
	if a.nav.screen() != ports.ScreenLoad {
		t.Fatalf("load = %s", a.nav.screen())
	}
	click(t, a, "load.back")
	click(t, a, "menu.credits")
	if a.nav.screen() != ports.ScreenCredits {
		t.Fatalf("credits = %s", a.nav.screen())
	}
	if lines := mainmenu.CreditsLines(); len(lines) == 0 || lines[0] != "Created by arxdsilva" {
		t.Fatalf("credits open with %q, want the creator first", lines)
	}
	click(t, a, "credits.back")
	if a.nav.screen() != ports.ScreenMainMenu {
		t.Fatalf("credits back = %s", a.nav.screen())
	}
}

func TestScenarioSelectionStaysOnScreen(t *testing.T) {
	t.Parallel()
	a := testApp(&stubSim{w: 4, h: 4})
	a.finishBoot()
	click(t, a, "menu.newgame")
	click(t, a, "scen.row.gilded_expedition") // the third built-in
	if sel := a.wiz.Draft().Selected(); sel == nil || sel.Key != "gilded_expedition" || a.nav.screen() != ports.ScreenScenario {
		t.Fatalf("select the third scenario: sel=%+v screen=%s", a.wiz.Draft().Selected(), a.nav.screen())
	}
}

// The setup screens' columns must not run under each other (the intent of
// the old left-rail body origin): the scenario list clears the details,
// the storyteller cards clear the centre and the difficulty column, and
// everything sits inside the dialog's content.
func TestBodyOriginClearsLeftRail(t *testing.T) {
	t.Parallel()
	a := testApp(&stubSim{w: 4, h: 4})
	a.finishBoot()
	click(t, a, "menu.newgame")

	for _, size := range [][2]int{{1280, 720}, {1920, 1080}} {
		_, _, content, _ := uikit.DialogFrame(size[0], size[1])
		L := wizardpkg.ScenarioRects(content)
		if L.Right.X < L.Left.Right() || L.List.Right() > L.Left.Right()+0.5 {
			t.Fatalf("%v scenario details start at %v, list ends at %v", size, L.Right.X, L.Left.Right())
		}
		if L.Right.X < content.X+content.W*0.38 {
			t.Fatalf("%v scenario details x=%v, want right of the 38%% list column", size, L.Right.X)
		}
		if L.Right.Right() > content.Right()+0.5 || L.Buttons.Bottom() > content.Bottom()+0.5 {
			t.Fatalf("%v scenario columns leave the content area", size)
		}
		S := wizardpkg.StoryRects(content)
		if S.Centre.X < S.Left.Right() || S.Right.X < S.Centre.Right() || S.Right.Right() > content.Right()+0.5 {
			t.Fatalf("%v storyteller columns overlap: %+v", size, S)
		}
	}

	click(t, a, "wizard.next")
	if a.nav.screen() != ports.ScreenStoryteller {
		t.Fatalf("screen = %s, want storyteller", a.nav.screen())
	}
	click(t, a, "wizard.back")
	click(t, a, "wizard.back")
	click(t, a, "menu.load")
	x, _ := bodyOrigin(a.opt.LoadButtons(a.sw, a.sh), a.sh)
	if x != bodyDefaultX {
		t.Fatalf("load body x=%d, want %d (footer only)", x, bodyDefaultX)
	}
	click(t, a, "load.back")
	click(t, a, "menu.options")
	x, _ = bodyOrigin(a.opt.Buttons(a.sw, a.sh), a.sh)
	if x < 280 {
		t.Fatalf("options body x=%d, want right of the toggle", x)
	}
}

func TestWorldGenerateStaysInFooter(t *testing.T) {
	t.Parallel()
	a := testApp(&stubSim{w: 4, h: 4})
	a.finishBoot()
	click(t, a, "menu.newgame")
	click(t, a, "wizard.next")
	click(t, a, "wizard.next")
	if a.nav.screen() != ports.ScreenWorldGen {
		t.Fatalf("screen = %s, want world_gen", a.nav.screen())
	}
	settleUI(t, a)
	w, ok := a.uiSt().Find("world.generate")
	if !ok {
		t.Fatal("missing world.generate")
	}
	_, _, _, footer := uikit.DialogFrame(a.sw, a.sh)
	if w.Disabled || w.R.Y < footer.Y || w.R.Bottom() > footer.Bottom() {
		t.Fatalf("Generate at %+v should be enabled in the footer %+v", w.R, footer)
	}
}

// art_atlas.go: pages cut in the background, not mid-frame

// dev frame-rate probe (fps_probe.go)

// perf_stats.go: the overlay's timing (PERF-002)

// commitment: save before the window closes (saves.go)

// ui_scale.go: before any measure or hit test

// background.go: Run in background, from this frame's focus

// F10 screenshot, F11 hide UI (hotkeys.go)

// minimap (minimap.go): the camera moved

// hud.go: letters, alerts, command options

// hotkeys.go

// captures frame the camera themselves
// physical (camera.go)

// click outside a menu just closes it

// move_mode.go: placing a moved building
// zone_select.go: a selected zone's Expand or Shrink tool

// selection.go / multiselect.go / formation.go

// background.go: ~20 fps while running in the background
// perf_stats.go: runs before the throttle sleeps (PERF-002)
// ui_scale.go: the canvas and the font draw at this scale

// logical

// the viewport is in this frame's map pixels

// tree_focus.go: hover, selection and ghost see through canopies
// world_scale.go: the map, and what the tools mark on it, draw
// into the render-scaled offscreen when there is one; its canvas
// labels are queued and drawn on the screen after it

// mapfields_overlay.go: dev only

// hud.go: selection marks, ghosts, the drag box

// minimap.go: big maps only

// perf_overlay.go: also with the UI hidden (PERF-002)

// widget kit: widgets, modals, tooltips (ui_state.go)

// F10: this frame, before the toast that reports it
