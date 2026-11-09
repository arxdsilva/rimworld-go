package inspector

import (
	"testing"

	"rimworld-go/internal/application/game"
)

// TestAnimalActionLines (EAI): the pane says when an animal is breaking
// through a door, grieving or defending, and says nothing otherwise.
func TestAnimalActionLines(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		an   game.AnimalView
		want string
	}{
		{game.AnimalView{Bashing: true}, "Breaking through the door"},
		{game.AnimalView{Grief: true, Manhunter: true}, "Manhunter (grief)"},
		{game.AnimalView{Grief: true}, "Grieving a herd mate"},
		{game.AnimalView{Defending: true}, "Defending its herd"},
		{game.AnimalView{}, ""},
	} {
		if got := AnimalActionLine(c.an); got != c.want {
			t.Errorf("%+v: %q, want %q", c.an, got, c.want)
		}
	}
}
