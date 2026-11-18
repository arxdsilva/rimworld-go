package game

import (
	"path/filepath"
	"slices"
	"testing"

	"rimworld-go/internal/adapters/outbound/modloader"
	"rimworld-go/internal/application/game/contentid"
	"rimworld-go/internal/domain/pawn/backstory"
)

// The shipped example plugin, enabled, reaches the game's books and the
// save header (PLG-024, PLG-043, PLG-050); disabled, nothing changes.
func TestContentFromEnabledPlugin(t *testing.T) {
	src := modloader.New(filepath.Join("..", "..", "..", "mods"))
	on := buildContent(src, []string{"example_frontier"}, "0.1.35")
	if len(on.problems) != 0 {
		t.Fatalf("problems: %v", on.problems)
	}
	if got := on.game().Backstories.Len(); got != backstory.Core().Len()+3 {
		t.Fatalf("game books hold %d backstories", got)
	}
	h := on.save()
	if !slices.Equal(h.Plugins, []string{"core", "example_frontier"}) || h.Hash == contentid.Fingerprint() {
		t.Fatalf("save content = %+v", h)
	}
	off := buildContent(src, nil, "0.1.35")
	if h := off.save(); !slices.Equal(h.Plugins, []string{"core"}) || h.Hash != contentid.Current().Hash {
		t.Fatalf("disabled: save content = %+v", h)
	}
}

// Captures never load plugins, whatever is installed or enabled.
func TestCapturesLoadNoPlugin(t *testing.T) {
	c := loadContent(true, []string{"example_frontier"}, "0.1.35")
	if len(c.found) != 0 || c.save().Hash != contentid.Current().Hash || c.game().Backstories.Len() != backstory.Core().Len() {
		t.Fatalf("capture content = %+v", c.save())
	}
}

// The Plugins screens' view (PLG-070): the example plugin enabled is a
// loaded row at position 1 with its reason and the set the run started
// with; off, the same row is not loaded; a broken enabled set lists its
// problems; a capture's view is empty.
func TestContentView(t *testing.T) {
	src := modloader.New(filepath.Join("..", "..", "..", "mods"))
	on := buildContent(src, []string{"example_frontier"}, "0.1.35").view()
	if len(on.Rows) != 1 || !slices.Equal(on.Enabled, []string{"example_frontier"}) {
		t.Fatalf("on: %+v", on)
	}
	if r := on.Rows[0]; r.ID != "example_frontier" || !r.Loaded || r.Position != 1 || r.Reason == "" || r.Name == "" {
		t.Fatalf("on: row %+v", r)
	}
	off := buildContent(src, nil, "0.1.35").view()
	if len(off.Rows) != 1 || off.Rows[0].Loaded || off.Enabled != nil || len(off.Problems) != 0 {
		t.Fatalf("off: %+v", off)
	}
	bad := buildContent(src, []string{"example_frontier", "missing"}, "0.1.35").view()
	if bad.Errors() != 1 || bad.Problems[0].Plugin != "missing" {
		t.Fatalf("missing plugin: problems %+v", bad.Problems)
	}
	if v := loadContent(true, []string{"example_frontier"}, "0.1.35").view(); len(v.Rows)+len(v.Problems)+len(v.Enabled)+len(v.Folders) != 0 {
		t.Fatalf("capture view %+v", v)
	}
}
