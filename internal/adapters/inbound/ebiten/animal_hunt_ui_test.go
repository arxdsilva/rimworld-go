package ebiten

import (
	"slices"
	"testing"

	"rimworld-go/internal/application/game"
)

// PLAY-017 §8: a tame predator's inspector notes that it hunts, and the
// state line reads what it is eating or hunting.
func TestTamePredatorHuntLine(t *testing.T) {
	t.Parallel()
	a := &App{}
	rex := game.AnimalView{Species: "timber_wolf", Tame: true, State: "hunting a hare", Diet: "meat", Hunts: true, Food: 30, Rest: 80}
	if got := animalHuntLine(rex); got != "Hunts wild animals when hungry" {
		t.Errorf("hunt line %q", got)
	}
	if got := a.animalStateLine(rex); got != "Hunting a hare" {
		t.Errorf("state %q", got)
	}
	// Food + Rest, the diet line and the hunting note (the pane's lines)
	if ls := a.animalPaneLines(rex); !animalHasNeeds(rex) || !slices.Contains(ls, "Diet: meat") || !slices.Contains(ls, "Hunts wild animals when hungry") {
		t.Errorf("needs %v, pane lines %q", animalHasNeeds(rex), ls)
	}
	dog := game.AnimalView{Species: "husky", Tame: true, State: "eating from the stockpile", Diet: "meat"}
	if ls := a.animalPaneLines(dog); animalHuntLine(dog) != "" || !slices.Contains(ls, "Diet: meat") || slices.Contains(ls, "Hunts wild animals when hungry") {
		t.Errorf("a pet has no hunting note: %q %q", animalHuntLine(dog), ls)
	}
	if got := a.animalStateLine(dog); got != "Eating from the stockpile" {
		t.Errorf("state %q", got)
	}
	if got := animalDoing(game.AnimalView{State: "eating berries"}); got != "eating berries" {
		t.Errorf("colony list %q", got)
	}
}

// PLAY-018: the same lines on the pane. A tame predator's pane carries the
// hunting note after its diet, with Food and Rest bars; a pet has no note.
func TestTamePredatorPaneHuntLine(t *testing.T) {
	t.Parallel()
	s := &stubSim{world: game.WorldView{Animals: []game.AnimalView{
		{ID: 1, Species: "timber_wolf", Label: "timber wolf", Name: "Rex", Tame: true, State: "hunting a hare", Diet: "meat", Hunts: true, Health: 100, Food: 30, Rest: 80, X: 3, Y: 3},
		{ID: 2, Species: "husky", Label: "husky", Name: "Fido", Tame: true, State: "eating from the stockpile", Diet: "meat", Health: 100, Food: 50, Rest: 50, X: 4, Y: 3},
	}}}
	a, _ := colonyApp(t, s)
	a.selectCand(selCand{Kind: selAnimal, ID: 1})
	sp := a.animalPane(1)
	lines := sp.Lines
	diet, note := -1, -1
	for i, l := range lines {
		switch l {
		case "Diet: meat":
			diet = i
		case "Hunts wild animals when hungry":
			note = i
		}
	}
	if diet < 0 || note != diet+1 {
		t.Fatalf("the hunting note follows the diet line: %q", lines)
	}
	if lines[0] != "Hunting a hare" {
		t.Errorf("state line %q", lines[0])
	}
	for label, pct := range map[string]int{"Health": 100, "Food": 30, "Rest": 80} {
		if b, ok := thingBar(sp, label); !ok || b.Pct != pct {
			t.Errorf("bar %s = %+v (%v), want %d", label, b, ok, pct)
		}
	}
	a.selectCand(selCand{Kind: selAnimal, ID: 2})
	pet := a.animalPane(2)
	for _, l := range pet.Lines {
		if l == "Hunts wild animals when hungry" {
			t.Errorf("a pet has no hunting note: %q", pet.Lines)
		}
	}
	if pet.Lines[0] != "Eating from the stockpile" {
		t.Errorf("pet state line %q", pet.Lines[0])
	}
}
