package body

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/clock"
)

// TestFoodPoisoningStageEffects (FOOD-6, wiki: Food poisoning): per
// stage the capacity changes (blood filtration x0.95 / x0.85 included),
// pain and eating speed (x0.5 / x0.3). The pain itself lowers
// consciousness further, as every pain does.
func TestFoodPoisoningStageEffects(t *testing.T) {
	t.Parallel()
	var b Body
	if FoodPoisonEatSpeed(&b) != 1000 || FoodPoisonEatSpeed(nil) != 1000 {
		t.Fatal("no poisoning: full eating speed")
	}
	b.Poison(0)
	type want struct{ cons, move, manip, filt, pain, eat int32 }
	mild, major := want{-400, -200, -100, 950, 200, 500}, want{-500, -500, -200, 850, 400, 300}
	for _, tc := range []struct {
		hour uint64
		w    want
	}{{0, mild}, {10, major}, {22, mild}} {
		b.StepFoodPoisoning(tc.hour * clock.TicksPerHour)
		c, _ := b.Poisoned()
		off := map[Capacity]int32{}
		for _, m := range DefOfCond(CondFoodPoisoning).Stages[DefOfCond(CondFoodPoisoning).StageAt(c.Sev)].Mods {
			off[m.Cap] += m.Off
		}
		got := want{off[Consciousness], off[Moving], off[Manipulation], b.Capacity(BloodFiltration), b.Pain(), FoodPoisonEatSpeed(&b)}
		if got != tc.w {
			t.Errorf("hour %d: %+v, want %+v", tc.hour, got, tc.w)
		}
	}
	if b.Capacity(Consciousness) >= 600 || b.Capacity(Moving) > 800 {
		t.Fatal("the capacities fall at least by the stage's changes")
	}
}

// TestSynthStomachHalvesFoodPoisoning (wiki: Food poisoning, Bionic
// stomach): the chance factor is halved only with a fitted synth stomach.
func TestSynthStomachHalvesFoodPoisoning(t *testing.T) {
	t.Parallel()
	var b Body
	if FoodPoisonChance(&b) != 1000 || FoodPoisonChance(nil) != 1000 {
		t.Fatal("a natural stomach keeps the chance")
	}
	b.SetImplant(Stomach, item.SynthStomach)
	if FoodPoisonChance(&b) != 500 {
		t.Fatal("a synth stomach halves the chance")
	}
}

// TestNeedRatesAddOffsets (wiki: Saturation): hunger offsets add up
// before they scale the rate (gut worms +100 % and trivial malnutrition
// +50 % give x2.5, not x3), stoned adds +30 %, the stat multiplier
// (gourmand x1.5) scales the sum, and muscle parasites double the rest.
func TestNeedRatesAddOffsets(t *testing.T) {
	t.Parallel()
	if hr, rr := NeedRates(nil, 1000, 1000, false); hr != 1000 || rr != 1000 {
		t.Fatalf("no body: neutral %d %d", hr, rr)
	}
	if hr, _ := NeedRates(nil, 1000, 1000, true); hr != 1000+needs.StonedHunger {
		t.Fatalf("stoned: %d, want 1300", hr)
	}
	var b Body
	b.AddCond(Condition{Kind: CondGutWorms, Part: Torso, Sev: 1000})
	b.AddCond(Condition{Kind: CondMalnutrition, Part: Torso, Sev: 100, Rate: MalnutritionRateMin})
	if hr, rr := NeedRates(&b, 1000, 1000, false); hr != 2500 || rr != 1000 {
		t.Fatalf("worms + trivial malnutrition: %d %d, want 2500 1000", hr, rr)
	}
	if hr, _ := NeedRates(&b, 1500, 1000, true); hr != 4200 {
		t.Fatalf("gourmand, worms, malnutrition, stoned: %d, want 1500 x 2.8 = 4200", hr)
	}
	var p Body
	p.AddCond(Condition{Kind: CondMuscleParasites, Part: Torso, Sev: 1000})
	if hr, rr := NeedRates(&p, 1000, 1000, false); hr != 1000 || rr != 2000 {
		t.Fatalf("muscle parasites: %d %d, want 1000 2000", hr, rr)
	}
}
