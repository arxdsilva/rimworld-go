package dialogs

import (
	"slices"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// TestWandererCardShowsPortraitAndPassions (MAP-22; wiki: Ending: "a menu
// similar to the game creation menu"): the chosen person's card draws their
// portrait and a glyph on each skill they have a passion for.
func TestWandererCardShowsPortraitAndPassions(t *testing.T) {
	t.Parallel()
	s, dl := newFake()
	s.w.AX.WanderersMax, s.w.AX.WanderersLeft, s.w.AX.WanderersOK = 5, 7, true
	dl.OpenWanderers()
	passions := func() bool { // someone with a passion, in the slot on show
		for i := range dl.wand.slots {
			for _, sk := range dl.wand.slots[i].bio.Skills {
				if sk.Passion > 0 && !sk.Incapable {
					dl.wand.sel = i
					return true
				}
			}
		}
		return false
	}
	if !passions() {
		t.Fatal("five people, no passion among them")
	}
	s.frame(dl)
	s.kit.Paint(ebiten.NewImage(1280, 720))
	if s.portraits == 0 {
		t.Fatal("no portrait")
	}
	if !slices.ContainsFunc(s.icons, func(k string) bool { return k == "ui_icon/passion_minor" || k == "ui_icon/passion_major" }) {
		t.Fatalf("no passion glyph: %v", s.icons)
	}
}
