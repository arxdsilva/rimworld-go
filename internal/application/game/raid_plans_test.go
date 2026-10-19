package game

import (
	"strings"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/world/terrain"
)

func TestRaidsGrowWithWealth(t *testing.T) {
	poor := New(Config{Seed: 1, Width: 30, Height: 20})
	_, _ = poor.SpawnNamedPawn(15, 10, "Ada")
	rich := New(Config{Seed: 1, Width: 30, Height: 20})
	_, _ = rich.SpawnNamedPawn(15, 10, "Ada")
	for i := 0; i < 24; i++ {
		for j, k := range []item.Kind{item.Rifle, item.Parka, item.Duster} {
			_, _ = rich.SpawnStack(k, 1, 2+i, 2+j)
		}
	}
	if rich.Wealth() <= poor.Wealth() {
		t.Fatal("goods count towards wealth")
	}
	if p, r := len(poor.raidKit(poor.raidPoints(5))), len(rich.raidKit(rich.raidPoints(5))); p != 1 || r <= p {
		t.Fatalf("a poor colony faces one raider, a rich one more: %d vs %d", p, r)
	}
	if k := rich.raidKit(1 << 20); len(k) != maxScaledRaiders {
		t.Fatalf("capped at %d, got %d", maxScaledRaiders, len(k))
	}
}

func TestSiegeDigsInThenStorms(t *testing.T) {
	g := New(Config{Seed: 2, Width: 60, Height: 30})
	ada, _ := g.SpawnNamedPawn(30, 15, "Ada")
	g.Siege(2)
	if len(g.raiderIDs) != 2 || g.raidGroups[0].Plan != planSiege {
		t.Fatal("two besiegers")
	}
	for i := 0; i < 200 && !g.raidGroups[0].Camped; i++ {
		g.Step(25)
		g.SetNeeds(ada, needs.Full, needs.Full)
	}
	grp := g.raidGroups[0]
	if !grp.Camped {
		t.Fatal("the besiegers dig in")
	}
	sandbags := 0
	for _, b := range g.World().Buildings {
		if b.Kind == "sandbags" {
			sandbags++
		}
	}
	dug := false
	for _, m := range g.World().Messages {
		dug = dug || strings.Contains(m.Text, "dug in") && m.Danger
	}
	if sandbags == 0 || !dug {
		t.Fatalf("the camp throws up sandbags (%d) and is announced (%v)", sandbags, dug)
	}
	g.clock.Advance(siegeHold)
	g.Step(1)
	if !g.raidGroups[0].Stormed {
		t.Fatal("after holding the camp, the besiegers storm the colony")
	}
}

func TestSappersTunnelThroughRock(t *testing.T) {
	g := New(Config{Seed: 3, Width: 30, Height: 16})
	ada, _ := g.SpawnNamedPawn(15, 8, "Ada")
	for y := 5; y <= 11; y++ {
		for x := 12; x <= 18; x++ {
			if x == 12 || x == 18 || y == 5 || y == 11 {
				g.setTerrain(x, y, terrain.Rock)
			}
		}
	}
	if !g.enclosed() {
		t.Fatal("Ada is sealed in by rock")
	}
	g.Sappers(1)
	rid := g.raiderIDs[0]
	g.equip[rid] = item.Club
	opened := false
	for i := 0; i < 400 && !opened; i++ {
		g.Step(25)
		g.SetNeeds(ada, needs.Full, needs.Full)
		opened = !g.enclosed() || g.VitalOf(ada).Injury > 0
	}
	if !opened {
		j, _ := g.jobs.Get(rid)
		t.Fatalf("sappers dig through the rock (job %v %v)", j.Kind, j.State)
	}
}

func TestSappersStepAroundTraps(t *testing.T) {
	g := New(Config{Seed: 4, Width: 20, Height: 10})
	if !g.BuildNow("spike trap", "wood", "south", 10, 5) {
		t.Fatal("trap")
	}
	g.Sappers(1)
	rid := g.raiderIDs[0]
	if g.pathQuery(rid).Walkable(10, 5) {
		t.Fatal("sappers plan around traps")
	}
	g.Raid(1)
	if !g.pathQuery(g.raiderIDs[1]).Walkable(10, 5) {
		t.Fatal("ordinary raiders don't know where the traps are")
	}
}
