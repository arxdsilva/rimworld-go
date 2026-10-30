package cue

import (
	"reflect"
	"testing"

	"rimworld-go/internal/application/game"
)

func ids(b *Bus) []string {
	var out []string
	for _, c := range b.Take() {
		out = append(out, c.ID)
	}
	return out
}

// TestWave3StateCuesOnceOnTheEdge: a stagger, tantrum and paste job sound
// when they begin, not on the first frame, not while they last, and again
// only after they ended.
func TestWave3StateCuesOnceOnTheEdge(t *testing.T) { // not parallel: AllocsPerRun
	r := newBus(1)
	hear := func(p game.PawnView, a game.AnimalView) []string {
		r.b.listenWave3(game.WorldView{Animals: []game.AnimalView{a}}, []game.PawnView{p})
		return ids(r.b)
	}
	calm, still := game.PawnView{ID: 1}, game.AnimalView{ID: 9}
	if got := hear(game.PawnView{ID: 1, Staggered: true}, still); got != nil {
		t.Fatalf("first frame: %v", got)
	}
	hear(calm, still)
	busy := game.PawnView{ID: 1, Staggered: true, MentalBreak: "tantrum", Job: "dispense paste"}
	want := []string{"sfx.stagger_thud", "sfx.tantrum_smash", "sfx.paste_dispense"}
	if got := hear(busy, game.AnimalView{ID: 9, Staggered: true}); !reflect.DeepEqual(got, append(want, "sfx.stagger_thud")) {
		t.Fatalf("onset: %v", got)
	}
	if got := hear(busy, game.AnimalView{ID: 9, Staggered: true}); got != nil {
		t.Fatalf("repeated: %v", got)
	}
	hear(calm, still)
	if got := hear(game.PawnView{ID: 1, Staggered: true}, still); !reflect.DeepEqual(got, want[:1]) {
		t.Fatalf("second stagger: %v", got)
	}
	if n := testing.AllocsPerRun(50, func() { hear(busy, still); r.b.Take() }); n > 4 {
		t.Fatalf("%v allocations per frame", n)
	}
}

// TestShieldPopsOnceWhenItBreaks: a shield falling to 0 pops once; a
// shield that was never up, or stays down, is silent.
func TestShieldPopsOnceWhenItBreaks(t *testing.T) {
	t.Parallel()
	r := newBus(1)
	hear := func(pct int) []string {
		r.b.ListenShields([]ShieldState{{ID: 3, Pct: pct, X: 2, Y: 2}})
		return ids(r.b)
	}
	for _, c := range []struct {
		pct  int
		want []string
	}{{0, nil}, {100, nil}, {40, nil}, {0, []string{"sfx.shield_pop"}}, {0, nil}, {100, nil}, {0, []string{"sfx.shield_pop"}}} {
		if got := hear(c.pct); !reflect.DeepEqual(got, c.want) {
			t.Fatalf("pct %d: %v, want %v", c.pct, got, c.want)
		}
	}
}

// TestSmokeBurstHissesOnce: the smoke blast kind has its sound.
func TestSmokeBurstHissesOnce(t *testing.T) {
	t.Parallel()
	r := newBus(1)
	b := []game.BurstView{{X: 3, Y: 4, Tick: 5, Blast: "smoke", Radius: 72}}
	r.b.ListenBursts(b)
	r.b.ListenBursts(b)
	if got := ids(r.b); !reflect.DeepEqual(got, []string{"sfx.smokepop_burst"}) {
		t.Fatalf("cues %v", got)
	}
}
