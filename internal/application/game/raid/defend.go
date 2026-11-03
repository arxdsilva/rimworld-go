package raid

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Settlement defenders (MAP-05, wiki: Faction base; the tuning is
// original, the wiki publishes none). A Defend group lives in its
// faction's compound: the group's camp (CampX, CampY) is the compound's
// centre. A holding defender stays within defendLeash of it, walking back
// to its post when it has strayed. It engages a colonist (or ally) that
// comes within SightRange, or one that hurts it, and then fights as any
// raider does; it lets go only once no target is left within
// defendRelease (twice the sight range), so it never flaps between
// holding and fighting at the edge of its sight. Defenders never give up
// or leave idle (raiders.go givenUp); once the group has lost
// DefendFleeLoss percent it flees off the map through the raiders' own
// retreat (Group.FleeLoss, broken).
//
// MAP-17: the compound's doors are shut and its people walk through them.
// SightRange never needed a clear line, so a colonist outside the wall still
// draws them out; a blow on any piece of the compound does too (Alarm), from
// anywhere. Once engaged a defender stays so at least defendHold while it has
// anyone to go for, and every blow or wound starts that over.

const (
	// DefendFleeLoss is the share of a settlement's defenders (percent)
	// whose loss breaks the rest (original tuning).
	DefendFleeLoss = 50
	// defendLeash is how far from the compound's centre a holding
	// defender may stand before it walks back to its post (original
	// tuning).
	defendLeash = 8
	// defendRelease is the distance past which a fighting defender lets go
	// of its target and returns to its post (hysteresis: it engaged at
	// SightRange).
	defendRelease = 2 * SightRange
	// defendPost spreads the posts around the centre (cells either way).
	defendPost = 3
	// defendHold is the least a defender stays engaged (original tuning): a
	// shot from far off, or a colonist in and out of range, can't flap it
	// through its door.
	defendHold = clock.TicksPerHour
)

// Alarm: a piece of a compound was hit at now, so its defenders turn out
// (the game's building damage). No group defends on a colony's map.
func (rs *Raiders) Alarm(now uint64) {
	groups := rs.h.Groups()
	for i := range groups {
		if groups[i].Plan == Defend {
			groups[i].Alarm = now
		}
	}
}

// Defending counts the defenders still on their feet and not fleeing.
func (rs *Raiders) Defending() int {
	h, groups, n := rs.h, rs.h.Groups(), 0
	for _, id := range h.RaiderIDs() {
		if r := h.Raider(id); r.Group < len(groups) && groups[r.Group].Plan == Defend && r.Mode != Fleeing && h.StandingRaider(id) {
			n++
		}
	}
	return n
}

// defend runs a settlement defender with injury hurt standing on (x, y).
// It reports whether the defender held (true: nothing more to do this
// tick); false hands it to the fight, with r as it now stands.
func (rs *Raiders) defend(id entity.EntityID, r *Raider, j job.Record, hurt, x, y int, now uint64) bool {
	h := rs.h
	grp := &h.Groups()[r.Group]
	if grp.Plan != Defend {
		return false
	}
	shot, alarm := hurt > r.Hurt, grp.Alarm > r.Engaged
	if !shot && !alarm && (now+uint64(id))%stageEvery != 0 {
		return r.Mode != Assaulting // between looks: as it was
	}
	d, foe := 1<<20, false
	if _, tx, ty, ok := h.RaidTarget(x, y); ok {
		d, foe = weapon.Distance(x, y, tx, ty), true
	}
	switch {
	case r.Mode == Assaulting && (d <= defendRelease || shot || alarm || foe && now < r.Engaged+defendHold):
		if shot || alarm { // its hold starts over
			r.Hurt, r.Engaged = hurt, now
			h.SetRaider(id, *r)
		}
		return false // still a fight in reach
	case r.Mode == Assaulting:
		r.Mode, r.Idle, r.Hurt = Arriving, 0, hurt // back to its post
		h.SetRaider(id, *r)
		h.Interrupt(id, job.Cancelled)
		j = job.Record{}
	case d <= SightRange || shot || alarm:
		r.Mode, r.Hurt, r.Idle, r.Engaged = Assaulting, hurt, 0, now
		h.SetRaider(id, *r)
		return false
	}
	if !j.Active() && weapon.Distance(x, y, grp.CampX, grp.CampY) > defendLeash {
		px := grp.CampX + int(uint64(id)%(2*defendPost+1)) - defendPost
		py := grp.CampY + int(uint64(id)/3%(2*defendPost+1)) - defendPost
		if tx, ty, ok := h.WalkableNear(px, py, defendPost); ok {
			h.StartJob(id, job.Record{Kind: job.Move, TX: tx, TY: ty})
		}
	}
	return true
}
