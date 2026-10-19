package animal

import "testing"

func TestSpeciesRules(t *testing.T) {
	t.Parallel()
	s := State{Species: Dustbuck}
	if s.HP() != 40 || s.Dead() {
		t.Fatal("fresh dustbuck")
	}
	s.Injury = 41
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
