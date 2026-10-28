package wizard

import (
	"errors"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/scenario"
)

// toWorld opens New Colony on a fake shell and walks to Create World.
func toWorld(t *testing.T) (*fakeShell, *Flow) {
	t.Helper()
	sh, w := newFakeShell(t)
	if !w.Click(ids.MenuNewGame) || sh.top() != ports.ScreenScenario || sh.kept == nil || sh.kept != w.Draft() {
		t.Fatalf("New Colony opens Choose Scenario with a fresh draft: %s kept %v", sh.top(), sh.kept)
	}
	sh.idle()
	sh.click(ids.WizardNext)
	if sh.top() != ports.ScreenStoryteller {
		t.Fatalf("Next on Choose Scenario: %s", sh.top())
	}
	sh.idle()
	sh.click(ids.WizardNext)
	if sh.top() != ports.ScreenWorldGen || sh.saved != 1 || sh.settings.Gameplay.LastStoryteller != w.Draft().Storyteller {
		t.Fatalf("Next on Choose Storyteller stores the choices: %s saved %d", sh.top(), sh.saved)
	}
	sh.idle()
	return sh, w
}

// generate makes a small world and steps it to the landing screen.
func generate(t *testing.T, sh *fakeShell, w *Flow) {
	t.Helper()
	w.Draft().World.Coverage = 5
	sh.idle()
	sh.click("world.generate")
	if w.Draft().Builder == nil || sh.kit.Top() != WorldProgress {
		t.Fatalf("Generate starts the builder behind the progress dialog (modal %q)", sh.kit.Top())
	}
	w.Draft().GenStep = 1 << 22
	for i := 0; i < 200 && w.Draft().Builder != nil; i++ {
		w.WorldStep()
	}
	if w.Draft().Planet == nil || sh.top() != ports.ScreenLanding {
		t.Fatalf("a finished world opens the landing site: %s", sh.top())
	}
}

func TestFlowWalksEveryScreenToStart(t *testing.T) {
	t.Parallel()
	sh, w := toWorld(t)
	generate(t, sh, w)
	sh.idle()
	sh.click("land.random")
	if w.Draft().Tile < 0 {
		t.Fatal("Select random site picks a tile")
	}
	sh.click("land.advanced")
	sh.idle()
	sh.click("modal.land.advanced.close")
	sh.click("land.factions")
	sh.idle()
	sh.click("modal." + FacModalID + ".close")
	sh.click("land.legend")
	sh.click(ids.WizardNext)
	if sh.top() != ports.ScreenPawns {
		t.Fatalf("Next on the landing site: %s", sh.top())
	}
	sh.idle()
	if w.Draft().Pool == nil || w.PoolKeyFor(w.Draft()) != w.Draft().PoolKey {
		t.Fatal("Create Characters makes the candidate pool")
	}
	sh.click("pawns.reroll")
	sh.loader = &fakeLoader{need: 3}
	sh.click(ids.WizardStart)
	if sh.loads != 1 || w.Draft().Load == nil {
		t.Fatalf("Start loads the setup: loads %d", sh.loads)
	}
	if got := sh.loader.setup; got.Tile != w.Draft().Tile || got.Seed != SetupSeedOf(w.Draft()) || len(got.Colonists) == 0 {
		t.Fatalf("the setup comes from the draft: %+v", got)
	}
	sh.frame(sh.f.Raw) // the loading screen steps the builder and draws
	for i := 0; i < 5 && sh.started == 0; i++ {
		sh.idle()
	}
	if sh.started != 1 || sh.kept != nil || w.d != nil {
		t.Fatalf("a finished builder starts the colony and drops the draft: started %d kept %v", sh.started, sh.kept)
	}
}

func TestFlowStartFailureKeepsTheDraft(t *testing.T) {
	t.Parallel()
	sh, w := newFakeShell(t)
	w.Click(ids.MenuNewGame)
	sh.stack = append(sh.stack, ports.ScreenPawns)
	w.beginLoading() // no planet: the setup is incomplete
	if sh.loads != 1 || w.Draft().Load != nil {
		t.Fatal("an incomplete setup is refused by the shell")
	}
	w.Draft().Load = &loadState{b: &fakeLoader{s: sh, need: 1, err: errors.New("disk full")}}
	w.loadingStep()
	if w.d == nil || w.d.Load != nil || sh.started != 0 {
		t.Fatal("a failed build returns to Create Characters with the draft")
	}
}

func TestFlowBackAsksBeforeDiscarding(t *testing.T) {
	t.Parallel()
	sh, w := newFakeShell(t)
	w.Click(ids.MenuNewGame)
	sh.idle()
	// Nothing changed: Back pops to the menu and forgets the draft.
	sh.click(ids.WizardBack)
	if sh.top() != ports.ScreenMainMenu || sh.kept != nil {
		t.Fatalf("Back from an untouched setup: %s kept %v", sh.top(), sh.kept)
	}
	// A later step changed: Back asks first.
	w.Click(ids.MenuNewGame)
	w.Draft().Preset = difficulty.Custom
	sh.idle()
	sh.click(ids.WizardBack)
	if sh.top() != ports.ScreenScenario || sh.kit.Top() != "scen.discard" {
		t.Fatalf("Back asks: %s modal %q", sh.top(), sh.kit.Top())
	}
	sh.click("modal.scen.discard.ok")
	if sh.top() != ports.ScreenMainMenu || sh.kept != nil || w.confirmed {
		t.Fatalf("Discard pops: %s kept %v", sh.top(), sh.kept)
	}
	// Back is refused while the colony is built.
	w.Click(ids.MenuNewGame)
	w.Draft().Load = &loadState{b: &fakeLoader{s: sh, need: 9}}
	if w.GoBack() {
		t.Fatal("no Back while loading")
	}
}

func TestFlowEditorRoundTrip(t *testing.T) {
	t.Parallel()
	sh, w := newFakeShell(t)
	w.Click(ids.MenuNewGame)
	sh.idle()
	sh.click("scen.new")
	if sh.top() != ports.ScreenScenarioEditor || w.Draft().Ed == nil {
		t.Fatalf("New scenario opens the editor: %s", sh.top())
	}
	sh.idle()
	sh.click("sced.add")
	sh.idle()
	sh.click("sced.addkind." + scenario.PartStartAnimal.Key())
	sh.idle()
	sh.click("sced.save")
	if len(sh.store.(*memStore).files) != 1 {
		t.Fatal("Save writes the scenario file")
	}
	w.Draft().Ed.Def.Summary = "changed after saving"
	sh.idle()
	sh.click("sced.done")
	if sh.top() != ports.ScreenScenario || w.Draft().Ed != nil || len(w.Unsaved) != 1 {
		t.Fatalf("Done keeps a changed scenario as unsaved: %s ed %v unsaved %d", sh.top(), w.Draft().Ed, len(w.Unsaved))
	}
	if sel := w.Draft().Selected(); sel == nil || !sel.Unsaved {
		t.Fatalf("the kept scenario is selected: %+v", sel)
	}
	sh.click("scen.import")
	sh.idle()
	if sh.kit.Top() != "scen.load" {
		t.Fatalf("Load file lists the folder: %q", sh.kit.Top())
	}
}

func TestFlowStorytellerAndCustomDifficulty(t *testing.T) {
	t.Parallel()
	sh, w := newFakeShell(t)
	w.Click(ids.MenuNewGame)
	sh.idle()
	sh.click(ids.WizardNext)
	sh.idle()
	sh.click("diff.custom")
	sh.idle()
	if sh.kit.Top() != "diff.custom" {
		t.Fatalf("the first Custom opens Custom Difficulty: %q", sh.kit.Top())
	}
	sh.click("diff.f.pd.inc")
	sh.click("modal.diff.custom.accept")
	sh.idle()
	sh.click("diff.customize") // Custom is selected: Customize opens it again
	if sh.kit.Top() != "diff.custom" {
		t.Fatalf("Customize reopens Custom Difficulty: %q", sh.kit.Top())
	}
	if w.Draft().Preset != difficulty.Custom || w.Draft().Custom.PeacefulDays != difficulty.Of(difficulty.Settled).PeacefulDays+1 {
		t.Fatalf("Accept stores Custom: %v %d", w.Draft().Preset, w.Draft().Custom.PeacefulDays)
	}
}

func TestFlowShellHelpers(t *testing.T) {
	t.Parallel()
	sh, w := newFakeShell(t)
	if got := w.CoverageOptions(); len(got) == 0 || got[0] == 5 {
		t.Fatalf("coverages without dev mode: %v", got)
	}
	if w.Buttons(ports.ScreenScenario, 1280, 720) != nil {
		t.Fatal("the wizard's controls are kit widgets")
	}
	if w.Click("colony.nothing") || !w.Click("scen.stale") {
		t.Fatal("Click answers only the wizard's ids")
	}
	for s := ports.ScreenScenario; s <= ports.ScreenPawns; s++ {
		if Next(s) == s && s != ports.ScreenPawns {
			t.Fatalf("Next(%s) stays", s)
		}
	}
	if Next(ports.ScreenColony) != ports.ScreenColony {
		t.Fatal("Next past the wizard stays")
	}
	sp := DiffSpec("ts")
	if sp.Min >= sp.Max || sp.Step <= 0 {
		t.Fatalf("the threat scale's range: %+v", sp)
	}
	_ = sh
}
