package ebiten

import (
	"fmt"
	"testing"
)

func htOverlaps(a, b rect) bool {
	return !a.empty() && !b.empty() && a.X < b.right() && b.X < a.right() && a.Y < b.bottom() && b.Y < a.bottom()
}

// PLAY-018: the search box sits centred under the colonist bar, clear of
// the bar, the resource list and the right column, at every window width.
func TestSearchBoxClearOfTheTopRegions(t *testing.T) {
	for _, tc := range []struct{ n, sw, sh int }{
		{6, 1280, 720}, {30, 1280, 720}, {30, 1024, 640}, {0, 1280, 720}, {12, 700, 600},
	} {
		t.Run(fmt.Sprintf("%d colonists in %dx%d", tc.n, tc.sw, tc.sh), func(t *testing.T) {
			a, _ := htApp(t, &stubSim{pawns: htPawns(tc.n)}, tc.sw, tc.sh)
			a.openSearch()
			r := a.searchRect(tc.sw)
			box := rect{r.x, r.y, r.w, r.h}
			l := a.hud()
			for name, other := range map[string]rect{"bar": l.Bar, "resources": l.Resources, "right column": l.Right, "learning helper": l.Learn} {
				if htOverlaps(box, other) {
					t.Errorf("search box %+v overlaps the %s %+v", box, name, other)
				}
			}
			if !l.Bar.empty() && box.Y < l.Bar.bottom() {
				t.Errorf("search box top %v is not under the bar (bottom %v)", box.Y, l.Bar.bottom())
			}
			if box.X < 0 || box.right() > float32(tc.sw) {
				t.Errorf("search box %+v leaves the %d px window", box, tc.sw)
			}
			if mid := box.X + box.W/2; mid < float32(tc.sw)/2-1 || mid > float32(tc.sw)/2+1 {
				t.Errorf("search box centre %v, want the window centre %v", mid, float32(tc.sw)/2)
			}
			// the result rows follow the box
			for _, b := range a.searchButtons(tc.sw) {
				if b.x < box.X || b.x+b.w > box.right() || b.y < box.Y {
					t.Errorf("search button %s %+v outside the box", b.id, b)
				}
			}
			// and the box blocks the map under it
			if !a.colonyChromeHit(int(box.X)+10, int(box.Y)+10, tc.sw, tc.sh) {
				t.Error("a click on the search box reaches the map")
			}
		})
	}
}
