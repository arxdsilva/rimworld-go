package game

import (
	"testing"

	"rimworld-go/internal/domain/sim/clock"
)

func TestWidthHeightReflectConfig(t *testing.T) {
	g := New(Config{Seed: 1, Width: 7, Height: 4})

	if g.Width() != 7 || g.Height() != 4 {
		t.Fatalf("Width()/Height() = %d/%d, want 7/4", g.Width(), g.Height())
	}
}

func TestDateMatchesTick(t *testing.T) {
	g := New(Config{Seed: 1, Width: 5, Height: 5})
	g.Step(clock.TicksPerDay + clock.TicksPerHour*3)

	want := clock.DateAt(g.Tick())
	if got := g.Date(); got != want {
		t.Fatalf("Date() = %+v, want %+v", got, want)
	}
}

func TestSpawnPawnAssignsPosition(t *testing.T) {
	g := New(Config{Seed: 1, Width: 5, Height: 5})

	id, err := g.SpawnPawn(2, 3)
	if err != nil {
		t.Fatalf("SpawnPawn: %v", err)
	}

	snap := g.Snapshot()
	if len(snap) != 1 {
		t.Fatalf("Snapshot() has %d pawns, want 1", len(snap))
	}
	if snap[0].ID != id || snap[0].X != 2 || snap[0].Y != 3 {
		t.Fatalf("Snapshot()[0] = %+v, want {ID:%d X:2 Y:3}", snap[0], id)
	}
}

func TestSpawnPawnRejectsOutOfBounds(t *testing.T) {
	g := New(Config{Seed: 1, Width: 5, Height: 5})

	if _, err := g.SpawnPawn(99, 99); err == nil {
		t.Fatal("SpawnPawn should reject out-of-bounds coordinates")
	}
	if len(g.Snapshot()) != 0 {
		t.Fatal("a rejected spawn must not create a pawn")
	}
}

func TestStartsInTheMorning(t *testing.T) {
	g := New(Config{Seed: 1, Width: 5, Height: 5})
	if g.Tick() != clock.StartingMorning() {
		t.Fatalf("Tick() = %d, want %d", g.Tick(), clock.StartingMorning())
	}
	if got := g.Date().Period(); got != clock.Morning {
		t.Fatalf("Period() = %s, want morning", got)
	}
}

func TestStepAdvancesClock(t *testing.T) {
	g := New(Config{Seed: 1, Width: 5, Height: 5})

	g.Step(10)
	want := clock.StartingMorning() + 10
	if g.Tick() != want {
		t.Fatalf("Tick() = %d, want %d", g.Tick(), want)
	}
}

func TestStepKeepsPawnsInBounds(t *testing.T) {
	g := New(Config{Seed: 1, Width: 5, Height: 5})
	if _, err := g.SpawnPawn(0, 0); err != nil {
		t.Fatalf("SpawnPawn: %v", err)
	}
	if _, err := g.SpawnPawn(4, 4); err != nil {
		t.Fatalf("SpawnPawn: %v", err)
	}

	g.Step(200)

	for _, p := range g.Snapshot() {
		if p.X < 0 || p.X >= 5 || p.Y < 0 || p.Y >= 5 {
			t.Fatalf("pawn %d escaped grid bounds: %+v", p.ID, p)
		}
	}
}

func TestStepIsDeterministicForSameSeed(t *testing.T) {
	run := func() []PawnView {
		g := New(Config{Seed: 42, Width: 10, Height: 10})
		for i := 0; i < 5; i++ {
			if _, err := g.SpawnPawn(i, i); err != nil {
				t.Fatalf("SpawnPawn: %v", err)
			}
		}
		g.Step(100)
		return g.Snapshot()
	}

	a, b := run(), run()
	if len(a) != len(b) {
		t.Fatalf("snapshot length mismatch: %d != %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("snapshot[%d] diverged: %+v != %+v", i, a[i], b[i])
		}
	}
}
