package ebiten

import "testing"

func TestSpeedFramesPerTick(t *testing.T) {
	t.Parallel()
	cases := []struct {
		s    speed
		want int
	}{
		{speedPaused, 0},
		{speedNormal, 15},
		{speedFast, 6},
		{speedVeryFast, 2},
	}
	for _, tc := range cases {
		if got := tc.s.framesPerTick(); got != tc.want {
			t.Fatalf("%s framesPerTick = %d, want %d", tc.s.label(), got, tc.want)
		}
	}
}

func TestWalkSustainHoldsAfterMove(t *testing.T) {
	t.Parallel()
	if !walkHold(20, 10, walkSustainFrames) {
		t.Fatal("expected walk shortly after lastMove")
	}
	if walkHold(10+walkSustainFrames, 10, walkSustainFrames) {
		t.Fatal("walk should end after sustain window")
	}
	if walkHold(50, 0, walkSustainFrames) {
		t.Fatal("never-moved pawn should idle")
	}
}
