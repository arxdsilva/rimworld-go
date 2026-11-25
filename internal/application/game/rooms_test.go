// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/skill"
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
	t.Parallel()
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
	t.Parallel()
	g := New(Config{Seed: 1, Width: 14, Height: 10})
	id, _ := g.SpawnNamedPawn(10, 5, "Ada")
	g.SpawnStack(item.Potatoes, 4, 11, 5) // FOOD-4: berries are exempt from the raw-food thought
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
	if p.Mood >= mood.Base+mood.DefOf(mood.ExpectationsForWealth(g.Wealth())).Value || p.MoodLabel == "" {
		t.Fatalf("raw food outdoors should lower mood: %d %s", p.Mood, p.MoodLabel)
	}
	// a fine meal at a table cheers them up (FOOD-4: the wiki's +5; a
	// table adds nothing, its lack -3)
	box(t, g, 2, 2, 7, 7, [2]int{4, 7})
	placeBuilt(t, g, construction.Table, construction.South, 4, 4)
	g.positions.Set(id, Position{X: 3, Y: 4})
	g.memories[id] = nil
	g.SpawnStack(item.FineMeal, 1, 3, 4)
	g.SetNeeds(id, needs.CriticalHunger-1, needs.Full)
	g.Step(needs.EatWorkTicks + 10)
	p = pawn(t, g, id)
	labels = map[string]bool{}
	for _, th := range p.Thoughts {
		labels[th.Label] = true
	}
	if !labels["ate a fine meal"] || labels["ate at a table"] || labels["ate without a table"] {
		t.Fatalf("thoughts after meal = %+v", p.Thoughts)
	}
}

func TestLowMoodDoesNotReduceDirectWorkSpeed(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 6, Height: 4})
	id, _ := g.SpawnNamedPawn(1, 1, "Bo")
	for _, k := range []mood.Kind{mood.SleptOutside, mood.AteRaw, mood.AteWithoutTable, mood.SleptInAwfulRoom, mood.OrganHarvested} {
		g.remember(id, k)
	}
	g.SetNeeds(id, needs.CriticalHunger-1, needs.CriticalRest-1)
	if g.moodOf(id) >= mood.LowMood {
		t.Fatalf("fixture mood %d should be low", g.moodOf(id))
	}
	for i := 0; i < 400; i++ {
		if got := g.workProgress(id, skill.Mining, false); got != 1 {
			t.Fatalf("low mood changed direct work progress at tick %d: %d", i, got)
		}
		g.clock.Advance(1)
	}
}

func TestThoughtViewDoesNotRewriteMemories(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 3, Width: 14, Height: 10})
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	now := uint64(g.clock.Now())
	expired := mood.Memory{Kind: mood.AteRaw, Expires: now}
	live := mood.Memory{Kind: mood.SleptInNiceRoom, Expires: now + 100}
	g.memories[id] = mood.Memories{expired, live}

	for attempt := 1; attempt <= 2; attempt++ {
		thoughts := g.thoughts(id)
		liveCount := 0
		for _, th := range thoughts {
			if th.Kind == mood.AteRaw {
				t.Fatalf("query %d: expired thought returned: %+v", attempt, thoughts)
			}
			if th.Kind == mood.SleptInNiceRoom {
				liveCount++
			}
		}
		if liveCount != 1 {
			t.Fatalf("query %d: expected one live thought, got %d: %+v", attempt, liveCount, thoughts)
		}
		if got := g.memories[id]; len(got) != 2 || got[0] != expired || got[1] != live {
			t.Fatalf("query %d changed saved memories: %+v", attempt, got)
		}
	}
}

func TestMoodStepExpiresStoredMemories(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 3, Width: 14, Height: 10})
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	now := uint64(g.clock.Now())
	live := mood.Memory{Kind: mood.SleptInNiceRoom, Expires: now + 100}
	g.memories[id] = mood.Memories{
		{Kind: mood.AteRaw, Expires: now},
		live,
	}
	g.stepMood(id)
	if got := g.memories[id]; len(got) != 1 || got[0] != live {
		t.Fatalf("mood step did not drop expired memory: %+v", got)
	}
	g.clock.Advance(100)
	g.stepMood(id)
	if _, ok := g.memories[id]; ok {
		t.Fatalf("expired-only memory entry remains saved: %+v", g.memories[id])
	}
}

func TestMoodExpirySaveLoadReplay(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 3, Width: 14, Height: 10})
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	now := uint64(g.clock.Now())
	g.memories[id] = mood.Memories{
		{Kind: mood.AteRaw, Expires: now},
		{Kind: mood.SleptInNiceRoom, Expires: now + 500},
	}
	g.stepMood(id)
	loaded := saveAndLoad(t, g)
	if g.StateHash() != loaded.StateHash() {
		t.Fatal("save/load hash diverged after mood pruning")
	}
	g.Step(750)
	loaded.Step(750)
	if g.StateHash() != loaded.StateHash() {
		t.Fatal("loaded simulation diverged after expiring memories")
	}
}

func TestMoodOfUsesSmoothedValueBetweenUpdates(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 5, Width: 14, Height: 10})
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	if got, want := g.moodOf(id), g.thoughtLevel(id); got != want {
		t.Fatalf("mood before first update = %d, want %d", got, want)
	}
	g.stepMood(id)
	before := g.moodOf(id)
	g.remember(id, mood.AteRaw)
	if target := g.thoughtLevel(id); target == before {
		t.Fatal("test needs a changed mood target")
	}
	if got := g.moodOf(id); got != before {
		t.Fatalf("mood changed between updates: %d -> %d", before, got)
	}
	g.stepMood(id)
	if got := g.moodOf(id); got >= before {
		t.Fatalf("mood did not approach lower target: %d -> %d", before, got)
	}
}

func TestMoodFreezesWhileSleepingOrUnconscious(t *testing.T) {
	t.Parallel()
	for _, condition := range []string{"sleeping", "downed"} {
		t.Run(condition, func(t *testing.T) {
			g := New(Config{Seed: 6, Width: 14, Height: 10})
			id, _ := g.SpawnNamedPawn(5, 5, "Ada")
			g.stepMood(id)
			before := g.moodOf(id)
			now := uint64(g.clock.Now())
			g.memories[id] = mood.Memories{
				{Kind: mood.AteRaw, Expires: now},
				{Kind: mood.Catharsis, Expires: now + 1000},
			}
			switch condition {
			case "sleeping":
				g.jobs.Set(id, job.Record{Kind: job.Sleep, State: job.Do})
			case "downed":
				v, _ := g.vital.Get(id)
				v.Status = health.Downed
				g.vital.Set(id, v)
			}
			g.stepMood(id)
			if got := g.moodOf(id); got != before {
				t.Fatalf("mood changed while %s: %d -> %d", condition, before, got)
			}
			if got := g.memories[id]; len(got) != 1 || got[0].Kind != mood.Catharsis {
				t.Fatalf("expired memories left while %s: %+v", condition, got)
			}
			g.jobs.Set(id, job.Record{})
			v, _ := g.vital.Get(id)
			v.Status = health.Alive
			g.vital.Set(id, v)
			g.stepMood(id)
			if got := g.moodOf(id); got <= before {
				t.Fatalf("mood did not resume after %s: %d -> %d", condition, before, got)
			}
		})
	}
}

func TestAwakeBedriddenPatientMoodStillMoves(t *testing.T) {
	t.Parallel()
	g, id, _ := sickPatient(t)
	if !g.lyingAwake(id) {
		g.SetNeeds(id, 9000, 9000)
		restFed(g, 1)
	}
	if !g.lyingAwake(id) {
		t.Fatal("patient is still unconscious")
	}
	g.stepMood(id)
	before := g.moodOf(id)
	g.memories[id] = mood.Memories{{Kind: mood.Catharsis, Expires: uint64(g.clock.Now()) + 1000}}
	g.stepMood(id)
	if got := g.moodOf(id); got <= before {
		t.Fatalf("awake patient did not recover mood: %d -> %d", before, got)
	}
	if !(mindPawns{g}).Awake(id) {
		t.Fatal("awake patient is treated as asleep for mental checks")
	}
}
