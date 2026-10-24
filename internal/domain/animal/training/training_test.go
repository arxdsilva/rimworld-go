package training

import "testing"

func TestStepsAndLearned(t *testing.T) {
	t.Parallel()
	if Needed(Obedience) != 3 || Needed(Guard) != 3 || Needed(Release) != 2 || Needed(NumSkills) != 0 {
		t.Fatal("steps")
	}
	if !Learned(Release, 2) || Learned(Release, 1) || Learned(NumSkills, 9) {
		t.Fatal("learned")
	}
}

func TestStepChance(t *testing.T) {
	t.Parallel()
	if StepChance(0, false) != 100 || StepChance(10, false) != 350 {
		t.Fatal("plain")
	}
	if StepChance(10, true) != 1000 || StepChance(0, true) != 500 {
		t.Fatalf("bonded %d %d", StepChance(10, true), StepChance(0, true))
	}
	if StepChance(-4, false) != 100 || StepChance(99, false) != 600 {
		t.Fatal("clamp")
	}
}

func TestDecay(t *testing.T) {
	t.Parallel()
	if Decay(Guard, 3, DecayTicks-1) != 3 || Decay(Guard, 3, DecayTicks) != 2 || Decay(Guard, 3, 2*DecayTicks) != 1 {
		t.Fatal("steps lost")
	}
	if Decay(Guard, 3, 99*DecayTicks) != 0 {
		t.Fatal("floor zero")
	}
	if Decay(Obedience, 3, 99*DecayTicks) != 1 {
		t.Fatal("obedience floor")
	}
	if Decay(Obedience, 2, 99*DecayTicks) != 0 || Decay(Obedience, 0, 5) != 0 || Decay(NumSkills, 4, 1<<40) != 4 {
		t.Fatal("edge")
	}
}

func TestConstants(t *testing.T) {
	t.Parallel()
	if BondTame != 10 || BondTrain != 7 || BondTend != 4 || BondProximity != 1 {
		t.Fatal("bond")
	}
}
