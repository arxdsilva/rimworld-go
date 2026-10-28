package ebiten

import "testing"

// PLAY-021: a hidden conduit is drawn only in the power overlay, so a map
// click selects it only while the overlay shows.
func TestHiddenConduitSelectableOnlyInThePowerOverlay(t *testing.T) {
	t.Parallel()
	a, g, _ := liveColony(t, 0)
	if !g.BuildNow("hidden conduit", "steel", "south", 9, 9) {
		t.Fatal("hidden conduit not built")
	}
	clickTile(a, 9, 9)
	if a.sel.HasBuilding {
		t.Fatal("a click selected an invisible hidden conduit")
	}
	a.renderer.overlays().Power.On = true
	clickTile(a, 9, 9)
	if !a.sel.HasBuilding {
		t.Fatal("with the power overlay on, a click selects the hidden conduit")
	}
}
