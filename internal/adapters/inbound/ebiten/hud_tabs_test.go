package ebiten

import (
	"math"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hud"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
)

// The tab strip at 1280 and 1920 logical px (UIS-041 to UIS-043, QST-138):
// nine text tabs of unit = floor(W / (9 + 3 x IconTabShare)), 182 at 1920, then the History,
// Factions and Menu icon tabs at half of it, the last one taking the rest;
// the tabs tile the strip, the icon tabs carry their name in the tooltip,
// and every text label fits left-aligned at TabLabelX without an ellipsis.
func TestTabStripWidthsAt1280And1920(t *testing.T) {
	t.Parallel()
	a, _ := winApp(t, winStub(3))
	want := []string{"tab.architect", "tab.work", "tab.schedule", "tab.assign", "tab.animals", "tab.wildlife", "tab.research", "tab.quests", "tab.world", "tab.history", "tab.factions", "tab.menu"}
	icons := map[string]bool{"tab.history": true, "tab.factions": true, "tab.menu": true}
	for _, size := range [][2]int{{1280, 720}, {1920, 1080}} {
		a.sw, a.sh = size[0], size[1]
		bs := a.chrome.Tabs().Buttons(a.sw, a.sh)
		if len(bs) != len(want) {
			t.Fatalf("%v: %d tabs, want %d", size, len(bs), len(want))
		}
		unit := float32(math.Floor(float64(a.sw) / (9 + 3*hudlayout.IconTabShare)))
		half := float32(math.Floor(float64(unit) * hudlayout.IconTabShare))
		x := float32(0)
		for i, b := range bs {
			if b.ID != want[i] {
				t.Errorf("%v: tab %d is %q, want %q", size, i, b.ID, want[i])
			}
			w := unit
			if icons[b.ID] {
				w = half
			}
			if i == len(bs)-1 {
				w = float32(a.sw) - x
			}
			if b.X != x || b.W != w {
				t.Errorf("%v: %s is x %v w %v, want x %v w %v", size, b.ID, b.X, b.W, x, w)
			}
			if b.Y != float32(a.sh-hudlayout.TabsH) || b.H != hudlayout.TabsH {
				t.Errorf("%v: %s is y %v h %v, want y %d h %d", size, b.ID, b.Y, b.H, a.sh-hudlayout.TabsH, hudlayout.TabsH)
			}
			x += b.W
			if icons[b.ID] {
				if len(b.Tip) < len(b.Label) || b.Tip[:len(b.Label)] != b.Label {
					t.Errorf("%v: icon tab %s tooltip %q lacks its name %q", size, b.ID, b.Tip, b.Label)
				}
				continue
			}
			lx := float32(math.Floor(float64(b.W-hudlayout.TabSep) * hudlayout.TabLabelX))
			if room := b.W - hudlayout.TabSep - lx - 2; gfx.TextW(b.Label, gfx.Body) > room {
				t.Errorf("%v: %s label %q is %v wide in %v: it would ellipsize", size, b.ID, b.Label, gfx.TextW(b.Label, gfx.Body), room)
			}
		}
		if x != float32(a.sw) {
			t.Errorf("%v: the strip ends at %v, want the screen's edge", size, x)
		}
		if ws := hud.TabWidths(float32(a.sw)); len(ws) != len(bs) {
			t.Errorf("%v: TabWidths has %d widths for %d tabs", size, len(ws), len(bs))
		}
	}
}

// A text tab's label is drawn left-aligned at TabLabelX of the tab, its
// baseline TabBaseline below the strip's top; an icon tab draws no label
// when its icon is there.
func TestTabLabelsSitOnTheBaseline(t *testing.T) {
	// serial: it installs the draw trace
	a, _ := winApp(t, winStub(3))
	a.sw, a.sh = 1280, 720
	var ev []gfx.TraceEvent
	gfx.SetDrawTrace(func(e gfx.TraceEvent) { ev = append(ev, e) })
	t.Cleanup(func() { gfx.SetDrawTrace(nil) })
	a.chrome.Tabs().Draw(ebiten.NewImage(a.sw, a.sh), a.sw, a.sh)
	for _, b := range a.chrome.Tabs().Buttons(a.sw, a.sh) {
		if b.ID == "tab.history" || b.ID == "tab.factions" || b.ID == "tab.menu" {
			continue
		}
		wantX := b.X + float32(math.Floor(float64(b.W-hudlayout.TabSep)*hudlayout.TabLabelX))
		wantY := hud.TabLabelTop(b.Y, gfx.Body)
		found := false
		for _, e := range ev {
			if e.Kind == "text" && e.Text == b.Label {
				found = true
				if math.Abs(float64(e.X-wantX)) > 1 || math.Abs(float64(e.Y-wantY)) > 1 {
					t.Errorf("%s label at %v,%v, want %v,%v", b.ID, e.X, e.Y, wantX, wantY)
				}
			}
		}
		if !found {
			t.Errorf("%s: label %q not drawn", b.ID, b.Label)
		}
	}
}
