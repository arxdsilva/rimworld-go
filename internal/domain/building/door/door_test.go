package door

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
)

func TestSpeedByStuff(t *testing.T) {
	t.Parallel()
	cases := []struct {
		k       construction.Kind
		stuff   item.Kind
		powered bool
		want    int
	}{
		{construction.Door, item.Wood, false, 120},
		{construction.Door, item.Steel, true, 100},
		{construction.Door, item.GraniteBlocks, false, 45},
		{construction.Door, item.Stone, false, 45},
		{construction.Autodoor, item.Wood, true, 480},
		{construction.Autodoor, item.Gold, true, 400},
		{construction.Autodoor, item.MarbleBlocks, true, 180},
		{construction.Autodoor, item.Steel, false, 100}, // unpowered: a plain door
	}
	for _, c := range cases {
		if got := Speed(c.k, c.stuff, c.powered); got != c.want {
			t.Errorf("Speed(%v, %v, %v) = %d, want %d", c.k, c.stuff, c.powered, got, c.want)
		}
	}
	if OpenTicks(100) != BaseOpenTicks || OpenTicks(400) != 11 || OpenTicks(45) != 100 || OpenTicks(0) != BaseOpenTicks || OpenTicks(1<<20) != 1 {
		t.Fatal("open ticks")
	}
}

func TestOpenWaitAndClose(t *testing.T) {
	t.Parallel()
	var s State
	if !s.Shut() || s.Open(10, 5) || s.Wait(10, 5) != 5 || s.Step(10) {
		t.Fatal("shut door")
	}
	s.Use(10)
	if s.Open(12, 5) || s.Wait(12, 5) != 3 || !s.Open(15, 5) || s.Wait(15, 5) != 0 {
		t.Fatalf("opening %+v", s)
	}
	if s.Step(10 + StayOpen - 1) {
		t.Fatal("closed before StayOpen")
	}
	if !s.Step(10+StayOpen) || !s.Shut() {
		t.Fatalf("did not close %+v", s)
	}
	s.Use(0) // tick 0 still opens
	if s.OpenedAt == 0 {
		t.Fatal("tick 0 use")
	}
}

// TestAutoCloseDoesNotFlap: a stream of passers a few ticks apart keeps
// the door open throughout; it closes once, StayOpen after the last.
func TestAutoCloseDoesNotFlap(t *testing.T) {
	t.Parallel()
	var s State
	closes := 0
	for now := uint64(1); now < 2000; now++ {
		if now < 1000 && now%37 == 0 { // someone passes every 37 ticks
			s.Use(now)
		}
		if s.Step(now) {
			closes++
		}
	}
	if closes != 1 {
		t.Fatalf("door closed %d times, want once after the last passer", closes)
	}
}

func TestHold(t *testing.T) {
	t.Parallel()
	var s State
	s.SetHold(true, 5)
	if s.Step(5000) || !s.Open(5000, 10) {
		t.Fatal("held door closed")
	}
	s.SetHold(false, 5000)
	if s.Step(5000+StayOpen-1) || !s.Step(5000+StayOpen) {
		t.Fatalf("released door %+v", s)
	}
}
