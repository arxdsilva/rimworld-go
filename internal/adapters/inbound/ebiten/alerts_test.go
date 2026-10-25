package ebiten

import (
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

func TestBuildAlerts(t *testing.T) {
	t.Parallel()
	pawns := []game.PawnView{
		{Name: "Ada", Status: "downed", Reason: "injured", X: 3, Y: 4},
		{Name: "Bo", Status: "downed", Reason: "injured", InBed: true},
		{Name: "Cora", Status: "alive", Mood: 20, X: 7, Y: 7},
	}
	w := game.WorldView{Food: 2, Animals: []game.AnimalView{{Species: "ridge_wolf", Label: "ridge wolf", Hostile: true, X: 9, Y: 9}}}
	got := buildAlerts(pawns, w)
	// the label ("ridge wolf"), not the species key; no target, so generic
	want := []string{"A ridge wolf is attacking!", "Ada needs rescue", "Low food (2 left)", "Cora is unhappy"}
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

// hostileLabel is the "hostile" alert's text for animals and pawns.
func hostileLabel(t *testing.T, pawns []game.PawnView, animals ...game.AnimalView) alert {
	t.Helper()
	for _, a := range buildAlerts(pawns, game.WorldView{Animals: animals}) {
		if a.key == "hostile" {
			return a
		}
	}
	t.Fatalf("no hostile alert for %+v", animals)
	return alert{}
}

// PLAY-017: the hostile alert says what is chasing whom.
func TestHostileAlertNamesTheChasedColonist(t *testing.T) {
	t.Parallel()
	ada := game.PawnView{ID: entity.EntityID(7), Name: "Ada", Status: "alive", Mood: 80, X: 3, Y: 4}
	wolf := game.AnimalView{ID: 20, Species: "timber_wolf", Label: "timber wolf", Hostile: true, TargetID: ada.ID, X: 9, Y: 9}

	for _, c := range []struct {
		desc    string
		pawns   []game.PawnView
		animals []game.AnimalView
		want    string
	}{
		{"label and target name", []game.PawnView{ada}, []game.AnimalView{wolf}, "A timber wolf is chasing Ada!"},
		{"a vowel takes an", []game.PawnView{ada},
			[]game.AnimalView{{ID: 21, Species: "arctic_fox", Label: "arctic fox", Hostile: true, TargetID: ada.ID, X: 1, Y: 1}},
			"An arctic fox is chasing Ada!"},
		{"no label falls back to the spaced key", []game.PawnView{ada},
			[]game.AnimalView{{ID: 22, Species: "timber_wolf", Hostile: true, TargetID: ada.ID, X: 1, Y: 1}},
			"A timber wolf is chasing Ada!"},
		{"no target stays generic", []game.PawnView{ada},
			[]game.AnimalView{{ID: 23, Species: "timber_wolf", Label: "timber wolf", Hostile: true, X: 1, Y: 1}},
			"A timber wolf is attacking!"},
		{"a target nobody knows stays generic", nil, []game.AnimalView{wolf}, "A timber wolf is attacking!"},
		{"several animals stay a count", []game.PawnView{ada},
			[]game.AnimalView{wolf, {ID: 24, Species: "boar", Label: "boar", Hostile: true, X: 5, Y: 5}},
			"2 animals are attacking!"},
	} {
		al := hostileLabel(t, c.pawns, c.animals...)
		if al.label != c.want {
			t.Errorf("%s: label %q, want %q", c.desc, al.label, c.want)
		}
		if !al.danger || len(al.targets) != len(c.animals) {
			t.Errorf("%s: danger=%v targets=%v, want a danger alert aimed at every animal", c.desc, al.danger, al.targets)
		}
	}

	// a peaceful animal raises nothing, and a chased prisoner is still named
	if got := buildAlerts(nil, game.WorldView{Animals: []game.AnimalView{{Species: "deer", Label: "deer", X: 1, Y: 1}}}); len(got) != 0 {
		t.Errorf("peaceful animal alerts: %+v", got)
	}
	pris := game.PawnView{ID: 8, Name: "Bo", Prisoner: true, Status: "alive", Mood: 80}
	boar := game.AnimalView{ID: 25, Species: "boar", Label: "boar", Hostile: true, TargetID: pris.ID, X: 1, Y: 1}
	if al := hostileLabel(t, []game.PawnView{pris}, boar); al.label != "A boar is chasing Bo!" {
		t.Errorf("prisoner target: %q", al.label)
	}
}
