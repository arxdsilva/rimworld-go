package saves

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/domain/sim/clock"
)

// kitShell gives a fake shell a widget kit for the book's questions.
type kitShell struct {
	Shell
	kit *uikit.Kit
}

func (k kitShell) Kit() *uikit.Kit { return k.kit }

// withKit is a book over sh with a kit, and the kit.
func withKit(sh Shell) (*Book, *uikit.Kit) {
	k := &uikit.Kit{}
	return New(kitShell{sh, k}), k
}

// answer presses button id of the open modal m, as the kit does: its
// OnClick, then it closes.
func answer(t *testing.T, k *uikit.Kit, m, id string) {
	t.Helper()
	for _, md := range k.Modals {
		if md.ID != m {
			continue
		}
		for _, bt := range md.Buttons {
			if bt.ID == id {
				if bt.OnClick != nil {
					bt.OnClick()
				}
				if bt.Closes {
					k.CloseModal(m)
				}
				return
			}
		}
	}
	t.Fatalf("no button %s in modal %s (open %v)", id, m, k.Top())
}

// reloadAnytime is a running reload-anytime colony at tick 5 hours in,
// never saved.
func reloadAnytime() (*fakeShell, *Book, *uikit.Kit) {
	f, _ := newFake()
	f.env = Env{CanSave: true, CanLoad: true}
	b, k := withKit(f)
	b.SaveSlot = "col"
	b.NewColony(0)
	b.SaveSlot = "col"
	f.tick = 5 * clock.TicksPerHour
	return f, b, k
}

// Unsaved progress asks before leaving: Cancel stays, Leave goes without
// saving, Save & leave saves first; the leave runs at the next PollSaves,
// never inside the modal's callback.
func TestLeaveAsksAboutUnsavedProgress(t *testing.T) {
	t.Parallel()
	f, b, k := reloadAnytime()
	left := 0
	leave := func() { left++ }

	b.Leave(leave)
	if k.Top() != unsavedModal || left != 0 {
		t.Fatalf("modal %q, left %d: want the question and no leave", k.Top(), left)
	}
	answer(t, k, unsavedModal, "cancel")
	b.PollSaves()
	if left != 0 || len(f.saved) != 0 || k.Top() != "" {
		t.Fatalf("Cancel: left %d, saved %v", left, f.saved)
	}

	b.Leave(leave)
	answer(t, k, unsavedModal, "leave")
	if left != 0 {
		t.Fatal("the leave ran inside the modal's callback")
	}
	b.PollSaves()
	b.PollSaves()
	if left != 1 || len(f.saved) != 0 {
		t.Fatalf("Leave: left %d (once), saved %v (nothing)", left, f.saved)
	}

	f.saveErr = errDisk
	b.Leave(leave)
	answer(t, k, unsavedModal, "save")
	b.PollSaves()
	if left != 1 || f.hints[len(f.hints)-1] != "save failed: disk full" {
		t.Fatalf("a failed Save & leave must keep the player: left %d, hints %v", left, f.hints)
	}
	f.saveErr = nil
	b.Leave(leave)
	answer(t, k, unsavedModal, "save")
	b.PollSaves()
	if left != 2 || len(f.saved) != 2 || f.saved[1] != "col" {
		t.Fatalf("Save & leave: left %d, saved %v", left, f.saved)
	}
	// Saved and not changed since: leaving asks nothing.
	b.Leave(leave)
	if left != 3 || k.Top() != "" {
		t.Fatalf("nothing unsaved: left %d, modal %q", left, k.Top())
	}
}

// The question names when the colony was last saved.
func TestUnsavedQuestionNamesTheDate(t *testing.T) {
	t.Parallel()
	f, b, _ := reloadAnytime()
	f.tick = clock.TicksPerDay + 7*clock.TicksPerHour
	b.SaveNow("col")
	f.tick += 3 * clock.TicksPerHour
	if got := b.dateAt(b.progress.savedAt); got != "Day 2, 07:00" {
		t.Fatalf("since %q, want Day 2, 07:00", got)
	}
}

// Save & Quit saves, then proceeds through Leave with nothing left to
// save: a commitment colony is written once, a reload-anytime one is not
// asked. A failed save keeps the player.
func TestSaveAndLeaveSavesOnce(t *testing.T) {
	t.Parallel()
	for _, commit := range []bool{false, true} {
		f, b, k := reloadAnytime()
		f.env.Commitment = commit
		left := 0
		b.SaveAndLeave(func() { b.Leave(func() { left++ }) })
		if left != 1 || len(f.saved) != 1 || f.saved[0] != "col" || k.Top() != "" {
			t.Fatalf("commitment %v: left %d, saved %v, modal %q", commit, left, f.saved, k.Top())
		}
		f.saveErr = errDisk
		f.tick += clock.TicksPerHour
		b.SaveAndLeave(func() { left++ })
		if left != 1 || len(f.saved) != 2 {
			t.Fatalf("commitment %v: a failed save left (%d)", commit, left)
		}
	}
	// With background saves Save & Quit waits for the write.
	a, s := newAsync()
	s.SaveSlot = "col"
	a.done = []SaveResult{{Slot: "col"}}
	left := 0
	s.SaveAndLeave(func() { left++ })
	if left != 1 || a.waits != 1 {
		t.Fatalf("background: left %d, waits %d", left, a.waits)
	}
	a.done = []SaveResult{{Slot: "col", Err: errDisk}}
	s.SaveAndLeave(func() { left++ })
	if left != 1 {
		t.Fatal("a failed background save let the player leave")
	}
}

// A commitment colony saves itself when left unsaved; nothing runs or asks
// without a colony to save.
func TestLeaveWithoutQuestion(t *testing.T) {
	t.Parallel()
	f, b, k := reloadAnytime()
	f.env.Commitment = true
	left := 0
	b.Leave(func() { left++ })
	if left != 1 || len(f.saved) != 1 || k.Top() != "" {
		t.Fatalf("commitment: left %d, saved %v", left, f.saved)
	}
	f.tick += clock.TicksPerHour
	f.saveErr = errDisk
	b.Leave(func() { left++ })
	if left != 1 {
		t.Fatal("a failed commitment save let the player leave")
	}
	f.env = Env{}
	b.Leave(func() { left++ })
	if left != 2 || k.Top() != "" {
		t.Fatalf("no colony: left %d, modal %q", left, k.Top())
	}
}

// The pause menu's status line.
func TestPauseStatus(t *testing.T) {
	t.Parallel()
	f, b, _ := reloadAnytime()
	if got := b.pauseStatus(); got != "Not saved yet" {
		t.Fatalf("status %q", got)
	}
	b.SaveNow("col")
	if got := b.pauseStatus(); got != "Saved less than an in-game hour ago" {
		t.Fatalf("status %q", got)
	}
	f.tick += clock.TicksPerHour
	if got := b.pauseStatus(); got != "Saved 1 in-game hour ago" {
		t.Fatalf("status %q", got)
	}
	f.tick += 2 * clock.TicksPerHour
	if got := b.pauseStatus(); got != "Saved 3 in-game hours ago" {
		t.Fatalf("status %q", got)
	}
	f.env.CanSave = false
	if got := b.pauseStatus(); got != "" {
		t.Fatalf("no colony: status %q", got)
	}
	a, s := newAsync()
	s.SaveNow("col")
	if got := s.pauseStatus(); !strings.HasPrefix(got, "Saving") {
		t.Fatalf("in flight: %q", got)
	}
	_ = a
}
