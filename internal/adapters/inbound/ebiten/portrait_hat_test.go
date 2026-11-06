package ebiten

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
)

// TestPortraitLeavesTheHatOffOnlyWhenAsked: Hats only on the map (OPT-014)
// drops the hat from portrait layers; the map figure keeps it.
func TestPortraitLeavesTheHatOffOnlyWhenAsked(t *testing.T) {
	t.Parallel()
	art := testArt(t)
	p := game.PawnView{ID: 1, Apparel: []string{"t-shirt", "pants", "cowboy hat"}}
	hat := func(ls []portraitLayer) bool {
		for _, l := range ls {
			if strings.Contains(l.Key, "cowboy_hat") {
				return true
			}
		}
		return false
	}
	if !hat(pawnFigureLayers(art, p, "south")) {
		t.Fatal("map figure lost its hat")
	}
	if !hat(pawnDressLayers(art, p, "south", false)) {
		t.Fatal("portrait without the option lost its hat")
	}
	if hat(pawnDressLayers(art, p, "south", true)) {
		t.Fatal("portrait with Hats only on the map still has its hat")
	}
}
