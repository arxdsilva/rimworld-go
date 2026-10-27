package ebiten

import (
	"testing"

	"rimworld-go/internal/application/game"
)

// TestColonistBarSkipsVisitorsAndPrisoners: traders and guests are not
// colonists, so like prisoners they stay off the colonist bar.
func TestColonistBarSkipsVisitorsAndPrisoners(t *testing.T) {
	s := &stubSim{pawns: []game.PawnView{{ID: 1, Name: "Ada"}, {ID: 2, Name: "Trader", Visitor: true}, {ID: 3, Name: "Pris", Prisoner: true}, {ID: 4, Name: "Bo"}}}
	a := testApp(s)
	a.finishBoot()
	a.startGame()
	got := a.barPawns()
	if len(got) != 2 || got[0].ID != 1 || got[1].ID != 4 {
		t.Fatalf("bar pawns = %+v", got)
	}
}
