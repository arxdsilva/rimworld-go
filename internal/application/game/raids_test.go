package game

import (
	"bytes"
	"strings"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/clock"
)

func raidGame(t *testing.T) (*Game, uint64) {
	t.Helper()
	g := New(Config{Seed: 11, Width: 30, Height: 20})
	ada, _ := g.SpawnNamedPawn(15, 10, "Ada")
	g.SetNeeds(ada, needs.Full, needs.Full)
	return g, uint64(ada)
}

func TestRaidersAttackAndArmedColonistFightsBack(t *testing.T) {
	g, ada := raidGame(t)
	g.equip[uint64ToID(ada)] = item.Rifle
	g.Raid(1)
	if s, n := g.Raiders(); s != 1 || n != 1 {
		t.Fatalf("raiders %d/%d", s, n)
	}
	msgs := g.World().Messages
	if len(msgs) == 0 || !strings.HasPrefix(msgs[len(msgs)-1].Text, "Raid!") {
		t.Fatalf("messages %+v", msgs)
	}
	hostileSeen := false
	for _, p := range g.Snapshot() {
		if p.Hostile {
			hostileSeen = true
		}
	}
	if !hostileSeen {
		t.Fatal("raiders should be in the snapshot, marked hostile")
	}
	for i := 0; i < 60; i++ {
		g.Step(100)
		g.SetNeeds(uint64ToID(ada), needs.Full, needs.Full)
		if s, _ := g.Raiders(); s == 0 {
			break // dead raiders now stay as bodies: stop once none stands
		}
	}
	if s, _ := g.Raiders(); s != 0 {
		t.Fatal("the raider should be beaten (down, dead, fled or gone)")
	}
	if v, _ := g.vital.Get(uint64ToID(ada)); v.Status == health.Dead {
		t.Fatal("an armed colonist should survive a lone raider")
	}
}

func TestRaiderFleesWhenBadlyHurt(t *testing.T) {
	g, _ := raidGame(t)
	g.Raid(1)
	rid := g.raiderIDs[0]
	g.hurt(rid, health.MaxHP/2, 0)
	g.Step(1)
	if r, ok := g.raiders[rid]; ok && r.Mode != raidFlee {
		t.Fatal("a badly hurt raider should flee (or already be off the map)")
	}
	for i := 0; i < 40 && len(g.raiderIDs) > 0; i++ {
		g.Step(100)
	}
	if len(g.raiderIDs) != 0 {
		t.Fatal("a fleeing raider should leave the map")
	}
}

// TestDownedRaiderBleedsOutAndDropsWeapon became TestRaiderBleedsOutLikeColonist
// (bleed_test.go, MED-01): a downed raider bleeds out by its wounds, not a timer.

func TestStorytellerSendsTheFirstRaid(t *testing.T) {
	g := NewColony(Config{Seed: 5, Width: 48, Height: 40})
	if g.nextThreat < firstRaidDay*clock.TicksPerDay || g.nextThreat > (firstRaidDay+1)*clock.TicksPerDay {
		t.Fatalf("first threat at tick %d", g.nextThreat)
	}
	jumpTo(g, firstRaidDay+1, 0)
	g.Step(1)
	if _, n := g.Raiders(); n == 0 {
		t.Fatal("the first raid should arrive around day 5")
	}
	if g.nextThreat <= uint64(g.clock.Now()) {
		t.Fatal("the next threat should be scheduled")
	}
}

func TestSaveLoadMidRaid(t *testing.T) {
	g, _ := raidGame(t)
	g.Raid(2)
	g.Step(500)
	var buf bytes.Buffer
	if err := g.Save(&buf, "raid"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		g.Step(400)
		h.Step(400)
		sameState(t, "mid-raid", g, h)
	}
}
