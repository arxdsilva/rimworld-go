package game

import (
	"bytes"
	"strings"
	"testing"

	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

func logTexts(g *Game, id entity.EntityID) []string {
	var out []string
	for _, l := range g.logLines(id) {
		out = append(out, l.Text)
	}
	return out
}

func hasLog(g *Game, id entity.EntityID, sub string) bool {
	for _, s := range logTexts(g, id) {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func TestPredatorBiteIsLoggedOnBothSides(t *testing.T) {
	g := New(Config{Seed: 3, Width: 20, Height: 10})
	wolf, _ := g.SpawnAnimal("ridgewolf", 5, 5)
	buck, _ := g.SpawnAnimal("dustbuck", 6, 5)
	g.strikeUntilHit(t, wolf, buck, weapon.Fists)
	if !hasLog(g, wolf, "Bit deer (-") {
		t.Fatalf("wolf log %v", logTexts(g, wolf))
	}
	if !hasLog(g, buck, "Bitten by ") || !hasLog(g, buck, "(-") {
		t.Fatalf("buck log %v", logTexts(g, buck))
	}
}

func TestColonistHitAndMissLogged(t *testing.T) {
	g := New(Config{Seed: 7, Width: 30, Height: 12})
	a, _ := g.SpawnNamedPawn(5, 5, "Ada")
	b, _ := g.SpawnNamedPawn(6, 5, "Bo")
	g.strikeUntilHit(t, a, b, weapon.Knife)
	if !hasLog(g, a, "Hit Bo with knife (-") || !hasLog(g, b, "Attacked by Ada with knife (-") {
		t.Fatalf("%v / %v", logTexts(g, a), logTexts(g, b))
	}
	for i := 0; i < 300 && !hasLog(g, a, "Missed Bo"); i++ {
		g.strike(a, b, weapon.Knife)
	}
	if !hasLog(g, a, "Missed Bo") || !hasLog(g, b, "Ada missed") {
		t.Fatalf("%v / %v", logTexts(g, a), logTexts(g, b))
	}
}

func TestTrapAndDownedLogged(t *testing.T) {
	g := New(Config{Seed: 2, Width: 20, Height: 10})
	g.BuildNow("spike trap", "wood", "south", 10, 5)
	g.Raid(1)
	rid := g.raiderIDs[0]
	g.positions.Set(rid, Position{X: 10, Y: 5})
	for i := 0; i < 20 && g.trapAt(10, 5) != 0; i++ {
		g.springTrap(rid, 10, 5)
	}
	if !hasLog(g, rid, "Hurt by a trap (-") {
		t.Fatalf("raider log %v", logTexts(g, rid))
	}
	ada, _ := g.SpawnNamedPawn(2, 2, "Ada")
	g.Injure(ada, 200)
	if !hasLog(g, ada, "Downed") && !hasLog(g, ada, "Died") {
		t.Fatalf("ada log %v", logTexts(g, ada))
	}
}

func TestEatAndSleepLogged(t *testing.T) {
	g := New(Config{Seed: 1, Width: 10, Height: 10})
	id, _ := g.SpawnNamedPawn(2, 2, "Bo")
	g.SpawnStack(item.Food, 4, 4, 2)
	g.SetNeeds(id, needs.CriticalHunger, 9000)
	g.Step(TicksPerCell*20 + needs.EatWorkTicks + 5)
	if !hasLog(g, id, "Ate ") {
		t.Fatalf("log %v", logTexts(g, id))
	}
	g.SetNeeds(id, 8000, 0)
	g.Step(needs.WakeRest/needs.SleepGain*needs.SleepPeriod + 3000) // collapsed: sleeps to 80%
	if !hasLog(g, id, "Collapsed from exhaustion") || !hasLog(g, id, "Slept on the ground") {
		t.Fatalf("log %v", logTexts(g, id))
	}
}

func TestTamingLoggedOnBothSides(t *testing.T) {
	won, lost := false, false
	for seed := uint64(1); seed <= 30 && !(won && lost); seed++ {
		g := New(Config{Seed: seed, Width: 24, Height: 14})
		ada, _ := g.SpawnNamedPawn(4, 7, "Ada")
		buck, _ := g.SpawnAnimal("dustbuck", 16, 7)
		g.Enqueue(command.Command{Kind: command.DesignateTame, X: 16, Y: 7, X2: 16, Y2: 7})
		for i := 0; i < 300; i++ {
			g.Step(50)
			g.SetNeeds(ada, 1<<20, 1<<20)
			if hasLog(g, ada, "Tamed ") || hasLog(g, ada, "Failed to tame") {
				break
			}
		}
		switch {
		case hasLog(g, ada, "Tamed "):
			won = true
			if !hasLog(g, buck, "Tamed by Ada") {
				t.Fatalf("seed %d animal log %v", seed, logTexts(g, buck))
			}
		case hasLog(g, ada, "Failed to tame"):
			lost = true
			if !hasLog(g, buck, "Resisted taming by Ada") {
				t.Fatalf("seed %d animal log %v", seed, logTexts(g, buck))
			}
		}
	}
	if !won || !lost {
		t.Fatalf("expected both outcomes across seeds: won=%v lost=%v", won, lost)
	}
}

func TestLogCapAndOrder(t *testing.T) {
	g := New(Config{Seed: 1, Width: 10, Height: 10})
	id, _ := g.SpawnNamedPawn(2, 2, "Bo")
	for i := 0; i < 45; i++ {
		g.logEvent(id, logEntry{Kind: logAte, Name: "meal"})
		g.Step(1)
	}
	ls := g.logLines(id)
	if len(ls) != maxLogEntries {
		t.Fatalf("len %d", len(ls))
	}
	for i := 1; i < len(ls); i++ {
		if ls[i-1].Tick < ls[i].Tick {
			t.Fatal("newest first")
		}
	}
	if ls[0].Day != 1 {
		t.Fatalf("day %d", ls[0].Day)
	}
}

func TestLogSurvivesSaveAndAnimalRemovalDeletes(t *testing.T) {
	g := New(Config{Seed: 3, Width: 20, Height: 10})
	ada, _ := g.SpawnNamedPawn(2, 2, "Ada")
	buck, _ := g.SpawnAnimal("dustbuck", 6, 5)
	g.logEvent(ada, logEntry{Kind: logAte, Name: "meal"})
	g.logEvent(buck, logEntry{Kind: logBitten, Name: "Ridgewolf", Amount: 4})
	var buf bytes.Buffer
	if err := g.Save(&buf, "x"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if !hasLog(h, ada, "Ate meal") || !hasLog(h, buck, "Bitten by Ridgewolf (-4)") {
		t.Fatalf("%v %v", logTexts(h, ada), logTexts(h, buck))
	}
	g.removeAnimal(buck)
	if len(g.logLines(buck)) != 0 {
		t.Fatal("removed animal keeps its log")
	}
}
