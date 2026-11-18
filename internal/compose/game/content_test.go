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
