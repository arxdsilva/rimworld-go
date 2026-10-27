package game

import (
	"fmt"
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Military aid (PLAY-024 Phase 4, original tuning in
// .cursor/PLAY024_PLAN.md). "Request aid" over the radio (comms.go) costs
// a friendly faction's goodwill and queues an aidCall; after
// allyDelayMinH..allyDelayMaxH hours a group of allyMin..allyMax armed
// fighters walks in from a map edge. They are visitors (visitors.go:
// Where Visitor, on raiderIDs as non-hostile pawns) marked Ally, so they
// stay out of the colonist bar, take no orders and are never turret,
// colonist or draft targets (none of those pick a non-hostile pawn). Each
// tick visitorStep hands a standing ally to allyStep: it fights the
// standing raider or hostile animal it is on (doCombat), looks for a new
// one every allyScan ticks through the creature index, and otherwise
// strolls near the colony. Raiders fight them back (raidTarget). The
// group leaves (the visitor leave path) once no hostile has been about
// for allyQuiet, or allyStay after it came. Arrival, departure and every
// ally that falls get a letter; a death costs no goodwill.

const (
	// allyDelayMinH..allyDelayMaxH: hours from the request to arrival.
	allyDelayMinH = 2
	allyDelayMaxH = 4
	// allyMin..allyMax fighters come.
	allyMin = 3
	allyMax = 5
	// allyStay is the longest a group stays; allyQuiet the calm (no
	// hostile seen) after which it leaves sooner. Leaving is one-way, so
	// the group never flaps between staying and going.
	allyStay  = clock.TicksPerDay
	allyQuiet = 2 * clock.TicksPerHour
	// allyScan is how often (staggered by id) an ally without a fight
	// looks for one; it doubles as the retry after a failed attack.
	allyScan = 30
	// allySight is how far an ally sees a foe to go for (Chebyshev).
	allySight = 20
	// allyRetry: an arrival that found no way in tries again this much
	// later.
	allyRetry = clock.TicksPerHour
)

// Comms refusals for military aid.
const (
	ReasonNoFighters = "they have no fighters to send"
	ReasonAidComing  = "their fighters are on the way"
	ReasonAlliesHere = "their fighters are already here"
)

// aidCall is military aid from faction Faction (faction.Faction.ID),
// saved in the trade block: due at At until it arrives (Here); then it
// stays until LeaveAt at most, and Calm is the last tick a hostile was
// about. Leaving: the group has turned for home.
type aidCall struct {
	Faction int
	At      uint64
	Here    bool
	LeaveAt uint64
	Calm    uint64
	Leaving bool
}

// AllyGroupView is one aid group on the map for the alerts: its faction,
// how many of its fighters stand and are on the map, whether it is
// leaving, the hours left at most, and where its standing fighters are.
type AllyGroupView struct {
	Faction   string
	Standing  int
	Total     int
	Leaving   bool
	HoursLeft int
	Spots     [][2]int
}

// Allies lists the aid groups on the map (requests not yet here are not
// listed), in request order.
func (g *Game) Allies() []AllyGroupView {
	var out []AllyGroupView
	now := uint64(g.clock.Now())
	for _, c := range g.trade.aid {
		if !c.Here {
			continue
		}
		v := AllyGroupView{Leaving: c.Leaving, HoursLeft: hoursLeft(c.LeaveAt, now)}
		if f := g.factionByID(c.Faction); f != nil {
			v.Faction = f.Name
		}
		for _, id := range g.allyIDs(c.Faction) {
			v.Total++
			if st, _ := g.vital.Get(id); st.Status == health.Alive {
				p, _ := g.positions.Get(id)
				v.Standing++
				v.Spots = append(v.Spots, [2]int{p.X, p.Y})
			}
		}
		if v.Total > 0 {
			out = append(out, v)
		}
	}
	return out
}

// aidRequestOK reports whether faction f sends military aid if asked
// now, or why not.
func (g *Game) aidRequestOK(f faction.Faction, now uint64) (bool, string) {
	if !f.Type.Human() {
		return false, ReasonNoFighters
	}
	if ok, why := faction.CanRequestAid(f, g.cooldownOf(f.ID), now); !ok {
		return false, why
	}
	if c := g.aidOf(f.ID); c != nil {
		if c.Here {
			return false, ReasonAlliesHere
		}
		return false, ReasonAidComing
	}
	return true, ""
}

// requestAid charges faction f for military aid, starts its cooldown and
// queues the group; it reports the hours until it arrives.
func (g *Game) requestAid(f *faction.Faction, now uint64) int {
	faction.Adjust(f, -faction.AidCost, fmt.Sprintf("Day %d: military aid requested (%d goodwill)", g.relDay(now)+1, -faction.AidCost))
	cd := g.cooldownOf(f.ID)
	cd.AidUntil = now + faction.AidCooldownDays*clock.TicksPerDay
	g.setCooldown(f.ID, cd)
	hours := allyDelayMinH + g.stream(streamTrade).IntN(allyDelayMaxH-allyDelayMinH+1)
	g.addAid(aidCall{Faction: f.ID, At: now + uint64(hours)*clock.TicksPerHour})
	return hours
}

// addAid queues c in arrival order (ties: lower faction id first).
func (g *Game) addAid(c aidCall) {
	cs := g.trade.aid
	i, _ := slices.BinarySearchFunc(cs, c, func(a, b aidCall) int {
		if a.At != b.At {
			if a.At < b.At {
				return -1
			}
			return 1
		}
		return a.Faction - b.Faction
	})
	g.trade.aid = slices.Insert(cs, i, c)
}

// aidOf is faction id's aid call, coming or here (nil: none).
func (g *Game) aidOf(id int) *aidCall {
	for i := range g.trade.aid {
		if g.trade.aid[i].Faction == id {
			return &g.trade.aid[i]
		}
	}
	return nil
}

// aidTick runs the aid calls once a tick (tradeTick): due groups arrive,
// groups on the map note whether hostiles are about and turn for home
// when it has been calm long enough or their stay is over, and a group
// whose fighters are all gone (left, dead or taken) is dropped.
func (g *Game) aidTick(now uint64) {
	t := &g.trade
	kept := t.aid[:0]
	for _, c := range t.aid {
		switch {
		case !c.Here && now >= c.At:
			if !g.aidArrive(&c, now) {
				continue // the faction can no longer send it
			}
		case c.Here:
			if len(g.allyIDs(c.Faction)) == 0 {
				continue // all gone
			}
			if g.hostilesAbout(c.Faction) {
				c.Calm = now
			}
			if !c.Leaving && (now >= c.LeaveAt || now-c.Calm >= allyQuiet) {
				g.alliesLeave(&c)
			}
		}
		kept = append(kept, c)
	}
	clear(t.aid[len(kept):])
	t.aid = kept
	if len(kept) == 0 {
		t.aid = nil // a save without aid keeps no empty list
	}
}

// aidArrive brings aid call c's fighters in from a map edge; false: the
// faction fell or turned hostile and the call is dropped. With no way in
// it tries again allyRetry later.
func (g *Game) aidArrive(c *aidCall, now uint64) bool {
	ref := g.factionRef(c.Faction)
	f := g.factionOf(ref)
	if f == nil || f.Defeated || f.Relation == faction.Hostile {
		return false
	}
	ex, ey, ok := g.worldEdge()
	if !ok {
		c.At = now + allyRetry
		return true
	}
	r := g.stream(streamTrade)
	n := allyMin + r.IntN(allyMax-allyMin+1)
	var used [][2]int
	var ids []entity.EntityID
	for k := 0; k < n; k++ {
		x, y, ok := g.insetLandNear(ex, ey, used)
		if !ok {
			break
		}
		used = append(used, [2]int{x, y})
		id := g.spawnVisitor(ref, f, x, y, now)
		g.markAlly(id, now+allyStay)
		g.equip[id] = allyWeapon(faction.DefOf(f.Type).Tech, k)
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		c.At = now + allyRetry
		return true
	}
	c.Here, c.LeaveAt, c.Calm = true, now+allyStay, now
	g.letter(letterGood, "Military aid", fmt.Sprintf("%d fighters of %s have come to help defend the colony. They will fight any raiders and hostile animals, and go home once all is quiet, within a day.",
		len(ids), f.Name), ids[0])
	return true
}

// allyWeapon is the k-th fighter's weapon: neolithic peoples bring spears
// and clubs, the others rifles with one knife among every three.
func allyWeapon(tech faction.Tech, k int) item.Kind {
	if tech == faction.Neolithic {
		if k%2 == 0 {
			return item.Spear
		}
		return item.Club
	}
	if k%3 == 2 {
		return item.Knife
	}
	return item.Rifle
}

// markAlly makes visitor id an ally leaving at leaveAt.
func (g *Game) markAlly(id entity.EntityID, leaveAt uint64) {
	w := g.worldS()
	for i := range w.visitors {
		if w.visitors[i].ID == id {
			w.visitors[i].Ally, w.visitors[i].LeaveAt = true, leaveAt
		}
	}
}

// allyIDs lists faction id's allies on the map, ascending.
func (g *Game) allyIDs(id int) []entity.EntityID {
	if g.worldSt == nil {
		return nil
	}
	ref := g.factionRef(id)
	var out []entity.EntityID
	for _, v := range g.worldSt.visitors {
		if v.Ally && v.Faction == ref {
			out = append(out, v.ID)
		}
	}
	slices.Sort(out)
	return out
}

// isAlly reports id is an ally on the map.
func (g *Game) isAlly(id entity.EntityID) bool {
	if g.worldSt == nil {
		return false
	}
	for _, v := range g.worldSt.visitors {
		if v.ID == id {
			return v.Ally
		}
	}
	return false
}

// hostilesAbout reports a hostile is still about for faction id's group:
// a standing raider on the map (the raider list, never the animals), or
// one of its fighters in a fight (a hostile animal it went for).
func (g *Game) hostilesAbout(id int) bool {
	for _, rid := range g.raiderIDs {
		if g.standingRaider(rid) {
			return true
		}
	}
	for _, aid := range g.allyIDs(id) {
		if j, _ := g.jobs.Get(aid); j.Kind == job.Attack && j.Active() {
			return true
		}
	}
	return false
}

// alliesLeave turns aid group c for home: every fighter drops what it is
// doing and takes the visitor leave path.
func (g *Game) alliesLeave(c *aidCall) {
	c.Leaving = true
	ref := g.factionRef(c.Faction)
	w := g.worldSt
	var first entity.EntityID
	for i := range w.visitors {
		v := &w.visitors[i]
		if !v.Ally || v.Faction != ref || v.Leaving {
			continue
		}
		if first == 0 {
			first = v.ID
		}
		v.Leaving, v.RetryAt = true, 0
		g.reserve.ReleaseOwner(v.ID)
		g.jobs.Set(v.ID, job.Record{})
	}
	name := "Your allies"
	if f := g.factionByID(c.Faction); f != nil {
		name = "The fighters of " + f.Name
	}
	g.letter(letterInfo, "Allies leave", name+" are heading home.", first)
}

// allyStep runs standing ally v (not leaving) for one tick: keep fighting
// its foe, look for a new one on its cadence, else stroll near the
// colony like any visitor.
func (g *Game) allyStep(v *visitorRec, pos Position, j job.Record, now uint64) bool {
	if j.Kind == job.Attack && j.Active() {
		g.doCombat(v.ID, j, now)
		return true
	}
	if j.State == job.Complete || j.State == job.Fail || j.State == job.Do && j.Kind != job.Visit {
		g.reserve.ReleaseOwner(v.ID)
		j = job.Record{}
		g.jobs.Set(v.ID, j)
	}
	if (now+uint64(v.ID))%allyScan == 0 && g.allyEngage(v.ID, pos, now) {
		return true
	}
	switch {
	case j.Kind == job.Visit && j.State == job.Do:
		g.lingerVisit(v.ID, j)
	case j.Active():
	case now >= v.RetryAt:
		if !g.startStroll(v.ID) {
			v.RetryAt = now + visitorRetry
		}
	}
	return true
}

// allyEngage sends ally id after the nearest foe it can see within
// allySight (the creature index: a standing raider or a hostile animal,
// as a turret picks them); it reports whether it went.
func (g *Game) allyEngage(id entity.EntityID, pos Position, now uint64) bool {
	foe, fp, ok := g.nearestWhere(pos.X, pos.Y, allySight, ckRaider|ckAnimal, func(t entity.EntityID, p Position) bool {
		return g.turretTargetable(t, now) && g.lineOfSight(pos.X, pos.Y, p.X, p.Y) // turret.go
	})
	if !ok {
		return false
	}
	g.reserve.ReleaseOwner(id)
	d := weaponDefOf(g, id)
	rec := job.Record{Kind: job.Attack, Target: foe, TX: fp.X, TY: fp.Y, WorkLeft: aimTicks(d)}
	if g.canHit(pos, d, fp) { // already in reach: no walking needed
		rec.State = job.Do
		g.jobs.Set(id, rec)
		return true
	}
	return g.startJob(id, rec)
}

// allyGone notes ally v leaving the visitor list: one that died gets a
// letter (no goodwill is lost).
func (g *Game) allyGone(v visitorRec) {
	if st, ok := g.vital.Get(v.ID); !ok || st.Status != health.Dead {
		return
	}
	of := ""
	if f := g.factionOf(v.Faction); f != nil {
		of = " of " + f.Name
	}
	g.letter(letterCaution, "Ally killed", fmt.Sprintf("%s%s fell defending the colony.", g.name(v.ID), of), v.ID)
}

// raidTarget is what a raider goes for from p: the nearest standing
// colonist or ally (ties: the colonist).
func (g *Game) raidTarget(p Position) (entity.EntityID, Position, bool) {
	best, bp, ok := g.nearestColonist(p)
	bestD := 1 << 30
	if ok {
		bestD = weapon.Distance(p.X, p.Y, bp.X, bp.Y)
	}
	if g.worldSt == nil {
		return best, bp, ok
	}
	for _, v := range g.worldSt.visitors {
		if !v.Ally || v.Leaving || !g.standingPawn(v.ID) {
			continue
		}
		ap, _ := g.positions.Get(v.ID)
		if d := weapon.Distance(p.X, p.Y, ap.X, ap.Y); d < bestD { // visitors ascend: the lower id wins a tie
			best, bp, bestD = v.ID, ap, d
		}
	}
	return best, bp, best != 0
}
