package hud

import (
	"testing"

	"rimworld-go/internal/application/game"
)

// RSCH-2: WorldView.ResearchAlert raises "Need research project" with its
// explanation; without it there is no such alert.
func TestResearchAlertFollowsTheWorldFlag(t *testing.T) {
	t.Parallel()
	pawns := []game.PawnView{{ID: 1, Name: "Ada", Status: "alive", Mood: 80, HasBed: true}}
	w := game.WorldView{FoodDays: 100}
	for _, al := range BuildAlerts(pawns, w) {
		if al.Key == ResearchAlertKey {
			t.Fatal("research alert without the flag")
		}
	}
	w.ResearchAlert = true
	var got Alert
	for _, al := range BuildAlerts(pawns, w) {
		if al.Key == ResearchAlertKey {
			got = al
		}
	}
	if got.Label != "Need research project" || got.Danger || got.Explain == AlertExplain("nosuchkey") {
		t.Fatalf("research alert %+v", got)
	}
}
