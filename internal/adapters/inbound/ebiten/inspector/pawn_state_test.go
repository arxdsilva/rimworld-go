package inspector

import (
	"slices"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

func TestPawnStateLines(t *testing.T) {
	t.Parallel()
	name := func(id entity.EntityID) string { return "" }
	has := func(p game.PawnView, want string) bool {
		return slices.Contains(PawnStatusLines(p, name), want)
	}
	if !has(game.PawnView{Status: "alive", BedRest: true, InBed: true}, "On bed rest") {
		t.Error("bed rest line missing")
	}
	if has(game.PawnView{Status: "dead", BedRest: true}, "On bed rest") {
		t.Error("dead pawn shows bed rest")
	}
	if !has(game.PawnView{Status: "alive", Visitor: true}, "Stranger") {
		t.Error("stranger line missing")
	}
	if has(game.PawnView{Status: "alive", Visitor: true, Prisoner: true}, "Stranger") {
		t.Error("prisoner shown as stranger")
	}
	if has(game.PawnView{Status: "alive"}, "Stranger") {
		t.Error("colonist shown as stranger")
	}
	if !has(game.PawnView{Status: "alive", MentalBreak: "berserk"}, "MENTAL BREAK: berserk") {
		t.Error("break line missing")
	}
	if got := mentalBreakLine(game.PawnView{}, name); got != "" {
		t.Errorf("no break line = %q", got)
	}
}
