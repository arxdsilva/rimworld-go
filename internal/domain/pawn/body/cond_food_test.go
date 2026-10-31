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
	// wiki: Ailments: vomit every 0.3 / 0.2 / 0.4 days; major 4-20 h
	if FoodVomitEvery(FoodStageInitial) != 7 || FoodVomitEvery(FoodStageMajor) != 5 || FoodVomitEvery(FoodStageRecovering) != 10 ||
		FoodVomitEvery(9) != 0 || FoodPoisonInitialHours != 4 || FoodPoisonMajorHours != 20 || FoodPoisonHours != 24 {
		t.Fatal("vomit intervals and stage hours")
	}
	major := d.Stages[1]
	if major.PainAdd != 400 || d.Stages[2].PainAdd != 200 || len(major.Mods) != 4 || major.Mods[0].Off != -500 {
		t.Fatalf("stage effects: %+v", d.Stages)
	}
	var none *Body
	if _, ok := none.Poisoned(); ok {
		t.Fatal("a nil body is poisoned")
	}
}
