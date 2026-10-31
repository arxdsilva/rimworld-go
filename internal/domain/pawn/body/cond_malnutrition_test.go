package body

import "testing"

// rollAt is a fixed RNG: every roll returns v (clamped to the range).
func rollAt(v int) func(int) int { return func(n int) int { return min(v, n-1) } }

// TestMalnutritionStagesAndDeath (FOOD-1, wiki: Malnutrition): starving,
// severity rises by the pawn's rolled rate through the five stages in
// about two days (1.51-2.27 % an hour), lowering consciousness, and kills
// at full severity.
func TestMalnutritionStagesAndDeath(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		roll           int
		minH, maxH     int
		rate           int32
		wantConsc80Max int32
	}{
		{0, 66, 67, MalnutritionRateMin, 100}, {1 << 20, 44, 45, MalnutritionRateMax, 100},
	} {
		var b Body
		stages, dead, hours := map[uint8]bool{}, false, 0
		for h := uint64(0); !dead && h < 200; h++ {
			_, dead = StepMalnutrition(&b, true, h, rollAt(c.roll))
			hours++
			if cond, ok := b.Cond(CondMalnutrition); ok {
				stages[cond.Stage] = true
				if cond.Rate != c.rate {
					t.Fatalf("rolled rate %d, want %d", cond.Rate, c.rate)
				}
				if cond.Stage == 4 && b.Capacity(Consciousness) > c.wantConsc80Max {
					t.Fatalf("extreme stage consciousness %d", b.Capacity(Consciousness))
				}
			}
		}
		if !dead || hours < c.minH || hours > c.maxH {
			t.Fatalf("roll %d: dead %v after %d hours, want %d-%d", c.roll, dead, hours, c.minH, c.maxH)
		}
		if len(stages) != 5 {
			t.Fatalf("stages seen %v, want all five", stages)
		}
	}
}

// TestMalnutritionRecovers: fed again, severity falls at the same rate and
// the condition goes; nothing happens to a fed body without it.
func TestMalnutritionRecovers(t *testing.T) {
	t.Parallel()
	var b Body
	if changed, _ := StepMalnutrition(&b, false, 0, rollAt(0)); changed || Malnutrition(&b) != 0 {
		t.Fatal("a fed body grows no malnutrition")
	}
	h := uint64(0)
	for ; h < 30; h++ {
		StepMalnutrition(&b, true, h, rollAt(0))
	}
	peak := Malnutrition(&b)
	if peak < 400 || peak > 460 || MalnourishedMood(peak) != -32 {
		t.Fatalf("30 starving hours at 1.51 %%: severity %d, mood %d", peak, MalnourishedMood(peak))
	}
	gone := false
	for i := 0; i < 40 && !gone; i++ {
		changed, dead := StepMalnutrition(&b, false, h, rollAt(0))
		h++
		if dead {
			t.Fatal("recovering is never fatal")
		}
		gone = changed && Malnutrition(&b) == 0
	}
	if !gone || len(b.Conds) != 0 {
		t.Fatalf("not recovered after 40 fed hours: %d", Malnutrition(&b))
	}
}

// TestMalnutritionEffects: hunger offsets by stage, a disease's own
// multiplier kept, the mood stages, and wounds that don't heal.
func TestMalnutritionEffects(t *testing.T) {
	t.Parallel()
	if HungerOffset(Condition{Kind: CondMalnutrition}) != 500 || HungerOffset(Condition{Kind: CondMalnutrition, Stage: 3}) != 600 ||
		HungerOffset(Condition{Kind: CondFoodPoisoning}) != 0 {
		t.Fatal("hunger offsets: +50 % trivial, +60 % after")
	}
	if dz, _ := DiseaseOf(CondGutWorms); HungerOffset(Condition{Kind: CondGutWorms}) != dz.HungerMul-1000 {
		t.Fatal("a disease keeps its hunger offset")
	}
	for sev, want := range map[int32]int{0: -20, 200: -26, 450: -32, 650: -38, 999: -44} {
		if got := MalnourishedMood(sev); got != want {
			t.Errorf("mood at %d: %d, want %d", sev, got, want)
		}
	}
	var b Body
	b.Wounds = []Wound{{Part: Torso, Dmg: 5, Kind: Cut}}
	b.AddCond(Condition{Kind: CondMalnutrition, Part: Torso, Sev: 100, Rate: MalnutritionRateMin})
	for range 200 {
		b.HealHour(rollAt(999), LieBed)
	}
	if len(b.Wounds) != 1 || b.Wounds[0].Dmg != 5 {
		t.Fatalf("a malnourished body healed: %+v", b.Wounds)
	}
	var none *Body
	if Malnutrition(none) != 0 {
		t.Fatal("a nil body is not malnourished")
	}
}
