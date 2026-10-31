// Package herd is animal husbandry (FOOD-3): taming with food, milking and
// shearing, slaughter, auto-slaughter, feeding downed tame animals and a
// hunter's kill carried to storage.
//
// It is a consumer sub-package of internal/application/game
// (.cursor/SOLID_GO.md §10): it never imports the game package; the roles
// in deps.go are its only way in. It draws dice only from the host's
// stream, reads time only from the host's tick, and walks animals in id
// order.
package herd

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/sim/entity"
)

// Herd is the root, built by New: components as fields; a feature's
// entry points go on its component.
type Herd struct {
	h         Host
	Taming    *Taming
	Collect   *Collect
	Slaughter *Slaughter
	Auto      *Auto
	Feed      *Feed
	// kills are the hunters' kills still to carry (hunter -> corpse),
	// careful the animal being slaughtered right now; never saved.
	kills   map[entity.EntityID]entity.EntityID
	careful entity.EntityID
}

// New builds a herd on host h with the saved settings s (kept by the
// game: the herd writes them in place).
func New(h Host, s *Settings) *Herd {
	hd := &Herd{h: h}
	hd.Taming = &Taming{h: h}
	hd.Collect = &Collect{h: h}
	hd.Slaughter = &Slaughter{hd: hd}
	hd.Auto = &Auto{h: h, s: s}
	hd.Feed = &Feed{h: h}
	return hd
}

// Do runs one tick of a herd job and reports whether j was one: taming,
// milking, shearing, slaughter, or a feeding whose patient (Target) is
// an animal (a care feeding's Target is its food).
func (hd *Herd) Do(id entity.EntityID, j job.Record) bool {
	switch j.Kind {
	case job.Tame:
		hd.Taming.Do(id, j)
	case job.Milk, job.Shear:
		hd.Collect.Do(id, j)
	case job.Slaughter:
		hd.Slaughter.Do(id, j)
	case job.FeedPatient:
		if _, ok := hd.h.Animal(j.Target); !ok {
			return false
		}
		hd.Feed.Do(id, j)
	default:
		return false
	}
	return true
}

// Assign is the Handle work giver after roping: feed a downed animal,
// tame, milk or shear, slaughter (the game's training sits between taming
// and collecting: AssignTame and AssignRest split around it).
func (hd *Herd) AssignTame(id entity.EntityID) bool {
	return hd.Feed.Assign(id) || hd.Taming.Assign(id)
}

// AssignRest is the givers after training: collect, then slaughter.
func (hd *Herd) AssignRest(id entity.EntityID) bool {
	return hd.Collect.Assign(id) || hd.Slaughter.Assign(id)
}

// Holds reports a herd job of id's that keeps animal aid still (the
// game's beingTamed): a slaughter, or a feeding of it.
func Holds(j job.Record, aid entity.EntityID) bool {
	return (j.Kind == job.Slaughter || j.Kind == job.FeedPatient) && j.Target == aid
}

// Killed records a colonist's kill: by's hunt of prey ended with its body
// in stack corpse, which by carries to storage next (CarryKill).
func (hd *Herd) Killed(by, prey, corpse entity.EntityID) {
	j, ok := hd.h.Job(by)
	if !ok || j.Kind != job.Hunt || j.Target != prey || corpse == 0 {
		return
	}
	if hd.kills == nil {
		hd.kills = make(map[entity.EntityID]entity.EntityID)
	}
	hd.kills[by] = corpse
}

// CarryKill starts hunter id carrying its last kill to storage (wiki:
// Hunt: the hunter hauls the corpse to a stockpile with room); false when
// it has none, the body is gone or held, or there is no room.
func (hd *Herd) CarryKill(id entity.EntityID) bool {
	sid, ok := hd.kills[id]
	if !ok {
		return false
	}
	delete(hd.kills, id)
	st, ok := hd.h.Stack(sid)
	if !ok || st.Carrier != 0 {
		return false
	}
	return hd.h.HaulToStorage(id, sid)
}

// Careful reports aid dying by a slaughter right now: its body yields in
// full (animal.CorpseYieldOf).
func (hd *Herd) Careful(aid entity.EntityID) bool { return aid != 0 && hd.careful == aid }

// grown reports a grown animal at tick now.
func grown(a animal.State, now uint64) bool {
	return animal.Adult(a.Species, animal.AgeDays(a.Born, now, a.Species))
}

// claimed reports an animal someone holds.
func claimed(h Host, aid entity.EntityID) bool {
	_, taken := h.Owner(aid)
	return taken
}

// fail ends job j of id with reason r.
func fail(h Host, id entity.EntityID, j job.Record, r job.FailReason) {
	j.State, j.Fail = job.Fail, r
	h.SetJob(id, j)
}

// done ends job j of id.
func done(h Host, id entity.EntityID, j job.Record) {
	j.State = job.Complete
	h.SetJob(id, j)
}

// adjacent reports id standing beside (or on) aid.
func adjacent(h Host, id, aid entity.EntityID) bool {
	x, y, _ := h.Pos(id)
	ax, ay, ok := h.Pos(aid)
	return ok && abs(x-ax) <= 1 && abs(y-ay) <= 1
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// walkBeside paths id beside animal aid for job j and stores it; it
// reports whether id is there already (the job goes on this tick). With
// no way there the job fails.
func walkBeside(h Host, id, aid entity.EntityID, j job.Record) (job.Record, bool) {
	ax, ay, _ := h.Pos(aid)
	sx, sy, ok := h.BesideSpot(id, ax, ay)
	if ok {
		j.Path, ok = h.PathTo(id, sx, sy)
	}
	if !ok {
		fail(h, id, j, job.Unreachable)
		return j, false
	}
	j.TX, j.TY, j.PathI = sx, sy, 0
	j.State = job.Goto
	if len(j.Path) == 0 {
		j.State = job.Do
	}
	h.SetJob(id, j)
	return j, j.State == job.Do
}

// fetch is the first leg of a job that brings food to animal aid (the
// job's Target; its food is j.Dest): at the food id picks up qty and
// heads beside the animal. It reports
// the job and whether id stands beside the animal with the food now.
func fetch(h Host, id, aid entity.EntityID, j job.Record, qty int) (job.Record, bool) {
	if j.Carry == 0 {
		carry, ok := h.Take(id, j.Dest, qty)
		if !ok {
			fail(h, id, j, job.TargetGone)
			return j, false
		}
		j.Carry = carry
		var there bool
		if j, there = walkBeside(h, id, aid, j); !there {
			return j, false
		}
	}
	if !adjacent(h, id, aid) {
		if j.Count++; j.Count > chaseTries {
			fail(h, id, j, job.TargetGone)
			return j, false
		}
		j, _ = walkBeside(h, id, aid, j)
		return j, false
	}
	return j, true
}

// chaseTries is how often a handler follows an animal that moved.
const chaseTries = 4
