package saves

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
)

// A new colony gets a fresh slot, no autosave day and a colony id.
func TestNewColonyStartsTheSession(t *testing.T) {
	t.Parallel()
	_, b := newFake()
	b.newID = func() string { return "c0ffee" }
	b.SaveSlot, b.AutosaveDay = "old", 9
	b.NewColony(0)
	if !strings.HasPrefix(b.SaveSlot, "colony-") || b.AutosaveDay != 0 {
		t.Fatalf("slot %q day %d, want a new colony- slot from day 0", b.SaveSlot, b.AutosaveDay)
	}
	want := session{Colony: "c0ffee", ColonyName: b.SaveSlot, Name: b.SaveSlot, Base: b.SaveSlot}
	if b.session != want {
		t.Fatalf("session %+v, want %+v", b.session, want)
	}
}

func TestRandomIDIs16HexDigits(t *testing.T) {
	t.Parallel()
	a, b := randomID(), randomID()
	if len(a) != 16 || strings.Trim(a, "0123456789abcdef") != "" || a == b {
		t.Fatalf("ids %q %q, want two different 16-digit hex ids", a, b)
	}
}

// Click takes every load id but Back, which the root's screen intent
// handles.
func TestClickRoutesTheLoadIDs(t *testing.T) {
	t.Parallel()
	f, b := newFake()
	f.env.CanLoad = true
	f.slots = []ports.SaveSlot{{Name: "a", Path: "/a"}}
	b.RefreshSlots()
	if !b.Click(ids.PrefixLoadSlot+"0") || len(f.loaded) != 1 {
		t.Fatalf("load.slot.0: loaded %v", f.loaded)
	}
	for _, id := range []string{ids.PrefixLoadSlot + "x", ids.PrefixLoadDelete + "0", ids.LoadSort, ids.LoadColony} {
		if !b.Click(id) {
			t.Fatalf("%s not handled", id)
		}
	}
	if b.Click("load.back") || len(f.loaded) != 1 || len(f.deleted) != 0 {
		t.Fatalf("load.back handled, or a stub acted: loaded %v deleted %v", f.loaded, f.deleted)
	}
}

// The Save dialog is dialog_test.go's; leaving and Save & Quit are
// leave_test.go's (SAV-005, SAV-013).
