package body

import "testing"

// TestExposureStagesReachCapacities (TEMP-3, wiki: Hypothermia,
// Heatstroke): each stage's consciousness, manipulation (cold only),
// moving and pain; the extreme stage caps consciousness at 10 %.
func TestExposureStagesReachCapacities(t *testing.T) {
	b := &Body{}
	if !SetExposure(b, CondHypothermia, 360) || ExposureSev(b, CondHypothermia) != 360 {
		t.Fatal("hypothermia set")
	}
	if c, m, mv, p := b.Capacity(Consciousness), b.Capacity(Manipulation), b.Capacity(Moving), b.Pain(); c >= 800 || m > 300 || mv > 700 || p != 150 {
		t.Errorf("serious hypothermia: cons %d manip %d moving %d pain %d", c, m, mv, p)
	}
	if SetExposure(b, CondHypothermia, 370) {
		t.Error("no stage change inside a stage")
	}
	SetExposure(b, CondHypothermia, 700)
	if c := b.Capacity(Consciousness); c != 100 {
		t.Errorf("extreme: consciousness %d, want 100", c)
	}
	if !SetExposure(b, CondHypothermia, 0) || b.Conds != nil {
		t.Error("cleared at 0")
	}
	cold := &Body{}
	SetExposure(cold, CondHypothermia, 250)
	SetExposure(b, CondHeatstroke, 250)
	if m, mv := b.Capacity(Manipulation), b.Capacity(Moving); m != b.Capacity(Consciousness) || mv >= 900 || cold.Capacity(Manipulation) >= m {
		t.Errorf("heatstroke has no manipulation term of its own: manip %d moving %d", m, mv)
	}
	SetExposure(b, CondHypothermia, 50) // both at once
	if len(b.Conds) != 2 || DefOfCond(CondHeatstroke).Lethal || DefOfCond(CondHypothermia).TendEvery != 0 {
		t.Error("both present, neither lethal by the race nor tended")
	}
}

// TestFrostbiteTakesExtremities: only an extremity of the plan, never the
// whole part, no bleeding, a quarter infected untended (infection
// package), a 1 % scar.
func TestFrostbiteTakesExtremities(t *testing.T) {
	ext := map[Part]bool{EarL: true, EarR: true, Nose: true, HandL: true, HandR: true, FootL: true, FootR: true}
	b := &Body{}
	for i := 0; i < 200; i++ {
		p, dmg, ok := StrikeFrostbite(b, func(n int) int { return (i * 7) % n }, 1)
		if !ok {
			break
		}
		if !ext[p] || dmg < 1 || dmg > frostbiteMaxDmg {
			t.Fatalf("frostbite on %s", p.Label())
		}
	}
	if b.Missing != 0 || b.BleedPerDay() != 0 || FrostbiteParts(b) != 7 {
		t.Errorf("missing %b bleed %d parts %d", b.Missing, b.BleedPerDay(), FrostbiteParts(b))
	}
	for _, p := range frostParts[Humanlike] {
		if b.partDmg(p) != b.maxHP(p)-1 {
			t.Errorf("%s at %d of %d", p.Label(), b.partDmg(p), b.maxHP(p))
		}
	}
	if _, _, ok := StrikeFrostbite(b, func(int) int { return 0 }, 1); ok {
		t.Error("nothing left to take")
	}
	if _, _, ok := StrikeFrostbite(&Body{Anatomy: Insect}, func(int) int { return 0 }, 1); ok {
		t.Error("insects never frostbite")
	}
	if ScarOdds(Frostbite, 1) != 10 || ScarOdds(Frostbite, 20) != 10 {
		t.Error("frostbite scars 1 % of the time")
	}
}
