package visit

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/entity"
)

// Visitors defend themselves (EAI-011, original tuning). A neutral
// visitor or a trader's guard takes on a raider or a hostile animal it can
// see within DefendSight, or within DefendKeep when it was just hurt; the
// trader itself fights only when hurt. Once in it, it holds the fight for
// at least DefendHold and goes on taking foes within DefendKeep; it lets
// go only when none is that near and the hold is over. Starting at one
// range and stopping at a wider one, with a minimum hold, keeps a visitor
// from flapping between fighting and strolling. The checks run every
// defendScan ticks, staggered by id; a fight in progress strikes every
// tick. Allies (aid.go) and turned visitors (turn.go) fight their own way.

const (
	// DefendSight is how near a foe must be for a visitor to start a fight.
	DefendSight = 10
	// DefendKeep is how near one must be for it to keep fighting (and how
	// far a hurt visitor looks for whoever hurt it).
	DefendKeep = 16
	// DefendHold is the least a defence lasts once it starts.
	DefendHold = 600
	// defendScan is how often (staggered by id) a visitor looks.
	defendScan = 30
)

// JoinedFight is the message when a group's first visitor joins a fight
// (a format: the faction's name).
const JoinedFight = "Visitors from %s are joining the fight against the colony's attackers."

// Defending is what a visitor's defence needs from the simulation (the
// Host has it all).
type Defending interface {
	Pos(id entity.EntityID) (x, y int)
	SetJob(id entity.EntityID, rec job.Record)
	Release(id entity.EntityID)
	Strike(id entity.EntityID, j job.Record, now uint64)
	Engage(id entity.EntityID, x, y, r int, now uint64) bool
	// HitAt is the tick id was last hit (0: never).
	HitAt(id entity.EntityID) uint64
	// Trader reports id is a trader caravan's trader (its guards are not).
	Trader(id entity.EntityID) bool
}

// defend runs visitor r's defence at (x, y) with job j: fight reports it
// is fighting this tick (the stroll and the walk out wait), joined that it
// has just started.
func defend(h Defending, r *Rec, x, y int, j job.Record, now uint64) (fight, joined bool) {
	attacking := j.Kind == job.Attack && j.Active()
	if (now+uint64(r.ID))%defendScan != 0 {
		if attacking {
			h.Strike(r.ID, j, now)
		}
		return attacking, false
	}
	if r.Defend != 0 {
		if attacking {
			tx, ty := h.Pos(j.Target)
			if now < r.Defend || within(x, y, tx, ty, DefendKeep) {
				h.Strike(r.ID, j, now)
				return true, false
			}
			h.Release(r.ID) // the foe got away, the hold is over: let it go
			h.SetJob(r.ID, job.Record{})
		}
		if h.Engage(r.ID, x, y, DefendKeep, now) {
			return true, false
		}
		if now >= r.Defend {
			r.Defend = 0 // nobody near, the hold over: back to the visit
		}
		return false, false
	}
	if attacking { // a fight it was given elsewhere
		h.Strike(r.ID, j, now)
		return true, false
	}
	hit := h.HitAt(r.ID)
	hurt := hit != 0 && hit <= now && now-hit < defendScan
	if !hurt && h.Trader(r.ID) {
		return false, false // the trader minds the trade
	}
	reach := DefendSight
	if hurt {
		reach = DefendKeep
	}
	if !h.Engage(r.ID, x, y, reach, now) {
		return false, false
	}
	r.Defend = now + DefendHold
	return true, true
}

// within reports (bx, by) within r tiles of (ax, ay) (Chebyshev, as the
// creature index's rings).
func within(ax, ay, bx, by, r int) bool {
	return max(ax-bx, bx-ax, ay-by, by-ay) <= r
}

// joinMessage posts JoinedFight for r's group unless another of it is
// already fighting.
func (v Visits) joinMessage(r *Rec, x, y int) {
	for _, o := range v.recs() {
		if o.ID != r.ID && o.Faction == r.Faction && o.Defend != 0 {
			return
		}
	}
	name := "the world"
	if f := ofRef(v.h.Factions(), r.Faction); f != nil {
		name = f.Name
	}
	v.h.Message(x, y, JoinedFight, name)
}
