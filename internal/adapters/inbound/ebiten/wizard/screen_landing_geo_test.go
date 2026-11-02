package wizard

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
)

// The title has to fit between the screen's edge and the top-row buttons at
// the smallest window the wizard allows, and at a large one.
func TestLandingTitleClearsTheButtons(t *testing.T) {
	t.Parallel()
	for _, sz := range [][2]int{{uikit.MinSetupW, uikit.MinSetupH}, {2560, 1440}} {
		g := LandRects(sz[0], sz[1])
		if tw := gfx.TextW(LandTitle, gfx.Title); tw > g.title.W {
			t.Errorf("%dx%d: the title is %.0f wide in a %.0f gap", sz[0], sz[1], tw, g.title.W)
		}
		if g.title.X+g.title.W > g.search.X {
			t.Errorf("%dx%d: the title rect ends at %.0f, the buttons start at %.0f", sz[0], sz[1], g.title.X+g.title.W, g.search.X)
		}
	}
}
