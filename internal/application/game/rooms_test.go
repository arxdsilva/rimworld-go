package game

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
)

// box builds a completed wall ring x0..x1, y0..y1 with an optional door.
func box(t *testing.T, g *Game, x0, y0, x1, y1 int, door [2]int) {
	t.Helper()
	for x := x0; x <= x1; x++ {
		for _, y := range []int{y0, y1} {
			k := construction.Wall
			if [2]int{x, y} == door {
				k = construction.Door
			}
			placeBuilt(t, g, k, construction.South, x, y)
		}
	}
	for y := y0 + 1; y < y1; y++ {
		for _, x := range []int{x0, x1} {
			k := construction.Wall
			if [2]int{x, y} == door {
				k = construction.Door
			}
			placeBuilt(t, g, k, construction.South, x, y)
		}
	}
}

func TestRoomsDetectedAndScored(t *testing.T) {
	g := New(Config{Seed: 1, Width: 14, Height: 10})
	box(t, g, 2, 2, 7, 7, [2]int{4, 7})
	w := g.World()
	if len(w.Rooms) != 1 || w.Rooms[0].Cells != 16 {
		t.Fatalf("rooms = %+v", w.Rooms)
	}
	bare := w.Rooms[0].Score
	if info, ok := g.RoomInfo(3, 3); !ok || info.Role != "room" {
		t.Fatalf("room info = %+v %v", info, ok)
	}
	if _, ok := g.RoomInfo(10, 5); ok {
		t.Fatal("outside the box is outdoors")
	}
	placeBuilt(t, g, construction.Bed, construction.South, 3, 3)
	litLamp(t, g, 6, 3, 5, 5) // PLAY-021: a lamp lights its room on power
	for y := 3; y <= 6; y++ {
		for x := 3; x <= 6; x++ {
			placeBuilt(t, g, construction.Floor, construction.South, x, y)
		}
	}
	r := g.World().Rooms[0]
	if r.Role != "bedroom" || !r.Lit || r.Score <= bare {
		t.Fatalf("furnished room = %+v (bare score %d)", r, bare)
	}
}

func TestMoodFromEatingAndSleeping(t *testing.T) {
	g := New(Config{Seed: 1, Width: 14, Height: 10})
	id, _ := g.SpawnNamedPawn(10, 5, "Ada")
	g.SpawnStack(item.Berries, 2, 11, 5)
	g.SetNeeds(id, needs.CriticalHunger-1, needs.Full)
	g.Step(TicksPerCell*10 + needs.EatWorkTicks + 5)
	p := pawn(t, g, id)
	labels := map[string]bool{}
	for _, th := range p.Thoughts {
		labels[th.Label] = true
	}
	if !labels["ate raw food"] || !labels["ate without a table"] {
		t.Fatalf("thoughts = %+v", p.Thoughts)
	}
	if p.Mood >= mood.Base || p.MoodLabel == "" {
		t.Fatalf("raw food outdoors should lower mood: %d %s", p.Mood, p.MoodLabel)
	}
	// a cooked meal at a table cheers them up
	box(t, g, 2, 2, 7, 7, [2]int{4, 7})
	placeBuilt(t, g, construction.Table, construction.South, 4, 4)
	g.positions.Set(id, Position{X: 3, Y: 4})
	g.SpawnStack(item.Meal, 1, 3, 4)
	g.SetNeeds(id, needs.CriticalHunger-1, needs.Full)
	g.Step(needs.EatWorkTicks + 10)
	p = pawn(t, g, id)
	labels = map[string]bool{}
	for _, th := range p.Thoughts {
		labels[th.Label] = true
	}
	if !labels["ate a cooked meal"] || !labels["ate at a table"] || labels["ate without a table"] {
		t.Fatalf("thoughts after meal = %+v", p.Thoughts)
	}
}

func TestUnhappyPawnsWorkSlower(t *testing.T) {
	g := New(Config{Seed: 1, Width: 6, Height: 4})
	id, _ := g.SpawnNamedPawn(1, 1, "Bo")
	for _, k := range []mood.Kind{mood.SleptOutside, mood.AteRaw, mood.AteWithoutTable, mood.SleptInAwfulRoom} {
		g.remember(id, k)
	}
	g.SetNeeds(id, needs.CriticalHunger-1, needs.CriticalRest-1)
	if g.moodOf(id) >= mood.LowMood {
		t.Fatalf("mood %d should be low", g.moodOf(id))
	}
	slow := 0
	for i := 0; i < 400; i++ {
		if !g.workTick(id) {
			slow++
		}
		g.clock.Advance(1)
	}
	if slow != 100 {
		t.Fatalf("unhappy pawn lost %d of 400 work ticks, want 100", slow)
	}
}
