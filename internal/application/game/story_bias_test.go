package game

import (
	"testing"

	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/pkg/randx"
)

// biasGame is a day-30 map with three colonists, a setup colony of
// storyteller key st.
func biasGame(t *testing.T, st string) *Game {
	t.Helper()
	g := eventMap(t, 5, 60, 40, biome.TemperateForest)
	g.SpawnNamedPawn(18, 15, "Bo")
	g.SpawnNamedPawn(22, 15, "Cy")
	p := difficulty.Wanderer
	g.ng.setup = &Setup{Preset: p, Difficulty: difficulty.Of(p), Storyteller: st}
	return g
}

func helpfulShare(g *Game, n int) int {
	now := uint64(g.clock.Now())
	helpful, all := 0, 0
	for i := 0; i < n; i++ {
		g.incidents.Last = nil
		if id, ok := g.pickNature(now); ok {
			all++
			if incident.DefOf(id).Helpful {
				helpful++
			}
		}
	}
	return helpful * 100 / max(all, 1)
}

// TestPositiveBiasApplied (EAI-033): a storyteller with a positive bias
// draws more helpful events than the same colony without one.
func TestPositiveBiasApplied(t *testing.T) {
	t.Parallel()
	plain := 100 - helpfulShare(biasGame(t, "legacy"), 1000) // bias 0: the share of the rest
	kind := 100 - helpfulShare(biasGame(t, "hearth"), 1000)  // bias 45
	if kind*4 > plain*3 {
		t.Fatalf("other events %d%% with the bias, %d%% without: it should cut them by a quarter", kind, plain)
	}
}

// TestLegacyNoExtraDraw: a colony without setup draws once per nature pick,
// as before, so its saves replay; a setup colony with a bias draws twice.
func TestLegacyNoExtraDraw(t *testing.T) {
	t.Parallel()
	draws := func(g *Game) int {
		st, _ := g.rng.MarshalBinary()
		g.incidents.Last = nil
		if _, ok := g.pickNature(uint64(g.clock.Now())); !ok {
			t.Fatal("nothing to pick")
		}
		for n := 1; n <= 3; n++ {
			r := randx.New(0)
			if err := r.UnmarshalBinary(st); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < n; i++ {
				r.IntN(1000)
			}
			a, _ := r.MarshalBinary()
			if b, _ := g.rng.MarshalBinary(); string(a) == string(b) {
				return n
			}
		}
		return 0
	}
	legacy := biasGame(t, "hearth")
	legacy.ng.setup = nil
	if n := draws(legacy); n != 1 {
		t.Fatalf("a legacy colony draws %d times", n)
	}
	if n := draws(biasGame(t, "hearth")); n != 2 {
		t.Fatalf("a biased setup colony draws %d times", n)
	}
}
