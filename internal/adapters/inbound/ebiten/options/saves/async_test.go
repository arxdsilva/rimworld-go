package saves

import (
	"errors"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/ports"
)

// asyncShell is the fake with background saves: Save only starts one,
// Finished hands back what the test queued, Wait counts.
type asyncShell struct {
	*fakeShell
	done   []SaveResult
	waits  int
	alarms []string
}

func (a *asyncShell) SaveEnv() Env {
	e := a.fakeShell.SaveEnv()
	e.AsyncSaves = true
	return e
}
func (a *asyncShell) Finished() []SaveResult {
	out := a.done
	a.done = nil
	return out
}
func (a *asyncShell) Wait()                  { a.waits++ }
func (a *asyncShell) SaveFailed(text string) { a.alarms = append(a.alarms, text) }

func newAsync() (*asyncShell, *Book) {
	f, _ := newFake()
	a := &asyncShell{fakeShell: f}
	a.env = Env{CanSave: true, CanLoad: true}
	return a, New(a)
}

// TestBackgroundSaveReportsWhenWritten: SaveNow says nothing while the
// save is written; PollSaves hints the result and raises a failure.
func TestBackgroundSaveReportsWhenWritten(t *testing.T) {
	t.Parallel()
	a, s := newAsync()
	if !s.SaveNow("col") || len(a.hints) != 0 || len(a.saved) != 1 {
		t.Fatalf("started: hints %v, saved %v", a.hints, a.saved)
	}
	s.PollSaves()
	if len(a.hints) != 0 {
		t.Fatalf("a hint before the save finished: %v", a.hints)
	}
	a.done = []SaveResult{{Slot: "col"}}
	s.PollSaves()
	if len(a.hints) != 1 || a.hints[0] != "saved col" || len(a.alarms) != 0 {
		t.Fatalf("written: hints %v, alarms %v", a.hints, a.alarms)
	}
	a.done = []SaveResult{{Slot: "col", Err: errDisk}}
	s.PollSaves()
	if a.hints[1] != "save failed: disk full" || len(a.alarms) != 1 || a.alarms[0] != a.hints[1] {
		t.Fatalf("failed: hints %v, alarms %v", a.hints, a.alarms)
	}
	// A save that cannot start fails at once, as before.
	a.saveErr = errors.New("encode")
	if s.SaveNow("col") || a.hints[2] != "save failed: encode" {
		t.Fatalf("failed to start: hints %v", a.hints)
	}
}

// TestBackgroundSaveBeforeLeaving: a commitment colony waits for its save
// and stays when it failed; the load list waits for a save in flight.
func TestBackgroundSaveBeforeLeaving(t *testing.T) {
	t.Parallel()
	a, s := newAsync()
	a.env.Commitment = true
	s.SaveSlot = "one"
	a.done = []SaveResult{{Slot: "one"}}
	if !s.SaveBeforeLeaving() || a.waits != 1 {
		t.Fatalf("a written save must let the player leave (waits %d)", a.waits)
	}
	a.done = []SaveResult{{Slot: "one", Err: errDisk}}
	if s.SaveBeforeLeaving() || len(a.alarms) != 1 {
		t.Fatalf("a failed save must keep the player (alarms %v)", a.alarms)
	}
	// An earlier failure is not this save's: the last one of the slot counts.
	a.done = []SaveResult{{Slot: "one", Err: errDisk}, {Slot: "one"}}
	if !s.SaveBeforeLeaving() {
		t.Fatal("the last save of the slot was written")
	}
	s.RefreshSlots()
	s.Slots = []ports.SaveSlot{{Name: "one", Path: "one.rwsave"}}
	s.LoadSlot(0)
	if a.waits != 5 {
		t.Fatalf("waits %d: leaving, the list and loading each wait", a.waits)
	}
}
