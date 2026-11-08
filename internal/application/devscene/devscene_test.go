package devscene

import (
	"testing"

	"rimworld-go/internal/domain/world/biome"
)

// Every set piece builds on a map big enough for the zoo, and the -hour
// jump lands on the asked hour.
func TestEverySceneBuilds(t *testing.T) {
	t.Parallel()
	for name := range scenes {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			g, err := Build(Config{Scenario: name, Width: 80, Height: 60, Hour: 9}, 1, biome.TemperateForest)
			if err != nil || g == nil {
				t.Fatalf("%v", err)
			}
		})
	}
}

func TestKnownAndBare(t *testing.T) {
	t.Parallel()
	for _, n := range []string{"walls", "grown", "social", "lategame", "zoo"} {
		if !Known(n) {
			t.Errorf("%q unknown", n)
		}
	}
	if Known("nope") || Bare("lategame") || !Bare("walls") {
		t.Error("Known/Bare")
	}
}
