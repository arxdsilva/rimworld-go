package body

import (
	"testing"

	"rimworld-go/internal/domain/sim/clock"
)

func TestFoodPoisoningStagesByTime(t *testing.T) {
	var b Body
	if !b.Poison(0) || b.Poison(0) {
		t.Fatal("a body takes one food poisoning at a time")
	}
	d := DefOfCond(CondFoodPoisoning)
	if d.Lethal || !d.NoImmunity || d.TendEvery != 0 || d.SevPerDay != 0 {
		t.Fatalf("food poisoning def %+v: never lethal, no immunity, no tending, no race", d)
	}
	want := func(h uint64) (uint8, bool) {
		switch {
		case h >= FoodPoisonHours:
			return 0, false
		case h >= FoodPoisonMajorHours:
			return FoodStageRecovering, true
		case h >= FoodPoisonInitialHours:
			return FoodStageMajor, true
		}
		return FoodStageInitial, true
	}
	for h := uint64(0); h <= FoodPoisonHours; h++ {
		b.StepFoodPoisoning(h * clock.TicksPerHour)
		c, ok := b.Poisoned()
		ws, wok := want(h)
		if ok != wok || ok && (c.Stage != ws || d.StageAt(c.Sev) != c.Stage) {
			t.Fatalf("hour %d: %+v (present %v), want stage %d (present %v)", h, c, ok, ws, wok)
		}
	}
	if FoodVomitEvery(FoodStageMajor) == 0 || FoodVomitEvery(FoodStageRecovering) != 0 || FoodVomitEvery(9) != 0 {
		t.Fatal("vomit intervals")
	}
	var none *Body
	if _, ok := none.Poisoned(); ok {
		t.Fatal("a nil body is poisoned")
	}
}
