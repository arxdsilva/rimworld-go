package game

import (
	"reflect"
	"testing"

	"rimworld-go/internal/application/game/combat"
	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// aidGame is a set-up colony with one friendly human faction (goodwill
// 70: friendly still after one aid request) and a comms session open.
func aidGame(t *testing.T) (*Game, *faction.Faction) {
	t.Helper()
	g := visitorGame(t)
	var f *faction.Faction
	for i := range g.ng.factions {
		if g.ng.factions[i].Relation == faction.Neutral && g.ng.factions[i].Type.Human() {
			f = &g.ng.factions[i]
		}
	}
	if f == nil {
		t.Fatal("no neutral human faction")
	}
	f.Relation, f.Goodwill = faction.Friendly, 70
	g.trade.comms = &commsSession{operator: g.pawnIDs[0]}
	return g, f
}

// aidHere requests aid from f and brings it in at once (the delay is
// TestAidArrivesAfterDelay's).
func aidHere(t *testing.T, g *Game, f *faction.Faction) []entity.EntityID {
	t.Helper()
	g.Enqueue(command.Command{Kind: command.CommsRequestAid, Index: f.ID})
	g.ApplyPending()
	c := g.aidOf(f.ID)
	if c == nil {
		t.Fatal("no aid call")
	}
	c.At = uint64(g.clock.Now()) + 1
	for i := 0; i < 5 && !c.Here; i++ {
		g.Step(1)
		c = g.aidOf(f.ID)
	}
	ids := g.allyIDs(f.ID)
	if c == nil || !c.Here || len(ids) == 0 {
		t.Fatalf("aid did not arrive: %+v", c)
	}
	return ids
}

// moveTo puts creature id on a free walkable tile near (x, y).
func moveTo(t *testing.T, g *Game, id entity.EntityID, x, y int) Position {
	t.Helper()
	tx, ty, ok := g.combat.Tiles.WalkableNear(x, y, 6)
	if !ok {
		t.Fatalf("no walkable tile near %d,%d", x, y)
	}
	g.reserve.ReleaseOwner(id)
	g.jobs.Set(id, job.Record{})
	g.positions.Set(id, Position{X: tx, Y: ty})
	g.invalidateCreatures()
	return Position{X: tx, Y: ty}
}

// commsRow is faction id's comms row.
func commsRow(t *testing.T, g *Game, id int) CommsFactionView {
	t.Helper()
	cv, ok := g.CommsView()
	if !ok {
		t.Fatal("no comms session")
	}
	for _, r := range cv.Factions {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("no row for faction %d", id)
	return CommsFactionView{}
}

func TestRequestAidSpendsGoodwillAndCooldown(t *testing.T) {
	t.Parallel()
	g, f := aidGame(t)
	comms := g.trade.comms
	g.trade.comms = nil
	g.Enqueue(command.Command{Kind: command.CommsRequestAid, Index: f.ID})
	g.ApplyPending()
	if f.Goodwill != 70 || len(g.trade.aid) != 0 {
		t.Fatal("a request without a comms session went through")
	}
	g.trade.comms = comms
	if r := commsRow(t, g, f.ID); !r.Aid.Allowed || r.Aid.Cost != faction.AidCost || r.Aid.Reason != "" {
		t.Fatalf("aid action %+v", r.Aid)
	}
	now := uint64(g.clock.Now())
	g.Enqueue(command.Command{Kind: command.CommsRequestAid, Index: f.ID})
	g.Enqueue(command.Command{Kind: command.CommsRequestAid, Index: f.ID}) // a second ask is refused
	g.ApplyPending()
	if f.Goodwill != 70-faction.AidCost || len(g.trade.aid) != 1 {
		t.Fatalf("goodwill %d, aid %+v", f.Goodwill, g.trade.aid)
	}
	c := g.trade.aid[0]
	if c.Faction != f.ID || c.Here || c.At < now+allyDelayMinH*clock.TicksPerHour || c.At > now+allyDelayMaxH*clock.TicksPerHour {
		t.Fatalf("aid call %+v (now %d)", c, now)
	}
	if cd := g.cooldownOf(f.ID); cd.AidUntil != now+faction.AidCooldownDays*clock.TicksPerDay {
		t.Fatalf("cooldown %+v", cd)
	}
	if r := commsRow(t, g, f.ID); r.Aid.Allowed || r.Aid.Reason == "" {
		t.Fatalf("aid while cooling: %+v", r.Aid)
	}
	// cooling down, then on the way, then here: each with its own reason
	if ok, why := g.aidRequestOK(*f, now); ok || why != faction.ReasonCoolingOff {
		t.Fatalf("again: %v %q", ok, why)
	}
	g.setCooldown(f.ID, faction.Cooldowns{})
	if ok, why := g.aidRequestOK(*f, now); ok || why != ReasonAidComing {
		t.Fatalf("while on the way: %v %q", ok, why)
	}
	g.trade.aid[0].Here = true
	if ok, why := g.aidRequestOK(*f, now); ok || why != ReasonAlliesHere {
		t.Fatalf("while here: %v %q", ok, why)
	}
	g.trade.aid = nil
	f.Relation = faction.Neutral
	if ok, why := g.aidRequestOK(*f, now); ok || why != faction.ReasonNotFriend {
		t.Fatalf("neutral: %v %q", ok, why)
	}
}

func TestAidArrivesAfterDelay(t *testing.T) {
	t.Parallel()
	g, f := aidGame(t)
	g.Enqueue(command.Command{Kind: command.CommsRequestAid, Index: f.ID})
	g.ApplyPending()
	at := g.trade.aid[0].At
	// the wait itself is long: skip most of it, then watch the last ticks
	g.clock.Advance(int(at - uint64(g.clock.Now()) - 3))
	g.Step(2)
	if len(g.allyIDs(f.ID)) != 0 || g.trade.aid[0].Here {
		t.Fatal("aid arrived early")
	}
	g.Step(2)
	ids := g.allyIDs(f.ID)
	if n := len(ids); n < allyMin || n > allyMax {
		t.Fatalf("%d allies came", n)
	}
	if !hasLetter(g, "have come to help defend the colony") {
		t.Fatal("no arrival letter")
	}
	for _, id := range ids {
		if _, ok := g.equip[id]; !ok {
			t.Fatalf("ally %d came unarmed", id)
		}
		if !g.isAlly(id) || g.isRaider(id) || g.standingRaider(id) {
			t.Fatalf("ally %d: ally %v raider %v", id, g.isAlly(id), g.isRaider(id))
		}
	}
	seen := 0
	for _, p := range g.Snapshot() {
		for _, id := range ids {
			if p.ID == id {
				seen++
				if !p.Visitor || p.Hostile || p.Prisoner {
					t.Fatalf("ally view %+v", p)
				}
			}
		}
	}
	if seen != len(ids) {
		t.Fatalf("%d of %d allies in the snapshot", seen, len(ids))
	}
	gs := g.Allies()
	if len(gs) != 1 || gs[0].Standing != len(ids) || gs[0].Faction != f.Name || gs[0].Leaving || len(gs[0].Spots) != len(ids) {
		t.Fatalf("Allies() = %+v", gs)
	}
}

// TestAlliesFightRaiders: an ally goes for a raider in sight and shoots
// it, and keeps that target while it stands even when another raider
// comes closer (no flapping).
func TestAlliesFightRaiders(t *testing.T) {
	t.Parallel()
	g, f := aidGame(t)
	ids := aidHere(t, g, f)
	ally := ids[0]
	g.equip[ally] = item.Rifle
	for _, id := range ids[1:] { // the others stay out of it
		moveTo(t, g, id, 3, 3)
	}
	ap := moveTo(t, g, ally, 30, 50)
	g.raidWith([]item.Kind{item.Club}, raid.Assault)
	a := g.raiderIDs[len(g.raiderIDs)-1]
	moveTo(t, g, a, ap.X+8, ap.Y)
	var b entity.EntityID
	struck := false
	for i := 0; i < 600 && g.standingRaider(a); i++ {
		g.Step(1)
		j, _ := g.jobs.Get(ally)
		if i == 60 {
			if j.Kind != job.Attack || j.Target != a {
				t.Fatalf("the ally is not on the raider: %+v", j)
			}
			g.raidWith([]item.Kind{item.Club}, raid.Assault)
			b = g.raiderIDs[len(g.raiderIDs)-1]
			p, _ := g.positions.Get(ally)
			moveTo(t, g, b, p.X+2, p.Y+1)
		}
		if i > 60 && g.standingRaider(a) && g.standingPawn(ally) && j.Target != a {
			t.Fatalf("tick %d: the ally switched from %d to %d", i, a, j.Target)
		}
		for _, tr := range g.traces {
			if tr.Attacker == ally && tr.Target == a {
				struck = true
			}
		}
	}
	if !struck {
		t.Fatal("the ally never fired at the raider")
	}
	if b == 0 {
		t.Fatal("the raider went down before the second came")
	}
}

// TestRaidersTargetAllies: a raider with an ally closer than any colonist
// goes for the ally, and stays on it while it stands (no flapping) even
// when a colonist comes nearer.
func TestRaidersTargetAllies(t *testing.T) {
	t.Parallel()
	g, f := aidGame(t)
	ids := aidHere(t, g, f)
	ally := ids[0]
	for _, id := range ids[1:] {
		moveTo(t, g, id, 3, 3)
	}
	ap := moveTo(t, g, ally, 50, 50)
	g.raidWith([]item.Kind{item.Club}, raid.Assault)
	rid := g.raiderIDs[len(g.raiderIDs)-1]
	rp := moveTo(t, g, rid, ap.X+3, ap.Y)
	if tg, _, ok := g.raidTarget(rp); !ok || tg != ally {
		t.Fatalf("raidTarget = %d, want the ally %d", tg, ally)
	}
	moved := false
	for i := 0; i < 400 && g.standingPawn(ally); i++ {
		g.Step(1)
		j, _ := g.jobs.Get(rid)
		if !g.standingRaider(rid) {
			break
		}
		if j.Kind == job.Attack && j.Target != ally {
			t.Fatalf("tick %d: the raider attacks %d, not the ally %d", i, j.Target, ally)
		}
		if i == 30 && !moved {
			if j.Kind != job.Attack {
				t.Fatalf("the raider is not attacking: %+v", j)
			}
			p, _ := g.positions.Get(rid)
			moveTo(t, g, g.pawnIDs[0], p.X-1, p.Y+1) // a colonist right beside it
			moved = true
		}
	}
	if !moved {
		t.Fatal("the fight ended before the colonist came")
	}
}

// TestTurretsIgnoreAllies: a powered turret never locks on an ally beside
// it, a drafted colonist never engages one, and a raider among them is
// still picked.
func TestTurretsIgnoreAllies(t *testing.T) {
	t.Parallel()
	out := 1000
	g, ts := turretRig(t, 3, 40, 30, &out, 20)
	tid := ts[0]
	g.DevAddFactions()
	var f *faction.Faction
	for i := range g.ng.factions {
		if g.ng.factions[i].Type.Human() && !faction.DefOf(g.ng.factions[i].Type).Fixed {
			f = &g.ng.factions[i]
			break
		}
	}
	f.Relation, f.Goodwill = faction.Friendly, 60
	g.addAid(aidCall{Faction: f.ID, At: uint64(g.clock.Now())})
	g.aidTick(uint64(g.clock.Now()))
	ids := g.allyIDs(f.ID)
	if len(ids) == 0 {
		t.Fatal("no allies")
	}
	ada, err := g.SpawnNamedPawn(20, 12, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	g.equip[ada] = item.Rifle
	g.draft[ada] = true
	keep := map[entity.EntityID]Position{ada: {X: 20, Y: 12}}
	for i, id := range ids {
		keep[id] = Position{X: 17 + i*2, Y: 8}
	}
	place := func() {
		for id, p := range keep {
			g.positions.Set(id, p)
		}
		g.invalidateCreatures()
	}
	place()
	for i := 0; i < 120; i++ {
		g.combat.Strikes.AutoEngage(ada)
		g.Step(1)
		place()
		if tg := g.combat.Turrets.Aim(tid).Target; tg != 0 {
			t.Fatalf("tick %d: the turret locked on %d (allies %v)", g.Tick(), tg, ids)
		}
		if j, _ := g.jobs.Get(ada); j.Kind == job.Attack {
			t.Fatalf("tick %d: the colonist attacks %d (allies %v)", g.Tick(), j.Target, ids)
		}
	}
	rid := turretRaider(t, g, 20, 14)
	for i := 0; i < combat.TurretScanEvery+1 && g.combat.Turrets.Aim(tid).Target == 0; i++ {
		g.Step(1)
		place()
	}
	if tg := g.combat.Turrets.Aim(tid).Target; tg != rid {
		t.Fatalf("the turret should pick the raider %d past the allies, got %d", rid, tg)
	}
}

// TestAlliesLeaveWhenQuiet: a standing raider keeps the group's calm
// clock at zero; with none about for allyQuiet the group turns for home
// (a letter), stays turned even when a raider shows up again (no
// flapping), and walks off the map. The day cap sends them home even
// with raiders about.
func TestAlliesLeaveWhenQuiet(t *testing.T) {
	t.Parallel()
	g, f := aidGame(t)
	ids := aidHere(t, g, f)
	c := g.aidOf(f.ID)
	now := uint64(g.clock.Now())
	c.Calm = now - allyQuiet + 5
	g.raidWith([]item.Kind{item.Club}, raid.Assault)
	rid := g.raiderIDs[len(g.raiderIDs)-1]
	g.Step(10)
	if c = g.aidOf(f.ID); c.Leaving || c.Calm != uint64(g.clock.Now())-1 {
		t.Fatalf("with a raider about: %+v (now %d)", c, g.clock.Now())
	}
	g.removeRaider(rid)
	c.Calm = uint64(g.clock.Now()) - allyQuiet + 5
	g.Step(10)
	if c = g.aidOf(f.ID); c == nil || !c.Leaving {
		t.Fatalf("not leaving after the calm: %+v", c)
	}
	if !hasLetter(g, "are heading home") {
		t.Fatal("no leaving letter")
	}
	for _, v := range g.worldSt.visitors {
		if v.Ally && !v.Leaving {
			t.Fatalf("ally %d is not leaving", v.ID)
		}
	}
	g.raidWith([]item.Kind{item.Club}, raid.Assault)
	for i := 0; i < 300; i++ {
		g.Step(1)
		for _, id := range g.allyIDs(f.ID) {
			if j, _ := g.jobs.Get(id); j.Kind == job.Attack {
				t.Fatalf("leaving ally %d turned back to fight", id)
			}
		}
	}
	for i := 0; i < 40 && len(g.allyIDs(f.ID)) > 0; i++ {
		g.Step(250)
	}
	if left := g.allyIDs(f.ID); len(left) > 0 {
		t.Fatalf("allies %v never left", left)
	}
	if g.aidOf(f.ID) != nil {
		t.Fatal("the aid call outlived its fighters")
	}
	for _, id := range ids {
		if p := g.personOf(id); p == nil || p.Where != whereOffMap && p.Where != whereVisitor {
			t.Fatalf("ally %d ended %+v", id, p)
		}
	}
	// the day cap
	g2, f2 := aidGame(t)
	aidHere(t, g2, f2)
	g2.raidWith([]item.Kind{item.Club}, raid.Assault)
	c2 := g2.aidOf(f2.ID)
	c2.LeaveAt = uint64(g2.clock.Now()) + 2
	g2.Step(4)
	if c2 = g2.aidOf(f2.ID); !c2.Leaving {
		t.Fatalf("the day cap did not send them home: %+v", c2)
	}
}

// TestAllyDeathLetter: an ally that dies gets a letter and costs no
// goodwill.
func TestAllyDeathLetter(t *testing.T) {
	t.Parallel()
	g, f := aidGame(t)
	ids := aidHere(t, g, f)
	gw := f.Goodwill
	g.die(ids[0], health.CauseInjury)
	g.Step(2)
	if !hasLetter(g, "fell defending the colony") {
		t.Fatal("no letter for the fallen ally")
	}
	if f.Goodwill != gw {
		t.Fatalf("goodwill %d, was %d", f.Goodwill, gw)
	}
}

// TestAidSurvivesSave: a pending call and a group on the map (the ally
// flag, the group's clocks) survive a save, and the loaded game runs on
// identically.
func TestAidSurvivesSave(t *testing.T) {
	t.Parallel()
	g, f := aidGame(t)
	g.Enqueue(command.Command{Kind: command.CommsRequestAid, Index: f.ID})
	g.ApplyPending()
	h := saveLoad(t, g)
	if !reflect.DeepEqual(h.trade.aid, g.trade.aid) || len(h.trade.aid) != 1 {
		t.Fatalf("pending aid: %+v vs %+v", h.trade.aid, g.trade.aid)
	}
	g.trade.aid[0].At = uint64(g.clock.Now()) + 1
	g.Step(3)
	ids := g.allyIDs(f.ID)
	if len(ids) == 0 {
		t.Fatal("no allies")
	}
	g.raidWith([]item.Kind{item.Rifle}, raid.Assault)
	g.Step(50)
	g.trade.comms = nil // sessions close on load
	before := g.StateHash()
	h = saveLoad(t, g)
	if h.StateHash() != before {
		t.Fatal("the hash moved over a round trip")
	}
	if got := h.allyIDs(f.ID); !reflect.DeepEqual(got, ids) {
		t.Fatalf("allies after load %v, want %v", got, ids)
	}
	if !reflect.DeepEqual(h.trade.aid, g.trade.aid) || !reflect.DeepEqual(h.worldSt.visitors, g.worldSt.visitors) {
		t.Fatal("aid state differs after load")
	}
	g.Step(400)
	h.Step(400)
	if g.StateHash() != h.StateHash() {
		t.Fatal("the loaded game drifted")
	}
}

// TestAidSameSeed: two colonies from the same seed run the same aid and
// fight.
func TestAidSameSeed(t *testing.T) {
	t.Parallel()
	run := func() (*Game, int) {
		g, f := aidGame(t)
		aidHere(t, g, f)
		g.raidWith([]item.Kind{item.Rifle, item.Club, item.Rifle}, raid.Assault)
		return g, f.ID
	}
	a, fa := run()
	b, fb := run()
	for i := 0; i < 6; i++ {
		a.Step(400)
		b.Step(400)
		if a.StateHash() != b.StateHash() {
			t.Fatalf("hashes diverged after %d ticks", (i+1)*400)
		}
	}
	if !reflect.DeepEqual(a.trade.aid, b.trade.aid) || !reflect.DeepEqual(a.allyIDs(fa), b.allyIDs(fb)) {
		t.Fatal("aid state diverged")
	}
}

// TestAllyFlagInSnapshot: an ally reads as a friendly visitor with the
// Ally flag, so the inspector can name it.
func TestAllyFlagInSnapshot(t *testing.T) {
	t.Parallel()
	g, f := aidGame(t)
	ids := aidHere(t, g, f)
	for _, p := range g.Snapshot() {
		if p.ID != ids[0] {
			continue
		}
		if !p.Ally || !p.Visitor || p.Hostile {
			t.Fatalf("ally view: Ally=%v Visitor=%v Hostile=%v", p.Ally, p.Visitor, p.Hostile)
		}
		return
	}
	t.Fatal("ally missing from the snapshot")
}
