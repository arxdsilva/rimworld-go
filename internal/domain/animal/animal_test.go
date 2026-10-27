package animal

import (
	"testing"

	"rimworld-go/internal/domain/pawn/body"
)

func TestSpeciesRules(t *testing.T) {
	t.Parallel()
	s := State{Species: Dustbuck}
	if s.HP() != 55 || s.Dead() { // a deer (formerly dustbuck): size M
		t.Fatal("fresh dustbuck")
	}
	s.Injury = 56
	if !s.Dead() || s.HP() != 0 {
		t.Fatal("dead at zero hp")
	}
	if s.Hungry() {
		t.Fatal("grazers never hunt")
	}
	w := State{Species: Ridgewolf, Hunger: DefOf(Ridgewolf).HungerTicks}
	if !w.Hungry() {
		t.Fatal("wolf hungry after HungerTicks")
	}
	s.Mode = Flee
	if s.Step() >= DefOf(Dustbuck).StepTicks {
		t.Fatal("fleeing dustbucks run faster")
	}
	if sp, ok := ParseSpecies("ridgewolf"); !ok || sp != Ridgewolf {
		t.Fatal("parse")
	}
}

func TestFleeGoalPointsAway(t *testing.T) {
	t.Parallel()
	x, y := FleeGoal(10, 10, 7, 10, 6)
	if x != 16 || y != 10 {
		t.Fatalf("flee east from a west threat: got %d,%d", x, y)
	}
	x, y = FleeGoal(10, 10, 12, 13, 6)
	if x >= 10 || y >= 10 {
		t.Fatalf("flee up-left from a down-right threat: got %d,%d", x, y)
	}
}

func TestStepCostScalesDiagonals(t *testing.T) {
	t.Parallel()
	cases := []struct {
		s          State
		straight   int
		diagonally int
	}{
		{State{Species: Dustbuck}, 16, 23},             // round(16 * 1.414)
		{State{Species: Gazelle}, 12, 17},              // round(12 * 1.414)
		{State{Species: Dustbuck, Mode: Flee}, 10, 14}, // round(10 * 1.414)
	}
	for _, c := range cases {
		if got := c.s.StepCost(false); got != c.straight {
			t.Errorf("%v straight: %d, want %d", c.s.Species, got, c.straight)
		}
		if got := c.s.StepCost(true); got != c.diagonally {
			t.Errorf("%v diagonal: %d, want %d", c.s.Species, got, c.diagonally)
		}
	}
}

func TestFighterAndCharger(t *testing.T) {
	cases := []struct {
		s                Species
		fighter, charger bool
	}{
		{Deer, false, false}, {Sheep, false, false}, {Cat, false, false}, {Tortoise, false, false},
		{Elk, false, false}, {WildBoar, true, false}, {Cougar, true, true}, {TimberWolf, true, true},
		{Glimmerstag, true, false}, {HiveBeetle, false, false},
	}
	for _, c := range cases {
		if Fighter(c.s) != c.fighter || Charger(c.s) != c.charger {
			t.Errorf("%s: fighter %v charger %v, want %v %v", c.s, Fighter(c.s), Charger(c.s), c.fighter, c.charger)
		}
	}
}

// Downed thresholds: down at 70% injury or severe blood loss, up only at
// 50% injury with blood loss below moderate (the gap is the hysteresis).
func TestDownedThresholds(t *testing.T) {
	t.Parallel()
	hp := DefOf(Dustbuck).MaxHP // 55
	s := State{Species: Dustbuck, Injury: hp*DownInjuryPct/100 - 1}
	if s.DownsNow() {
		t.Fatal("below 70% injury it stands")
	}
	s.Injury = (hp*DownInjuryPct + 99) / 100
	if !s.DownsNow() {
		t.Fatal("at 70% injury it goes down")
	}
	s.Downed = true
	if s.DownsNow() || s.StandsUp() {
		t.Fatal("down already; too hurt to stand")
	}
	s.Injury = hp * UpInjuryPct / 100
	if !s.StandsUp() {
		t.Fatal("at 50% injury it stands")
	}
	s.Body = &body.Body{BloodLoss: UpBloodLoss}
	if s.StandsUp() {
		t.Fatal("moderate blood loss keeps it down")
	}
	s = State{Species: Dustbuck, Body: &body.Body{BloodLoss: DownBloodLoss}}
	if !s.DownsNow() {
		t.Fatal("severe blood loss downs an unhurt animal")
	}
	s.Injury = hp
	if s.DownsNow() {
		t.Fatal("the dead do not go down")
	}
}
