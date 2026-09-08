package ebiten

import (
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
)

func TestSpeedTicksPerFrame(t *testing.T) {
	t.Parallel()
	cases := []struct {
		s    speed
		want int
	}{
		{speedPaused, 0},
		{speedNormal, 1},
		{speedFast, 3},
		{speedVeryFast, 6},
	}
	for _, tc := range cases {
		if got := tc.s.ticksPerFrame(); got != tc.want {
			t.Fatalf("%s ticksPerFrame = %d, want %d", tc.s.label(), got, tc.want)
		}
	}
	if speedNormal.label() != "normal" || speedFast.label() != "fast" || speedVeryFast.label() != "superfast" {
		t.Fatalf("speed labels = %s/%s/%s", speedNormal.label(), speedFast.label(), speedVeryFast.label())
	}
}

func TestLerpClamps(t *testing.T) {
	t.Parallel()
	if got := lerp(2, 4, 0); got != 2 {
		t.Fatalf("t=0: got %v", got)
	}
	if got := lerp(2, 4, 1); got != 4 {
		t.Fatalf("t=1: got %v", got)
	}
	if got := lerp(2, 4, 0.5); got != 3 {
		t.Fatalf("t=0.5: got %v", got)
	}
	if got := lerp(2, 4, -1); got != 2 {
		t.Fatalf("t<0 should clamp: got %v", got)
	}
	if got := lerp(2, 4, 2); got != 4 {
		t.Fatalf("t>1 should clamp: got %v", got)
	}
}

func TestMoveProgress(t *testing.T) {
	t.Parallel()
	if got := moveProgress(0, 15); got != 0 {
		t.Fatalf("start of interval: got %v", got)
	}
	if got := moveProgress(15, 15); got != 1 {
		t.Fatalf("end of interval: got %v", got)
	}
	if got := moveProgress(3, 6); got != 0.5 {
		t.Fatalf("mid interval: got %v", got)
	}
	if got := moveProgress(4, 0); got != 1 {
		t.Fatalf("paused/unknown period should sit on dest: got %v", got)
	}
}

func TestNextVisualLerpsThenSettles(t *testing.T) {
	t.Parallel()
	first := nextVisual(pawnVisual{}, false, 2, 3, 1, 10)
	if first.fromX != 2 || first.fromY != 3 {
		t.Fatalf("first sight should snap, from=%d,%d", first.fromX, first.fromY)
	}

	moved := nextVisual(first, true, 3, 3, 2, 20)
	if moved.fromX != 2 || moved.fromY != 3 || moved.x != 3 {
		t.Fatalf("move should lerp from previous tile, got from=%d,%d dest=%d,%d", moved.fromX, moved.fromY, moved.x, moved.y)
	}
	if moved.flip {
		t.Fatal("moving +x should not flip")
	}

	sameTick := nextVisual(moved, true, 3, 3, 2, 21)
	if sameTick.fromX != 2 || sameTick.fromY != 3 {
		t.Fatalf("same tick must keep lerp origin, from=%d,%d", sameTick.fromX, sameTick.fromY)
	}

	mid := nextVisual(sameTick, true, 3, 3, 3, 22)
	if mid.fromX != 2 || mid.fromY != 3 {
		t.Fatalf("in-progress walk must keep lerp origin, from=%d,%d", mid.fromX, mid.fromY)
	}

	settled := nextVisual(mid, true, 3, 3, 2+clock.Tick(game.TicksPerCell), 40)
	if settled.fromX != 3 || settled.fromY != 3 {
		t.Fatalf("walk should settle after TicksPerCell, from=%d,%d", settled.fromX, settled.fromY)
	}

	left := nextVisual(settled, true, 2, 3, 4, 50)
	if !left.flip {
		t.Fatal("moving -x should flip")
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
