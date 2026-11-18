package game

import (
	"path/filepath"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	appgame "rimworld-go/internal/application/game"
	"slices"
	"strings"
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

// The missing-plugin notice works on a real save (PLG-050): saved with
// the example plugin on, the slot listed by a run without it names it;
// listed by a run with it, there is no notice; and the save still loads.
func TestSaveFromPluginRunNamesTheMissingPlugin(t *testing.T) {
	dir := t.TempDir()
	src := modloader.New(filepath.Join("..", "..", "..", "mods"))
	on := buildContent(src, []string{"example_frontier"}, "v1.2.3")
	h := saveHooks(dir, "v1.2.3", on)
	meta := ports.SaveMeta{Name: "Frontier", Colony: "col-9", ColonyName: "Frontier", Base: "Frontier"}
	if err := h.Save(appgame.NewColony(appgame.Config{Seed: 4, Width: 30, Height: 30, Content: on.game()}), meta); err != nil {
		t.Fatal(err)
	}
	closeSaves(h)

	off := saveHooks(dir, "v1.2.3", buildContent(src, nil, "v1.2.3"))
	slots, err := off.List()
	if err != nil || len(slots) != 1 {
		t.Fatalf("slots %+v, %v", slots, err)
	}
	if !strings.Contains(slots[0].Notice, "example_frontier") {
		t.Fatalf("notice %q does not name the missing plugin", slots[0].Notice)
	}
	if _, _, err := off.Load(slots[0].Path); err != nil {
		t.Fatalf("a run without the plugin cannot load the save: %v", err)
	}
	same, _ := saveHooks(dir, "v1.2.3", on).List()
	if len(same) != 1 || same[0].Notice != "" {
		t.Fatalf("the same plugin set gives a notice: %+v", same)
	}
}
