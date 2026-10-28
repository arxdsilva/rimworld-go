package orders

import (
	"testing"

	"rimworld-go/internal/application/game"
)

func TestFormationCellsDeterministic(t *testing.T) {
	t.Parallel()
	free := func(x, y int) bool { return !(x == 5 && y == 5) && x >= 0 && y >= 0 }
	got := FormationCells(5, 5, 4, free)
	want := [][2]int{{5, 4}, {4, 5}, {6, 5}, {5, 6}}
	for i := range want {
		if len(got) != 4 || got[i] != want[i] {
			t.Fatalf("cells = %v, want %v", got, want)
		}
	}
	line := LineCells(0, 0, 8, 0, 3, func(x, y int) bool { return true })
	if len(line) != 3 || line[1] != [2]int{4, 0} || line[2] != [2]int{8, 0} {
		t.Fatalf("line = %v", line)
	}
	ps := []game.PawnView{{ID: 1, X: 9}, {ID: 2}}
	m := AssignCells(ps, [][2]int{{0, 1}, {9, 1}})
	if m[0].Pawn != 2 || m[1].Pawn != 1 {
		t.Fatalf("nearest colonist takes each cell: %+v", m)
	}
}

func TestFormationMovesSingleColonistGoesExactly(t *testing.T) {
	t.Parallel()
	m := FormationMoves([]game.PawnView{{ID: 4}}, 3, 3, 3, 3, func(x, y int) bool { return false })
	if len(m) != 1 || m[0].X != 3 || m[0].Y != 3 {
		t.Fatalf("moves %+v", m)
	}
	if got := draftedOf([]game.PawnView{{ID: 1}, {ID: 2, Drafted: true}}); len(got) != 1 || got[0].ID != 2 {
		t.Fatalf("drafted %+v", got)
	}
}

func TestWalkableFnBlocksWallsAndEdges(t *testing.T) {
	t.Parallel()
	w := game.WorldView{Buildings: []game.BuildingView{{Kind: "wall", X: 1, Y: 1}}}
	free := walkableFn(w, 4, 4)
	if free(1, 1) || free(-1, 0) || free(4, 0) || !free(2, 2) {
		t.Fatal("walls and the map edge block")
	}
}
