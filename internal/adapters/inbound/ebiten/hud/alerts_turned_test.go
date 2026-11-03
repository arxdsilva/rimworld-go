package hud

import (
	"testing"

	"rimworld-go/internal/application/game"
)

// MAP-18: visitors who turned run for the edge. The view has them fleeing,
// so they raise no raid alert (and so no pause).
func TestFleeingEnemiesRaiseNoRaidAlert(t *testing.T) {
	t.Parallel()
	pawns := []game.PawnView{{Name: "Ada"}, {Name: "Guest", Hostile: true, Fleeing: true}}
	for _, a := range raiseAlerts(pawns, game.WorldView{}) {
		if a.Key == "raid" || a.Danger {
			t.Fatalf("alert %+v", a)
		}
	}
}
