package ebiten

import (
	"errors"
	"strings"
	"testing"
	"time"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// journey_flow_test.go: game-flow journeys (QA-001). The debts below are what this
// file still owes the guard (TestEveryInputActionHasAJourney); delete a
// name here once a journey in this file covers it.
var _ = owe("journey_flow_test.go",
	[]string{},
	[]string{})

// newMenuJourney boots to the main menu with no colony: a wizard Start
// builds a real colony step by step (the composition root's builder), so
// the journey's g is filled in by attach once the colony exists.
func newMenuJourney(t *testing.T, cfg ...func(*AppConfig)) *journey {
	t.Helper()
	a := testApp(&stubSim{w: 8, h: 8})
	a.cfg.NewSimBuilder = func(ng NewGame) (SimBuilder, error) {
		return &realBuilder{game.NewColonyBuilder(game.Config{Seed: ng.Seed, Biome: ng.Biome, Setup: ng.Setup})}, nil
	}
	for _, c := range cfg {
		c(&a.cfg)
	}
	a.finishBoot()
	if a.nav.screen() != screenMainMenu {
		t.Fatalf("boot ended on %s, want the main menu", a.nav.screen())
	}
	j := &journey{t: t, a: a, clock: time.Unix(1000, 0)}
	a.now = func() time.Time { return j.clock }
	j.f = fakeOn(a)
	j.f.Cur.Focused = true
	j.f.Cur.Mx, j.f.Cur.My = -50, -50
	j.update(1) // the menu's kit widgets are declared
	return j
}

// flowAttach takes the colony the menu started: j.g is the real game and the
// commands the renderer sends are recorded, as newJourney does.
func flowAttach(j *journey) {
	j.t.Helper()
	if j.a.renderer == nil || j.a.nav.screen() != screenColony {
		j.t.Fatalf("no colony: screen %s, %s", j.a.nav.screen(), j.a.sessionErr)
	}
	g, ok := j.a.renderer.game.(*game.Game)
	if !ok {
		j.t.Fatalf("the colony is a %T, want *game.Game", j.a.renderer.game)
	}
	j.g = g
	j.update(1) // the session settles: camera placed, learn hook installed
	r := j.a.renderer
	prev := r.onEnqueue
	r.onEnqueue = func(cmd command.Command) {
		if prev != nil {
			prev(cmd)
		}
		j.cmds = append(j.cmds, cmd)
	}
}

// flowUntil runs Updates until cond holds (a progress screen steps its
// builder once per Update), failing after max.
func flowUntil(j *journey, what string, cond func() bool, max int) {
	j.t.Helper()
	for i := 0; i < max && !cond(); i++ {
		j.update(1)
	}
	if !cond() {
		j.t.Fatalf("%s: not reached after %d Updates (screen %s, modal %q, %s)", what, max, j.a.nav.screen(), j.a.uiModalTop(), j.a.sessionErr)
	}
}

// wizardToColony walks New Colony with the mouse: the Hearthkeeper on
// Seasoned, a tiny world, a random site at the smallest map size, Start;
// the intro letter is closed with Escape.
func wizardToColony(t *testing.T, j *journey) {
	t.Helper()
	a := j.a
	j.clickID("menu.newgame")
	if a.nav.screen() != screenScenario {
		t.Fatalf("New Colony opened %s, want the scenario screen", a.nav.screen())
	}
	j.clickID("wizard.next")
	if a.nav.screen() != screenStoryteller {
		t.Fatalf("Next opened %s, want Choose Storyteller", a.nav.screen())
	}
	j.clickID("story.pick.hearth")
	j.clickID("diff.seasoned")
	if ng := a.draft(); ng.Storyteller != "hearth" || ng.Preset.Key() != "seasoned" {
		t.Fatalf("draft storyteller %q preset %q, want hearth on seasoned", ng.Storyteller, ng.Preset.Key())
	}
	j.clickID("wizard.next")
	if a.nav.screen() != screenWorldGen {
		t.Fatalf("Next opened %s, want Create World", a.nav.screen())
	}
	ng := a.draft()
	ng.World.SeedText, ng.World.Coverage = tinySeed, 5 // a tiny world keeps the journey fast
	j.clickID("world.generate")
	flowUntil(j, "the landing screen", func() bool { return a.nav.screen() == screenLanding }, 4000)
	j.clickID("land.random")
	j.clickID("land.advanced")
	j.update(1)
	j.clickID("land.mapsize.0")
	if side := a.draft().MapSize; side != game.MapSizes()[0].Side {
		t.Fatalf("map size %d after picking the first row, want %d", side, game.MapSizes()[0].Side)
	}
	j.clickID("modal.land.advanced.close")
	j.clickID("wizard.next")
	if a.nav.screen() != screenPawns {
		t.Fatalf("Next opened %s, want Create Characters (tile %d)", a.nav.screen(), a.draft().Tile)
	}
	j.clickID("wizard.start")
	flowUntil(j, "the colony", func() bool { return a.nav.screen() == screenColony && a.renderer != nil }, 4000)
	flowAttach(j)
	// the pods land, then the scenario's intro letter opens by itself
	flowUntil(j, "the intro letter", a.uiModalOpen, 2000)
	j.key("Escape", 0)
	if a.uiModalOpen() {
		t.Fatalf("Escape left the modal %q open", a.uiModalTop())
	}
}

// TestJourneyFlowWizardToColony: the main menu to a running colony through
// the wizard's screens with the mouse; the colony keeps the storyteller and
// difficulty picked there, runs a tick, and Options › Story changes both
// during play (SetStoryteller, SetDifficulty).
func TestJourneyFlowWizardToColony(t *testing.T) {
	t.Parallel()
	j := newMenuJourney(t)
	wizardToColony(t, j)
	a := j.a
	if a.renderer == nil {
		t.Fatal("no renderer after Start")
	}
	if v := j.g.Story(); v.Storyteller != "Bram Holloway" || !strings.EqualFold(v.Preset, "seasoned") {
		t.Fatalf("the colony's story is %q on %q, want the Hearthkeeper on Seasoned", v.Storyteller, v.Preset)
	}
	if j.a.renderer.speed != speedPaused {
		j.key("Space", 0) // a wizard colony starts running: pause it so tick counts exactly
	}
	j.tick(1)

	j.key("Escape", 0)
	if a.nav.overlay() != overlayPause {
		t.Fatalf("Escape on the colony did not open the pause menu (screen %s)", a.nav.screen())
	}
	j.clickID("pause.options")
	j.clickID("options.tab.story")
	j.clickID("opt.story.pick.gambler")
	j.clickID("opt.diff.grim")
	// Options › Story sends through the simulation, not the renderer
	// (story_tab.go sendNow), so the journey asserts what the command did.
	appliedBy(j, command.SetStoryteller, func() bool { return j.g.Story().Storyteller == "Kestrel Quay" })
	appliedBy(j, command.SetDifficulty, func() bool { return strings.EqualFold(j.g.Story().Preset, "grim") })
}

// appliedBy asserts the effect only a command of kind produces. It serves
// the panes that enqueue on the simulation directly, past the renderer, so
// the journey's command log never sees their commands; kind names the
// command in the failure.
func appliedBy(j *journey, kind command.Kind, effect func() bool) {
	j.t.Helper()
	if !effect() {
		_, rec := j.lastCmd(kind)
		j.t.Fatalf("command %v: recorded %v, but its effect does not hold", kind, rec)
	}
}

// flowSaves is an in-memory SaveHooks: one listed slot, loads a fresh game.
type flowSaves struct {
	saves []string
	slots []SaveSlot
}

func (s *flowSaves) hooks() SaveHooks {
	return SaveHooks{
		Save: func(_ Simulation, slot string) error { s.saves = append(s.saves, slot); return nil },
		List: func() ([]SaveSlot, error) { return s.slots, nil },
		Load: func(path string) (Simulation, string, error) {
			if path != "saves/kept.sav" {
				return nil, "", errors.New("no such save " + path)
			}
			g := game.New(game.Config{Seed: 3, Width: 24, Height: 24})
			if _, err := g.SpawnPawn(8, 4); err != nil {
				return nil, "", err
			}
			return g, "kept", nil
		},
	}
}

// TestJourneyFlowPauseSaveHook: Escape opens the pause menu; Save calls the
// save hook once, says so and resumes.
func TestJourneyFlowPauseSaveHook(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	saves := &flowSaves{}
	j.a.cfg.Saves = saves.hooks()
	j.key("Escape", 0)
	if j.a.nav.overlay() != overlayPause {
		t.Fatal("Escape did not open the pause menu")
	}
	if !j.clickID("pause.save") {
		t.Fatal("Save is disabled with save hooks wired")
	}
	if len(saves.saves) != 1 {
		t.Fatalf("the save hook ran %d times, want once", len(saves.saves))
	}
	if !strings.HasPrefix(j.a.hint, "saved") {
		t.Fatalf("hint %q after Save, want it to say saved", j.a.hint)
	}
	if j.a.nav.overlay() == overlayPause || j.a.nav.screen() != screenColony {
		t.Fatalf("Save left the pause menu up (screen %s)", j.a.nav.screen())
	}
}

// TestJourneyFlowReturnToMenuDropsRenderer: Return to Main Menu ends the
// session; the menu runs headless.
func TestJourneyFlowReturnToMenuDropsRenderer(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	j.key("Escape", 0)
	j.clickID("pause.menu")
	if j.a.renderer != nil {
		t.Fatal("the renderer survived Return to Main Menu")
	}
	if j.a.nav.screen() != screenMainMenu {
		t.Fatalf("Return to Main Menu opened %s", j.a.nav.screen())
	}
	j.update(2) // the menu runs without a colony
}

// TestJourneyFlowOptionsRowApplies: a row on Options › Accessibility
// applies at once: Reduced Motion makes the camera snap.
func TestJourneyFlowOptionsRowApplies(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	if j.a.renderer.smooth.Instant {
		t.Fatal("the camera snaps before Reduced Motion is on")
	}
	j.key("Escape", 0)
	j.clickID("pause.options")
	if j.a.nav.screen() != screenOptions {
		t.Fatalf("Options opened %s", j.a.nav.screen())
	}
	j.clickID("options.tab.access")
	j.clickID("options.reducedmotion")
	if !j.a.settings.Accessibility.ReducedMotion {
		t.Fatal("the Reduced Motion row did not turn the setting on")
	}
	if !j.a.renderer.smooth.Instant {
		t.Fatal("Reduced Motion did not reach the colony's camera live")
	}
}

// TestJourneyFlowContinueLoads: Continue on the main menu loads the newest
// save into a colony.
func TestJourneyFlowContinueLoads(t *testing.T) {
	t.Parallel()
	saves := &flowSaves{slots: []SaveSlot{{Name: "kept", Path: "saves/kept.sav", Colonists: 1, Modified: time.Unix(500, 0)}}}
	j := newMenuJourney(t, func(c *AppConfig) { c.Saves = saves.hooks() })
	if !j.clickID("menu.continue") {
		t.Fatal("Continue is disabled with a save listed")
	}
	flowAttach(j)
	if j.a.opt.SaveSlot != "kept" || !strings.HasPrefix(j.a.hint, "loaded") {
		t.Fatalf("slot %q hint %q after Continue, want the kept save loaded", j.a.opt.SaveSlot, j.a.hint)
	}
	if len(j.g.Snapshot()) != 1 {
		t.Fatalf("the loaded colony has %d colonists, want 1", len(j.g.Snapshot()))
	}
}
