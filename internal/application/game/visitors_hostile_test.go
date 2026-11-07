package game

// visitors_hostile_test.go: visitors whose faction turns hostile while they
// are here (MAP-18). They stop visiting and walk out as enemies, for good.

import (
	"slices"
	"testing"

	"rimworld-go/internal/application/game/care"
	"rimworld-go/internal/application/game/letters"
	"rimworld-go/internal/application/game/visit"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/temperature"
	"rimworld-go/pkg/mathx"
)

// turnHostile drops faction ref's goodwill as play would.
func turnHostile(t *testing.T, g *Game, ref int) {
	t.Helper()
	f := g.factionOf(ref)
	f.Goodwill = -100
	faction.Update(f)
	if f.Relation != faction.Hostile {
		t.Fatalf("fixture: relation %v", f.Relation)
	}
}

// calmDown makes faction ref friendly enough again.
func calmDown(t *testing.T, g *Game, ref int) {
	t.Helper()
	f := g.factionOf(ref)
	f.Goodwill = 50
	faction.Update(f)
	if f.Relation == faction.Hostile {
		t.Fatalf("fixture: still hostile at %d", f.Goodwill)
	}
}

// turnedLetters are the letters about faction ref's visitors turning.
func turnedLetters(g *Game) []letters.Letter {
	var out []letters.Letter
	for _, l := range g.letters.List() {
		if l.Title == visit.TurnedTitle {
			out = append(out, l)
		}
	}
	return out
}

func raiderViewOf(g *Game, id entity.EntityID) (PawnView, bool) {
	for _, v := range g.raiderViews() {
		if v.ID == id {
			return v, true
		}
	}
	return PawnView{}, false
}

func TestVisitorsOfHostileFactionLeaveAsEnemies(t *testing.T) {
	t.Parallel()
	g, _, a, fac := guestScene(t)
	now := uint64(g.clock.Now())
	b := g.spawnVisitor(fac, g.factionOf(fac), 12, 8, now)
	groups := len(g.raidGroups)
	g.Step(5)
	if g.isRaider(a) || !g.visitor(a) {
		t.Fatal("fixture: a guest before the faction turns")
	}
	turnHostile(t, g, fac)
	g.Step(2)
	for _, id := range []entity.EntityID{a, b} {
		if g.visitor(id) || !g.isRaider(id) || !g.standingRaider(id) {
			t.Fatalf("%d: visitor %v, raider %v, standing %v", id, g.visitor(id), g.isRaider(id), g.standingRaider(id))
		}
		if !g.combat.Turrets.Targetable(id, uint64(g.clock.Now())) {
			t.Fatalf("%d: turrets may not target it", id)
		}
		v, ok := raiderViewOf(g, id)
		if !ok || !v.Hostile || !v.Fleeing || v.Visitor || v.Prisoner {
			t.Fatalf("%d view %+v", id, v)
		}
		if g.personOf(id).Where != whereRaider {
			t.Fatalf("%d registry place %v", id, g.personOf(id).Where)
		}
	}
	if s, _ := g.Raiders(); s != 2 {
		t.Fatalf("%d standing raiders counted", s)
	}
	for gi := groups; gi < len(g.raidGroups); gi++ {
		if !g.raidGroups[gi].Over {
			t.Fatal("a turned visitor's group would cost goodwill as a repelled raid")
		}
	}
	ls := turnedLetters(g)
	if len(ls) != 1 || ls[0].Focus != a && ls[0].Focus != b {
		t.Fatalf("want one letter with a jump: %+v", ls)
	}
	gw := g.factionOf(fac).Goodwill
	for i := 0; i < 2000 && (g.onMap(a) || g.onMap(b)); i++ {
		for _, id := range []entity.EntityID{a, b} {
			if j, _ := g.jobs.Get(id); j.Kind == job.Visit {
				t.Fatalf("%d went on visiting", id)
			}
		}
		g.Step(5)
	}
	for _, id := range []entity.EntityID{a, b} {
		if g.onMap(id) || g.personOf(id).Where != whereOffMap {
			t.Fatalf("%d never left the map: %v", id, g.personOf(id).Where)
		}
		if _, here := visitorRecOf(g, id); here {
			t.Fatalf("%d keeps a visitor record", id)
		}
	}
	if g.factionOf(fac).Goodwill != gw {
		t.Fatalf("goodwill moved %d -> %d", gw, g.factionOf(fac).Goodwill)
	}
	if len(g.worldSt.visitors) != 0 {
		t.Fatalf("records of those who left block new visitors: %+v", g.worldSt.visitors)
	}
}

// TestTurnedVisitorsDoNotFlap: once turned, a visitor stays turned for that
// visit, also when its faction calms down while it walks out.
func TestTurnedVisitorsDoNotFlap(t *testing.T) {
	t.Parallel()
	g, _, vis, fac := guestScene(t)
	turnHostile(t, g, fac)
	g.Step(2)
	if !g.isRaider(vis) {
		t.Fatal("not turned")
	}
	calmDown(t, g, fac)
	for i := 0; i < 2000 && g.onMap(vis); i++ {
		if j, _ := g.jobs.Get(vis); j.Kind == job.Visit || g.visitor(vis) || !g.isRaider(vis) {
			t.Fatalf("step %d: flapped back: job %v, visitor %v, raider %v", i, j.Kind, g.visitor(vis), g.isRaider(vis))
		}
		g.Step(5)
	}
	if g.onMap(vis) {
		t.Fatal("it never left")
	}
	if len(turnedLetters(g)) != 1 {
		t.Fatalf("%d letters", len(turnedLetters(g)))
	}
}

// TestLeavingVisitorsTurnToo: one already walking out for home counts too.
func TestLeavingVisitorsTurnToo(t *testing.T) {
	t.Parallel()
	g, _, vis, fac := guestScene(t)
	for i := range g.worldSt.visitors {
		g.worldSt.visitors[i].LeaveAt = uint64(g.clock.Now()) + 1
	}
	g.Step(3)
	if v, here := visitorRecOf(g, vis); !here || !v.Leaving {
		t.Fatalf("fixture: leaving %+v", v)
	}
	turnHostile(t, g, fac)
	g.Step(2)
	if !g.isRaider(vis) || g.visitor(vis) {
		t.Fatal("a visitor on its way out stays friendly")
	}
}

func TestKillingTurnedVisitorCostsNoGoodwill(t *testing.T) {
	t.Parallel()
	g, doc, vis, fac := guestScene(t)
	turnHostile(t, g, fac)
	g.Step(2)
	gw, events := g.factionOf(fac).Goodwill, len(g.factionOf(fac).Events)
	g.hurt(vis, health.DownedInjury+10, doc)
	g.Step(1)
	g.die(vis, health.CauseInjury)
	g.Step(2)
	if g.factionOf(fac).Goodwill != gw || len(g.factionOf(fac).Events) != events {
		t.Fatalf("goodwill %d -> %d, events %q", gw, g.factionOf(fac).Goodwill, g.factionOf(fac).Events)
	}
	if _, here := visitorRecOf(g, vis); here {
		t.Fatal("the dead keep a visitor record")
	}
}

// TestDownedVisitorStaysDownWhenFactionTurns: it stays where it lies, an
// enemy; its death is nobody's goodwill.
func TestDownedVisitorStaysDownWhenFactionTurns(t *testing.T) {
	t.Parallel()
	g, doc, vis, fac := guestScene(t)
	g.draft[doc] = true // nobody comes for it
	g.hurt(vis, health.DownedInjury+10, 0)
	g.Step(1)
	turnHostile(t, g, fac)
	gw := g.factionOf(fac).Goodwill
	at, _ := g.positions.Get(vis)
	for range 30 {
		g.Step(2)
		if v, _ := g.vital.Get(vis); v.Status == health.Downed {
			if p, _ := g.positions.Get(vis); p != at {
				t.Fatalf("a downed visitor moved %v -> %v", at, p)
			}
		}
	}
	if !g.isRaider(vis) || g.standingRaider(vis) {
		t.Fatalf("raider %v standing %v", g.isRaider(vis), g.standingRaider(vis))
	}
	g.die(vis, health.CauseInjury)
	g.Step(2)
	if hasEvent(g, fac, care.ReasonDiedInCare) || g.factionOf(fac).Goodwill != gw {
		t.Fatalf("goodwill %d -> %d, events %q", gw, g.factionOf(fac).Goodwill, g.factionOf(fac).Events)
	}
}

// TestGuestInBedRecoversThenLeavesHostile: a rescued guest stays in its bed
// until it is well, then walks out as an enemy.
func TestGuestInBedRecoversThenLeavesHostile(t *testing.T) {
	t.Parallel()
	g, doc, vis, fac := guestScene(t)
	g.climate = temperature.Climate{YearMean: 200}
	g.hurt(vis, health.DownedInjury+10, 0)
	g.Step(1)
	if !runCare(g, 600, func() bool { return g.inBed(vis) }, doc, vis) {
		t.Fatal("fixture: never reached the bed")
	}
	turnHostile(t, g, fac)
	g.Step(2)
	if !g.visitor(vis) || g.isRaider(vis) || !g.inBed(vis) {
		t.Fatalf("in care: visitor %v, raider %v, in bed %v", g.visitor(vis), g.isRaider(vis), g.inBed(vis))
	}
	if len(turnedLetters(g)) != 1 {
		t.Fatalf("%d letters", len(turnedLetters(g)))
	}
	calmDown(t, g, fac) // keeps leaving hostile even so
	for i := 0; i < 15000 && g.visitor(vis); i++ {
		if v, _ := g.vital.Get(vis); v.Status == health.Alive && !g.inBed(vis) {
			t.Fatalf("step %d: up and out of bed while still a guest", i)
		}
		g.Step(10)
		g.SetNeeds(doc, needs.Full, needs.Full)
		g.SetNeeds(vis, needs.Full, needs.Full)
	}
	if g.visitor(vis) || !g.isRaider(vis) || g.inBed(vis) {
		t.Fatalf("recovered: visitor %v, raider %v, in bed %v", g.visitor(vis), g.isRaider(vis), g.inBed(vis))
	}
	for i := 0; i < 2000 && g.onMap(vis); i++ {
		g.Step(5)
	}
	if g.onMap(vis) {
		t.Fatal("it never left")
	}
}

// TestCarriedGuestReachesBedBeforeTurning: a rescue under way ends in the bed.
func TestCarriedGuestReachesBedBeforeTurning(t *testing.T) {
	t.Parallel()
	g, doc, vis, fac := guestScene(t)
	g.climate = temperature.Climate{YearMean: 200}
	g.hurt(vis, health.DownedInjury+10, 0)
	g.Step(1)
	if !runCare(g, 600, func() bool { return g.care.Carry.Carrier(vis) != 0 }, doc, vis) {
		t.Fatal("fixture: never picked up")
	}
	turnHostile(t, g, fac)
	g.Step(1)
	if g.isRaider(vis) {
		t.Fatal("turned in its rescuer's arms")
	}
	if !runCare(g, 600, func() bool { return g.inBed(vis) }, doc, vis) {
		t.Fatal("the rescue was dropped")
	}
	if g.isRaider(vis) {
		t.Fatal("turned in bed")
	}
}

func TestTurnedVisitorFightsBack(t *testing.T) {
	t.Parallel()
	g, doc, vis, fac := guestScene(t)
	g.equip[vis] = item.Rifle
	turnHostile(t, g, fac)
	g.Step(2)
	// a colonist in gun range, nobody hurt: it walks on
	vp, _ := g.positions.Get(vis)
	moveTo(t, g, doc, vp.X+6, vp.Y)
	g.Step(1)
	if j, _ := g.jobs.Get(vis); j.Kind == job.Attack {
		t.Fatal("it stopped to shoot unprovoked")
	}
	// shot: it shoots back, standing where it is
	vp, _ = g.positions.Get(vis)
	moveTo(t, g, doc, vp.X+6, vp.Y)
	g.hurt(vis, 3, doc)
	g.Step(1)
	j, _ := g.jobs.Get(vis)
	if j.Kind != job.Attack || j.Target != doc {
		t.Fatalf("no shot back: %+v", j)
	}
	// the fight runs out: no new wound for a while, and it walks on
	g.Step(shootBackHold + 2)
	if j, _ := g.jobs.Get(vis); j.Kind == job.Attack {
		t.Fatalf("it stays and fights: %+v", j)
	}
	if !g.isRaider(vis) {
		t.Fatal("no longer an enemy")
	}
}

// TestTurnedVisitorNeverChases: a target out of reach ends the fight.
func TestTurnedVisitorNeverChases(t *testing.T) {
	t.Parallel()
	g, doc, vis, fac := guestScene(t)
	g.equip[vis] = item.Revolver
	turnHostile(t, g, fac)
	g.Step(2)
	vp, _ := g.positions.Get(vis)
	moveTo(t, g, doc, vp.X+6, vp.Y)
	g.hurt(vis, 3, doc)
	g.Step(1)
	if j, _ := g.jobs.Get(vis); j.Kind != job.Attack {
		t.Fatalf("no shot back: %+v", j)
	}
	g.positions.Set(vis, Position{X: 37, Y: 6})
	moveTo(t, g, doc, 1, 6)
	g.Step(2)
	if j, _ := g.jobs.Get(vis); j.Kind == job.Attack {
		t.Fatalf("it gave chase: %+v", j)
	}
}

func TestTurnedVisitorHitsWhoeverStandsBesideIt(t *testing.T) {
	t.Parallel()
	g, doc, vis, fac := guestScene(t)
	turnHostile(t, g, fac)
	g.Step(2)
	vp, _ := g.positions.Get(vis)
	moveTo(t, g, doc, vp.X+1, vp.Y)
	dp, _ := g.positions.Get(doc)
	if max(mathx.Abs(dp.X-vp.X), mathx.Abs(dp.Y-vp.Y)) > 1 {
		t.Skip("fixture: no tile beside it")
	}
	g.Step(1)
	if j, _ := g.jobs.Get(vis); j.Kind != job.Attack || j.Target != doc {
		t.Fatalf("no self defence: %+v", j)
	}
}

// TestTurnedVisitorKidnapsNobody: it is leaving, not raiding.
func TestTurnedVisitorKidnapsNobody(t *testing.T) {
	t.Parallel()
	g, doc, vis, fac := guestScene(t)
	g.hurt(doc, health.DownedInjury+10, 0)
	vp, _ := g.positions.Get(vis)
	moveTo(t, g, doc, vp.X+3, vp.Y)
	turnHostile(t, g, fac)
	for i := 0; i < 300 && g.onMap(vis); i++ {
		g.Step(1)
		if j, _ := g.jobs.Get(vis); j.Kind == job.Kidnap {
			t.Fatal("it went for the downed colonist")
		}
	}
}

func TestTurnedVisitorsSaveRoundTrip(t *testing.T) {
	t.Parallel()
	g, _, vis, fac := guestScene(t)
	turnHostile(t, g, fac)
	g.Step(2)
	h := saveLoad(t, g)
	if !h.isRaider(vis) || h.visitor(vis) {
		t.Fatalf("after a load: raider %v, visitor %v", h.isRaider(vis), h.visitor(vis))
	}
	if r, here := visitorRecOf(h, vis); !here || !r.Turned {
		t.Fatalf("record after a load: %+v", r)
	}
	if h.StateHash() != g.StateHash() {
		t.Fatal("hash moved over the round trip")
	}
	for i := 0; i < 2000 && h.onMap(vis); i++ {
		h.Step(5)
	}
	if h.onMap(vis) {
		t.Fatal("it never left after the load")
	}
}

// TestHostileTraderClosesTradeAndKeepsItsGoods: the session closes, new trade
// is refused with a reason, and a trader that gets away takes its goods.
func TestHostileTraderClosesTradeAndKeepsItsGoods(t *testing.T) {
	t.Parallel()
	g, c := caravanGame(t)
	openCaravanTrade(t, g, c)
	trader, neg := c.Trader, g.pawnIDs[0]
	ref := g.factionRef(c.Faction)
	items := len(g.itemIDs)
	turnHostile(t, g, ref)
	g.Step(2)
	if _, ok := g.TradeView(); ok {
		t.Fatal("the session stayed open")
	}
	if g.traderOpen(trader) {
		t.Fatal("the trader still trades")
	}
	if _, ok := g.TradeStatus(); ok {
		t.Fatal("the caravan still counts as visiting")
	}
	tp, _ := g.positions.Get(trader)
	o := optionAt(g, neg, tp.X, tp.Y, "Trade with ")
	if o == nil || !o.Disabled || o.Reason != ReasonNoTrade {
		t.Fatalf("trade order %+v", o)
	}
	if _, why := g.tradeJob(neg, tp.X, tp.Y); why != ReasonNoTrade {
		t.Fatalf("refused because %q", why)
	}
	gw := g.factionOf(ref).Goodwill
	h := saveLoad(t, g)
	if h.trade.caravan == nil || !h.trade.caravan.Turned {
		t.Fatalf("caravan after a load: %+v", h.trade.caravan)
	}
	for i := 0; i < 8000 && g.trade.caravan != nil; i++ {
		g.Step(5)
	}
	if g.trade.caravan != nil {
		t.Fatal("the caravan never left")
	}
	if len(g.itemIDs) != items || g.factionOf(ref).Goodwill != gw {
		t.Fatalf("items %d -> %d, goodwill %d -> %d", items, len(g.itemIDs), gw, g.factionOf(ref).Goodwill)
	}
}

// TestKilledHostileTraderDropsGoodsFreeOfGoodwill: its goods lie where it
// fell and nobody minds.
func TestKilledHostileTraderDropsGoodsFreeOfGoodwill(t *testing.T) {
	t.Parallel()
	g, c := caravanGame(t)
	trader, ref := c.Trader, g.factionRef(c.Faction)
	items := len(g.itemIDs)
	turnHostile(t, g, ref)
	g.Step(2)
	gw := g.factionOf(ref).Goodwill
	g.die(trader, health.CauseInjury)
	g.Step(2)
	if g.trade.caravan != nil {
		t.Fatal("the caravan outlives its trader")
	}
	if len(g.itemIDs) <= items {
		t.Fatal("the goods did not drop")
	}
	if g.factionOf(ref).Goodwill != gw {
		t.Fatalf("goodwill %d -> %d", gw, g.factionOf(ref).Goodwill)
	}
}

func TestHostileAlliesLeaveAsEnemies(t *testing.T) {
	t.Parallel()
	g, f := aidGame(t)
	ids := aidHere(t, g, f)
	ref := g.factionRef(f.ID)
	turnHostile(t, g, ref)
	g.Step(2)
	if got := aidOf(g).AllyIDs(f.ID); len(got) != 0 {
		t.Fatalf("still allies: %v", got)
	}
	if len(g.Allies()) != 0 {
		t.Fatalf("still listed: %+v", g.Allies())
	}
	for _, id := range ids {
		if !g.isRaider(id) || g.isAlly(id) || g.visitor(id) {
			t.Fatalf("%d: raider %v, ally %v, visitor %v", id, g.isRaider(id), g.isAlly(id), g.visitor(id))
		}
	}
	g.Step(2)
	if aidOf(g).Of(f.ID) != nil {
		t.Fatal("the call outlives its group")
	}
	g.die(ids[0], health.CauseInjury)
	g.Step(2)
	if slices.ContainsFunc(g.letters.List(), func(l letters.Letter) bool { return l.Title == "Ally killed" }) {
		t.Fatal("a letter mourns an enemy")
	}
}

// TestAttackOnSettlementTurnsVisitors: a relation forced hostile without any
// goodwill drop (maps/settle.go) turns them the same.
func TestAttackOnSettlementTurnsVisitors(t *testing.T) {
	t.Parallel()
	g, _, vis, fac := guestScene(t)
	g.factionOf(fac).Relation = faction.Hostile
	g.Step(2)
	if !g.isRaider(vis) {
		t.Fatal("not turned")
	}
}

// TestStrangerNeverTurns: a visitor of no faction has nobody to turn with.
func TestStrangerNeverTurns(t *testing.T) {
	t.Parallel()
	g, _, _, fac := guestScene(t)
	stranger := g.spawnVisitor(0, nil, 12, 8, uint64(g.clock.Now()))
	turnHostile(t, g, fac)
	g.Step(2)
	if g.isRaider(stranger) || !g.visitor(stranger) {
		t.Fatalf("raider %v, visitor %v", g.isRaider(stranger), g.visitor(stranger))
	}
}
