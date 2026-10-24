package game

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/faction"
)

// visitorGame is a landed set-up colony whose first human faction is
// neutral (others hostile), so exactly it may visit.
func visitorGame(t *testing.T) *Game {
	t.Helper()
	g := buildSetup(t, setupFor(t, "broken_hull", func(s *Setup) { s.MapSize = 60 }))
	stepUntilLanded(t, g)
	first := true
	for i := range g.ng.factions {
		f := &g.ng.factions[i]
		if !f.Type.Human() {
			continue
		}
		f.Defeated = false
		f.Relation = faction.Hostile
		if first {
			f.Relation, first = faction.Neutral, false
		}
	}
	return g
}

func TestVisitorsArriveSocialiseAndLeave(t *testing.T) {
	t.Parallel()
	g := visitorGame(t)
	now := uint64(g.clock.Now())
	if !g.visitorsFit(now) {
		t.Fatal("a neutral faction should be able to visit")
	}
	if !g.visitorsArrive(now) {
		t.Fatal("visitors did not arrive")
	}
	if g.visitorsFit(now) {
		t.Fatal("a second group fits while the first is here")
	}
	vs := g.visitorIDs()
	if len(vs) < 2 || len(vs) > 4 {
		t.Fatalf("%d visitors", len(vs))
	}
	for _, v := range vs {
		p := g.personOf(v)
		if p == nil || p.Where != whereVisitor || g.factionOf(p.Faction) == nil || g.factionOf(p.Faction).Relation == faction.Hostile {
			t.Fatalf("visitor %d registry %+v", v, p)
		}
		if g.isRaider(v) || !g.visitor(v) || !slices.Contains(g.raiderIDs, v) || !g.canSocialize(v, false) {
			t.Fatalf("visitor %d: hostile %v, visitor %v, may talk %v", v, g.isRaider(v), g.visitor(v), g.canSocialize(v, false))
		}
	}
	// a colonist beside a visitor talks to it (M3's social phase)
	v := vs[0]
	c := g.pawnIDs[0]
	vp, _ := g.positions.Get(v)
	x, y, ok := g.freeLandNear(vp.X, vp.Y, 2, [][2]int{{vp.X, vp.Y}})
	if !ok {
		t.Fatal("no tile beside the visitor")
	}
	g.positions.Set(c, Position{x, y})
	g.invalidateCreatures()
	g.socState()
	if got := g.socialPartner(c); got != v && !slices.Contains(vs, got) {
		t.Fatalf("social partner %d, want a visitor", got)
	}
	g.interact(c, v, uint64(g.clock.Now()))
	if g.edgeOf(c, v) == nil {
		t.Fatal("no relation after talking")
	}

	strolled := false
	for i := 0; i < 20; i++ {
		g.Step(250)
		if j, _ := g.jobs.Get(v); j.Kind == job.Visit {
			strolled = true
		}
	}
	if !strolled {
		t.Fatal("the visitor never strolled")
	}
	// shorten the stay; they leave and never stroll again (no flap)
	for i := range g.worldSt.visitors {
		g.worldSt.visitors[i].LeaveAt = uint64(g.clock.Now()) + 10
	}
	g.Step(20)
	for i := 0; i < 400 && len(g.visitorIDs()) > 0; i++ {
		g.Step(10)
		for _, id := range g.visitorIDs() {
			if j, _ := g.jobs.Get(id); j.Kind == job.Visit {
				t.Fatalf("visitor %d strolls again after leaving", id)
			}
		}
	}
	if len(g.visitorIDs()) != 0 {
		t.Fatalf("visitors still on the map: %v", g.visitorIDs())
	}
	for _, id := range vs {
		if p := g.personOf(id); p == nil || p.Where != whereOffMap {
			t.Fatalf("departed visitor %d registry %+v", id, p)
		}
		if slices.Contains(g.raiderIDs, id) || g.onMap(id) {
			t.Fatalf("departed visitor %d still on the map", id)
		}
	}
	if g.edgeOf(c, v) == nil {
		t.Fatal("the relation was lost when the visitor left")
	}
	g.pruneStrangers() // the day rollover keeps a person with a tie
	if g.personOf(v) == nil {
		t.Fatal("the registry forgot a visitor the colony knows")
	}
}

func TestNoVisitorsFromHostilePeoples(t *testing.T) {
	t.Parallel()
	g := visitorGame(t)
	for i := range g.ng.factions {
		g.ng.factions[i].Relation = faction.Hostile
	}
	if g.visitorsFit(uint64(g.clock.Now())) {
		t.Fatal("hostile peoples sent visitors")
	}
	legacy := NewColony(Config{Seed: 1, Width: 40, Height: 40})
	if legacy.visitorsFit(0) || legacy.wandererFits(0) {
		t.Fatal("a legacy colony gets visitors or wanderers")
	}
}

func TestWandererJoins(t *testing.T) {
	t.Parallel()
	g := visitorGame(t)
	now := uint64(g.clock.Now())
	before := slices.Clone(g.pawnIDs)
	if !g.wandererFits(now) || !g.wandererJoins(now) {
		t.Fatal("no wanderer joined")
	}
	if len(g.pawnIDs) != len(before)+1 {
		t.Fatalf("%d colonists, want %d", len(g.pawnIDs), len(before)+1)
	}
	id := g.pawnIDs[len(g.pawnIDs)-1]
	if slices.Contains(before, id) {
		t.Fatal("no new colonist")
	}
	if p := g.personOf(id); p == nil || p.Where != whereColony {
		t.Fatalf("wanderer registry %+v", p)
	}
	if _, ok := g.bioFor(id); !ok {
		t.Fatal("the wanderer has no bio")
	}
	found := false
	for _, m := range g.messages {
		found = found || m.Letter && m.Title == "Wanderer joins" && m.Focus == id
	}
	if !found {
		t.Fatal("no letter for the wanderer")
	}
	g.Step(500)
	if !g.alive(id) {
		t.Fatal("the wanderer is not alive")
	}
}
