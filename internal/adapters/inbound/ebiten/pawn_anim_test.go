package ebiten

import (
	"testing"

	"rimworld-go/internal/application/game"
)

func TestTargetFacingTurnsToAttackTarget(t *testing.T) {
	shooter := game.PawnView{ID: 1, X: 5, Y: 5, Action: game.ActionAim, ActTarget: 9}
	pawns := []game.PawnView{shooter, {ID: 9, X: 5, Y: 1}}
	if f, ok := targetFacing(shooter, pawns, nil, faceSouth); !ok || f != faceNorth {
		t.Fatalf("aiming at a pawn to the north: got %v, %v", f, ok)
	}
	animals := []game.AnimalView{{ID: 9, X: 9, Y: 5}}
	if f, ok := targetFacing(shooter, nil, animals, faceSouth); !ok || f != faceEast {
		t.Fatalf("aiming at an animal to the east: got %v, %v", f, ok)
	}
	idle := game.PawnView{ID: 1, X: 5, Y: 5, ActTarget: 9}
	if _, ok := targetFacing(idle, pawns, nil, faceSouth); ok {
		t.Fatal("no attack action: no target facing")
	}
	gone := game.PawnView{ID: 1, X: 5, Y: 5, Action: game.ActionMelee, ActTarget: 42}
	if _, ok := targetFacing(gone, pawns, animals, faceSouth); ok {
		t.Fatal("target not in view: no target facing")
	}
}
