package options

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/ports"
)

func TestLoadSlotRestoresSlotNameAndAutosaveDay(t *testing.T) {
	t.Parallel()
	f, s := newFake()
	f.env.CanLoad = true
	f.slots = []ports.SaveSlot{{Name: "a", Path: "/a"}, {Name: "bad", Path: "/b", Err: "corrupt"}}
	s.RefreshSlots()
	if len(s.Slots) != 2 {
		t.Fatalf("slots = %d, want 2", len(s.Slots))
	}
	s.LoadSlot(1)
	if len(f.loaded) != 0 {
		t.Fatal("an unreadable save must not load")
	}
	s.LoadSlot(0)
	if len(f.loaded) != 1 || s.SaveSlot != "colony" || s.AutosaveDay != 3 {
		t.Fatalf("loaded %v slot %q day %d; want /a, colony (autosave suffix dropped), day 3", f.loaded, s.SaveSlot, s.AutosaveDay)
	}
}

func TestLoadButtonsCapAtNineRowsAndDisableBrokenSaves(t *testing.T) {
	t.Parallel()
	_, s := newFake()
	for i := 0; i < 12; i++ {
		s.Slots = append(s.Slots, ports.SaveSlot{Name: "x"})
	}
	s.Slots[0].Err = "corrupt"
	bs := s.LoadButtons(1280, 720)
	if len(bs) != 1+slotRows || !bs[1].Disabled || bs[2].Disabled {
		t.Fatalf("%d buttons, first slot disabled=%v; want back + %d rows with only the broken one off", len(bs), bs[1].Disabled, slotRows)
	}
}
