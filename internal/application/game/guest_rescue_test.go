package game

// guest_rescue_test.go: guests in the colony's care through the whole game
// (SIM-063): a downed visitor is rescued, tended and leaves, its faction
// grateful; a guest dying down costs goodwill; a rescued stranger asks to
// join, and the player accepts or declines the letter.

import (
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/application/game/care"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/temperature"
)

// guestScene is Doc at (6, 6), at work around the clock, with herbal medicine, a bed at (14, 3), the
// default factions with the first human one neutral, and that faction's
// visitor standing at (12, 6). fac is the visitor's faction (1 + index).
func guestScene(t *testing.T) (g *Game, doc, vis entity.EntityID, fac int) {
	t.Helper()
	g = New(Config{Seed: 3, Width: 40, Height: 14})
	g.DevAddFactions()
	for i := range g.ng.factions {
		if f := &g.ng.factions[i]; f.Type.Human() && fac == 0 {
			f.Relation, f.Defeated, fac = faction.Neutral, false, i+1
		}
	}
	if fac == 0 {
		t.Fatal("fixture: no human faction")
	}
	doc, _ = g.SpawnNamedPawn(6, 6, "Doc")
	scheduleAll(g, schedule.Work)
	stack(t, g, item.HerbalMedicine, 5, 6, 9)
	buildNow(t, g, construction.Bed, item.Wood, construction.South, 14, 3)
	vis = g.spawnVisitor(fac, g.factionOf(fac), 12, 6, uint64(g.clock.Now()))
	return g, doc, vis, fac
}

// hasEvent: faction fac's event list has a line containing text.
func hasEvent(g *Game, fac int, text string) bool {
	return slices.ContainsFunc(g.factionOf(fac).Events, func(e string) bool { return strings.Contains(e, text) })
}

// visitorRecOf is visitor id's record (ok false: no longer a visitor here).
func visitorRecOf(g *Game, id entity.EntityID) (visitorRec, bool) {
	for _, v := range g.worldS().visitors {
		if v.ID == id {
			return v, true
		}
	}
	return visitorRec{}, false
}

func TestDownedVisitorRescuedTendedAndLeaves(t *testing.T) {
	g, doc, vis, fac := guestScene(t)
	g.hurt(vis, health.DownedInjury+10, 0)
	g.Step(1)
	if v, _ := g.vital.Get(vis); v.Status != health.Downed {
		t.Fatalf("fixture: the visitor is down: %+v", v)
	}
	if !(careGuests{g}).IsGuest(vis) || (careGuests{g}).FactionName(vis) != g.factionOf(fac).Name {
		t.Fatal("a visitor is a guest of its faction")
	}
	vp, _ := g.positions.Get(vis)
	r := orderRow(g, doc, vp.X, vp.Y, "Rescue")
	if r == nil || r.Disabled {
		t.Fatalf("Rescue a downed visitor: %+v in %q", r, labelsAt(g, doc, vp.X, vp.Y))
	}
	// no order: Doctor work rescues a downed guest on its own
	if !runCare(g, 600, func() bool { return g.inBed(vis) }, doc, vis) {
		t.Fatal("the visitor never reaches the bed")
	}
	if !runCare(g, 600, func() bool { return g.tended(vis) }, doc, vis) {
		t.Fatal("the guest in bed is never tended")
	}
	left := func() bool { v, here := visitorRecOf(g, vis); return !here || v.Leaving }
	for i := 0; i < 600 && !left(); i++ {
		if v, _ := g.vital.Get(vis); v.Status == health.Alive && !g.inBed(vis) {
			t.Fatalf("step %d: up and out of bed before it left: %+v", i, v)
		}
		g.Step(250)
		g.SetNeeds(doc, needs.Full, needs.Full)
		g.SetNeeds(vis, needs.Full, needs.Full)
	}
	if !left() || g.inBed(vis) {
		t.Fatalf("recovered, it leaves and gives the bed back (in bed %v)", g.inBed(vis))
	}
	if !hasEvent(g, fac, care.ReasonHealed) {
		t.Fatalf("its faction thanks the colony: %q", g.factionOf(fac).Events)
	}
	for i := 0; i < 50 && g.visitor(vis); i++ {
		g.Step(250)
	}
	if g.visitor(vis) || g.personOf(vis).Where != whereOffMap {
		t.Fatal("it walks home")
	}
}

func TestGuestDyingDownedCostsGoodwill(t *testing.T) {
	g, _, vis, fac := guestScene(t)
	before := g.factionOf(fac).Goodwill
	g.hurt(vis, health.DownedInjury+10, 0)
	g.Step(1)
	g.die(vis, health.CauseInjury)
	g.Step(1)
	if !hasEvent(g, fac, care.ReasonDiedInCare) || g.factionOf(fac).Goodwill != before+care.GoodwillDiedDowned {
		t.Fatalf("goodwill %d -> %d, events %q", before, g.factionOf(fac).Goodwill, g.factionOf(fac).Events)
	}

	// a guest killed standing never lay in the colony's care
	g2, _, vis2, fac2 := guestScene(t)
	g2.Step(1)
	g2.die(vis2, health.CauseInjury)
	g2.Step(1)
	if hasEvent(g2, fac2, care.ReasonDiedInCare) {
		t.Fatal("a guest that died standing is no death in care")
	}
}

// strangerInBed puts a healthy faction-less stranger in the scene's bed,
// as a rescue that has healed leaves it.
func strangerInBed(t *testing.T, g *Game) entity.EntityID {
	t.Helper()
	s := g.spawnVisitor(0, nil, 20, 8, uint64(g.clock.Now()))
	bid := g.bldgIDs[len(g.bldgIDs)-1]
	b, _ := g.bldgs.Get(bid)
	if !b.Kind.Sleeps() {
		t.Fatalf("fixture: building %d is no bed", bid)
	}
	b = b.Claim(uint64(s))
	g.bldgs.Set(bid, b)
	x, y := g.sleepCellFor(s, b)
	g.positions.Set(s, Position{X: x, Y: y})
	if g.bedUnder(s) != bid || (careGuests{g}).FactionOf(s) != 0 {
		t.Fatal("fixture: a stranger in bed")
	}
	return s
}

// joinLetter is the join offer letter for s (nil: none).
func joinLetter(g *Game, s entity.EntityID) *LetterView {
	for _, l := range g.letterViews() {
		if l.JoinOffer && l.Focus == s {
			return &l
		}
	}
	return nil
}

func TestRescuedStrangerAsksToJoinAndIsAccepted(t *testing.T) {
	g, _, _, _ := guestScene(t)
	g.climate = temperature.Climate{YearMean: -500} // deadly cold out: leaving is unsafe, so it always asks
	s := strangerInBed(t, g)
	g.Step(1)
	l := joinLetter(g, s)
	if l == nil || g.bedUnder(s) != 0 {
		t.Fatalf("a recovered stranger asks to join and gets up: letters %+v", g.letterViews())
	}
	if v, _ := visitorRecOf(g, s); v.Leaving {
		t.Fatal("it waits for the answer")
	}
	g.Enqueue(command.Command{Kind: command.AcceptLetter, Target: l.ID})
	g.Step(1)
	if !slices.Contains(g.pawnIDs, s) || slices.Contains(g.raiderIDs, s) || !g.colonist(s) || g.personOf(s).Where != whereColony {
		t.Fatalf("accepted, the stranger joins: pawn %v raider %v", slices.Contains(g.pawnIDs, s), slices.Contains(g.raiderIDs, s))
	}
	if _, here := visitorRecOf(g, s); here || joinLetter(g, s) != nil {
		t.Fatal("no longer a visitor, the letter is gone")
	}
	g.Step(10) // it lives on as a colonist
}

func TestStrangerDeclinedLeaves(t *testing.T) {
	g, _, _, _ := guestScene(t)
	g.climate = temperature.Climate{YearMean: -500}
	s := strangerInBed(t, g)
	g.Step(1)
	l := joinLetter(g, s)
	if l == nil {
		t.Fatal("no join offer")
	}
	g.Enqueue(command.Command{Kind: command.AcceptLetter, Target: l.ID + 100}) // unknown: nothing
	g.Enqueue(command.Command{Kind: command.DismissLetter, Target: l.ID})
	g.Step(1)
	if v, _ := visitorRecOf(g, s); !v.Leaving || slices.Contains(g.pawnIDs, s) {
		t.Fatalf("declined, the stranger goes its way: %+v", v)
	}
	// accepting an offer whose stranger is gone does nothing but say so
	g.letters.list = append(g.letters.list, savedLetter{ID: 999, Join: s})
	g.departVisitor(s)
	g.acceptLetter(999)
	if slices.Contains(g.pawnIDs, s) {
		t.Fatal("a stranger who left cannot join")
	}
}
