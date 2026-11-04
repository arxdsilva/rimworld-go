package ebiten

import (
	"strings"
	"testing"

	worldpkg "rimworld-go/internal/adapters/inbound/ebiten/render/world"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
)

// TestNeatArtCoverage: every Neat Storage piece has a sprite in each facing
// it draws, as many slots in that sprite's metadata as the zone table gives
// a cell (the dump spot's twelve become its heap points), a slot scale and
// an icon.
func TestNeatArtCoverage(t *testing.T) {
	c := realArt(t)
	seen := 0
	for _, d := range construction.All() {
		p, ok := construction.NeatProfile(d.Kind)
		if !ok {
			continue
		}
		seen++
		want := zone.Profile(p).Stacks()
		for _, f := range []string{"south", "east", "north", "west"} {
			key := worldpkg.NeatKey(d.Name, f)
			if key == "" || !c.Has(key) {
				t.Errorf("%s %s: no sprite %q", d.Name, f, key)
				continue
			}
			m := c.MetaOf(key)
			if got := len(m.Slots); got != want {
				t.Errorf("%s: %d slots in the metadata, the zone table gives %d", key, got, want)
			}
			if m.SlotScale <= 0 {
				t.Errorf("%s: no slot_scale", key)
			}
		}
		if icon := "ui_icon/" + strings.ReplaceAll(d.Name, " ", "_"); !c.Has(icon) {
			t.Errorf("%s: no icon %q", d.Name, icon)
		}
	}
	if seen != 27 {
		t.Fatalf("%d neat kinds, want 27", seen)
	}
}
