package combat

import "rimworld-go/internal/domain/sim/entity"

// Threat response (CBT-4). wiki: Threat response (the Assign tab's
// hostility response): Flee, Attack or Ignore, per colonist. A colonist
// set to Attack meets hostiles in reach with its weapon (melee within
// EngageMelee, ranged within 70 % of its range up to EngageRanged); one
// set to Flee runs from any hostile coming near (FleeReach, RaiderReach),
// not only from one attacking it; one set to Ignore carries on. A colonist
// incapable of violence never attacks (wiki), nor does an unarmed one
// here (original: it has nothing to fight with). The default, before the
// player picks, is Attack for an armed colonist able to fight and Flee for
// everyone else.

// Response is a colonist's threat response.
type Response uint8

const (
	RespondDefault Response = iota // Attack when armed and able, else Flee
	RespondFlee
	RespondAttack
	RespondIgnore
)

const (
	// EngageMelee and EngageRanged: a colonist set to Attack goes for a
	// hostile this close (wiki: Threat response, 8 for melee, 70 % of
	// the weapon's range up to about 20 for guns).
	EngageMelee  = 8
	EngageRanged = 20
	// engagePct is the share of a gun's range it engages within (wiki).
	engagePct = 70
	// foeEvery paces the scan for hostiles that are not attacking the
	// colonist (staggered by id; every tick while it already flees).
	foeEvery = 10
)

// String names r for the read model.
func (r Response) String() string {
	switch r {
	case RespondFlee:
		return "flee"
	case RespondAttack:
		return "attack"
	case RespondIgnore:
		return "ignore"
	}
	return "default"
}

// Response is colonist id's threat response as the player set it.
func (d *Drafts) Response(id entity.EntityID) Response { return d.set[id].Response }

// SetResponse sets colonist id's threat response (the game routes the
// command here; an unknown value is ignored).
func (d *Drafts) SetResponse(id entity.EntityID, r Response) {
	if r > RespondIgnore {
		return
	}
	s := d.set[id]
	s.Response = r
	d.put(id, s)
}

// Fights reports that colonist id, armed and able to fight (able), meets
// threats with its weapon: its response is Attack, or left at the
// default.
func (d *Drafts) Fights(id entity.EntityID, able bool) bool {
	r := d.set[id].Response
	return able && (r == RespondDefault || r == RespondAttack)
}

// EngageReach is how far a colonist set to Attack goes for a hostile
// with weapon range rng (tiles; 1 or less: melee).
func EngageReach(rng int) int {
	if rng <= 1 {
		return EngageMelee
	}
	return min(rng*engagePct/100, EngageRanged)
}
