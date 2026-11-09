package ebiten

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/hud"
	"rimworld-go/internal/adapters/inbound/ebiten/options"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/outbound/filesave"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
)

// Background saves (CI-002): SaveHooks.Queue set, so Save and SaveView only
// start a save and the queue's worker writes it. These are the async twins
// of the synchronous save tests (saves_test.go, saves_commit_test.go,
// options_test.go, journey_options_test.go), which keep a nil Queue.

// realQueue is a filesave.Queue as internal/compose/game wires it. It
// also records every result it hands the options screen (reported), so a
// test can see a save was reported whichever Update polled it.
type realQueue struct {
	*filesave.Queue
	reported *[]SaveResult
}

func (q realQueue) Done() []SaveResult {
	var out []SaveResult
	for _, r := range q.Queue.Done() {
		if !r.View {
			out = append(out, SaveResult{Slot: r.Slot, Err: r.Err})
		}
	}
	*q.reported = append(*q.reported, out...)
	return out
}

// realSaves are save hooks over a real queue writing into dir.
func realSaves(t *testing.T, dir string) SaveHooks {
	t.Helper()
	store := filesave.Store{Dir: dir}
	q := filesave.NewQueue(store)
	t.Cleanup(q.Close)
	return SaveHooks{
		Queue: realQueue{q, new([]SaveResult)},
		Save:  func(sim Simulation, slot string) error { return q.Save(sim.(*game.Game), slot) },
		List: func() ([]SaveSlot, error) {
			slots, err := store.List()
			out := make([]SaveSlot, 0, len(slots))
			for _, sl := range slots {
				out = append(out, SaveSlot{Name: sl.Name, Path: sl.Path, Date: sl.Info.Date, Colonists: sl.Info.Colonists, Modified: sl.Modified})
			}
			return out, err
		},
		Load: func(path string) (Simulation, string, error) {
			g, info, err := store.Load(path)
			if err != nil {
				return nil, "", err
			}
			return g, info.Name, nil
		},
		SaveView: func(slot string, v map[game.MapID]camera.View) error {
			files := map[game.MapID]filesave.ViewFile{}
			for id, c := range v {
				files[id] = filesave.ViewFile{CX: c.CX, CY: c.CY, Zoom: c.Zoom}
			}
			return q.SaveView(slot, filesave.ViewsOf(files))
		},
	}
}

// flushSaves waits for the background saves and reports them, as the next
// Update would once they are written.
func flushSaves(a *App) {
	if q := a.cfg.Saves.Queue; q != nil {
		q.Wait()
	}
	a.opt.PollSaves()
}

// fakeQueue is a background writer the test finishes by hand.
type fakeQueue struct {
	busy  bool
	done  []SaveResult
	waits int
}

func (q *fakeQueue) Busy() bool { return q.busy }
func (q *fakeQueue) Done() []SaveResult {
	out := q.done
	q.done = nil
	return out
}
func (q *fakeQueue) Wait() { q.waits++ }

// queued gives the saveLog's colony a fake queue: each save starts, and
// finishes with log.fail when the test next polls or waits.
func queued(a *App, log *saveLog) *fakeQueue {
	q := &fakeQueue{}
	start := func(slot string) error {
		q.done = append(q.done, SaveResult{Slot: slot, Err: log.fail})
		return nil
	}
	a.cfg.Saves.Queue = q
	a.cfg.Saves.Save = func(_ Simulation, slot string) error { log.saves = append(log.saves, slot); return start(slot) }
	a.cfg.Saves.SaveCommit = func(_ Simulation, slot string) error {
		log.commits = append(log.commits, slot)
		return start(slot)
	}
	return q
}

func dangerToast(a *App, text string) bool {
	for _, ts := range a.chrome.LiveToasts() {
		if ts.Danger && ts.Text == text {
			return true
		}
	}
	return false
}

// TestJourneySaveAsyncAutosaveRotation: the rotation of
// TestJourneyOptionsAutosaveRotation through a real queue in a temp folder.
func TestJourneySaveAsyncAutosaveRotation(t *testing.T) {
	t.Parallel()
	j, _ := savesJourney(t)
	dir := t.TempDir()
	j.a.cfg.Saves = realSaves(t, dir)
	j.a.settings.General.AutosavesKept = 3
	j.a.settings.Gameplay.AutosaveDays = 1
	var wrote []string
	for day := 1; day <= 4; day++ {
		morning := uint64(day)*clock.TicksPerDay + options.AutosaveHour*clock.TicksPerHour
		if d := morning - 5 - uint64(j.g.Tick()); d > 0 {
			j.g.Step(int(d))
		}
		j.tick(10)
		flushSaves(j.a)
		slots, err := j.a.cfg.Saves.List() // newest first
		if err != nil || len(slots) != min(day, 3) {
			t.Fatalf("day %d: %d saves on disk (%v), want %d", day, len(slots), err, min(day, 3))
		}
		wrote = append(wrote, slots[0].Name)
	}
	want := []string{"col-autosave-1", "col-autosave-2", "col-autosave-3", "col-autosave-1"}
	if fmt.Sprint(wrote) != fmt.Sprint(want) {
		t.Fatalf("autosaves went to %v, want %v (oldest first)", wrote, want)
	}
	for _, n := range want[:3] {
		if _, err := os.Stat(filepath.Join(dir, n+filesave.Ext)); err != nil {
			t.Fatalf("no file for %s: %v", n, err)
		}
	}
}

// TestJourneySaveAsyncLoadsTheSavedTick: the twin of
// TestSaveFromPauseThenLoad. The colony steps on after Pause › Save (with
// -race: while the worker writes); the Load list waits for the file and
// lists it, the save is reported exactly once, and it loads to the state
// of the save's tick. The report comes from whichever call finds the save
// written: an Update's PollSaves (the hint then shows until the Load click
// clears it, as any click does) or the Load list's own wait (the hint then
// shows on the list).
func TestJourneySaveAsyncLoadsTheSavedTick(t *testing.T) {
	t.Parallel()
	j, _ := savesJourney(t)
	j.a.cfg.Saves = realSaves(t, t.TempDir())
	reported := j.a.cfg.Saves.Queue.(realQueue).reported
	j.key("Escape", 0)
	tick, hash := j.g.Tick(), j.g.StateHash() // the pause menu holds the clock
	j.clickID("pause.save")
	j.g.Step(600) // while the worker may still be writing
	j.key("Escape", 0)
	j.update(1)
	early := len(*reported) // reported by an Update before the Load click
	if early > 0 && j.a.hint != "saved col" {
		t.Fatalf("save reported by an Update, hint %q, want %q", j.a.hint, "saved col")
	}
	j.clickID("pause.load")
	if j.a.nav.screen() != ports.ScreenLoad || len(j.a.opt.Slots) != 1 {
		t.Fatalf("load list %s with %+v", j.a.nav.screen(), j.a.opt.Slots)
	}
	if want := []SaveResult{{Slot: "col"}}; fmt.Sprint(*reported) != fmt.Sprint(want) {
		t.Fatalf("reported %+v, want %+v once", *reported, want)
	}
	if early == 0 && j.a.hint != "saved col" {
		t.Fatalf("the Load list reported the save, hint %q, want %q", j.a.hint, "saved col")
	}
	sim, _, err := j.a.cfg.Saves.Load(j.a.opt.Slots[0].Path) // the file as written, before a loaded colony steps
	if g, ok := sim.(*game.Game); err != nil || !ok || g.Tick() != tick || g.StateHash() != hash {
		t.Fatalf("file: %v (%T); want the save's tick %d and state hash %x", err, sim, tick, hash)
	}
	j.clickID("load.slot.0")
	if g, ok := j.a.renderer.game.(*game.Game); j.a.nav.screen() != ports.ScreenColony || !ok || g == j.g {
		t.Fatalf("after load: %s", j.a.nav.screen())
	}
}

// TestJourneySaveAsyncFailure: a save the queue cannot write (the folder
// is under a regular file) is reported by the hint and a danger toast.
func TestJourneySaveAsyncFailure(t *testing.T) {
	t.Parallel()
	j, _ := savesJourney(t)
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	j.a.cfg.Saves = realSaves(t, filepath.Join(file, "saves"))
	j.key("Escape", 0)
	j.clickID("pause.save")
	j.a.cfg.Saves.Queue.Wait()
	j.update(1) // the next Update reports it
	if !strings.HasPrefix(j.a.hint, "save failed: ") || !dangerToast(j.a, j.a.hint) {
		t.Fatalf("hint %q, toasts %+v: want the failure in both", j.a.hint, j.a.chrome.LiveToasts())
	}
	j.draw(1)
}

// TestJourneySaveAsyncIndicator: the Saving row shows while the queue is
// busy, from the HUD's Env, and stays its minimum once the save is done.
func TestJourneySaveAsyncIndicator(t *testing.T) {
	t.Parallel()
	j, _ := savesJourney(t)
	q := &fakeQueue{busy: true}
	j.a.cfg.Saves.Queue = q
	j.update(1)
	if _, ok := j.a.chrome.SavingRect(j.a.sw, j.a.sh); !ok {
		t.Fatal("no Saving row while the queue is busy")
	}
	j.draw(1)
	q.busy = false
	j.update(hud.SavingMinFrames - 1)
	if !j.a.chrome.SavingShown() {
		t.Fatal("the Saving row flickered off before its minimum")
	}
	j.update(1)
	if j.a.chrome.SavingShown() {
		t.Fatal("the Saving row outstayed the save")
	}
}

// TestCommitmentAsyncSaveAndQuit: the twin of
// TestCommitmentSavesOnReturnToMenu. Leaving waits for the save and
// checks it: a failure keeps the player in the colony.
func TestCommitmentAsyncSaveAndQuit(t *testing.T) {
	for _, id := range []string{"pause.menu", "pause.savequit"} {
		log := &saveLog{}
		a, _ := newCommitColony(t, log)
		q := queued(a, log)
		fakeWin(a)
		a.apply(intent{kind: intentPause})
		click(t, a, id)
		if a.nav.screen() != ports.ScreenMainMenu || q.waits == 0 || len(log.commits) != 1 {
			t.Fatalf("%s: on %s after %d waits, saves %v", id, a.nav.screen(), q.waits, log.commits)
		}
	}
	log := &saveLog{fail: errors.New("disk full")}
	a, _ := newCommitColony(t, log)
	queued(a, log)
	a.apply(intent{kind: intentPause})
	click(t, a, "pause.savequit")
	if a.nav.screen() != ports.ScreenColony || a.renderer == nil || a.hint != "save failed: disk full" || !dangerToast(a, a.hint) {
		t.Fatalf("after a failed save: %s, hint %q", a.nav.screen(), a.hint)
	}
	log = &saveLog{}
	a, _ = newCommitColony(t, log)
	q := queued(a, log)
	a.apply(intent{kind: intentPause})
	click(t, a, "pause.quit")
	if !a.shouldQuit || len(log.commits) != 1 || q.waits == 0 {
		t.Fatalf("quit: shouldQuit %v, saves %v, waits %d", a.shouldQuit, log.commits, q.waits)
	}
}

// TestCommitmentAsyncSavesOnClose: the twin of TestCommitmentSavesOnClose:
// closing the window waits for the save, and closes even when it failed.
func TestCommitmentAsyncSavesOnClose(t *testing.T) {
	for _, fail := range []error{nil, errors.New("read-only")} {
		log := &saveLog{fail: fail}
		a, _ := newCommitColony(t, log)
		q := queued(a, log)
		w := fakeWin(a)
		w.closing = true
		if err := a.Update(); !errors.Is(err, ebiten.Termination) || len(log.commits) != 1 || q.waits == 0 {
			t.Fatalf("close (fail %v): %v, saves %v, waits %d", fail, err, log.commits, q.waits)
		}
	}
}

// TestReloadAnytimeAsyncSaves: the twin of TestReloadAnytimeUnchanged and
// TestAutosaveInterval: the same saves to the same slots, reported once
// written; leaving does not save.
func TestReloadAnytimeAsyncSaves(t *testing.T) {
	log := &saveLog{}
	s := &stubSim{w: 4, h: 4}
	a := startColony(t, s, log)
	q := queued(a, log)
	slot := a.opt.SaveSlot
	a.apply(intent{kind: intentPause})
	click(t, a, "pause.save")
	if len(log.saves) != 1 || log.saves[0] != slot || len(log.commits) != 0 {
		t.Fatalf("manual save %v (commit %v)", log.saves, log.commits)
	}
	flushSaves(a)
	if a.hint != "saved "+slot {
		t.Fatalf("hint %q once written", a.hint)
	}
	a.settings.Gameplay.AutosaveDays = 2
	for day := 1; day <= 5; day++ {
		s.tick = morning(day)
		a.opt.TickAutosave()
	}
	if fmt.Sprint(log.saves[1:]) != fmt.Sprint([]string{slot + "-autosave", slot + "-autosave"}) {
		t.Fatalf("autosaves %v, want days 2 and 4", log.saves[1:])
	}
	if len(q.done) != 2 {
		t.Fatalf("%d saves waiting, want the two autosaves", len(q.done))
	}
	a.apply(intent{kind: intentReturnToMenu})
	if a.nav.screen() != ports.ScreenMainMenu || len(log.saves) != 3 {
		t.Fatalf("return to menu: %s, saves %v", a.nav.screen(), log.saves)
	}
}
