package ebiten

import (
	"testing"

	"rimworld-go/internal/application/game"
)

func TestBuildAlerts(t *testing.T) {
	t.Parallel()
	pawns := []game.PawnView{
		{Name: "Ada", Status: "downed", Reason: "injured", X: 3, Y: 4},
		{Name: "Bo", Status: "downed", Reason: "injured", InBed: true},
		{Name: "Cora", Status: "alive", Mood: 20, X: 7, Y: 7},
	}
	w := game.WorldView{Food: 2, Animals: []game.AnimalView{{Species: "ridgewolf", Hostile: true, X: 9, Y: 9}}}
	got := buildAlerts(pawns, w)
	want := []string{"A ridgewolf is attacking!", "Ada needs rescue", "Low food (2 left)", "Cora is unhappy"}
	if len(got) != len(want) {
		t.Fatalf("alerts = %+v", got)
	}
	for i, a := range got {
		if a.label != want[i] {
			t.Fatalf("alert %d = %q, want %q", i, a.label, want[i])
		}
	}
	if !got[0].danger || !got[1].danger || got[2].danger || got[1].targets[0] != [2]int{3, 4} {
		t.Fatalf("severity/targets: %+v", got)
	}
}

func TestAlertClickCentresCamera(t *testing.T) {
	t.Parallel()
	a, g, id := liveColony(t, 0)
	g.SetNeeds(id, 1, 1)
	for i := 0; i < 50 && len(a.currentAlerts()) == 0; i++ {
		g.Step(100)
	}
	als := a.currentAlerts()
	if len(als) == 0 {
		t.Fatal("expected an alert for a starving/collapsed colonist")
	}
	before := a.renderer.cam
	a.focusAlert("alert." + als[0].key)
	if a.renderer.cam == before && len(als[0].targets) > 0 {
		t.Fatal("clicking an alert moves the camera")
	}
}
