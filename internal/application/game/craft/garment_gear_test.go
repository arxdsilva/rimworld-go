package craft

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
)

// TestWornViewGearFacts (GEAR-1): each worn garment's mass, cover and
// speed on its view row.
func TestWornViewGearFacts(t *testing.T) {
	t.Parallel()
	var o Outfit
	for _, k := range []item.Kind{item.PlateArmor, item.SimpleHelmet, item.ShieldBelt, item.TShirt} {
		o.On[k.Slot()] = Piece{Kind: k}
	}
	want := map[string]struct {
		grams int
		cover string
		move  int
	}{
		"plate armor":   {18000, "torso, neck, shoulders, arms, legs", -17},
		"simple helmet": {2000, "upper head", 0},
		"t-shirt":       {250, "torso, shoulders", 0},
	}
	seen := 0
	for _, v := range o.Views() {
		w, ok := want[v.Kind]
		if !ok {
			if v.Cover != "" || v.MovePct != 0 || v.Grams <= 0 {
				t.Errorf("%s: %+v", v.Kind, v)
			}
			continue
		}
		seen++
		if v.Grams != w.grams || v.Cover != w.cover || v.MovePct != w.move {
			t.Errorf("%s: %d g, %q, %d%%; want %d g, %q, %d%%", v.Kind, v.Grams, v.Cover, v.MovePct, w.grams, w.cover, w.move)
		}
	}
	if seen != len(want) {
		t.Errorf("saw %d of %d garments: %+v", seen, len(want), o.Views())
	}
}
