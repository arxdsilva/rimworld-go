package ebiten

import (
	"fmt"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"slices"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/config"
	"rimworld-go/internal/platform/input"
)

// The learning helper (PLAY-018): a collapsible top-right panel of short
// lessons. A lesson is done by the player doing the thing (a command seen
// through Renderer.onEnqueue, or a UI event read each Update) or by "Got it";
// progress lives in the settings.

// learnColony is a started stub colony (a 100x100 map, one colonist) on the
// settings in store, past the frames that place the camera and take the
// helper's baselines. pre runs after the colony starts, before any frame.
func learnColony(t *testing.T, store *fakeSettings, pre func(*App)) (*App, *stubSim, *fakeInput) {
	t.Helper()
	s := &stubSim{w: 100, h: 100, pawns: []game.PawnView{{ID: 1, Name: "Ada", X: 50, Y: 50, Status: "alive"}}}
	a := testApp(s)
	a.cfg.Settings = store.hooks()
	a.loadSettings()
	a.finishBoot()
	a.startGame()
	if pre != nil {
		pre(a)
	}
	f := fakeOn(a)
	f.Frame(t, a) // the first frame places the camera
	f.Frame(t, a) // the helper takes its baselines
	return a, s, f
}

func learnStore() *fakeSettings { return &fakeSettings{s: config.Defaults()} }

func learnedNow(a *App) []string { return a.settings.Gameplay.LearnedLessons }

func learnDone(a *App, id string) bool { return slices.Contains(learnedNow(a), id) }

func learnHasButton(bs []button, id string) bool {
	for _, b := range bs {
		if b.id == id {
			return true
		}
	}
	return false
}

func learnRowIDs(a *App) []string {
	var out []string
	for _, b := range a.learnButtons() {
		if id, ok := strings.CutPrefix(b.id, "learn.lesson."); ok {
			out = append(out, id)
		}
	}
	return out
}

func TestLessonCatalog(t *testing.T) {
	t.Parallel()
	want := []string{"camera", "select", "build", "zones", "work", "schedule", "draft", "forbid", "research", "speed"}
	var got []string
	seen := map[string]bool{}
	a := testApp(&stubSim{w: 4, h: 4})
	for _, l := range lessonCatalog {
		got = append(got, l.id)
		if seen[l.id] {
			t.Fatalf("duplicate lesson id %q", l.id)
		}
		seen[l.id] = true
		if l.onCmd == nil && l.onUI == nil {
			t.Errorf("%s: no way to be done on its own", l.id)
		}
		body := l.body(a)
		// 2-3 short sentences, our own words
		n := strings.Count(body, ". ") + strings.Count(body, "? ") + 1
		if n < 2 || n > 3 || !strings.HasSuffix(body, ".") {
			t.Errorf("%s: body has %d sentences, want 2-3: %q", l.id, n, body)
		}
		if len(body) > 330 {
			t.Errorf("%s: body is %d chars, keep it short", l.id, len(body))
		}
		for _, s := range []string{l.title, l.goal, body} {
			if strings.Contains(strings.ToLower(s), "rimworld") || strings.Contains(s, "{") || s == "" {
				t.Errorf("%s: bad text %q", l.id, s)
			}
		}
		if w, _ := measure(l.title, tsLabel); w > learnRowW-learnRowPad*2 {
			t.Errorf("%s: title %q is %.0f px, the row holds %d", l.id, l.title, w, learnRowW-learnRowPad*2)
		}
		if w, _ := measure(l.goal, tsSmall); w > learnRowW-learnRowPad*2 {
			t.Errorf("%s: goal %q is %.0f px, the row holds %d", l.id, l.goal, w, learnRowW-learnRowPad*2)
		}
	}
	if !slices.Equal(got, want) {
		t.Fatalf("lessons = %v, want %v", got, want)
	}
}

func TestPanelStartsWithTheFirstThreeLessons(t *testing.T) {
	t.Parallel()
	a, _, _ := learnColony(t, learnStore(), nil)
	if got := learnRowIDs(a); !slices.Equal(got, []string{"camera", "select", "build"}) {
		t.Fatalf("rows = %v, want the first three lessons", got)
	}
	l := a.hud().Learn
	if want := (rect{X: float32(a.sw) - hudMargin - rightW, Y: hudMargin, W: rightW, H: a.learnHeight()}); l != want {
		t.Fatalf("Learn rect = %+v, want %+v", l, want)
	}
	if h := a.learnHeight(); h <= learnHeaderH || h > 200 {
		t.Fatalf("expanded height = %v, want in (%d, 200]", h, learnHeaderH)
	}
	if !learnHasButton(a.learnButtons(), "learn.toggle") {
		t.Fatal("no collapse toggle")
	}
}

func TestLessonsDoneByTheirCommands(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, id string
		cmds     []command.Command
	}{
		{"blueprint", "build", []command.Command{{Kind: command.DesignateBuild, Build: 1}}},
		{"stockpile", "zones", []command.Command{{Kind: command.DesignateStockpile, X2: 2, Y2: 2}}},
		{"growing zone", "zones", []command.Command{{Kind: command.DesignateGrowZone, X2: 2, Y2: 2}}},
		{"work priority", "work", []command.Command{{Kind: command.SetWorkPriority, Pawn: 1, Work: command.WorkGrow, Count: 1}}},
		{"work enabled", "work", []command.Command{{Kind: command.SetWorkEnabled, Pawn: 1, Work: command.WorkHaul, Enabled: true}}},
		{"schedule", "schedule", []command.Command{{Kind: command.SetSchedule, Pawn: 1, Index: 3, Mode: 1}}},
		{"draft then move", "draft", []command.Command{{Kind: command.DraftPawn, Pawn: 1, Enabled: true}, {Kind: command.MovePawn, Pawn: 1, X: 4, Y: 4}}},
		{"forbid a stack", "forbid", []command.Command{{Kind: command.SetForbidden, Target: 9, Enabled: true}}},
		{"forbid an area", "forbid", []command.Command{{Kind: command.DesignateForbid, X2: 2, Y2: 2, Enabled: true}}},
		{"haul an area", "forbid", []command.Command{{Kind: command.DesignateHaul, X2: 2, Y2: 2, Enabled: true}}},
		{"haul one stack", "forbid", []command.Command{{Kind: command.SetHaulMark, Target: 9, Enabled: true}}},
		{"research", "research", []command.Command{{Kind: command.SetResearch, Build: 2}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			store := learnStore()
			a, sim, _ := learnColony(t, store, nil)
			for i, cmd := range c.cmds {
				if learnDone(a, c.id) {
					t.Fatalf("%s done after only %d of its %d commands", c.id, i, len(c.cmds))
				}
				a.renderer.enqueue(cmd)
			}
			if !learnDone(a, c.id) {
				t.Fatalf("%s not done by %+v (learned %v)", c.id, c.cmds, learnedNow(a))
			}
			if len(sim.cmds) != len(c.cmds) {
				t.Fatalf("the simulation saw %d commands, want %d: the hook must not swallow them", len(sim.cmds), len(c.cmds))
			}
			if !slices.Contains(store.s.Gameplay.LearnedLessons, c.id) {
				t.Fatalf("the done lesson was not saved: %v", store.s.Gameplay.LearnedLessons)
			}
			if title, ok := a.learnFlash(); !ok || title == "" {
				t.Fatal("a lesson done on its own should highlight briefly")
			}
		})
	}
}

func TestOtherCommandsFinishNoLesson(t *testing.T) {
	t.Parallel()
	a, _, _ := learnColony(t, learnStore(), nil)
	for _, c := range []command.Command{
		{Kind: command.DesignateHarvest, X2: 2, Y2: 2},
		{Kind: command.DesignateMine, X2: 2, Y2: 2},
		{Kind: command.MovePawn, Pawn: 1, X: 3, Y: 3},       // no draft first
		{Kind: command.DraftPawn, Pawn: 1, Enabled: false},  // undrafting is not drafting
		{Kind: command.MoveBuilding, Target: 3, X: 1, Y: 1}, // a building, not a colonist
	} {
		a.renderer.enqueue(c)
	}
	if got := learnedNow(a); len(got) != 0 {
		t.Fatalf("learned %v from commands that teach nothing", got)
	}
}

func TestDraftThenMoveNeedsTheDraftFirst(t *testing.T) {
	t.Parallel()
	a, _, _ := learnColony(t, learnStore(), nil)
	a.renderer.enqueue(command.Command{Kind: command.MovePawn, Pawn: 1, X: 3, Y: 3})
	a.renderer.enqueue(command.Command{Kind: command.DraftPawn, Pawn: 1, Enabled: true})
	if learnDone(a, "draft") {
		t.Fatal("a move before the draft taught drafting")
	}
	a.renderer.enqueue(command.Command{Kind: command.MovePawn, Pawn: 1, X: 3, Y: 3})
	if !learnDone(a, "draft") {
		t.Fatal("draft then move should finish the lesson")
	}
}

func TestCameraLessonNeedsRealPanOrZoomInput(t *testing.T) {
	t.Parallel()
	hold := func(key string, n int) func(*testing.T, *App, *fakeInput) {
		return func(t *testing.T, a *App, f *fakeInput) {
			f.Cur.Held = []string{key}
			for range n {
				f.Frame(t, a)
			}
			f.Cur.Held = nil
		}
	}
	wheel := func(n int) func(*testing.T, *App, *fakeInput) {
		return func(t *testing.T, a *App, f *fakeInput) {
			for range n {
				f.Cur.WheelY = 1
				f.Frame(t, a)
			}
		}
	}
	cases := []struct {
		name string
		do   func(*testing.T, *App, *fakeInput)
		done bool
	}{
		{"a tap of a pan key", hold("D", 3), false},
		{"holding a pan key", hold("D", learnPanFrames), true},
		{"holding an arrow key", hold("ArrowUp", learnPanFrames), true},
		{"one wheel notch", wheel(1), false},
		{"two wheel notches", wheel(2), true},
		{"zoom keys", func(t *testing.T, a *App, f *fakeInput) {
			f.Key(t, a, "Equal", 0)
			f.Key(t, a, "Minus", 0)
		}, true},
		{"dragging with the middle button", func(t *testing.T, a *App, f *fakeInput) {
			f.Cur.Down[mbMiddle] = true
			for range learnPanFrames {
				f.Frame(t, a)
			}
			f.Cur.Down[mbMiddle] = false
		}, true},
		{"the camera moved by other means", func(t *testing.T, a *App, f *fakeInput) {
			// a resize, a scale change or a jump to an alert is not the player panning
			a.renderer.cam.OffsetX += 20 * a.renderer.tilePx()
			a.renderer.cam.Zoom *= 2
			a.centerOnMap(10, 10)
			a.setPhysical(a.pw*3/2, a.ph*3/2)
			for range 40 {
				f.Frame(t, a)
			}
		}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			a, _, f := learnColony(t, learnStore(), nil)
			if learnDone(a, "camera") {
				t.Fatal("the lesson started out done")
			}
			c.do(t, a, f)
			if got := learnDone(a, "camera"); got != c.done {
				t.Fatalf("camera lesson done = %v, want %v (learned %v)", got, c.done, learnedNow(a))
			}
		})
	}
}

func TestSelectLesson(t *testing.T) {
	t.Parallel()
	t.Run("a colonist", func(t *testing.T) {
		t.Parallel()
		a, _, f := learnColony(t, learnStore(), nil)
		a.renderer.Select(1)
		f.Frame(t, a)
		if !learnDone(a, "select") {
			t.Fatal("selecting a colonist should finish the lesson")
		}
	})
	t.Run("a building", func(t *testing.T) {
		t.Parallel()
		a, s, f := learnColony(t, learnStore(), nil)
		s.world.Buildings = []game.BuildingView{{ID: 7}} // the panels drop a selection that is not in the world
		a.selBuilding, a.hasBuilding = 7, true
		f.Frame(t, a)
		if !learnDone(a, "select") {
			t.Fatal("selecting a building should finish the lesson")
		}
	})
	t.Run("already selected at the start", func(t *testing.T) {
		t.Parallel()
		a, _, f := learnColony(t, learnStore(), func(a *App) { a.renderer.Select(1) })
		f.Frame(t, a)
		if learnDone(a, "select") {
			t.Fatal("a selection that was there before the helper looked taught selecting")
		}
		a.renderer.ClearSelect()
		f.Frame(t, a)
		a.renderer.Select(1)
		f.Frame(t, a)
		if !learnDone(a, "select") {
			t.Fatal("clearing then picking again should finish the lesson")
		}
	})
}

func TestSpeedLesson(t *testing.T) {
	t.Parallel()
	t.Run("the player changes it", func(t *testing.T) {
		t.Parallel()
		a, _, f := learnColony(t, learnStore(), nil)
		a.renderer.setSpeed(speedFast)
		f.Frame(t, a)
		if !learnDone(a, "speed") {
			t.Fatal("changing the speed should finish the lesson")
		}
	})
	t.Run("the player pauses", func(t *testing.T) {
		t.Parallel()
		a, _, f := learnColony(t, learnStore(), nil)
		a.renderer.togglePause()
		f.Frame(t, a)
		if !learnDone(a, "speed") {
			t.Fatal("pausing is time control too")
		}
	})
	t.Run("a threat pauses the game by itself", func(t *testing.T) {
		t.Parallel()
		a, _, f := learnColony(t, learnStore(), nil)
		a.renderer.speed, a.renderer.threatPaused = speedPaused, true
		f.Frame(t, a)
		if learnDone(a, "speed") {
			t.Fatal("the game pausing itself on a threat is not the player changing the speed")
		}
		a.renderer.threatPaused = false
		a.renderer.setSpeed(speedNormal) // the player resumes
		f.Frame(t, a)
		if !learnDone(a, "speed") {
			t.Fatal("resuming after a threat is the player's own choice")
		}
	})
}

func TestGotItFinishesALessonWithoutAHighlight(t *testing.T) {
	t.Parallel()
	store := learnStore()
	a, _, _ := learnColony(t, store, nil)
	click(t, a, "learn.lesson.build")
	if !learnHasButton(a.learnButtons(), "learn.done.build") {
		t.Fatal("opening a lesson should offer Got it")
	}
	click(t, a, "learn.done.build")
	if !learnDone(a, "build") || !slices.Contains(store.s.Gameplay.LearnedLessons, "build") {
		t.Fatalf("Got it did not finish and save the lesson: %v", learnedNow(a))
	}
	if _, ok := a.learnFlash(); ok {
		t.Fatal("Got it is the player's own acknowledgement: no highlight")
	}
	if learnHasButton(a.learnButtons(), "learn.done.build") {
		t.Fatal("the detail card should close once the lesson is done")
	}
	if got := learnRowIDs(a); !slices.Equal(got, []string{"camera", "select", "zones"}) {
		t.Fatalf("rows after build = %v, want the next unfinished three", got)
	}
}

func TestCompletedLessonsDisappearAndTheNextOnesComeUp(t *testing.T) {
	t.Parallel()
	a, _, _ := learnColony(t, learnStore(), nil)
	a.renderer.enqueue(command.Command{Kind: command.DesignateBuild, Build: 1})
	a.renderer.enqueue(command.Command{Kind: command.DesignateStockpile, X2: 1, Y2: 1})
	if got := learnRowIDs(a); !slices.Equal(got, []string{"camera", "select", "work"}) {
		t.Fatalf("rows = %v, want camera, select, work", got)
	}
}

func TestOpeningALessonTwiceClosesItsCard(t *testing.T) {
	t.Parallel()
	a, _, _ := learnColony(t, learnStore(), nil)
	click(t, a, "learn.lesson.select")
	if l := a.learnLayout(); l.card.Empty() {
		t.Fatal("no detail card after clicking a lesson")
	}
	click(t, a, "learn.lesson.camera") // another lesson swaps the card
	if !learnHasButton(a.learnButtons(), "learn.done.camera") || learnHasButton(a.learnButtons(), "learn.done.select") {
		t.Fatal("the card should show the last lesson clicked")
	}
	click(t, a, "learn.lesson.camera")
	if l := a.learnLayout(); !l.card.Empty() {
		t.Fatal("clicking the open lesson again should close its card")
	}
}

func TestCollapseKeepsAHeaderOnlyAndPersists(t *testing.T) {
	t.Parallel()
	store := learnStore()
	a, _, _ := learnColony(t, store, nil)
	click(t, a, "learn.lesson.camera")
	click(t, a, "learn.toggle")
	if !a.settings.Gameplay.LearnCollapsed || !store.s.Gameplay.LearnCollapsed {
		t.Fatal("collapse not saved")
	}
	if h := a.learnHeight(); h != learnHeaderH {
		t.Fatalf("collapsed height = %v, want %d", h, learnHeaderH)
	}
	if got := a.hud().Learn; got.H != learnHeaderH || got.W != rightW {
		t.Fatalf("Learn rect = %+v", got)
	}
	bs := a.learnButtons()
	if len(bs) != 1 || bs[0].id != "learn.toggle" {
		t.Fatalf("collapsed buttons = %+v, want only the toggle", bs)
	}
	if rc := a.learnChrome(); len(rc) != 0 {
		t.Fatalf("a collapsed helper should have no detail card, got %v", rc)
	}
	// restart: it comes back collapsed
	b, _, _ := learnColony(t, store, nil)
	if b.learnHeight() != learnHeaderH {
		t.Fatal("collapse did not survive a restart")
	}
	click(t, b, "learn.toggle")
	if h := b.learnHeight(); h <= learnHeaderH || b.settings.Gameplay.LearnCollapsed {
		t.Fatalf("expanding: height %v collapsed %v", h, b.settings.Gameplay.LearnCollapsed)
	}
}

func TestHiddenWhenOffOrWhenEveryLessonIsDone(t *testing.T) {
	t.Parallel()
	t.Run("setting off", func(t *testing.T) {
		t.Parallel()
		store := learnStore()
		store.s.Gameplay.ShowLearningHelper = false
		a, _, _ := learnColony(t, store, nil)
		if a.learnHeight() != 0 || len(a.learnButtons()) != 0 || !a.hud().Learn.Empty() {
			t.Fatal("the helper should be hidden while the setting is off")
		}
		a.renderer.enqueue(command.Command{Kind: command.DesignateBuild, Build: 1})
		if len(learnedNow(a)) != 0 {
			t.Fatal("a hidden helper must not use up lessons")
		}
		// turning it back on shows it again, nothing lost
		s := a.settings
		s.Gameplay.ShowLearningHelper = true
		a.setSettings(s)
		if a.learnHeight() == 0 || !learnHasButton(a.learnButtons(), "learn.lesson.build") {
			t.Fatal("turning the helper on should show it with every lesson still to do")
		}
	})
	t.Run("all done", func(t *testing.T) {
		t.Parallel()
		a, _, _ := learnColony(t, learnStore(), nil)
		for _, l := range lessonCatalog {
			click(t, a, "learn.lesson."+l.id)
			click(t, a, "learn.done."+l.id)
		}
		if a.learnHeight() != 0 || len(a.learnButtons()) != 0 || !a.hud().Learn.Empty() || len(a.learnChrome()) != 0 {
			t.Fatal("with every lesson done the helper should hide itself")
		}
		if len(learnedNow(a)) != len(lessonCatalog) {
			t.Fatalf("learned %d of %d", len(learnedNow(a)), len(lessonCatalog))
		}
	})
}

func TestProgressSurvivesARestart(t *testing.T) {
	t.Parallel()
	store := learnStore()
	a, _, _ := learnColony(t, store, nil)
	a.renderer.enqueue(command.Command{Kind: command.DesignateBuild, Build: 1})
	click(t, a, "learn.lesson.camera")
	click(t, a, "learn.done.camera")

	b, _, _ := learnColony(t, store, nil)
	if !learnDone(b, "build") || !learnDone(b, "camera") {
		t.Fatalf("after a restart learned = %v, want camera and build", learnedNow(b))
	}
	if got := learnRowIDs(b); !slices.Equal(got, []string{"select", "zones", "work"}) {
		t.Fatalf("rows after a restart = %v", got)
	}
}

func TestHelperFollowsANewColony(t *testing.T) {
	t.Parallel()
	a, _, f := learnColony(t, learnStore(), nil)
	first := a.renderer
	a.startGame() // a second session: a new renderer
	if a.renderer == first {
		t.Fatal("expected a new renderer")
	}
	f.Frame(t, a)
	f.Frame(t, a)
	a.renderer.enqueue(command.Command{Kind: command.SetResearch, Build: 1})
	if !learnDone(a, "research") {
		t.Fatal("the helper did not hook the new colony's renderer")
	}
}

func TestHookKeepsAnExistingObserverAndWrapsOnce(t *testing.T) {
	t.Parallel()
	seen := 0
	a, _, f := learnColony(t, learnStore(), func(a *App) {
		a.renderer.onEnqueue = func(command.Command) { seen++ }
	})
	for range 5 {
		f.Frame(t, a)
	}
	a.renderer.enqueue(command.Command{Kind: command.DesignateBuild, Build: 1})
	if seen != 1 {
		t.Fatalf("the earlier observer saw %d commands, want 1", seen)
	}
	if !learnDone(a, "build") {
		t.Fatal("the helper missed the command")
	}
}

func TestCapturesAreNotThePlayer(t *testing.T) {
	t.Parallel()
	a, _, _ := learnColony(t, learnStore(), nil)
	a.cfg.Capture = &Capture{}
	if a.learnTracking() {
		t.Fatal("a scripted capture must not use up lessons")
	}
	a.learnObserve(command.Command{Kind: command.DesignateBuild, Build: 1})
	if len(learnedNow(a)) != 0 {
		t.Fatal("a capture's commands finished a lesson")
	}
}

func TestResetStartsTheLessonsAgain(t *testing.T) {
	t.Parallel()
	store := learnStore()
	a, _, f := learnColony(t, store, nil)
	pan := func() {
		f.Cur.Held = []string{"D"}
		for range learnPanFrames + 1 {
			f.Frame(t, a)
		}
		f.Cur.Held = nil
	}
	pan()
	if !learnDone(a, "camera") {
		t.Fatal("setup: the pan should finish the camera lesson")
	}
	a.optionsTab = "gameplay"
	a.applyOption("options.learnhelper.reset")
	if len(learnedNow(a)) != 0 || len(store.s.Gameplay.LearnedLessons) != 0 {
		t.Fatalf("reset left %v", learnedNow(a))
	}
	for range 3 {
		f.Frame(t, a)
	}
	if learnDone(a, "camera") {
		t.Fatal("the panning done before the reset must not finish the lesson again")
	}
	pan()
	if !learnDone(a, "camera") {
		t.Fatal("panning after the reset should finish the lesson again")
	}
}

func TestOptionsRowTogglesTheHelper(t *testing.T) {
	t.Parallel()
	store := learnStore()
	a := optionsApp(store)
	click(t, a, "menu.options")
	click(t, a, "options.tab.gameplay")
	if !a.settings.Gameplay.ShowLearningHelper {
		t.Fatal("the helper should start on")
	}
	click(t, a, "options.learnhelper")
	if a.settings.Gameplay.ShowLearningHelper || store.s.Gameplay.ShowLearningHelper {
		t.Fatal("the row should turn the helper off and save it")
	}
	click(t, a, "options.learnhelper")
	if !a.settings.Gameplay.ShowLearningHelper {
		t.Fatal("the row should turn it back on")
	}
	var row optionRow
	for _, r := range optionRows("gameplay") {
		if r.key == "learnhelper" {
			row = r
		}
	}
	if row.label != "Learning helper" || row.tip == "" {
		t.Fatalf("row = %+v, want the labelled, explained Learning helper row", row)
	}
	if w, _ := measure(row.label, tsBody); 56+w > optCtrlX-8 {
		t.Fatal("the label runs into its control")
	}
}

func TestOptionsResetLessonsAction(t *testing.T) {
	t.Parallel()
	store := learnStore()
	store.s.Gameplay.LearnedLessons = []string{"camera", "select"}
	a := optionsApp(store)
	click(t, a, "menu.options")
	click(t, a, "options.tab.gameplay")
	click(t, a, "options.learnhelper.reset")
	if len(a.settings.Gameplay.LearnedLessons) != 0 || len(store.s.Gameplay.LearnedLessons) != 0 {
		t.Fatalf("learned after reset: %v / saved %v", a.settings.Gameplay.LearnedLessons, store.s.Gameplay.LearnedLessons)
	}
	if !a.settings.Gameplay.ShowLearningHelper {
		t.Fatal("reset must not turn the helper off")
	}
	for _, b := range a.optionsButtons(a.sw, a.sh) {
		if b.id == "options.learnhelper.reset" {
			if b.label != "Reset lessons" || b.tip == "" {
				t.Fatalf("reset button = %+v", b)
			}
			return
		}
	}
	t.Fatal("no Reset lessons button")
}

// learnColonyAtScale is learnColony on a window of winW x winH points at
// UI scale pct (the logical screen is the window over the scale).
func learnColonyAtScale(t *testing.T, pct int, winW, winH float64) (*App, *fakeInput) {
	t.Helper()
	t.Cleanup(func() { gfx.SetScale(unitUIScale) })
	s := &stubSim{w: 100, h: 100, pawns: []game.PawnView{{ID: 1, Name: "Ada", X: 50, Y: 50, Status: "alive"}}}
	a := testApp(s)
	a.devFn = func() float64 { return 1 }
	st := a.settings
	st.Accessibility.UIScaleAuto, st.Accessibility.UIScale = false, pct
	a.setSettings(st)
	a.LayoutF(winW, winH)
	a.finishBoot()
	a.startGame()
	a.syncUIScale()
	f := fakeOn(a)
	f.Frame(t, a)
	f.Frame(t, a)
	return a, f
}

func TestDetailCardStaysLeftOfThePanelAndOnScreen(t *testing.T) {
	// not parallel: UI scale is a package global
	for _, pct := range []int{100, 150, 200} {
		for _, win := range [][2]float64{{1280, 720}, {1280 * float64(pct) / 100, 720 * float64(pct) / 100}} {
			t.Run(fmt.Sprintf("%d%% on %vx%v", pct, win[0], win[1]), func(t *testing.T) {
				a, _ := learnColonyAtScale(t, pct, win[0], win[1])
				if l := a.hud(); l.Learn.H < a.learnHeight() {
					// a logical screen too short for the lessons above the time
					// controls (1280x720 at 200%): the helper folds to its header,
					// clear of them, and opens no card
					if l.Learn.H != learnHeaderH || !l.Learn.Intersect(l.Global).Empty() || len(a.learnLayout().rows) != 0 || len(a.learnChrome()) != 0 {
						t.Fatalf("cramped helper %+v over Global %+v: want the header alone and no card", l.Learn, l.Global)
					}
					if a.sw >= 1280 {
						t.Fatalf("the helper folded on a %dx%d logical screen", a.sw, a.sh)
					}
					return
				}
				for _, l := range lessonCatalog[:3] {
					click(t, a, "learn.lesson."+l.id)
					lay := a.learnLayout()
					panel := a.hud().Learn
					card := lay.card
					if card.Empty() {
						t.Fatalf("%s: no card (logical %dx%d)", l.id, a.sw, a.sh)
					}
					if card.Right() > panel.X {
						t.Errorf("%s: card %+v is not left of the panel %+v", l.id, card, panel)
					}
					if card.X < 0 || card.Y < 0 || card.Right() > float32(a.sw) || card.Bottom() > float32(a.sh) {
						t.Errorf("%s: card %+v is off the %dx%d screen", l.id, card, a.sw, a.sh)
					}
					if tabs := a.hud().Tabs; card.Bottom() > tabs.Y {
						t.Errorf("%s: card %+v runs over the tab strip %+v", l.id, card, tabs)
					}
					done := lay.gotIt
					if done.Empty() || done.X < card.X || done.Right() > card.Right() || done.Y < card.Y || done.Bottom() > card.Bottom() {
						t.Errorf("%s: Got it %+v is not inside the card %+v", l.id, done, card)
					}
					for _, b := range a.learnButtons() {
						if b.id == "learn.done."+l.id && (b.x != done.X || b.y != done.Y || b.w != done.W || b.h != done.H) {
							t.Errorf("%s: the Got it button %+v is not the laid-out rect %+v", l.id, b, done)
						}
					}
					if rc := a.learnChrome(); len(rc) != 1 || rc[0] != card {
						t.Errorf("%s: chrome = %v, want the card %+v so clicks on it never reach the map", l.id, rc, card)
					}
				}
			})
		}
	}
}

func TestDrawStaysInsideThePanelAndCard(t *testing.T) {
	for _, pct := range []int{100, 150, 200} {
		t.Run(fmt.Sprintf("%d%%", pct), func(t *testing.T) {
			a, _ := learnColonyAtScale(t, pct, 1280*float64(pct)/100, 720*float64(pct)/100)
			a.renderer.enqueue(command.Command{Kind: command.DesignateBuild, Build: 1}) // a highlight in the header and footer
			if _, ok := a.learnFlash(); !ok {
				t.Fatal("setup: expected a highlight")
			}
			click(t, a, "learn.lesson.select") // open a detail card
			img := ebiten.NewImage(a.pw, a.ph)
			for _, collapsed := range []bool{false, true} {
				if collapsed {
					click(t, a, "learn.toggle")
				}
				if (len(a.learnChrome()) == 1) == collapsed {
					t.Fatalf("collapsed=%v: chrome %v, want a card exactly while expanded", collapsed, a.learnChrome())
				}
				got := collectTrace(t)
				a.drawLearn(img)
				if len(*got) == 0 {
					t.Fatalf("collapsed=%v: drew nothing", collapsed)
				}
				sc := a.scale.S
				regions := []rect{a.hud().Learn}
				regions = append(regions, a.learnChrome()...)
				for _, e := range *got {
					in := false
					for _, r := range regions {
						if float64(e.X) >= float64(r.X)*sc-1 && float64(e.Y) >= float64(r.Y)*sc-1 &&
							float64(e.X+e.W) <= float64(r.Right())*sc+1 && float64(e.Y+e.H) <= float64(r.Bottom())*sc+1 {
							in = true
						}
					}
					if !in {
						t.Errorf("collapsed=%v: %s is outside the panel and card %v", collapsed, e, regions)
					}
				}
				gfx.SetDrawTrace(nil)
			}
		})
	}
}

func TestHelperPanelDoesNotOverlapTheAlertsBelowIt(t *testing.T) {
	t.Parallel()
	a, _, _ := learnColony(t, learnStore(), nil)
	l := computeHUDLayout(a.sw, a.sh, hudState{LearnH: a.learnHeight(), Alerts: 5, Colonists: 1})
	if l.Alerts.Empty() {
		t.Fatal("setup: no alert rows")
	}
	if l.Alerts.Y < l.Learn.Bottom() {
		t.Fatalf("alerts %+v start inside the helper %+v", l.Alerts, l.Learn)
	}
}

func TestSelectLessonSeesEveryKindOfSelection(t *testing.T) {
	t.Parallel()
	kinds := map[string]func(*App){
		"animal": func(a *App) { a.selAnimal, a.hasAnimal = 3, true },
		"plant":  func(a *App) { a.selPlant, a.hasPlant = 4, true },
		"item":   func(a *App) { a.selItem, a.hasItem = 5, true },
		"frame":  func(a *App) { a.selFrame, a.hasFrame = 6, true },
		"group":  func(a *App) { a.group = selGroup{kind: selPawn, ids: []entity.EntityID{1, 2}} },
	}
	for name, sel := range kinds {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a, s, f := learnColony(t, learnStore(), nil)
			// the panels drop a selection that is not in the world: put each there
			s.world.Animals = []game.AnimalView{{ID: 3}}
			s.world.Plants = []game.PlantView{{ID: 4}}
			s.world.Items = []game.ItemView{{ID: 5}}
			s.world.Frames = []game.FrameView{{ID: 6}}
			sel(a)
			f.Frame(t, a)
			if !learnDone(a, "select") {
				t.Fatalf("selecting a %s did not finish the lesson", name)
			}
		})
	}
}

func TestLessonTextShowsLiveKeysAndSurvivesNoBinding(t *testing.T) {
	t.Parallel()
	a := testApp(&stubSim{w: 4, h: 4})
	draft := lessonByID("draft")
	if body := draft.body(a); !strings.Contains(body, " R ") && !strings.Contains(body, "press R") {
		t.Fatalf("draft lesson should name the Draft key, got %q", body)
	}
	a.binds = input.Bindings{} // nothing bound
	if body := draft.body(a); !strings.Contains(body, "its key") {
		t.Fatalf("an unbound action should read %q, got %q", "its key", body)
	}
	if lessonByID("nosuch") != nil {
		t.Fatal("unknown lesson id found")
	}
}

func TestDetailCardCutsItsTextOnAVeryShortScreen(t *testing.T) {
	// A 520x240 screen has no room for the lessons above the time controls:
	// the helper folds to its header, clear of them, and lists no lesson.
	small, _ := learnColonyAtScale(t, 100, 520, 240)
	if l := small.hud(); l.Learn.H > learnHeaderH || !l.Learn.Intersect(l.Global).Empty() || len(small.learnLayout().rows) != 0 {
		t.Fatalf("the helper %+v on a %dx%d screen, over Global %+v", l.Learn, small.sw, small.sh, l.Global)
	}
	// The card itself cuts its text to the room above the tabs: laid out
	// beside a panel where the 520x240 screen put it, against that screen's
	// tab strip (212 px down), it ends in an ellipsis.
	a, _ := learnColonyAtScale(t, 100, 1280, 720)
	click(t, a, "learn.lesson.camera")
	lay := a.learnLayout()
	if lay.card.Empty() || len(lay.rows) == 0 {
		t.Fatal("no card on the default screen")
	}
	short := a.hud()
	short.Tabs.Y = small.hud().Tabs.Y
	lay.panel.X = small.hud().Right.X
	a.learnCardLayout(&lay, short, lay.rows[0].r)
	if lay.card.Empty() {
		t.Fatal("no card above a short tab strip")
	}
	if lay.card.Y < hudMargin || lay.card.Bottom() > short.Tabs.Y-hudMargin+0.5 {
		t.Fatalf("card %+v does not fit between the top and the tabs (%v)", lay.card, short.Tabs.Y)
	}
	if last := lay.body[len(lay.body)-1]; !strings.HasSuffix(last, "…") {
		t.Fatalf("a cut text should end with an ellipsis, last line %q", last)
	}
}

func TestNoCardWhenThereIsNoRoomBesideThePanel(t *testing.T) {
	a, _ := learnColonyAtScale(t, 100, 300, 600)
	click(t, a, "learn.lesson.camera")
	if lay := a.learnLayout(); !lay.card.Empty() || lay.open != nil {
		t.Fatalf("a card was laid out on a %d px wide screen: %+v", a.sw, lay.card)
	}
	if learnHasButton(a.learnButtons(), "learn.done.camera") || len(a.learnChrome()) != 0 {
		t.Fatal("no card: no Got it button and no chrome")
	}
}

func TestUnknownLearnClicksAreIgnored(t *testing.T) {
	t.Parallel()
	a, _, _ := learnColony(t, learnStore(), nil)
	a.applyLearnClick("learn.lesson.nosuch")
	a.applyLearnClick("learn.done.nosuch")
	a.applyLearnClick("learn.mystery")
	if len(learnedNow(a)) != 0 || a.hst.learn.open != "" {
		t.Fatalf("unknown ids changed the helper: learned %v open %q", learnedNow(a), a.hst.learn.open)
	}
	click(t, a, "learn.lesson.camera")
	a.learnComplete("camera", false) // done behind the card's back
	a.applyLearnClick("learn.lesson.camera")
	if a.hst.learn.open != "" {
		t.Fatal("a finished lesson cannot be opened")
	}
}

func TestHighlightEnds(t *testing.T) {
	t.Parallel()
	a, _, f := learnColony(t, learnStore(), nil)
	a.renderer.enqueue(command.Command{Kind: command.DesignateBuild, Build: 1})
	if title, ok := a.learnFlash(); !ok || title != lessonByID("build").title {
		t.Fatalf("flash = %q %v, want the build lesson", title, ok)
	}
	for range learnFlashFrames + 1 {
		f.Frame(t, a)
	}
	if _, ok := a.learnFlash(); ok {
		t.Fatal("the highlight should end")
	}
}

func TestCloseLearnCard(t *testing.T) {
	t.Parallel()
	a, _, _ := learnColony(t, learnStore(), nil)
	if a.closeLearnCard() {
		t.Fatal("nothing to close yet")
	}
	click(t, a, "learn.lesson.camera")
	if !a.closeLearnCard() {
		t.Fatal("the card was up")
	}
	if len(a.learnChrome()) != 0 || a.closeLearnCard() {
		t.Fatal("the card should be gone")
	}
}
