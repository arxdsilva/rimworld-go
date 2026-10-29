package ebiten

import (
	"math"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"testing"
)

// PLAY-012 B.4: every world-mapping path uses physical pixels. These tests
// change curUIScale and must not run in parallel.

// TestJumpToCentresOnPhysicalViewport: centring on a colonist puts it at
// the middle of the physical map viewport at dev 2 and at UI scale 1.5.
func TestJumpToCentresOnPhysicalViewport(t *testing.T) {
	for _, c := range []struct {
		name string
		dev  float64
		ui   int
	}{{"dev2", 2, 0}, {"ui1.5", 1, 150}} {
		t.Run(c.name, func(t *testing.T) {
			a, _ := hidpiColony(t, c.dev, c.ui)
			a.renderer.cam.Zoom = 1
			a.centreOnPawn(7)
			v := a.mapViewport()
			sx, sy := a.renderer.worldToScreenF(21.5, 19.5)
			if math.Abs(float64(sx)-(v.X0+v.X1)/2) > 1.5 || math.Abs(float64(sy)-(v.Y0+v.Y1)/2) > 1.5 {
				t.Fatalf("pawn drawn at (%v,%v), want the viewport centre of %+v", sx, sy, v)
			}
		})
	}
}

// TestDevMenuPicksPhysicalTile: the dev actions menu targets the tile under
// the raw cursor at dev 2.
func TestDevMenuPicksPhysicalTile(t *testing.T) {
	a, f := hidpiColony(t, 2, 0)
	set := a.settings
	set.Gameplay.DevMode = true
	a.setSettings(set)
	f.MoveTo(a, 21, 19)
	f.Key(t, a, "Slash", 0)
	if !a.tools.State.Menu || a.tools.State.TX != 21 || a.tools.State.TY != 19 {
		t.Fatalf("dev menu %v at tile %d,%d, want 21,19", a.tools.State.Menu, a.tools.State.TX, a.tools.State.TY)
	}
}

// TestBoxSelectPhysical: a drag box around a colonist selects it at dev 2
// and UI scale 1.5, and the double click needs the same physical spot.
func TestBoxSelectPhysical(t *testing.T) {
	for _, c := range []struct {
		name string
		dev  float64
		ui   int
	}{{"dev2", 2, 0}, {"ui1.5", 1, 150}} {
		t.Run(c.name, func(t *testing.T) {
			a, f := hidpiColony(t, c.dev, c.ui)
			f.Drag(t, a, inputframe.Left, 20, 18, 22, 20, 0)
			if !a.sel.Grouped() && !(a.renderer.HasSelect() && a.renderer.Selected() == 7) {
				t.Fatal("a box around the pawn did not select it")
			}
			if a.sel.Grouped() {
				if _, ids := a.sel.SelectionSet(); len(ids) != 1 || ids[0] != 7 {
					t.Fatalf("group %v", ids)
				}
			}
		})
	}
}

// TestBoxSelectHidesBehindFarPixels: a box that does not reach the pawn
// does not select it (guards against a logical/physical mix-up).
func TestBoxSelectMissesPhysical(t *testing.T) {
	a, f := hidpiColony(t, 2, 0)
	f.Drag(t, a, inputframe.Left, 10, 10, 12, 12, 0)
	if a.sel.Grouped() || (a.renderer.HasSelect() && a.renderer.Selected() == 7) {
		t.Fatal("a box far from the pawn selected it")
	}
}

// TestStrokesKeepApparentThickness: zone and path strokes scale with dev.
func TestStrokesKeepApparentThickness(t *testing.T) {
	withMapScale(t, 1, 1)
	one := overlay.ZoneStroke(1)
	withMapScale(t, 2, 1)
	if two := overlay.ZoneStroke(1); two != 2*one {
		t.Fatalf("zone stroke %v at dev 2, want %v", two, 2*one)
	}
	if got, want := overlay.PathStroke(1), float32(2*1.5); got != want {
		t.Fatalf("path stroke %v at dev 2, want %v", got, want)
	}
}
