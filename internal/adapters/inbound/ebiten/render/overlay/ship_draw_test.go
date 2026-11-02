package overlay

import (
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/axview"
	"rimworld-go/internal/application/game/ship"
)

// AX-8: the connectivity highlight follows a selected ship part, and the
// launch flare burns at the launch letter for LaunchFlareTicks.
func TestShipSelAndLaunchFlare(t *testing.T) {
	t.Parallel()
	part := game.BuildingView{Kind: "ship engine", AX: axview.Building{ShipID: 7}}
	if ShipSelOf(part, true) != 7 || ShipSelOf(part, false) != 0 || ShipSelOf(game.BuildingView{Kind: "wall"}, true) != 0 {
		t.Fatal("ShipSelOf")
	}
	w := game.WorldView{Messages: []game.MessageView{{Tick: 100, Title: ship.LaunchTitle, X: 4, Y: 5}}}
	if _, _, _, _, ok := LaunchFlare(w, 120, false); ok {
		t.Fatal("a flare before the launch is in the view")
	}
	w.AX.ShipLaunched = true
	x, y, s, a, ok := LaunchFlare(w, 160, false)
	if !ok || x != 4.5 || y != 5.5 || s <= 2 || a >= 1 || a <= 0 {
		t.Fatalf("flare %v %v %v %v %v", x, y, s, a, ok)
	}
	if _, _, _, _, ok := LaunchFlare(w, 100+LaunchFlareTicks, false); ok {
		t.Fatal("the flare outlives its ticks")
	}
}
