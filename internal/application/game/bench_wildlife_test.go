package game

import (
	"testing"

	"rimworld-go/internal/domain/animal"
)

// Wildlife performance gate (SIM-024): a 120x120 temperate colony with 200
// wild animals in herds, stepped one in-game hour per op (24 ops make a
// day). Compare with
//   go test -run '^$' -bench 'Wildlife|Colony' -benchtime 5x ./internal/application/game/

// wildlifeColony is the benchmark's map: the start colony plus herds of
// the biome's species until 200 wild animals stand on it.
func wildlifeColony(tb testing.TB) *Game {
	tb.Helper()
	return addHerds(tb, NewColony(Config{Seed: 3, Width: 120, Height: 120}))
}

// wildMap is the same map with no colony at all: the animals' own cost.
func wildMap(tb testing.TB) *Game {
	tb.Helper()
	g := New(Config{Seed: 3, Width: 120, Height: 120})
	g.generate(3)
	g.seedGrass()
	return addHerds(tb, g)
}

func addHerds(tb testing.TB, g *Game) *Game {
	tb.Helper()
	for tries := 0; len(g.animalIDs) < 200 && tries < 1000; tries++ {
		s, ok := g.pickSpecies(func(s animal.Species) bool { return true })
		if !ok {
			tb.Fatal("no wild species")
		}
		if x, y, ok := g.randomTileAway(wildStartRadius); ok {
			g.spawnHerd(s, x, y, min(g.herdSizeOf(s), 200-len(g.animalIDs)))
		}
	}
	if len(g.animalIDs) < 200 {
		tb.Fatalf("only %d animals", len(g.animalIDs))
	}
	return g
}

// BenchmarkWildlife steps the 200-animal map through one in-game hour.
func BenchmarkWildlife(b *testing.B) {
	for b.Loop() {
		b.StopTimer()
		g := wildlifeColony(b)
		b.StartTimer()
		g.Step(2500)
	}
}

// BenchmarkWildlifeDay steps the 200-animal map through a whole day (needs
// cycle through feeding and a night's rest).
func BenchmarkWildlifeDay(b *testing.B) {
	for b.Loop() {
		b.StopTimer()
		g := wildlifeColony(b)
		b.StartTimer()
		g.Step(60000)
	}
}

// BenchmarkWildlifeAnimals steps the 200 animals of a map with no colony
// through one in-game hour (the pawns' own work search is left out).
func BenchmarkWildlifeAnimals(b *testing.B) {
	for b.Loop() {
		b.StopTimer()
		g := wildMap(b)
		b.StartTimer()
		g.Step(2500)
	}
}
