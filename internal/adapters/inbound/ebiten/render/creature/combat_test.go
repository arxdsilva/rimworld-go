package creature

import (
	"testing"

	"rimworld-go/internal/application/game"
)

func TestAttackReactions(t *testing.T) {
	t.Parallel()
	traces := []game.TraceView{
		{FromX: 2, FromY: 5, ToX: 6, ToY: 5, Age: 1},              // rifle shot east
		{FromX: 6, FromY: 5, ToX: 7, ToY: 5, Age: 5, Melee: true}, // bite east
		{FromX: 2, FromY: 5, ToX: 6, ToY: 5, Age: 2, Hit: true},
	}
	dx, _, fl := AttackReaction(traces, 2, 5)
	if dx >= 0 || fl != 0 {
		t.Fatalf("shooter recoils west: dx=%v flash=%v", dx, fl)
	}
	dx, _, fl = AttackReaction(traces, 6, 5)
	if dx <= 0 || fl <= 0 {
		t.Fatalf("bitten-at target lunges/knocked east and flashes: dx=%v flash=%v", dx, fl)
	}
	if dx, dy, fl := AttackReaction(traces, 0, 0); dx != 0 || dy != 0 || fl != 0 {
		t.Fatal("bystanders don't move")
	}
}
