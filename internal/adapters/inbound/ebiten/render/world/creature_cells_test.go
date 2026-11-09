package world

import (
	"testing"

	"rimworld-go/internal/application/game"
)

// TestCreatureCellsRebuildOnlyOnChange: the cell sets are kept while no
// creature moves, and follow one that does (GAR-153).
func TestCreatureCellsRebuildOnlyOnChange(t *testing.T) {
	var l LOD
	pawns := []game.PawnView{{X: 1, Y: 2}}
	animals := []game.AnimalView{{X: 3, Y: 4}}
	l.refreshCreatureCells(pawns, animals)
	if !l.occupied[[2]int{1, 2}] || !l.Creatures[[2]int{3, 4}] || l.occupied[[2]int{3, 4}] {
		t.Fatalf("first build wrong: %v %v", l.occupied, l.Creatures)
	}
	l.Creatures[[2]int{9, 9}] = true // a mark a rebuild would clear
	l.refreshCreatureCells(pawns, animals)
	if !l.Creatures[[2]int{9, 9}] {
		t.Fatal("rebuilt although nothing moved")
	}
	animals[0].X = 5
	l.refreshCreatureCells(pawns, animals)
	if l.Creatures[[2]int{9, 9}] || l.Creatures[[2]int{3, 4}] || !l.Creatures[[2]int{5, 4}] {
		t.Fatalf("did not follow the animal: %v", l.Creatures)
	}
}
