package creature

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/application/game"
)

func TestTargetFacingTurnsToAttackTarget(t *testing.T) {
	shooter := game.PawnView{ID: 1, X: 5, Y: 5, Action: game.ActionAim, ActTarget: 9}
	pawns := []game.PawnView{shooter, {ID: 9, X: 5, Y: 1}}
	if f, ok := TargetFacing(shooter, pawns, nil, art.FaceSouth); !ok || f != art.FaceNorth {
		t.Fatalf("aiming at a pawn to the north: got %v, %v", f, ok)
	}
	animals := []game.AnimalView{{ID: 9, X: 9, Y: 5}}
	if f, ok := TargetFacing(shooter, nil, animals, art.FaceSouth); !ok || f != art.FaceEast {
		t.Fatalf("aiming at an animal to the east: got %v, %v", f, ok)
	}
	idle := game.PawnView{ID: 1, X: 5, Y: 5, ActTarget: 9}
	if _, ok := TargetFacing(idle, pawns, nil, art.FaceSouth); ok {
		t.Fatal("no attack action: no target facing")
	}
	gone := game.PawnView{ID: 1, X: 5, Y: 5, Action: game.ActionMelee, ActTarget: 42}
	if _, ok := TargetFacing(gone, pawns, animals, art.FaceSouth); ok {
		t.Fatal("target not in view: no target facing")
	}
}
