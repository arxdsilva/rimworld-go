package ebiten

import (
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/devtools"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/application/game"
)

// With every panel on, they stack inside the column in order and do not
// overlap each other.
func TestDevPanelsStackInTheColumn(t *testing.T) {
	t.Parallel()
	s := newGlobalSim()
	a, _ := globalColonyP(t, s)
	set := a.settings
	set.Gameplay.DevMode = true
	a.setSettings(set)
	a.tools.State.Log, a.tools.State.Insp = true, true
	a.tools.State.Fields = devtools.FieldsOverlay{Shown: true, Layers: []game.MapFieldLayer{{Name: "fertility", W: 2, H: 2, Classes: []uint8{0, 1, 0, 1},
		Legend: []game.LegendEntry{{Label: "poor", RGBA: [4]uint8{1, 2, 3, 255}}, {Label: "rich", RGBA: [4]uint8{4, 5, 6, 255}}}}}}
	a.updateOnce(t)
	col := devtools.Column(a.hud())
	st := a.tools.Stack()
	var drawn []gfx.Rect
	for name, r := range map[string]gfx.Rect{"fields": st.Fields, "inspector": st.Insp, "log": st.Log} {
		if r.Empty() {
			continue
		}
		if r.X < col.X || r.Right() > col.Right()+0.5 || r.Y < col.Y || r.Bottom() > col.Bottom()+0.5 {
			t.Errorf("%s panel %+v is outside the dev column %+v", name, r, col)
		}
		for _, o := range drawn {
			if i := r.Intersect(o); i.W > 0 && i.H > 0 {
				t.Errorf("%s panel %+v overlaps another dev panel %+v", name, r, o)
			}
		}
		drawn = append(drawn, r)
	}
	if st.Fields.Empty() || st.Insp.Empty() || st.Log.Empty() {
		t.Fatalf("stack = %+v: with room all three panels show", st)
	}
	if !(st.Fields.Y < st.Insp.Y && st.Insp.Y < st.Log.Y) {
		t.Errorf("stack order is legend, inspector, log: %+v", st)
	}
	screen := ebiten.NewImage(a.sw, a.sh)
	a.tools.Draw(screen, a.sw, a.sh) // paints the three
}

// The debug menu stays on the window: under the cursor but clear of the
// tab strip.
func TestDevMenuStaysAboveTheTabStrip(t *testing.T) {
	t.Parallel()
	a, _ := globalColonyP(t, newGlobalSim())
	set := a.settings
	set.Gameplay.DevMode = true
	a.setSettings(set)
	a.tools.State.Menu, a.tools.State.MX, a.tools.State.MY = true, a.sw-10, a.sh-5
	r := a.tools.MenuRect()
	l := a.hud()
	if r.Y < hudlayout.HudMargin || r.Y+r.H > l.Tabs.Y {
		t.Errorf("debug menu %+v runs past the tab strip at y=%v", r, l.Tabs.Y)
	}
	if r.X+r.W > float32(a.sw) {
		t.Errorf("debug menu %+v runs off the window", r)
	}
}
