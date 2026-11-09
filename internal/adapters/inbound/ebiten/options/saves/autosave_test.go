package saves

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/domain/sim/clock"
)

func morning(day int) (clock.Date, clock.Tick) {
	return clock.Date{Hour: AutosaveHour}, clock.Tick(day * clock.TicksPerDay)
}

func TestAutosaveRunsOncePerIntervalToTheAutosaveSlot(t *testing.T) {
	t.Parallel()
	f, s := newFake()
	f.env.CanSave = true
	f.settings.Gameplay.AutosaveDays = 2
	s.SaveSlot = "colony"
	f.date, f.tick = morning(1)
	s.TickAutosave()
	if len(f.saved) != 0 {
		t.Fatalf("day 1 saved %v, want nothing before the interval", f.saved)
	}
	f.date, f.tick = morning(2)
	s.TickAutosave()
	s.TickAutosave() // the same morning again
	if len(f.saved) != 1 || f.saved[0] != "colony-autosave" {
		t.Fatalf("saved %v, want one colony-autosave", f.saved)
	}
}

func TestCommitmentAutosavesItsOwnSlotDaily(t *testing.T) {
	t.Parallel()
	f, s := newFake()
	f.env.CanSave, f.env.Commitment = true, true
	f.settings.Gameplay.AutosaveDays = 0 // Options say never; commitment saves anyway
	s.SaveSlot = "colony"
	for d := 1; d <= 3; d++ {
		f.date, f.tick = morning(d)
		s.TickAutosave()
	}
	if len(f.saved) != 3 || f.saved[2] != "colony" {
		t.Fatalf("saved %v, want the colony slot each day", f.saved)
	}
}

func TestLeavingStaysWhenTheCommitmentSaveFails(t *testing.T) {
	t.Parallel()
	f, s := newFake()
	f.env.CanSave, f.env.Commitment = true, true
	f.saveErr = errDisk
	if s.SaveBeforeLeaving() || len(f.hints) == 0 {
		t.Fatalf("a failed save must keep the player in (hints %v)", f.hints)
	}
	f.saveErr = nil
	if !s.SaveBeforeLeaving() {
		t.Fatal("a saved colony may leave")
	}
	f.env.Commitment = false
	f.saved = nil
	if !s.SaveBeforeLeaving() || len(f.saved) != 0 {
		t.Fatal("a reload-anytime colony leaves without saving")
	}
}

// OPT-006: with more than one autosave kept, the saves go round and the
// oldest is written over.
func TestAutosavesRotate(t *testing.T) {
	t.Parallel()
	f, s := newFake()
	f.env.CanSave, f.env.CanLoad = true, true
	f.settings.Gameplay.AutosaveDays = 1
	f.settings.General.AutosavesKept = 3
	s.SaveSlot = "colony"
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var want []string
	for day := 1; day <= 7; day++ {
		f.date, f.tick = morning(day)
		s.TickAutosave()
		name := f.saved[len(f.saved)-1]
		want = append(want, name)
		// the store lists what was written, with growing times
		found := false
		for i := range f.slots {
			if f.slots[i].Name == name {
				f.slots[i].Modified, found = base.Add(time.Duration(day)*time.Hour), true
			}
		}
		if !found {
			f.slots = append(f.slots, ports.SaveSlot{Name: name, Modified: base.Add(time.Duration(day) * time.Hour)})
		}
	}
	exp := []string{"colony-autosave-1", "colony-autosave-2", "colony-autosave-3", "colony-autosave-1", "colony-autosave-2", "colony-autosave-3", "colony-autosave-1"}
	if !reflect.DeepEqual(want, exp) {
		t.Fatalf("slots %v, want %v", want, exp)
	}
	if len(f.slots) != 3 {
		t.Fatalf("%d autosaves on disk, want 3", len(f.slots))
	}
}

// One autosave kept is the file it always was; lowering the number later
// never goes above it.
func TestOneAutosaveKeepsTheOldName(t *testing.T) {
	t.Parallel()
	f, s := newFake()
	f.env.CanSave, f.env.CanLoad = true, true
	s.SaveSlot = "colony"
	f.slots = []ports.SaveSlot{{Name: "colony-autosave-1"}, {Name: "colony-autosave-2"}}
	if got := autosaveSlot(s, 5); got != "colony-autosave" {
		t.Fatalf("slot %q", got)
	}
	f.settings.General.AutosavesKept = 2
	f.slots[0].Modified = time.Unix(10, 0)
	f.slots[1].Modified = time.Unix(5, 0)
	if got := autosaveSlot(s, 5); got != "colony-autosave-2" {
		t.Fatalf("the older of two: %q", got)
	}
	f.listErr = errors.New("gone")
	if got := autosaveSlot(s, 5); got != "colony-autosave-2" { // 5%2+1
		t.Fatalf("unreadable list: %q", got)
	}
}

// A long save list is shown a page at a time: every save is reachable, and
// the page row is the last button.
func TestLoadListPagesThroughEverySave(t *testing.T) {
	t.Parallel()
	f, s := newFake()
	f.env.CanLoad = true
	for i := 0; i < 30; i++ {
		f.slots = append(f.slots, ports.SaveSlot{Name: fmt.Sprintf("s%d", i), Path: fmt.Sprintf("/s%d", i)})
	}
	s.RefreshSlots()
	reached := map[int]bool{}
	for page := 0; page < 8; page++ {
		bs := s.LoadButtons(1280, 720)
		if len(bs) > 1+slotRows {
			t.Fatalf("page %d: %d buttons", page, len(bs))
		}
		for _, b := range bs[1 : len(bs)-1] {
			var i int
			fmt.Sscanf(b.ID, "load.slot.%d", &i)
			reached[i] = true
		}
		if last := bs[len(bs)-1]; last.ID != "load.slot.-1" {
			t.Fatalf("no page row: %s", last.ID)
		}
		s.LoadSlot(-1)
	}
	if len(reached) != 30 {
		t.Fatalf("reached %d of 30 saves", len(reached))
	}
	s.LoadSlot(29)
	if len(f.loaded) != 1 || f.loaded[0] != "/s29" {
		t.Fatalf("loaded %v", f.loaded)
	}
}
