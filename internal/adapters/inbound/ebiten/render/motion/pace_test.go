package motion

import (
	"testing"

	"rimworld-go/internal/domain/sim/clock"
)

const frameDt = 1.0 / 60

// TestPaceStaysBetweenKnownStates: the drawn clock never runs backwards,
// never passes the latest tick and never trails it by more than two
// Updates, whatever the Update pattern.
func TestPaceStaysBetweenKnownStates(t *testing.T) {
	t.Parallel()
	var mo State[struct{}]
	tick := clock.Tick(100)
	prev := -1.0
	pattern := []int{1, 2, 0, 0, 3, 1, 0, 2, 1, 1}
	for f := 0; f < 300; f++ {
		n := pattern[f%len(pattern)]
		for range n {
			tick += 3
			mo.Stepped = 3
		}
		mo.SetAlpha(0.1)
		mo.Pace(tick, frameDt, 3, 60)
		got := mo.AnimT(tick)
		if got < prev {
			t.Fatalf("frame %d: animT went back %.3f -> %.3f", f, prev, got)
		}
		if got > float64(tick) || got < float64(tick)-2*3 {
			t.Fatalf("frame %d: animT %.3f outside [tick-6, tick] at tick %d", f, got, tick)
		}
		prev = got
	}
}

// TestPaceSettlesWhenPaused: paused, the clock glides to the latest tick
// and then holds it exactly.
func TestPaceSettlesWhenPaused(t *testing.T) {
	t.Parallel()
	var mo State[struct{}]
	tick := clock.Tick(500)
	mo.Stepped = 1
	mo.SetAlpha(0)
	mo.Pace(tick, frameDt, 1, 60) // seeds at Tick-1
	if got := mo.AnimT(tick); got != 499 {
		t.Fatalf("seed animT %v, want 499", got)
	}
	mo.Stepped = 0 // the pause's Updates step nothing
	for range 240 {
		mo.Pace(tick, frameDt, 0, 60)
	}
	if got := mo.AnimT(tick); got < 499.999 || got > 500 {
		t.Fatalf("paused animT %v, want 500", got)
	}
	held := mo.AnimT(tick)
	mo.Pace(tick, frameDt, 0, 60)
	if mo.AnimT(tick) != held {
		t.Fatalf("paused animT moved %v -> %v", held, mo.AnimT(tick))
	}
}

// TestPaceSnapsAfterAHitchOrALoad: a long gap or an earlier tick (a load)
// takes the interpolation point at once; an unpaced tick reads it too.
func TestPaceSnapsAfterAHitchOrALoad(t *testing.T) {
	t.Parallel()
	var mo State[struct{}]
	mo.Stepped = 1
	mo.SetAlpha(0.5)
	mo.Pace(1000, frameDt, 1, 60)
	mo.Pace(1030, 0.5, 1, 60) // a half-second hitch
	if got := mo.AnimT(1030); got != 1029.5 {
		t.Fatalf("after a hitch animT %v, want 1029.5", got)
	}
	mo.Pace(10, frameDt, 1, 60) // a load went back in time
	if got := mo.AnimT(10); got != 9.5 {
		t.Fatalf("after a load animT %v, want 9.5", got)
	}
	if got := mo.AnimT(11); got != 10.5 {
		t.Fatalf("an unpaced tick reads the interpolation point: %v, want 10.5", got)
	}
}

// TestPaceResumesAtTheTierRate: the pause menu stops the ticks without
// changing the speed tier. Once ticks come back, the walker moves at the
// tier's rate from the first frames, not after the averaged rate recovers.
func TestPaceResumesAtTheTierRate(t *testing.T) {
	t.Parallel()
	var mo State[struct{}]
	tick := clock.Tick(1000)
	mo.Stepped = 1
	mo.SetAlpha(0)
	for range 120 { // running at 1x
		tick++
		mo.Pace(tick, frameDt, 1, 60)
	}
	for range 300 { // five seconds in the pause menu
		mo.Pace(tick, frameDt, 1, 60)
	}
	prev := mo.AnimT(tick)
	for f := range 20 {
		tick++
		mo.Pace(tick, frameDt, 1, 60)
		got := mo.AnimT(tick)
		if got-prev < 0.7 || got-prev > 1.3 {
			t.Fatalf("resumed frame %d advanced %.3f ticks, want about 1", f, got-prev)
		}
		prev = got
	}
}
