package raid

import (
	"fmt"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Kidnapping (ENG-003, M9b; moved from the game's kidnap.go by CBT-4). A
// raider running for the edge, or standing about with no one left to
// fight, that finds a downed colonist within KidnapRange which no standing
// colonist guards, picks it up (job.Kidnap, carried as a rescue does) and
// carries it to the nearest map edge. There both leave (Kidnap.Off: the
// colonist is stashed for the raider's faction, its relations grieve, a
// letter says who took it). Down or kill the kidnapper on the way and the
// colonist is dropped where it fell.
//
// No flap: a raider commits to the colonist it picked until the kidnap
// ends, and one that fails (the colonist got up, someone else took it,
// no way out) waits KidnapCooldown before it looks again.

const (
	// KidnapRange: how far (tiles) a raider looks for a downed colonist.
	KidnapRange = 10
	// KidnapScan: a raider looks once in this many ticks (staggered by id).
	KidnapScan = 60
	// KidnapCooldown: after a failed kidnap the raider waits this long.
	KidnapCooldown = clock.TicksPerHour
	// kidnapEdgeR: how far from its edge point the way out may end.
	kidnapEdgeR = 3
)

// Kidnap is what kidnappers need from the game (value only).
type Kidnap interface {
	// Colonists is the host's colonist ids, read only.
	Colonists() []entity.EntityID
	// Carrier is who carries t (0: nobody) and whether t has left the
	// map (stashed); Carrying is whom id carries (0: nobody); Pick lifts
	// t onto id.
	Carrier(t entity.EntityID) (by entity.EntityID, gone bool)
	Carrying(id entity.EntityID) entity.EntityID
	Pick(id, t entity.EntityID)
	// Claimed: someone other than id holds a reservation on t.
	Claimed(t, id entity.EntityID) bool
	// Beside is a free tile beside (x, y) for id; Path id's path there.
	Beside(id entity.EntityID, x, y int) (int, int, bool)
	Path(id entity.EntityID, x, y int) ([]pathing.Point, bool)
	// Off: raider id reached the edge carrying t; both leave the map.
	Off(id, t entity.EntityID)
}

// Kidnaps runs kidnappers.
type Kidnaps struct {
	h Host
	k Kidnap
}

// Try lets standing raider id at (x, y) look for a colonist to carry
// off; it reports whether the raider's turn went to it (a kidnap began, or
// one could not start this tick).
func (kn *Kidnaps) Try(id entity.EntityID, x, y int, now uint64) bool {
	h := kn.h
	if kn.k == nil || now < h.Raider(id).KidnapAt || (now+uint64(id))%KidnapScan != 0 || kn.k.Carrying(id) != 0 {
		return false
	}
	t, tx, ty, ok := kn.target(id, x, y)
	if !ok {
		return false
	}
	sx, sy, ok := x, y, true // already beside it
	if max(abs(x-tx), abs(y-ty)) > 1 {
		sx, sy, ok = kn.k.Beside(id, tx, ty)
	}
	if ok && h.StartJob(id, job.Record{Kind: job.Kidnap, Target: t, TX: sx, TY: sy}) {
		return true
	}
	kn.End(id, now)
	return true
}

// target is the nearest downed colonist raider id at (x, y) may carry
// off: in range, not carried, held or tended for by anyone else, and with
// no standing colonist beside it.
func (kn *Kidnaps) target(id entity.EntityID, x, y int) (entity.EntityID, int, int, bool) {
	h := kn.h
	var best entity.EntityID
	var bx, by int
	bestD := KidnapRange + 1
	for _, c := range kn.k.Colonists() {
		if h.Vital(c).Status != health.Downed {
			continue
		}
		cx, cy, ok := h.Pos(c)
		if !ok {
			continue
		}
		d := weapon.Distance(x, y, cx, cy)
		if carrier, _ := kn.k.Carrier(c); d >= bestD || carrier != 0 || kn.guarded(c, cx, cy) || kn.k.Claimed(c, id) {
			continue
		}
		best, bx, by, bestD = c, cx, cy, d
	}
	return best, bx, by, best != 0
}

// guarded reports a standing colonist beside downed colonist c at (x, y).
func (kn *Kidnaps) guarded(c entity.EntityID, x, y int) bool {
	for _, o := range kn.k.Colonists() {
		if o == c || kn.h.Vital(o).Status != health.Alive {
			continue
		}
		if ox, oy, ok := kn.h.Pos(o); ok && abs(ox-x) <= 1 && abs(oy-y) <= 1 {
			return true
		}
	}
	return false
}

// Step runs raider id's kidnap job: walk to the colonist, pick it up and
// head for the edge, and at the edge leave with it.
func (kn *Kidnaps) Step(id entity.EntityID, j job.Record, now uint64) {
	t := j.Target
	if j.State != job.Goto && j.State != job.Do {
		kn.End(id, now) // the walk failed (blocked, no way)
		return
	}
	if kn.h.Vital(t).Status != health.Downed {
		kn.End(id, now) // it got up, or died
		return
	}
	if kn.k.Carrying(id) != t {
		if c, gone := kn.k.Carrier(t); c != 0 && c != id || gone {
			kn.End(id, now) // someone else has it
			return
		}
		if j.State != job.Do {
			return // still walking over (the game's movement)
		}
		kn.pickUp(id, t, j, now)
		return
	}
	if j.State == job.Do {
		kn.k.Off(id, t)
	}
}

// pickUp: raider id beside colonist t lifts it and heads for the nearest
// edge.
func (kn *Kidnaps) pickUp(id, t entity.EntityID, j job.Record, now uint64) {
	h := kn.h
	x, y, _ := h.Pos(id)
	tx, ty, _ := h.Pos(t)
	if max(abs(x-tx), abs(y-ty)) > 1 {
		kn.End(id, now)
		return
	}
	ex, ey := h.NearestEdge(x, y)
	gx, gy, ok := h.WalkableNear(ex, ey, kidnapEdgeR)
	if !ok {
		kn.End(id, now)
		return
	}
	path, ok := kn.k.Path(id, gx, gy)
	if !ok {
		kn.End(id, now)
		return
	}
	h.DropGear(t) // the colonist's weapon stays where it fell
	kn.k.Pick(id, t)
	j.TX, j.TY, j.Path, j.PathI, j.State = gx, gy, path, 0, job.Goto
	if len(path) == 0 {
		j.State = job.Do
	}
	h.SetJob(id, j)
	h.Message(x, y, fmt.Sprintf("%s is carrying %s away!", h.Name(id), h.Name(t)))
}

// End ends raider id's kidnap (put down whoever it carries) and starts
// its cooldown.
func (kn *Kidnaps) End(id entity.EntityID, now uint64) {
	h := kn.h
	h.DropPatient(id)
	h.ReleaseOwner(id)
	h.SetJob(id, job.Record{})
	r := h.Raider(id)
	r.KidnapAt = now + KidnapCooldown
	h.SetRaider(id, r)
}
