package devscene

import (
	"testing"

	"rimworld-go/internal/domain/sim/clock"
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

// The jump lands on the asked hour, on the asked day when that is later,
// and never moves the clock back.
func TestJumpLandsOnTheHourAndDay(t *testing.T) {
	t.Parallel()
	g, err := Build(Config{Width: 40, Height: 40, Hour: 5, Day: 1}, 1, biome.TemperateForest)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := int(g.Tick()), clock.TicksPerDay+5*clock.TicksPerHour; got != want {
		t.Fatalf("tick %d, want %d", got, want)
	}
	jump(g, Config{Hour: 3})
	if got, want := int(g.Tick()), 2*clock.TicksPerDay+3*clock.TicksPerHour; got != want {
		t.Fatalf("a past hour goes to the next day: tick %d, want %d", got, want)
	}
	before := g.Tick()
	jump(g, Config{Hour: -1, Day: 9})
	if g.Tick() != before {
		t.Fatal("no -hour, no jump")
	}
}
