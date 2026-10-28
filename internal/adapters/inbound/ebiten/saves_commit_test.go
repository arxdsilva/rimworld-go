package ebiten

import (
	"errors"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
)

// Commitment mode in play (NG-1 §2.13). These tests are not parallel:
// they swap the package's window hooks for a fake, and a commitment
// colony's Update tells the window whether the game handles its close.

// commitSim is a stub colony started in commitment mode.
type commitSim struct{ *stubSim }

func (commitSim) Story() game.StoryView { return game.StoryView{Commitment: true} }

// fakeWindow records close handling and closes on demand.
type fakeWindow struct {
	handled []bool
	closing bool
}

func (w *fakeWindow) SetClosingHandled(v bool) { w.handled = append(w.handled, v) }
func (w *fakeWindow) BeingClosed() bool        { return w.closing }

// fakeWin gives a the fake window hooks (AppConfig.WindowHooks is the
// real window by default); the colony fixtures run Updates of their own,
// so the close handling is announced again to the fake.
func fakeWin(a *App) *fakeWindow {
	w := &fakeWindow{}
	a.cfg.WindowHooks, a.closeHandled = w, false // the fake hears the state on the next Update
	return w
}

// saveLog records the save hooks' calls.
type saveLog struct {
	saves, commits []string
	fail           error
}

func (l *saveLog) hooks() SaveHooks {
	return SaveHooks{
		Save: func(_ Simulation, slot string) error { l.saves = append(l.saves, slot); return l.fail },
		SaveCommit: func(_ Simulation, slot string) error {
			l.commits = append(l.commits, slot)
			return l.fail
		},
		List: func() ([]SaveSlot, error) { return nil, nil },
		Load: func(path string) (Simulation, string, error) { return nil, "", errors.New("unused") },
	}
}

// startColony starts sim through the wizard with saves wired to log.
func startColony(t *testing.T, sim Simulation, log *saveLog) *App {
	t.Helper()
	a := testApp(sim)
	a.cfg.Saves = log.hooks()
	a.finishBoot()
	quickStart(t, a)
	if a.opt.SaveSlot == "" {
		t.Fatal("a new colony has no save slot")
	}
	return a
}

func newCommitColony(t *testing.T, log *saveLog) (*App, *stubSim) {
	t.Helper()
	s := &stubSim{w: 4, h: 4}
	return startColony(t, commitSim{s}, log), s
}

// morning is tick of day's autosave hour.
func morning(day int) clock.Tick {
	return clock.Tick(day*clock.TicksPerDay + autosaveHour*clock.TicksPerHour)
}

func buttonByID(t *testing.T, bs []button, id string) button {
	t.Helper()
	b, ok := hasButton(bs, id) // home_carpet_ui_test.go
	if !ok {
		t.Fatalf("no button %q", id)
	}
	return b
}

func lacksButton(bs []button, id string) bool {
	_, ok := hasButton(bs, id)
	return !ok
}

func TestCommitmentSingleSlotNoAutosaveCopy(t *testing.T) {
	log := &saveLog{}
	a, s := newCommitColony(t, log)
	fakeWin(a)
	a.settings.Gameplay.AutosaveDays = 7 // commitment still saves daily
	for day := 1; day <= 3; day++ {
		s.tick = morning(day)
		a.tickAutosave()
		a.tickAutosave() // once per morning
	}
	s.tick = morning(3) + clock.TicksPerHour // not the autosave hour
	a.tickAutosave()
	if len(log.commits) != 3 || len(log.saves) != 0 {
		t.Fatalf("commit saves %v, plain saves %v: want one SaveCommit a day", log.commits, log.saves)
	}
	for _, slot := range log.commits {
		if slot != a.opt.SaveSlot {
			t.Fatalf("commitment saved to %q, want its one slot %q (no -autosave copy)", slot, a.opt.SaveSlot)
		}
	}
	// Options' "Off" does not turn a commitment colony's saving off.
	a.settings.Gameplay.AutosaveDays = 0
	s.tick = morning(4)
	a.tickAutosave()
	if len(log.commits) != 4 {
		t.Fatalf("autosave off must not stop commitment saves: %v", log.commits)
	}
	// Without SaveCommit wired, Save writes the same single slot.
	log2 := &saveLog{}
	b, s2 := newCommitColony(t, log2)
	b.cfg.Saves.SaveCommit = nil
	s2.tick = morning(1)
	b.tickAutosave()
	if len(log2.saves) != 1 || log2.saves[0] != b.opt.SaveSlot {
		t.Fatalf("fallback saves %v, want [%s]", log2.saves, b.opt.SaveSlot)
	}
}

func TestCommitmentPauseButtons(t *testing.T) {
	a, _ := newCommitColony(t, &saveLog{})
	fakeWin(a)
	a.apply(intent{kind: intentPause})
	bs := a.pauseButtons(a.size())
	if !lacksButton(bs, "pause.save") {
		t.Fatal("commitment has no plain Save")
	}
	if b := buttonByID(t, bs, "pause.savequit"); b.Label != "Save & Quit to Menu" || b.Disabled {
		t.Fatalf("save & quit button %+v", b)
	}
	if b := buttonByID(t, bs, "pause.load"); !b.Disabled || b.Reason != "Commitment: one timeline" {
		t.Fatalf("load button %+v", b)
	}
	click(t, a, "pause.load")
	if a.nav.screen() != screenColony || a.hint != "Commitment: one timeline" {
		t.Fatalf("Load must stay disabled: screen %s, hint %q", a.nav.screen(), a.hint)
	}
	// Developer mode is locked in Options while the colony runs.
	a.settings.Gameplay.DevMode = false
	click(t, a, "pause.options")
	click(t, a, "options.tab.gameplay")
	dev := buttonByID(t, a.optionsButtons(a.size()), "options.devmode")
	if !dev.Disabled || dev.Reason != "Commitment mode" {
		t.Fatalf("dev mode row %+v", dev)
	}
	click(t, a, "options.devmode")
	if a.settings.Gameplay.DevMode || a.hint != "Commitment mode" {
		t.Fatalf("dev mode toggled in commitment (hint %q)", a.hint)
	}
	if b := buttonByID(t, a.optionsButtons(a.size()), "options.autosave"); b.Disabled {
		t.Fatal("only developer mode is locked")
	}
}

func TestCommitmentSavesOnReturnToMenu(t *testing.T) {
	for _, id := range []string{"pause.menu", "pause.savequit"} {
		log := &saveLog{}
		a, _ := newCommitColony(t, log)
		fakeWin(a)
		slot := a.opt.SaveSlot
		a.apply(intent{kind: intentPause})
		click(t, a, id)
		if a.nav.screen() != screenMainMenu || a.renderer != nil {
			t.Fatalf("%s: on %s, want the main menu", id, a.nav.screen())
		}
		if len(log.commits) != 1 || log.commits[0] != slot {
			t.Fatalf("%s: saves %v, want [%s]", id, log.commits, slot)
		}
	}
	// A failed save keeps the player in the colony, with the reason.
	log := &saveLog{fail: errors.New("disk full")}
	a, _ := newCommitColony(t, log)
	a.apply(intent{kind: intentPause})
	click(t, a, "pause.savequit")
	if a.nav.screen() != screenColony || a.renderer == nil || a.hint != "save failed: disk full" {
		t.Fatalf("after a failed save: %s, hint %q", a.nav.screen(), a.hint)
	}
	// Quit saves too.
	log = &saveLog{}
	a, _ = newCommitColony(t, log)
	a.apply(intent{kind: intentPause})
	click(t, a, "pause.quit")
	if !a.shouldQuit || len(log.commits) != 1 {
		t.Fatalf("quit: shouldQuit %v, saves %v", a.shouldQuit, log.commits)
	}
}

func TestCommitmentSavesOnClose(t *testing.T) {
	log := &saveLog{}
	a, _ := newCommitColony(t, log)
	w := fakeWin(a)
	if err := a.Update(); err != nil {
		t.Fatal(err)
	}
	if len(w.handled) != 1 || !w.handled[0] {
		t.Fatalf("a commitment colony must handle the window close: %v", w.handled)
	}
	if err := a.Update(); err != nil || len(w.handled) != 1 {
		t.Fatalf("close handling is set once (%v, %v)", w.handled, err)
	}
	w.closing = true
	if err := a.Update(); !errors.Is(err, ebiten.Termination) {
		t.Fatalf("closing the window: Update = %v, want Termination", err)
	}
	if len(log.commits) != 1 || log.commits[0] != a.opt.SaveSlot {
		t.Fatalf("saves on close %v", log.commits)
	}
	// Even when that save fails the window closes.
	log2 := &saveLog{fail: errors.New("read-only")}
	b, _ := newCommitColony(t, log2)
	w2 := fakeWin(b)
	w2.closing = true
	if err := b.Update(); !errors.Is(err, ebiten.Termination) || len(log2.commits) != 1 {
		t.Fatalf("failed save on close: %v, saves %v", err, log2.commits)
	}
	// Back on the main menu the window closes normally again.
	c, _ := newCommitColony(t, &saveLog{})
	w3 := fakeWin(c)
	_ = c.Update()
	c.apply(intent{kind: intentReturnToMenu})
	_ = c.Update()
	if len(w3.handled) != 2 || !w3.handled[0] || w3.handled[1] {
		t.Fatalf("close handling %v, want [true false]", w3.handled)
	}
}

func TestReloadAnytimeUnchanged(t *testing.T) {
	log := &saveLog{}
	s := &stubSim{w: 4, h: 4}
	a := startColony(t, s, log)
	w := fakeWin(a)
	slot := a.opt.SaveSlot
	a.apply(intent{kind: intentPause})
	bs := a.pauseButtons(a.size())
	if b := buttonByID(t, bs, "pause.save"); b.Label != "Save" || b.Disabled {
		t.Fatalf("save button %+v", b)
	}
	if b := buttonByID(t, bs, "pause.load"); b.Disabled {
		t.Fatalf("load button %+v", b)
	}
	if !lacksButton(bs, "pause.savequit") {
		t.Fatal("reload anytime has no Save & Quit")
	}
	click(t, a, "pause.save")
	if len(log.saves) != 1 || log.saves[0] != slot || len(log.commits) != 0 {
		t.Fatalf("manual save %v (commit %v)", log.saves, log.commits)
	}
	// Autosave follows Options, to the -autosave copy.
	a.settings.Gameplay.AutosaveDays = 2
	s.tick = morning(1)
	a.tickAutosave()
	s.tick = morning(2)
	a.tickAutosave()
	if len(log.saves) != 2 || log.saves[1] != slot+"-autosave" {
		t.Fatalf("autosaves %v", log.saves)
	}
	a.settings.Gameplay.AutosaveDays = 0
	s.tick = morning(5)
	a.tickAutosave()
	if len(log.saves) != 2 {
		t.Fatal("autosave Off must not save")
	}
	// Developer mode stays free; the window closes as before; leaving does
	// not save.
	a.apply(intent{kind: intentPause})
	click(t, a, "pause.options")
	click(t, a, "options.tab.gameplay")
	if b := buttonByID(t, a.optionsButtons(a.size()), "options.devmode"); b.Disabled {
		t.Fatal("developer mode locked without commitment")
	}
	_ = a.Update()
	if len(w.handled) != 0 {
		t.Fatalf("reload anytime must not take over the close button: %v", w.handled)
	}
	a.apply(intent{kind: intentReturnToMenu})
	if a.nav.screen() != screenMainMenu || len(log.saves) != 2 {
		t.Fatalf("return to menu: %s, saves %v", a.nav.screen(), log.saves)
	}
}

func TestLoadListTagsCommitment(t *testing.T) {
	t.Parallel()
	a := testApp(&stubSim{w: 4, h: 4})
	a.opt.Slots = []SaveSlot{{Name: "a", Path: "a", Commitment: true}, {Name: "b", Path: "b"}}
	if bs := a.loadButtons(1280, 720); len(bs) != 3 || bs[1].Disabled {
		t.Fatalf("a commitment save is listed and loadable: %+v", bs)
	}
}
