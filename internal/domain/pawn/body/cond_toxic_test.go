package body

import "testing"

// TestToxicBuildupRatesAndDeath (FOOD-6, wiki: Toxic buildup): under the
// fallout a human gains 40 % a day and dies at 100 % on day 2.5; an
// animal (50 % resistance) gains half; out of it the buildup falls 8 % a
// day and is gone.
func TestToxicBuildupRatesAndDeath(t *testing.T) {
	t.Parallel()
	var b Body
	never := rollAt(1 << 20)
	for h := uint64(0); h < 24; h++ {
		StepToxic(&b, true, 0, h, never)
	}
	if got := ToxicBuildup(&b); got != ToxicPerDay {
		t.Fatalf("a day outdoors: %d, want %d", got, ToxicPerDay)
	}
	if c, _ := b.Cond(CondToxicBuildup); c.Stage != 3 || b.Capacity(Consciousness) > 850 {
		t.Fatalf("40 %%: stage %d consciousness %d, want moderate (-15 %%)", c.Stage, b.Capacity(Consciousness))
	}
	dead, hours := false, 24
	for h := uint64(24); !dead && h < 200; h++ {
		dead = StepToxic(&b, true, 0, h, never).Dead
		hours++
	}
	if !dead || hours != 60 {
		t.Fatalf("dead %v after %d hours, want 60", dead, hours)
	}
	var a Body
	for h := uint64(0); h < 24; h++ {
		StepToxic(&a, true, ToxicResistAnimal, h, never)
	}
	if got := ToxicBuildup(&a); got != ToxicPerDay/2 {
		t.Fatalf("an animal's day: %d, want %d", got, ToxicPerDay/2)
	}
	for h := uint64(24); h < 24+24*2; h++ {
		StepToxic(&a, false, ToxicResistAnimal, h, never)
	}
	if got := ToxicBuildup(&a); got != ToxicPerDay/2-2*ToxicRecoverPerDay {
		t.Fatalf("two days sheltered: %d, want %d", got, ToxicPerDay/2-2*ToxicRecoverPerDay)
	}
	for h := uint64(72); h < 72+24; h++ {
		StepToxic(&a, false, ToxicResistAnimal, h, never)
	}
	if _, ok := a.Cond(CondToxicBuildup); ok {
		t.Fatal("the buildup is gone once it falls to nothing")
	}
	var immune Body
	if st := StepToxic(&immune, true, 1000, 0, never); st.Changed || ToxicBuildup(&immune) != 0 {
		t.Fatal("full resistance: no buildup")
	}
}

// TestToxicBuildupChances: the moderate and worse stages roll vomiting,
// dementia and carcinoma; the lower stages never do.
func TestToxicBuildupChances(t *testing.T) {
	t.Parallel()
	always := rollAt(0)
	var b Body
	b.AddCond(Condition{Kind: CondToxicBuildup, Part: Torso, Sev: 100})
	if st := StepToxic(&b, true, 0, 0, always); st.Vomit || st.Dementia || st.Carcinoma {
		t.Fatalf("minor stage: %+v", st)
	}
	c, _ := b.Cond(CondToxicBuildup)
	c.Sev = 850
	if st := StepToxic(&b, true, 0, 0, always); !st.Vomit || !st.Dementia || !st.Carcinoma || b.Capacity(Consciousness) > 100 {
		t.Fatalf("extreme stage: %+v, consciousness %d", st, b.Capacity(Consciousness))
	}
}

// TestToxicBuildupNoFlap: sheltered after a fallout the buildup only
// falls, so its stage steps down once per threshold and never back.
func TestToxicBuildupNoFlap(t *testing.T) {
	t.Parallel()
	var b Body
	b.AddCond(Condition{Kind: CondToxicBuildup, Part: Torso, Sev: 700, Stage: DefOfCond(CondToxicBuildup).StageAt(700)})
	last, changes := int32(701), 0
	for h := uint64(0); h < 24*10; h++ {
		if StepToxic(&b, false, 0, h, rollAt(1<<20)).Changed {
			changes++
		}
		sev := ToxicBuildup(&b)
		if sev > last {
			t.Fatalf("hour %d: buildup rose to %d sheltered", h, sev)
		}
		last = sev
	}
	if changes != 5 { // serious → moderate → minor → initial → faint → gone
		t.Fatalf("%d stage changes, want 5", changes)
	}
}
