package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/mentalbreak"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/recreation"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
)

func TestRecreationDrainsAndFunRefillsIt(t *testing.T) {
	g := New(Config{Seed: 2, Width: 20, Height: 12})
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	g.joy[id] = 5000
	g.positions.Set(id, Position{5, 5})
	// a horseshoe pin nearby: the best fun there is
	if !g.BuildNow("horseshoe pin", "wood", "south", 10, 5) {
		t.Fatal("pin")
	}
	played := false
	for i := 0; i < 40; i++ {
		g.Step(100)
		g.SetNeeds(id, needs.Full, needs.Full)
		if j, _ := g.jobs.Get(id); j.Kind == job.Horseshoes && j.State == job.Do {
			played = true
		}
	}
	if !played {
		t.Fatal("an idle colonist should play horseshoes at the pin")
	}
	if g.joy[id] <= 5000 {
		t.Fatalf("recreation %d: fun should refill it", g.joy[id])
	}
	// with nothing fun going on (asleep is excluded) it drains
	g.joy[id] = 5000
	g.setDraft(id, true) // drafted: no free time
	g.Step(1400)
	if g.joy[id] != 5000-1400/recreation.DrainPeriod {
		t.Fatalf("recreation %d after 1400 ticks drafted", g.joy[id])
	}
}

func TestBoredomThoughts(t *testing.T) {
	g := New(Config{Seed: 2, Width: 10, Height: 8})
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	has := func(k mood.Kind) bool {
		for _, th := range g.thoughts(id) {
			if th.Kind == k {
				return true
			}
		}
		return false
	}
	g.joy[id] = 1000
	if !has(mood.RecreationStarved) {
		t.Error("recreation-starved thought missing")
	}
	g.joy[id] = 9000
	if !has(mood.Entertained) {
		t.Error("entertained thought missing")
	}
}

func TestMiserableColonistBreaksAndRecovers(t *testing.T) {
	g := New(Config{Seed: 2, Width: 20, Height: 12})
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	for _, k := range []mood.Kind{mood.SleptOutside, mood.AteRaw, mood.AteWithoutTable, mood.SleptInAwfulRoom} {
		g.remember(id, k)
	}
	g.joy[id] = 0
	if m := g.moodOf(id); m >= mentalbreak.DefaultThresholds().Major {
		t.Fatalf("mood %d: set-up should be miserable", m)
	}
	broke := false
	for i := 0; i < 48*25 && !broke; i++ {
		g.Step(clock.TicksPerHour / 25)
		// stays miserable: hungry with no food, nothing to enjoy, bad memories
		g.SetNeeds(id, needs.CriticalHunger, needs.Full)
		g.joy[id] = 0
		for _, k := range []mood.Kind{mood.SleptOutside, mood.AteRaw, mood.SleptInAwfulRoom} {
			g.remember(id, k)
		}
		broke = g.Breaking(id) != ""
	}
	if !broke {
		t.Fatal("a miserable colonist should break within two days")
	}
	// it won't take orders or be drafted
	g.Enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(id), Enabled: true})
	g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: uint64(id), X: 18, Y: 10})
	g.Step(2)
	if g.drafted(id) {
		t.Fatal("drafted during a mental break")
	}
	if j, _ := g.jobs.Get(id); j.Kind == job.Move {
		t.Fatal("took a move order during a mental break")
	}
	b := g.breaks[id]
	g.Step(int(b.until-uint64(g.clock.Now())) + 10) // its drawn duration (DurationFor)
	if g.Breaking(id) != "" {
		t.Fatalf("the %s break should have ended", b.kind)
	}
	found := false
	for _, th := range g.thoughts(id) {
		if th.Kind == mood.Catharsis {
			found = true
		}
	}
	if !found {
		t.Fatal("catharsis after the break")
	}
}

func TestFoodBingeEatsPastFull(t *testing.T) {
	g := New(Config{Seed: 2, Width: 14, Height: 8})
	id, _ := g.SpawnNamedPawn(2, 2, "Ada")
	g.SpawnStack(item.Lettuce, 60, 4, 2)
	g.SetNeeds(id, needs.Full, needs.Full)
	g.ForceBreak(id, mentalbreak.FoodBinge)
	g.Step(2000)
	st, _ := stackOf(g, item.Lettuce)
	if st.Qty > 50 {
		t.Fatalf("lettuce left %d: a binge should go through a lot of food", st.Qty)
	}
}
