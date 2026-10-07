package infection

import "testing"

func TestChanceFallsWithCare(t *testing.T) {
	t.Parallel()
	if !(Chance(Untreated) > Chance(SelfTended) && Chance(SelfTended) > Chance(Tended) && Chance(Tended) > Chance(Medicine)) {
		t.Fatal("better care, smaller chance")
	}
}

func TestStep(t *testing.T) {
	t.Parallel()
	if Step(Onset, false, Untreated) <= Onset {
		t.Fatal("untreated infections grow")
	}
	if Step(Onset, true, Tended) >= Onset || Step(Onset, true, Medicine) >= Step(Onset, true, Tended) {
		t.Fatal("treatment shrinks it, medicine faster")
	}
	if Step(Fatal, false, Untreated) != Fatal || Step(2, true, Medicine) != 0 {
		t.Fatal("clamped")
	}
}
