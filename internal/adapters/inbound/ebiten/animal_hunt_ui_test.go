package ebiten

import (
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
	// Food + Rest, the diet line and the hunting note
	if h := animalExtraH(rex); h != 40+16+16 {
		t.Errorf("extra height = %d", h)
	}
	dog := game.AnimalView{Species: "husky", Tame: true, State: "eating from the stockpile", Diet: "meat"}
	if animalHuntLine(dog) != "" || animalExtraH(dog) != 40+16 {
		t.Errorf("a pet has no hunting note: %q %d", animalHuntLine(dog), animalExtraH(dog))
	}
	if got := a.animalStateLine(dog); got != "Eating from the stockpile" {
		t.Errorf("state %q", got)
	}
	if got := animalDoing(game.AnimalView{State: "eating berries"}); got != "eating berries" {
		t.Errorf("colony list %q", got)
	}
}
