package inspector

import (
	"testing"

	"rimworld-go/internal/application/game"
)

// FEED-001: a tame animal's pane says which prepared food it eats.
func TestAnimalFeedLine(t *testing.T) {
	for _, c := range []struct {
		species, diet string
		tame          bool
		want          string
	}{
		{"deer", "plants", true, "Also eats kibble, pemmican and meals"},
		{"timbergnaw", "trees", true, "Also eats kibble and pemmican"},
		{"dire_wolf", "meat only", true, "Eats no kibble or meals"},
		{"deer", "plants", false, ""},
		{"hive_beetle", "", true, ""},
		{"no_such_animal", "plants", true, ""},
	} {
		if got := AnimalFeedLine(game.AnimalView{Species: c.species, Diet: c.diet, Tame: c.tame}); got != c.want {
			t.Errorf("%s (tame %v): %q, want %q", c.species, c.tame, got, c.want)
		}
	}
}
