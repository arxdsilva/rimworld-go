package modloader

import (
	"path/filepath"
	"testing"

	"rimworld-go/internal/application/modding"
	"rimworld-go/internal/domain/pawn/backstory"
	"rimworld-go/internal/domain/pawn/skill"
)

// The shipped example plugin loads with no problem through the whole
// pipeline: its backstories follow core's, its template is not one, and
// its patch reaches the core backstory it targets.
func TestExamplePluginBuilds(t *testing.T) {
	found, err := New(filepath.Join("..", "..", "..", "..", "mods")).Plugins()
	if err != nil {
		t.Fatal(err)
	}
	cat, probs := modding.Build(found, []string{"example_frontier"}, "0.1.35")
	for _, p := range probs {
		t.Errorf("problem: %s", p)
	}
	if !cat.Order.Has("example_frontier") {
		t.Fatalf("example not loaded: %+v", cat.Order)
	}
	core, b := backstory.Core(), cat.Backstories
	if b.Len() != core.Len()+3 {
		t.Fatalf("Len = %d, want core %d + 3", b.Len(), core.Len())
	}
	for id := backstory.ID(1); int(id) <= core.Len(); id++ {
		if b.Key(id) != core.Key(id) {
			t.Fatalf("core id %d moved: %q vs %q", id, b.Key(id), core.Key(id))
		}
	}
	first := backstory.ID(core.Len() + 1)
	if got := b.Key(first); got != "example_frontier.salt_flat_surveyor" {
		t.Fatalf("first plugin key = %q", got)
	}
	if d := b.DefOf(first); d.Slot != backstory.Adulthood || d.Weight != 6 || !d.Fits(backstory.TagNeolithic) {
		t.Fatalf("template fields not inherited: %+v", d)
	}
	var kid backstory.ID
	for id := backstory.ID(1); int(id) <= core.Len(); id++ {
		if core.Key(id) == "core.lighthouse_kid" {
			kid = id
		}
	}
	if kid == 0 {
		t.Fatal("core.lighthouse_kid not found")
	}
	if got := b.DefOf(kid).Skills[skill.Social]; got != 1 || core.DefOf(kid).Skills[skill.Social] != 0 {
		t.Fatalf("patch: Social = %d, core %d", got, core.DefOf(kid).Skills[skill.Social])
	}
}

// Disabled, the example changes nothing: the book is core's.
func TestExamplePluginDisabledIsCore(t *testing.T) {
	found, err := New(filepath.Join("..", "..", "..", "..", "mods")).Plugins()
	if err != nil {
		t.Fatal(err)
	}
	cat, probs := modding.Build(found, nil, "0.1.35")
	if len(probs) != 0 || cat.Backstories.Len() != backstory.Core().Len() || len(cat.Plugins) != 0 {
		t.Fatalf("disabled example changed the build: %v, %d defs", probs, cat.Backstories.Len())
	}
}
