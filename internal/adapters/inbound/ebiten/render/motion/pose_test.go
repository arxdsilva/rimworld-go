package motion

import (
	"math"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/application/game"
)

// walking at one tile per 15 ticks, the usual gait
var walkingState = MoveState{Speed: 1.0 / float64(game.TicksPerCell), Blend: 1, Moving: true}

// stand is a pawn's standing pose (the root's standPose: pivot at the feet).
var stand = Pose{Px: 0.5, Py: 0.52}

func TestWalkBobFollowsStride(t *testing.T) {
	t.Parallel()
	// one gait cycle (two footfalls) per StrideLen tiles of stride: the bob
	// repeats every half cycle, also walking a diagonal
	m := walkingState
	for _, stride := range []float64{0, 0.14, 0.5, 1.1} {
		a := BobRaise(stride, m.Speed, 1)
		if b := BobRaise(stride+ContactEvery, m.Speed, 1); math.Abs(a-b) > 1e-9 {
			t.Fatalf("stride %v: bob repeats every footfall: %v vs %v", stride, a, b)
		}
		if b := BobRaise(stride+StrideLen, m.Speed, 1); math.Abs(a-b) > 1e-9 {
			t.Fatalf("stride %v: and every cycle: %v vs %v", stride, a, b)
		}
	}
	// the pose follows the stride: bob up at a footfall, down between
	m.Stride = 0.5 / WalkFrames * StrideLen
	up := WalkPose(stand, art.FaceSouth, m)
	m.Stride += ContactEvery / 2
	down := WalkPose(stand, art.FaceSouth, m)
	if up.Dy >= down.Dy || up.Dy >= 0 {
		t.Fatalf("raised at the footfall (%v), lower between (%v)", up.Dy, down.Dy)
	}
}

func TestWalkBlendIdle(t *testing.T) {
	t.Parallel()
	m := walkingState
	m.Stride, m.Blend = 0.1, 0
	if ps := WalkPose(stand, art.FaceEast, m); ps.Dy != 0 {
		t.Fatalf("blend 0 is the static Pose, got %+v", ps)
	}
	if BobRaise(0, m.Speed, 0) != 0 {
		t.Fatal("idle does not bob")
	}
}

func TestBobPeaksAtContact(t *testing.T) {
	t.Parallel()
	frameMid := func(i int) float64 { return (float64(i) + 0.5) / 12 * StrideLen }
	sp := walkingState.Speed
	c1, c7 := BobRaise(frameMid(0), sp, 1), BobRaise(frameMid(6), sp, 1)
	if math.Abs(c1-c7) > 1e-9 || c1 <= 0 {
		t.Fatalf("frames 01 and 07 are the two peaks: %v %v", c1, c7)
	}
	for i := 0; i < 12; i++ {
		if i != 0 && i != 6 && BobRaise(frameMid(i), sp, 1) >= c1 {
			t.Fatalf("frame %d bobs as high as a contact frame", i+1)
		}
	}
	if BobRaise(frameMid(3), sp, 1) > 0.5*c1 {
		t.Fatal("the body is low mid-way between contacts")
	}
	if BobRaise(frameMid(0), sp, 0.5) >= c1 {
		t.Fatal("a half walk-blend bobs less")
	}
}

func TestSwayAndLeanScaleWithSpeed(t *testing.T) {
	t.Parallel()
	slow, fast := walkingState, walkingState
	slow.Speed *= 0.5
	slow.Stride, fast.Stride = 0.4, 0.4 // quarter cycle: sway at its extreme
	if a, b := math.Abs(GaitSway(slow)), math.Abs(GaitSway(fast)); a >= b || b > 0.015+1e-9 || b == 0 {
		t.Fatalf("sway grows with speed and stays within +-0.015: %v %v", a, b)
	}
	if GaitLean(art.FaceEast, slow) >= GaitLean(art.FaceEast, fast) || GaitLean(art.FaceEast, fast) <= 0 {
		t.Fatal("east lean grows with speed, forward = clockwise")
	}
	if GaitLean(art.FaceWest, fast) != -GaitLean(art.FaceEast, fast) {
		t.Fatal("west leans the other way")
	}
	if GaitLean(art.FaceSouth, fast) != 0 {
		t.Fatal("no forward lean seen head-on")
	}
	stopping := MoveState{Blend: 0.5, Speed: 0}
	if GaitLean(art.FaceEast, stopping) >= 0 {
		t.Fatal("decelerating leans back")
	}
}

func TestBedTransitionEases(t *testing.T) {
	t.Parallel()
	if SmoothEase(100, 100, 12) != 0 || SmoothEase(112, 100, 12) != 1 || SmoothEase(500, 100, 12) != 1 {
		t.Fatal("eases from 0 to 1 over 12 present frames")
	}
	if k := SmoothEase(106, 100, 12); math.Abs(k-0.5) > 1e-9 {
		t.Fatalf("midpoint = %v", k)
	}
	if SmoothEase(101, 100, 12) >= 1.0/12 || SmoothEase(111, 100, 12) <= 11.0/12 {
		t.Fatal("slow at both ends (smoothstep), not linear")
	}
	a := Pose{Px: 0.5, Py: 0.55, Rot: 0, Sx: 1, Sy: 1}
	b := Pose{Px: 0.5, Py: 0.55, Rot: math.Pi / 2, Sx: 0.85, Sy: 0.85, Dx: 1, Dy: 2}
	if got := LerpPose(a, b, 0); got != a {
		t.Fatalf("k=0 is the standing Pose: %+v", got)
	}
	if got := LerpPose(a, b, 1); got != b {
		t.Fatalf("k=1 is the bed Pose: %+v", got)
	}
	if got := LerpPose(a, b, 0.5); math.Abs(got.Rot-math.Pi/4) > 1e-9 || math.Abs(got.Dx-0.5) > 1e-9 {
		t.Fatalf("midway = %+v", got)
	}
}

func TestBlockedPawnFacesBlocker(t *testing.T) {
	t.Parallel()
	pawns := []game.PawnView{{ID: 1, X: 3, Y: 3, BlockedBy: 2}, {ID: 2, X: 4, Y: 3}}
	if f := BlockedFacing(pawns[0], pawns, nil, art.FaceSouth); f != art.FaceEast {
		t.Fatalf("faces the blocker to the east, got %v", f)
	}
	if f := BlockedFacing(pawns[1], pawns, nil, art.FaceWest); f != art.FaceWest {
		t.Fatal("unblocked keeps its art.Facing")
	}
	an := []game.AnimalView{{ID: 9, X: 3, Y: 1}}
	p := game.PawnView{ID: 5, X: 3, Y: 3, BlockedBy: 9}
	if f := BlockedFacing(p, nil, an, art.FaceSouth); f != art.FaceNorth {
		t.Fatalf("animals can block too, got %v", f)
	}
}

func TestStandingPoseUsesMotion(t *testing.T) {
	t.Parallel()
	m := walkingState
	m.Stride = 0.1 // near contact
	ps := WalkPose(stand, art.FaceEast, m)
	if ps.Dy >= 0 || ps.Rot <= 0 {
		t.Fatalf("east walker bobs up and leans forward: %+v", ps)
	}
	ps = WalkPose(stand, art.FaceNorth, MoveState{Stride: 0.4, Speed: walkingState.Speed, Blend: 1, Moving: true})
	if ps.Dx == 0 {
		t.Fatal("north walker sways sideways")
	}
	if idle := WalkPose(stand, art.FaceEast, MoveState{}); idle.Dx != 0 || idle.Dy != 0 || idle.Rot != 0 {
		t.Fatalf("idle has no gait motion: %+v", idle)
	}
}
