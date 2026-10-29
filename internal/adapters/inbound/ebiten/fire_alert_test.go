package ebiten

import (
	"testing"

	"rimworld-go/internal/application/game"
)

// TestFireAlert: "Fire in the colony!" shows, as a danger alert targeting
// the fires, only while a fire burns inside Home.
func TestFireAlert(t *testing.T) {
	t.Parallel()
	w := game.WorldView{TerrainW: 10, Home: make([]uint8, 100)}
	w.Home[5*10+5] = 1
	w.Fires = []game.FireView{{X: 1, Y: 1, Size: 40}}
	if _, ok := fireAlert(w); ok {
		t.Fatal("alert for a fire outside Home")
	}
	w.Fires = append(w.Fires, game.FireView{X: 5, Y: 5, Size: 20})
	al, ok := fireAlert(w)
	if !ok || al.Label != "Fire in the colony!" || !al.Danger || len(al.Targets) != 1 || al.Targets[0] != [2]int{5, 5} {
		t.Fatalf("alert %+v %v", al, ok)
	}
	found := false
	for _, a := range buildAlerts(nil, w) {
		found = found || a.Key == "fire"
	}
	if !found {
		t.Fatal("buildAlerts misses the fire alert")
	}
	w.Fires = nil
	if _, ok := fireAlert(w); ok {
		t.Fatal("alert with no fire")
	}
}
