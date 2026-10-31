package game

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/needs"
)

// TestHungerSlowsImmunity (FOOD-6, wiki: Saturation): a ravenously
// hungry colonist gains immunity at 90 %, a malnourished one at 70 %.
func TestHungerSlowsImmunity(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	id, _ := g.SpawnNamedPawn(4, 4, "Pat")
	fed := g.immunityGain(id)
	for band, want := range map[needs.Band]int32{needs.Hungry: fed, needs.Ravenous: fed * 900 / 1000, needs.Malnourished: fed * 700 / 1000} {
		n, _ := g.need.Get(id)
		n.Band = band
		g.need.Set(id, n)
		if got := g.immunityGain(id); got != want {
			t.Errorf("band %d: immunity gain %d, want %d", band, got, want)
		}
	}
}

// TestStonedAndWormsAddHunger (wiki: Saturation): an ashleaf high adds
// +30 % to the hunger rate, on top of gut worms' +100 % (offsets add).
func TestStonedAndWormsAddHunger(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	id, _ := g.SpawnNamedPawn(4, 4, "Pat")
	g.drugState(id).Ingest(item.AshleafTea, nil, uint64(g.clock.Now()))
	if hr, rr := g.diseaseNeedRates(id, 1000, 1000); hr != 1300 || rr != 1000 {
		t.Fatalf("stoned: %d %d, want 1300 1000", hr, rr)
	}
	g.GiveDisease(id, uint8(body.CondGutWorms))
	if hr, _ := g.diseaseNeedRates(id, 1000, 1000); hr != 2300 {
		t.Fatalf("stoned with gut worms: %d, want 2300", hr)
	}
}

// TestSynthStomachHalvesPoisoning (wiki: Food poisoning, Bionic
// stomach): a sure-poison meal poisons every time on a natural stomach
// and about half the time on a synth stomach.
func TestSynthStomachHalvesPoisoning(t *testing.T) {
	t.Parallel()
	hits := func(synth bool) int {
		g := New(Config{Seed: 3, Width: 12, Height: 8})
		id, _ := g.SpawnNamedPawn(4, 4, "Pat")
		v, _ := g.vital.Get(id)
		b := g.ensureBody(id, &v)
		if synth {
			b.SetImplant(body.Stomach, item.SynthStomach)
		}
		n := 0
		for range 400 {
			g.rollFoodPoison(id, item.Stack{Kind: item.Meal, Qty: 1, Poison: 1000})
			if g.poisoned(id) {
				n++
				g.bodies[id].RemoveCond(body.CondFoodPoisoning)
			}
		}
		return n
	}
	if n := hits(false); n != 400 {
		t.Fatalf("natural stomach: %d of 400", n)
	}
	if n := hits(true); n < 150 || n > 250 {
		t.Fatalf("synth stomach: %d of 400, want about 200", n)
	}
}
