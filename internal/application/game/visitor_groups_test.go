// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"slices"
	"testing"

	"rimworld-go/internal/application/game/visit"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

func caravanPack(g *Game, c *traderCaravan) entity.EntityID {
	for _, gr := range g.worldS().visitGroups {
		if gr.ID != c.GroupID {
			continue
		}
		for _, m := range gr.Members {
			if m.Role == visit.RolePackAnimal {
				return m.ID
			}
		}
	}
	return 0
}

func TestVisitorIndependentPartiesAndTrade(t *testing.T) {
	g := visitorGame(t)
	now := uint64(g.clock.Now())
	for i := 0; i < 2; i++ {
		if !g.visitorsArrive(now) {
			t.Fatalf("independent visitor group %d did not arrive", i+1)
		}
	}
	if len(g.worldSt.visitGroups) != 2 {
		t.Fatalf("groups %+v", g.worldSt.visitGroups)
	}
	for i := 0; i < 2; i++ {
		if !g.caravanArrive(now) {
			t.Fatalf("independent caravan %d did not arrive with visitors present", i+1)
		}
	}
	if len(g.worldSt.visitGroups) != 4 || len(caravansOf(&g.trade)) != 2 {
		t.Fatalf("groups %d caravans %d", len(g.worldSt.visitGroups), len(caravansOf(&g.trade)))
	}
	a, b := g.trade.caravan, g.trade.others[0]
	if a.GroupID == b.GroupID || a.Trader == b.Trader || a.Trader == 0 {
		t.Fatalf("caravans shared identity: %+v %+v", a, b)
	}
	for _, c := range []*traderCaravan{a, b} {
		if pack := caravanPack(g, c); pack == 0 || !g.animals.Has(pack) {
			t.Fatalf("missing physical pack animal: %+v", c)
		}
		for _, guard := range c.Members {
			if guard == c.Trader {
				continue
			}
			if _, ok := g.equip[guard]; !ok {
				t.Fatalf("guard %d spawned unarmed", guard)
			}
		}
	}
}

func TestVisitorTravelerCrossesOppositeEdge(t *testing.T) {
	g := visitorGame(t)
	if !visitsOf(g).ArriveKind(visit.KindTraveler, uint64(g.clock.Now())) {
		t.Fatal("travelers did not arrive")
	}
	group := g.worldSt.visitGroups[0]
	initial := group.Leader
	start, _ := g.positions.Get(initial)
	nearWest, nearEast := start.X <= visit.Inset, start.X >= g.grid.Width()-1-visit.Inset
	nearNorth, nearSouth := start.Y <= visit.Inset, start.Y >= g.grid.Height()-1-visit.Inset
	var last Position
	for i := 0; i < 14000 && g.visitor(initial); i++ {
		last, _ = g.positions.Get(initial)
		g.Step(1)
		if i < 10 && !g.visitor(initial) {
			t.Fatal("traveler vanished at its entry edge")
		}
	}
	if g.visitor(initial) {
		t.Fatalf("traveler did not cross map, start %v exit %d,%d", start, group.ExitX, group.ExitY)
	}
	end := last
	if nearWest && end.X < g.grid.Width()-1-visit.Inset ||
		nearEast && end.X > visit.Inset ||
		nearNorth && end.Y < g.grid.Height()-1-visit.Inset ||
		nearSouth && end.Y > visit.Inset {
		t.Fatalf("traveler departed on entry side %v -> %v", start, end)
	}
}

func TestVisitorPackAnimalDropsOnlyOwnedCargo(t *testing.T) {
	g, c := caravanGame(t)
	pack := caravanPack(g, c)
	if pack == 0 {
		t.Fatal("no physical pack animal")
	}
	want := map[item.Kind]int{}
	for _, hold := range c.Cargo {
		if hold.Owner != pack {
			continue
		}
		for _, ln := range hold.Lines {
			want[ln.Kind] += ln.Qty
		}
	}
	if len(want) == 0 {
		t.Fatal("pack animal carried no sale stock")
	}
	g.removeAnimal(pack)
	got := map[item.Kind]int{}
	for _, id := range g.itemIDs {
		if st := g.stacks.Ref(id); st != nil && st.Forbidden {
			got[st.Kind] += st.Qty
		}
	}
	for k, n := range want {
		if got[k] < n {
			t.Fatalf("%s lost cargo %d, dropped %d", k, n, got[k])
		}
	}
	before := len(g.itemIDs)
	packCargoLost(g, pack)
	if len(g.itemIDs) != before {
		t.Fatal("duplicate cargo dropped on repeated loss signal")
	}
	g.Step(1)
	found := false
	for _, gr := range g.worldSt.visitGroups {
		if gr.ID == c.GroupID && gr.Phase == visit.PhaseLeaving && gr.Reason == visit.LeaveLoss {
			found = true
		}
	}
	if !found {
		t.Fatal("pack animal loss did not trigger caravan retreat")
	}
}

func TestVisitorGroupSaveRestoreDuringVisit(t *testing.T) {
	g, c := caravanGame(t)
	if !g.visitorsArrive(uint64(g.clock.Now())) {
		t.Fatal("extra visitors did not arrive")
	}
	g.Step(180)
	h, _ := saveLoadInfo(t, g)
	if a, b := g.StateHash(), h.StateHash(); a != b {
		t.Fatalf("state mismatch after saved group restore: %#x != %#x", a, b)
	}
	if len(h.worldSt.visitGroups) != len(g.worldSt.visitGroups) ||
		!slices.EqualFunc(h.worldSt.visitGroups, g.worldSt.visitGroups, func(a, b visit.Group) bool {
			return a.ID == b.ID && a.Kind == b.Kind && a.Leader == b.Leader && slices.Equal(a.Members, b.Members)
		}) || h.trade.caravan.Trader != c.Trader || len(h.trade.caravan.Cargo) != len(c.Cargo) {
		t.Fatal("visit groups or cargo changed after save/load")
	}
	g.Step(250)
	h.Step(250)
	if g.StateHash() != h.StateHash() {
		t.Fatal("visitor replay diverged after restore")
	}
}

func TestVisitorSmallTraderHasRealStock(t *testing.T) {
	g := visitorGame(t)
	if !smallTraderArrive(g, uint64(g.clock.Now())) {
		t.Fatal("small trader arrival failed")
	}
	c := g.trade.caravan
	gr, ok := visitsOf(g).GroupOf(c.Trader)
	if !ok || gr.Kind != visit.KindSmallTrader || len(c.Members) > 2 || c.Stock.Silver > 500 {
		t.Fatalf("wrong small trader %+v %+v", gr, c)
	}
	if c.Cargo[0].Owner != c.Trader {
		t.Fatal("small trader stock is not held by trader")
	}
	g.die(c.Trader, health.CauseInjury)
	g.Step(1)
	if g.trade.caravan != nil {
		t.Fatal("dead small trader remained tradable")
	}
}

func TestVisitorRecoveredGoodwillAfterHealthyDeparture(t *testing.T) {
	g, _, id, fac := guestScene(t)
	now := uint64(g.clock.Now())
	for i := range g.worldSt.visitors {
		if g.worldSt.visitors[i].ID == id {
			g.worldSt.visitors[i].Healed = true
			g.worldSt.visitors[i].LeaveAt = now
			g.worldSt.visitors[i].Leaving = true
		}
	}
	before := g.factionOf(fac).Goodwill
	for i := 0; i < 10000 && g.visitor(id); i++ {
		g.Step(1)
	}
	if g.visitor(id) || g.factionOf(fac).Goodwill != before+12 {
		t.Fatalf("departure goodwill %d -> %d, still present %v", before, g.factionOf(fac).Goodwill, g.visitor(id))
	}
}

// TestVisitorDismissConversation uses the player's real typed command,
// neighbor path and completed job before the party starts leaving.
func TestVisitorDismissConversation(t *testing.T) {
	g := visitorGame(t)
	now := uint64(g.clock.Now())
	if !g.visitorsArrive(now) {
		t.Fatal("visitors did not arrive")
	}
	gr := g.worldSt.visitGroups[0]
	actor := g.pawnIDs[0]
	pos, _ := g.positions.Get(gr.Leader)
	x, y, ok := g.freeLandNear(pos.X, pos.Y, 2, [][2]int{{pos.X, pos.Y}})
	if !ok {
		t.Fatal("no approach cell for the conversation")
	}
	g.positions.Set(actor, Position{X: x, Y: y})
	g.invalidateCreatures()
	o := optionAt(g, actor, pos.X, pos.Y, "Ask ")
	if o == nil || o.Disabled {
		t.Fatalf("visitor dismissal menu option %+v", o)
	}
	if o.Cmd.Kind != command.DismissVisitorGroup || entity.EntityID(o.Cmd.Target) != gr.Leader {
		t.Fatalf("wrong command %+v", o.Cmd)
	}
	g.Enqueue(o.Cmd)
	for i := 0; i < 1500; i++ {
		g.Step(1)
		if r, ok := visitsOf(g).GroupOf(gr.Leader); ok && r.Phase == visit.PhaseLeaving {
			if r.Reason != visit.LeaveDismissed {
				t.Fatalf("leaving reason %+v", r)
			}
			return
		}
	}
	j, _ := g.jobs.Get(actor)
	r, ok := visitsOf(g).GroupOf(gr.Leader)
	a, _ := g.positions.Get(actor)
	t.Fatalf("conversation failed: actor job %+v actor pos %+v leader pos %+v group? %v %+v", j, a, pos, ok, r)
}

// TestTraderPackDepartsWithTheGroup verifies that the living animal
// eventually leaves the map with the seller's remaining goods.
func TestTraderPackDepartsWithTheGroup(t *testing.T) {
	g, c := caravanGame(t)
	pack := caravanPack(g, c)
	if pack == 0 {
		t.Fatal("pack animal absent")
	}
	visitsOf(g).LeaveGroup(c.GroupID, visit.LeaveScheduled, uint64(g.clock.Now()))
	for i := 0; i < 16000; i++ {
		_, groupHere := visitsOf(g).GroupOf(pack)
		if !g.visitor(c.Trader) && !g.animals.Has(pack) && !groupHere {
			break
		}
		g.Step(1)
	}
	if g.visitor(c.Trader) || g.animals.Has(pack) {
		t.Fatalf("group failed to depart: trader %v pack %v", g.visitor(c.Trader), g.animals.Has(pack))
	}
	if _, ok := visitsOf(g).GroupOf(pack); ok {
		g.Step(2)
		if gr, ok := visitsOf(g).GroupOf(pack); ok {
			t.Fatalf("finished group retained a pack member: group %+v, active persons %d", gr, len(g.worldSt.visitors))
		}
	}
}
