package game

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/application/game/visit"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// joinedFightMessages counts the "joining the fight" messages for faction
// name.
func joinedFightMessages(g *Game, name string) int {
	want := fmt.Sprintf(visit.JoinedFight, name)
	n := 0
	for _, m := range g.messages {
		if strings.Contains(m.Text, want) {
			n++
		}
	}
	return n
}

// TestVisitorsFightNearbyRaiders (EAI-011): a neutral visitor takes on a
// raider that comes within DefendSight and fires on it, and the colony
// hears of it once (the ranges and the hold: visit's TestVisitorDefenceNoFlap).
func TestVisitorsFightNearbyRaiders(t *testing.T) {
	t.Parallel()
	g := visitorGame(t)
	if !g.visitorsArrive(uint64(g.clock.Now())) {
		t.Fatal("visitors did not arrive")
	}
	vs := g.visitorIDs()
	near := vs[0]
	g.equip[near] = item.Rifle
	for _, id := range vs[1:] {
		moveTo(t, g, id, 3, 3)
	}
	vp := moveTo(t, g, near, 40, 40)
	g.raidWith([]item.Kind{item.Club}, raid.Assault)
	foe := g.raiderIDs[len(g.raiderIDs)-1]
	moveTo(t, g, foe, vp.X+visit.DefendSight-3, vp.Y)
	name := g.factionOf(g.personOf(near).Faction).Name
	engaged, struck := false, false
	for i := 0; i < 2*visit.DefendHold && g.standingRaider(foe) && g.standingPawn(near); i++ {
		g.Step(1)
		if j, _ := g.jobs.Get(near); j.Kind == job.Attack && j.Target == foe {
			engaged = true
		}
		for _, tr := range g.traces {
			struck = struck || tr.Attacker == near && tr.Target == foe
		}
	}
	if !engaged || !struck {
		t.Fatalf("the visitor beside the raider: engaged %v, fired %v", engaged, struck)
	}
	if n := joinedFightMessages(g, name); n != 1 {
		t.Fatalf("%d %q messages, want 1", n, fmt.Sprintf(visit.JoinedFight, name))
	}
}

// TestTraderGuardsDefend (EAI-011): a trader caravan's guards turn on a
// manhunting animal beside them; the trader keeps to the trade.
func TestTraderGuardsDefend(t *testing.T) {
	t.Parallel()
	g := visitorGame(t)
	ref := 0
	for i, f := range g.ng.factions {
		if f.Type.Human() && ref == 0 && f.Relation == faction.Neutral { // visitorGame's one neutral
			ref = i + 1
		}
	}
	now := uint64(g.clock.Now())
	if !g.traderCaravanFrom(ref, now, false) {
		t.Fatal("no trader caravan")
	}
	c := g.trade.caravan
	guards := slices.DeleteFunc(slices.Clone(c.Members), func(id entity.EntityID) bool { return id == c.Trader })
	if len(guards) == 0 {
		t.Fatal("a caravan without guards")
	}
	tp := moveTo(t, g, c.Trader, 40, 40)
	for _, id := range guards {
		g.equip[id] = item.Rifle
		moveTo(t, g, id, tp.X, tp.Y+2)
	}
	beast, err := g.SpawnAnimal("donkey", tp.X+5, tp.Y)
	if err != nil {
		t.Fatal(err)
	}
	g.animals.Ref(beast).Manhunter = now + clock.TicksPerDay
	moveTo(t, g, beast, tp.X+5, tp.Y)
	guarded := map[entity.EntityID]bool{}
	for i := 0; i < 3*defendScanTicks && len(guarded) < len(guards); i++ {
		g.Step(1)
		if j, _ := g.jobs.Get(c.Trader); j.Kind == job.Attack && g.lastHit[c.Trader].Tick == 0 {
			t.Fatalf("tick %d: the unhurt trader joined the fight", i)
		}
		for _, id := range guards {
			if j, _ := g.jobs.Get(id); j.Kind == job.Attack && j.Target == beast {
				guarded[id] = true
			}
		}
	}
	if len(guarded) == 0 {
		t.Fatalf("no guard turned on the manhunter beside the caravan (guards %v)", guards)
	}
}

// defendScanTicks is how often a visitor looks (visit's defendScan).
const defendScanTicks = 30
