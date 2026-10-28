package motion

import (
	"math"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/domain/sim/clock"
)

// testBody stands in for the caller's per-track body player.
type testBody struct{ n int }

// TestStateWalksOneStep drives a State by hand through one step from
// (2,2) to (3,2) and samples it the way a layer does, at animT.
func TestStateWalksOneStep(t *testing.T) {
	t.Parallel()
	var mo State[testBody]
	sync := func(now clock.Tick, v MoView) *Track[testBody] {
		mo.BeginSync(now)
		k := mo.Observe(7, v, art.FaceEast)
		mo.EndSync()
		return k
	}
	sync(100, MoView{x: 2, y: 2, nx: 3, ny: 2, hasNext: true, defDur: 15})
	// first seen one tick in: the step began at 100 and lasts 15 ticks,
	// and it ends at rest (no next cell)
	k := sync(101, MoView{x: 3, y: 2, stepTick: 100, stepTicks: 15, defDur: 15})
	if k.X != 3 || k.Y != 2 {
		t.Fatalf("track cell (%d,%d), want (3,2)", k.X, k.Y)
	}
	if _, ok := mo.MotionOf(8, 100); ok {
		t.Fatal("a creature without a track has no motion")
	}
	at := func(tt float64) MoveState {
		m, ok := mo.MotionOf(7, tt)
		if !ok {
			t.Fatalf("no motion at %v", tt)
		}
		return m
	}
	if m := at(100); math.Abs(m.X-2) > 1e-3 || m.Y != 2 {
		t.Fatalf("at the step's start it is on (2,2): %+v", m)
	}
	if m := at(107.5); math.Abs(m.X-2.5) > 1e-3 || !m.Moving || m.Facing != art.FaceEast || m.Blend <= 0 {
		t.Fatalf("mid-step it is half-way, walking east: %+v", m)
	}
	if m := at(107.5); m != mo.PawnMotion(7, 107.5) || m != mo.AnimalMotion(7, 107.5) {
		t.Fatal("a repeated sample is the same sample")
	}
	prev := at(100).X
	for tt := 100.5; tt <= 115; tt += 0.5 {
		x := at(tt).X
		if x < prev-1e-9 {
			t.Fatalf("x went back at %v: %v < %v", tt, x, prev)
		}
		prev = x
	}
	if m := at(120); m.X != 3 || m.Moving || m.Speed != 0 || m.Left != 0 {
		t.Fatalf("after the step it rests on (3,2): %+v", m)
	}
	k.Body = testBody{n: 3}
	k.Settle()
	if k.Body != (testBody{}) {
		t.Fatal("Settle starts the body player afresh")
	}
	if m := at(105); m.X != 3 || m.Moving {
		t.Fatalf("a settled track rests on its cell: %+v", m)
	}
	mo.BeginSync(102)
	mo.EndSync()
	if len(mo.Tracks) != 0 {
		t.Fatal("a creature not observed in a sync is dropped")
	}
}

func TestStateClock(t *testing.T) {
	t.Parallel()
	var mo State[testBody]
	for _, c := range []struct{ in, want float64 }{
		{-1, 0}, {math.NaN(), 0}, {0.25, 0.25}, {1, AlphaMax}, {5, AlphaMax},
	} {
		mo.SetAlpha(c.in)
		if mo.Alpha != c.want {
			t.Fatalf("SetAlpha(%v) = %v, want %v", c.in, mo.Alpha, c.want)
		}
	}
	// animT trails the sim by the unstepped share of the last advance
	mo.SetAlpha(0.5)
	mo.Stepped = 2
	if got := mo.AnimT(101); got != 100 {
		t.Fatalf("AnimT(101) = %v, want 100", got)
	}
	mo.Stepped = 0 // paused: the clock is the tick
	if got := mo.AnimT(101); got != 101 {
		t.Fatalf("paused AnimT = %v", got)
	}
}
