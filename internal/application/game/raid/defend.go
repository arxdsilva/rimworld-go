package raid

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/combat/weapon"
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
)

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
	shot := hurt > r.Hurt
	if !shot && (now+uint64(id))%stageEvery != 0 {
		return r.Mode != Assaulting // between looks: as it was
	}
	d := 1 << 20
	if _, tx, ty, ok := h.RaidTarget(x, y); ok {
		d = weapon.Distance(x, y, tx, ty)
	}
	switch {
	case r.Mode == Assaulting && d <= defendRelease:
		return false // still a fight in reach
	case r.Mode == Assaulting:
		r.Mode, r.Idle = Arriving, 0 // back to its post
		h.SetRaider(id, *r)
		h.Interrupt(id, job.Cancelled)
		j = job.Record{}
	case d <= SightRange || shot:
		r.Mode, r.Hurt, r.Idle = Assaulting, hurt, 0
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
