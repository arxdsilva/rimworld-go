package ebiten

import (
	"fmt"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// devLayouts are busy HUD states at the sizes the plan pins: every region
// that could collide with the developer panels is up.
func devLayouts() map[string]hudLayout {
	busy := hudState{Colonists: 30, PaneShown: true, Commands: 13, GlobalH: 190, MinimapOn: true, LearnH: 200,
		Alerts: 8, Letters: 8, ResourcesH: 600, Messages: 5, ExpandedW: 500, ExpandedH: 300}
	arch := busy
	arch.PaneShown, arch.ArchOpen, arch.ArchDescH, arch.ExpandedW, arch.ExpandedH = false, true, 120, 0, 0
	win := busy
	win.PaneShown, win.WindowW, win.WindowH, win.ExpandedW, win.ExpandedH = false, 900, 500, 0, 0
	idle := hudState{GlobalH: 150, ResourcesH: 100}
	out := map[string]hudLayout{}
	for _, sz := range [][2]int{{1280, 720}, {1920, 1080}, {1113, 626}} {
		for name, st := range map[string]hudState{"busy": busy, "architect": arch, "window": win, "idle": idle} {
			out[fmt.Sprintf("%s %dx%d", name, sz[0], sz[1])] = computeHUDLayout(sz[0], sz[1], st)
		}
	}
	return out
}

// The developer panels live in a column that touches no HUD region: right
// of the resource list, under the colonist bar and the messages, above the
// lower-left stack, left of the right column.
func TestDevColumnAvoidsEveryRegion(t *testing.T) {
	t.Parallel()
	for name, l := range devLayouts() {
		col := devColumn(l)
		if col.Empty() {
			continue // no room: nothing is drawn
		}
		regions := append(l.chrome(), l.Minimap)
		for _, r := range regions {
			if i := col.Intersect(r); i.W > 0 && i.H > 0 {
				t.Errorf("%s: dev column %+v collides with %+v", name, col, r)
			}
		}
		if col.X < l.Resources.Right() || col.Right() > l.Right.X {
			t.Errorf("%s: dev column %+v is not between the resource list and the right column", name, col)
		}
	}
}

// With every panel on, they stack inside the column in order and do not
// overlap each other.
func TestDevPanelsStackInTheColumn(t *testing.T) {
	t.Parallel()
	s := newGlobalSim()
	a, _ := globalColonyP(t, s)
	set := a.settings
	set.Gameplay.DevMode = true
	a.setSettings(set)
	a.dev.log, a.dev.insp = true, true
	a.dev.fields = mapFieldsOverlay{shown: true, layers: []game.MapFieldLayer{{Name: "fertility", W: 2, H: 2, Classes: []uint8{0, 1, 0, 1},
		Legend: []game.LegendEntry{{Label: "poor", RGBA: [4]uint8{1, 2, 3, 255}}, {Label: "rich", RGBA: [4]uint8{4, 5, 6, 255}}}}}}
	a.updateOnce(t)
	col := devColumn(a.hud())
	st := a.devStack()
	var drawn []rect
	for name, r := range map[string]rect{"fields": st.Fields, "inspector": st.Insp, "log": st.Log} {
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
	a.drawDevPanels(screen, a.sw, a.sh) // paints the three
}

// The debug menu stays on the window: under the cursor but clear of the
// tab strip.
func TestDevMenuStaysAboveTheTabStrip(t *testing.T) {
	t.Parallel()
	a, _ := globalColonyP(t, newGlobalSim())
	set := a.settings
	set.Gameplay.DevMode = true
	a.setSettings(set)
	a.dev.menu, a.dev.mx, a.dev.my = true, a.sw-10, a.sh-5
	r := a.devMenuRect()
	l := a.hud()
	if r.y < hudMargin || r.y+r.h > l.Tabs.Y {
		t.Errorf("debug menu %+v runs past the tab strip at y=%v", r, l.Tabs.Y)
	}
	if r.x+r.w > float32(a.sw) {
		t.Errorf("debug menu %+v runs off the window", r)
	}
}
