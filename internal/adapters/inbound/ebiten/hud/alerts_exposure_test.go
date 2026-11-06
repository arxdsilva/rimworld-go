package hud

import (
	"testing"

	"rimworld-go/internal/application/game"
)

// AIL-004: a downed colonist whose exposure can't heal where it lies
// raises "Not recovering", named alone and counted together.
func TestNotRecoveringAlert(t *testing.T) {
	note := "Not recovering: air 33 °C, needs 29 °C or cooler"
	ada := game.PawnView{Name: "Ada", Status: "downed", ExposureNote: note, X: 3, Y: 4}
	bo := game.PawnView{Name: "Bo", Status: "downed", Heatstroke: 800}
	var found *Alert
	alerts := BuildAlerts([]game.PawnView{ada, bo}, game.WorldView{FoodDays: 100})
	for i := range alerts {
		if alerts[i].Key == NotRecoveringAlertKey {
			found = &alerts[i]
		}
	}
	if found == nil || found.Label != "Ada is not recovering" || !found.Danger || len(found.Targets) != 1 || found.Explain == AlertExplain("") {
		t.Fatalf("alert: %+v", found)
	}
	bo.ExposureNote = note
	if a, ok := NotRecoveringAlert([]game.PawnView{ada, bo}); !ok || a.Label != "2 colonists are not recovering" {
		t.Fatalf("two: %+v", a)
	}
	if _, ok := NotRecoveringAlert([]game.PawnView{{Name: "Cy", Status: "downed", Heatstroke: 800}}); ok {
		t.Fatal("healing: no alert")
	}
}
