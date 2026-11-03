package hud

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
)

// TestFishDyingAlert: only a stocked tank in trouble raises it, with a
// target per tank, an explanation, and ordered by the usual rule.
func TestFishDyingAlert(t *testing.T) {
	t.Parallel()
	_, w := rightWorld()
	tank := func(x int, stocked bool, trouble string) game.BuildingView {
		return game.BuildingView{ID: 90 + 0, Kind: "fish tank", X: x, Y: 3, Tank: &game.TankView{Stocked: stocked, Trouble: trouble}}
	}
	w.Buildings = []game.BuildingView{tank(1, true, ""), tank(2, false, "No power")}
	if _, ok := FishDyingAlert(w); ok {
		t.Fatal("a healthy tank and an empty one raised the alert")
	}
	w.Buildings = append(w.Buildings, tank(3, true, "Too cold"))
	al, ok := FishDyingAlert(w)
	if !ok || al.Label != "Fish dying" || len(al.Targets) != 1 || al.Targets[0] != [2]int{3, 3} {
		t.Fatalf("alert %+v ok %v", al, ok)
	}
	w.Buildings = append(w.Buildings, tank(4, true, "No feed"))
	if al, _ := FishDyingAlert(w); !strings.Contains(al.Label, "2 tanks") || len(al.Targets) != 2 {
		t.Fatalf("two tanks: %+v", al)
	}
	found := false
	for _, a := range BuildAlerts(nil, w) {
		if a.Key == FishDyingAlertKey {
			found = len(a.Explain) > 24 && a.Sev == SevWarning
		}
	}
	if !found {
		t.Fatal("BuildAlerts lacks the fish alert or its explanation")
	}
}
