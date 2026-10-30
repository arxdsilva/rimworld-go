package raid

import (
	"cmp"
	"fmt"

	raiddef "rimworld-go/internal/domain/raid"
	"rimworld-go/internal/domain/sim/clock"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/entity"
)

// Raiders is the raider AI. Raiders are pawns of their own faction: they
// walk in from a map edge, attack the nearest standing colonist, and
// retreat when badly hurt or once half their group is down. A downed
// raider bleeds out.
type Raiders struct {
	h  Host
	m  Mischief   // thieves, burners, rivals (nil: none)
	sg SiegeWorks // siege mortars (nil: none)
	ch Chronicle  // the combat log (nil: none)
	// rivalAt, rivalOK, rivals: whether rival factions' raiders stand on
	// the map together, refreshed once a tick (mischief.go).
	rivalAt         uint64
	rivalOK, rivals bool
}

// Think runs raider id for one tick (the game's threat phase).
func (rs *Raiders) Think(id entity.EntityID, now uint64) {
	h := rs.h
	r := h.Raider(id)
	if r.Leaving || h.Prisoner(id) {
		h.LivePrisoner(id, now) // prison/life.go: out of the raid AI
		return
	}
	v := h.Vital(id)
	switch {
	case v.Status == health.Dead:
		if r.Dead == 0 {
			r.Dead = now
			h.DropGear(id)
			h.SetRaider(id, r)
			rs.log(fmt.Sprintf("Raider %s was killed.", h.Name(id)))
		}
		return // its body stays until it is buried or cremated (burial.go)
	case v.Status == health.Downed:
		h.DropPatient(id) // kidnap.go: a downed kidnapper lets go
		if r.Downed == 0 {
			r.Downed = now
			h.SetRaider(id, r)
			x, y, _ := h.Pos(id)
			h.Message(x, y, fmt.Sprintf("Raider %s is down.", h.Name(id)))
			rs.log(fmt.Sprintf("Raider %s was downed.", h.Name(id)))
		}
		// MED-01: no timer; it bleeds out unless captured and tended, and
		// its wounds close slowly, so one that stops bleeding gets up again
		if !h.BeingCaptured(id) {
			h.Heal(id, v, now) // combat.go: stands it up past the band (bleed.go)
		}
		return
	}
	if r.Downed != 0 { // back on its feet after going down: it has had enough
		r.Downed, r.Mode = 0, Fleeing
		h.SetRaider(id, r)
		h.Interrupt(id, job.Cancelled)
	}
	j, _ := h.Job(id)
	if j.Kind == job.Kidnap {
		h.StepKidnap(id, j, now) // kidnap.go: committed until it ends
		return
	}
	if r.Arrived == 0 {
		r.Arrived = now // raiders from before this field existed
	}
	if r.Mode != Fleeing && (v.Injury >= health.MaxHP/2 || rs.broken(r.Group, now) ||
		r.Idle != 0 && now-r.Idle >= raiderLeaveIdle || rs.givenUp(r, now)) {
		r.Mode = Fleeing
		h.SetRaider(id, r)
		h.Interrupt(id, job.Cancelled)
		j = job.Record{}
	}
	x, y, _ := h.Pos(id)
	if r.Mode != Fleeing {
		rs.fight(id, r, j, v.Injury, x, y, now)
		return
	}
	w, ht := h.Size()
	if x == 0 || y == 0 || x == w-1 || y == ht-1 {
		if r.LootQty > 0 {
			rs.log(fmt.Sprintf("%s got away with %d %s.", h.Name(id), r.LootQty, r.Loot))
		}
		h.RemoveRaider(id) // gone home
		return
	}
	if h.TryKidnap(id, x, y, now) { // kidnap.go
		return
	}
	if !j.Active() || (now+uint64(id))%120 == 0 && j.Kind != job.Move {
		ex, ey := h.NearestEdge(x, y)
		if tx, ty, ok := h.WalkableNear(ex, ey, 3); ok {
			h.StartJob(id, job.Record{Kind: job.Move, TX: tx, TY: ty})
		}
	}
}

// fight runs a raider that is arriving or assaulting, standing on (x, y)
// with injury hurt.
func (rs *Raiders) fight(id entity.EntityID, r Raider, j job.Record, hurt, x, y int, now uint64) {
	h := rs.h
	if rs.siege(id, r, hurt, now) || rs.stage(id, r, j, x, y, now) {
		return
	}
	if r.Mode == Arriving && rs.mischief(id, r, j, x, y, now) {
		return
	}
	if j.Kind == job.Attack && j.Active() {
		h.Strike(id, j, now)
		return
	}
	if j.Kind == job.Breach && j.Active() {
		// keep bashing unless a colonist (or ally) comes into reach
		// (re-planning now and then in case a way in opened up)
		_, tx, ty, ok := h.RaidTarget(x, y) // allies.go
		if (now+uint64(id))%300 != 0 && (!ok || !h.CanHit(x, y, h.Weapon(id), tx, ty)) {
			h.Breach(id, j)
			return
		}
	}
	if now < r.RetryAt && !j.Active() {
		return // no way in last time: wait before planning again
	}
	target, tx, ty, ok := h.RaidTarget(x, y) // allies.go: colonists and allies (PLAY-024)
	if !ok {
		// wiki: Raid, a reachable powered turret is a target too
		target, tx, ty, ok = h.TurretFor(x, y, 1<<20, 2*SightRange)
	}
	best := 1 << 20
	if ok {
		best = weapon.Distance(x, y, tx, ty)
	}
	if rid, rx, ry, rok := rs.rival(r, x, y, best, now); rok { // a rival faction's raider closer
		target, tx, ty, ok = rid, rx, ry, true
	}
	if !ok {
		if r.Idle == 0 {
			r.Idle = now
			h.SetRaider(id, r)
		}
		h.TryKidnap(id, x, y, now) // kidnap.go: no one left standing
		return
	}
	r.Idle = 0
	// combat/turret.go (PLAY-024): a powered turret in sight and well
	// closer than the nearest colonist draws the raider's fire instead
	if tid, bx, by, ok := h.TurretFor(x, y, weapon.Distance(x, y, tx, ty), SightRange); ok {
		target, tx, ty = tid, bx, by
	}
	if r.Mode == Arriving && weapon.Distance(x, y, tx, ty) > SightRange {
		// walk in towards the colony
		if !j.Active() || (now+uint64(id))%180 == 0 {
			if wx, wy, ok := h.WalkableNear(tx, ty, 4); ok &&
				!h.StartJob(id, job.Record{Kind: job.Move, TX: wx, TY: wy}) && rs.unreachable(id) && !h.StartBreach(id, tx, ty) {
				r.RetryAt = now + RaiderRetry
			}
		}
		h.SetRaider(id, r)
		return
	}
	r.Mode = Assaulting
	h.SetRaider(id, r)
	d := h.Weapon(id)
	rec := job.Record{Kind: job.Attack, Target: target, TX: tx, TY: ty, WorkLeft: h.AimTicks(d)}
	if h.CanHit(x, y, d, tx, ty) { // already in reach: no walking needed
		h.ReleaseOwner(id)
		rec.State = job.Do
		h.SetJob(id, rec)
		return
	}
	if !h.StartJob(id, rec) && rs.unreachable(id) && !h.StartBreach(id, tx, ty) {
		r.RetryAt = now + RaiderRetry
		h.SetRaider(id, r)
	}
}

// unreachable reports that a mover's last job failed for want of a path.
func (rs *Raiders) unreachable(id entity.EntityID) bool {
	j, _ := rs.h.Job(id)
	return j.State == job.Fail && j.Fail == job.Unreachable
}

// broken: the group has lost its flee share (wiki: Raid, 40 % to 70 %,
// rolled per group; pods still falling count as standing). The first
// raider to see it logs the retreat; fleeing is one way.
func (rs *Raiders) broken(group int, now uint64) bool {
	h := rs.h
	standing := 0
	for _, id := range h.RaiderIDs() {
		if h.Raider(id).Group == group && h.StandingRaider(id) {
			standing++
		}
	}
	grp := &h.Groups()[group]
	if !raiddef.Broken(standing+podsLeft(grp.Pods), grp.Size, grp.FleeLoss) {
		return false
	}
	if !grp.Fled {
		grp.Fled = true
		rs.log("The raiders have lost heart and are retreating.")
	}
	return true
}

// givenUp: raider r has been on the map a day (a staged or besieging
// raid's day starts when it storms; until then it holds).
func (rs *Raiders) givenUp(r Raider, now uint64) bool {
	start := r.Arrived
	if groups := rs.h.Groups(); r.Group < len(groups) {
		if grp := groups[r.Group]; grp.Plan == Siege || grp.Plan == Stage {
			if !grp.Stormed {
				return false
			}
			start = max(start, grp.Until)
		}
	}
	return now-start >= RaiderGiveUp
}

// Camp is where a siege arriving at edge tile (ex, ey) digs in:
// siegeRange tiles out from the colony towards the raiders' edge.
func (rs *Raiders) Camp(ex, ey int) (int, int) {
	hx, hy := rs.h.Home()
	dx, dy := ex-hx, ey-hy
	d := max(1, weapon.Distance(ex, ey, hx, hy))
	if d <= siegeRange {
		return ex, ey
	}
	x, y := hx+dx*siegeRange/d, hy+dy*siegeRange/d
	if cx, cy, ok := rs.h.WalkableNear(x, y, 4); ok {
		return cx, cy
	}
	return ex, ey
}

// siege runs a siege raider (with injury hurt) before the storm: walk to
// the camp, dig in and build the mortars, crew them, then shoot whoever
// comes in range. It reports whether it acted.
func (rs *Raiders) siege(id entity.EntityID, r Raider, hurt int, now uint64) bool {
	h := rs.h
	gi := r.Group
	grp := &h.Groups()[gi]
	if grp.Plan != Siege || r.Mode == Fleeing || grp.Stormed {
		return false
	}
	if grp.Started == 0 {
		grp.Started = now
	}
	x, y, _ := h.Pos(id)
	if hurt > r.Hurt { // wiki: Raid, 8 % a hit that the siege storms at once
		r.Hurt = hurt
		h.SetRaider(id, r)
		if grp.Camped && h.Roll(100) < raiddef.SiegeStormPct {
			rs.storm(gi, x, y, now, "Stung by your fire, the besiegers are storming the colony!")
			return false
		}
	}
	if !grp.Camped {
		if weapon.Distance(x, y, grp.CampX, grp.CampY) > 3 && now-grp.Started < siegeSetup {
			if j, _ := h.Job(id); !j.Active() || (now+uint64(id))%180 == 0 {
				if tx, ty, ok := h.WalkableNear(grp.CampX+h.Roll(5)-2, grp.CampY+h.Roll(5)-2, 2); ok {
					h.StartJob(id, job.Record{Kind: job.Move, TX: tx, TY: ty})
				}
			}
			return true
		}
		rs.digIn(gi, now)
	}
	if now >= grp.Until {
		rs.storm(gi, x, y, now, "The besiegers are storming the colony!")
		return false
	}
	if rs.crew(id, grp) {
		return true // at a mortar (combat/mortar.go fires it)
	}
	// hold the camp; shoot anyone who comes in range
	if j, _ := h.Job(id); j.Kind == job.Attack && j.Active() {
		h.Strike(id, j, now)
		if j2, _ := h.Job(id); j2.State == job.Goto {
			h.Interrupt(id, job.Cancelled) // never leave the camp to chase
		}
		return true
	}
	if t, tx, ty, ok := h.NearestColonist(x, y); ok && h.CanHit(x, y, h.Weapon(id), tx, ty) {
		h.ReleaseOwner(id)
		h.SetJob(id, job.Record{Kind: job.Attack, Target: t, TX: tx, TY: ty, State: job.Do,
			WorkLeft: h.AimTicks(h.Weapon(id))})
	}
	return true
}

// digIn throws up sandbags in front of each besieger of group gi.
func (rs *Raiders) digIn(gi int, now uint64) {
	h := rs.h
	grp := &h.Groups()[gi]
	grp.Camped = true
	grp.Until = now + uint64(raiddef.SiegeAssaultMin+h.Roll(raiddef.SiegeAssaultRolls))*clock.TicksPerHour
	hx, hy := h.Home()
	for _, rid := range h.RaiderIDs() {
		if h.Raider(rid).Group != gi || !h.StandingRaider(rid) {
			continue
		}
		px, py, _ := h.Pos(rid)
		h.Build("sandbags", "wood", "south", px+cmp.Compare(hx-px, 0), py+cmp.Compare(hy-py, 0))
		h.Interrupt(rid, job.Cancelled)
	}
	if rs.sg != nil { // wiki: Raid, a siege builds two mortars
		for i := 0; i < raiddef.SiegeMortars; i++ {
			if m := rs.mortarNear(grp.CampX+3*i-1, grp.CampY); m != 0 {
				grp.Mortars = append(grp.Mortars, m)
			}
		}
	}
	text := "The raiders have dug in for a siege and are setting up mortars. They will storm the colony within a few days."
	h.Threat("Siege", grp.CampX, grp.CampY, text)
	rs.log(text)
}

// mortarNear builds a raiders' mortar on the first free tile within four
// of (x, y), nearest first (0: none).
func (rs *Raiders) mortarNear(x, y int) entity.EntityID {
	for r := 0; r <= 4; r++ {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if max(dx, -dx, dy, -dy) != r || !rs.h.InBounds(x+dx, y+dy) {
					continue
				}
				if m := rs.sg.BuildMortar(x+dx, y+dy); m != 0 {
					return m
				}
			}
		}
	}
	return 0
}

// crew keeps besieger id at a mortar of its group: on one already, or sent
// to one nobody mans. It reports whether id crews (or walks to) one.
func (rs *Raiders) crew(id entity.EntityID, grp *Group) bool {
	if rs.sg == nil || len(grp.Mortars) == 0 {
		return false
	}
	h := rs.h
	if j, _ := h.Job(id); j.Kind == job.ManMortar && j.Active() {
		return true
	}
	for _, m := range grp.Mortars {
		if c, ok := rs.sg.Crew(m); ok && c == 0 {
			return rs.sg.Man(id, m)
		}
	}
	return false
}
