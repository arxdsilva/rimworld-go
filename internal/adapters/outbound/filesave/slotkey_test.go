package filesave

import (
	"testing"

	"rimworld-go/internal/application/game/savefmt"
)

// A slot's file name is savefmt.SlotKey of its name (SAV-003), so the
// client and the store agree on which file a name writes.
func TestSanitizeIsSlotKey(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"My colony!": "My-colony", "  My Colony 2 ": "My-Colony-2", "": "colony", "a/b..c": "abc",
		"x-autosave-3": "x-autosave-3", "Ünïcode": "ncode",
	} {
		if got := sanitize(in); got != want || got != savefmt.SlotKey(in) {
			t.Errorf("sanitize(%q) = %q, SlotKey %q, want %q", in, got, savefmt.SlotKey(in), want)
		}
	}
}
