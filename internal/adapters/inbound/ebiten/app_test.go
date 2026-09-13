package ebiten

import (
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/clock"
)

type stubSim struct {
	steps int
	w, h  int
	tick  clock.Tick
}

func (s *stubSim) Step(n int) {
	s.steps += n
	s.tick += clock.Tick(n)
}

func (s *stubSim) Snapshot() []game.PawnView { return nil }
func (s *stubSim) Width() int                { return s.w }
func (s *stubSim) Height() int               { return s.h }
func (s *stubSim) Tick() clock.Tick          { return s.tick }
func (s *stubSim) Date() clock.Date          { return clock.DateAt(s.tick) }
func (s *stubSim) Activity() schedule.Activity {
	return schedule.Work
}

func testApp(sim Simulation) *App {
	return NewApp(AppConfig{
		Version: "test",
		LoadPack: func() (*atlas, error) {
			return nil, nil
		},
		NewSim: func() (Simulation, error) {
			return sim, nil
		},
	})
}

func click(t *testing.T, a *App, id string) {
	t.Helper()
	for _, b := range a.layoutButtons(a.sw, a.sh) {
		if b.id != id {
			continue
		}
		if b.disabled {
			a.hint = b.reason
			return
		}
		a.applyClick(id)
		return
	}
	t.Fatalf("no button %q on %s", id, a.nav.screen())
}

func TestBootTicksToMainMenuWithoutSim(t *testing.T) {
	t.Parallel()
	sim := &stubSim{w: 8, h: 8}
	a := testApp(sim)
	if a.nav.screen() != screenBoot {
		t.Fatalf("start screen = %s, want boot", a.nav.screen())
	}
	for i := 0; i < bootFramesPerPhase*len(bootPhases)+1; i++ {
		a.tick()
	}
	if a.nav.screen() != screenMainMenu {
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
		scr screenID
	}{
		{"menu.newgame", screenScenario},
		{"wizard.next", screenStoryteller},
		{"wizard.next", screenWorldGen},
		{"wizard.next", screenLanding},
		{"wizard.next", screenPawns},
	}
	for _, step := range want {
		click(t, a, step.id)
		if a.nav.screen() != step.scr {
			t.Fatalf("after %s = %s, want %s", step.id, a.nav.screen(), step.scr)
		}
	}
	click(t, a, "wizard.start")
	if a.nav.screen() != screenColony {
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
	if a.nav.screen() != screenMainMenu {
		t.Fatalf("continue navigated to %s", a.nav.screen())
	}
	if a.hint != "no saves yet" {
		t.Fatalf("hint = %q, want no saves yet", a.hint)
	}
	click(t, a, "menu.plugins")
	if a.nav.screen() != screenMainMenu {
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
	if a.nav.screen() != screenScenario {
		t.Fatalf("escape = %s, want scenario", a.nav.screen())
	}
	click(t, a, "wizard.back")
	if a.nav.screen() != screenMainMenu {
		t.Fatalf("back = %s, want main_menu", a.nav.screen())
	}
}

func TestPauseBlocksSteppingAndResumeContinues(t *testing.T) {
	t.Parallel()
	sim := &stubSim{w: 8, h: 8}
	a := testApp(sim)
	a.finishBoot()
	click(t, a, "menu.newgame")
	for i := 0; i < 4; i++ {
		click(t, a, "wizard.next")
	}
	click(t, a, "wizard.start")

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
	click(t, a, "menu.newgame")
	for i := 0; i < 4; i++ {
		click(t, a, "wizard.next")
	}
	click(t, a, "wizard.start")
	a.apply(intent{kind: intentPause})
	click(t, a, "pause.menu")
	if a.nav.screen() != screenMainMenu {
		t.Fatalf("return = %s, want main_menu", a.nav.screen())
	}
	if a.renderer != nil {
		t.Fatal("return to menu must drop the renderer")
	}
	a.tick()
	if sim.steps != 0 {
		t.Fatalf("headless menu stepped %d", sim.steps)
	}
	click(t, a, "menu.newgame")
	for i := 0; i < 4; i++ {
		click(t, a, "wizard.next")
	}
	click(t, a, "wizard.start")
	if a.simsStarted != 2 {
		t.Fatalf("second start simsStarted=%d, want 2", a.simsStarted)
	}
}

func TestOptionsFromPauseReturnsToPause(t *testing.T) {
	t.Parallel()
	a := testApp(&stubSim{w: 4, h: 4})
	a.finishBoot()
	click(t, a, "menu.newgame")
	for i := 0; i < 4; i++ {
		click(t, a, "wizard.next")
	}
	click(t, a, "wizard.start")
	a.apply(intent{kind: intentPause})
	click(t, a, "pause.options")
	if a.nav.screen() != screenOptions {
		t.Fatalf("options = %s", a.nav.screen())
	}
	click(t, a, "options.back")
	if a.nav.screen() != screenColony || a.nav.overlay() != overlayPause {
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
	if a.nav.screen() != screenLoad {
		t.Fatalf("load = %s", a.nav.screen())
	}
	click(t, a, "load.back")
	click(t, a, "menu.credits")
	if a.nav.screen() != screenCredits {
		t.Fatalf("credits = %s", a.nav.screen())
	}
}

func TestScenarioSelectionStaysOnScreen(t *testing.T) {
	t.Parallel()
	a := testApp(&stubSim{w: 4, h: 4})
	a.finishBoot()
	click(t, a, "menu.newgame")
	click(t, a, "scenario.2")
	if a.scenario != 2 || a.nav.screen() != screenScenario {
		t.Fatalf("select scenario 2: idx=%d screen=%s", a.scenario, a.nav.screen())
	}
}
