package rest

import (
	"rimworld-go/internal/application/game/care"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/entity"
)

// Sleep is the root of the rest package: where and when a pawn sleeps.
// It is named Sleep, not Rest, so it never reads as care.Rest (bed rest
// for patients, which stays in the care package). Its components are
// exported fields; a feature's entry points go on its component.
type Sleep struct {
	h Host
	// Beds chooses a sleeper's bed and handles the owner command
	// (beds.go).
	Beds *Beds
	// Waking gives a pawn its thoughts on how and where it slept
	// (waking.go).
	Waking *Waking
	// Stats is beds' comfort, rest effectiveness and beauty (stats.go).
	Stats *Stats
	// Animals beds tame animals down in animal beds (animals.go).
	Animals *Animals
	// Lovin is couples' lovin' in a shared double bed (lovin.go).
	Lovin *Lovin
	// Needs is the comfort, beauty and outdoors needs (needs.go, BED-3).
	Needs *Needs
	// Seats is where pawns sit: at a table, to rest (seats.go, BED-3).
	Seats *Seats
	// rested: woke fully rested in the current Sleep block, so it has
	// free time until it tires (saved as RestedFree).
	rested map[entity.EntityID]bool
}

// New builds a Sleep over h.
func New(h Host) *Sleep {
	st := &Stats{h: h}
	nd := &Needs{h: h, stats: st, m: make(map[entity.EntityID]NeedRecord)}
	nd.seats = &Seats{h: h, n: nd}
	return &Sleep{h: h, Beds: &Beds{h: h}, Waking: &Waking{h: h}, Stats: st, Animals: &Animals{h: h, stats: st}, Lovin: &Lovin{h: h},
		Needs: nd, Seats: nd.seats, rested: make(map[entity.EntityID]bool)}
}

// Assign sends pawn id to bed (Beds.SleepIn), or to sleep on the floor
// when no bed can be had or it is not alive and well.
func (s *Sleep) Assign(id entity.EntityID) {
	if v, ok := s.h.Vital(id); !ok || v.Status != health.Alive || !s.Beds.SleepIn(id) {
		s.InPlace(id)
	}
}

// InPlace lays pawn id down on the floor: where it stands, or the nearest
// cell inside its allowed area.
func (s *Sleep) InPlace(id entity.EntityID) {
	x, y := s.h.Pos(id)
	s.h.SetSleptAt(id, 0)
	fx, fy := s.h.FloorSpot(id, x, y)
	s.h.StartJob(id, job.Record{Kind: job.Sleep, TX: fx, TY: fy, WorkLeft: needs.SleepWorkMin})
}

// Do runs one tick of sleeper id's sleep job j: rest comes back at the
// bed's rest effectiveness (the ground's with no bed, BED-1), and when it
// gets up the job completes, its log notes where it slept and it wakes
// with that night's thoughts.
func (s *Sleep) Do(id entity.EntityID, j job.Record, now uint64) {
	n := needs.SleepTickAt(s.h.Needs(id), now, s.Stats.RestEffect(j.Target))
	s.h.SetNeeds(id, n)
	s.Needs.Tick(id, now) // comfort in bed, outdoors (BED-3)
	j.WorkLeft--
	if s.done(id, n, j) {
		j.State = job.Complete
		bed, _ := s.h.SleptAt(id)
		s.h.Slept(id, bed != 0)
		s.Waking.After(id)
	}
	s.h.SetJob(id, j)
}

// done reports whether a sleeper gets up now. A collapsed colonist sleeps
// to WakeRest whatever happens; a healthy one in a medical bed gets up at
// once; otherwise it wakes to eat, stays while on bed rest, and wakes at
// full rest in Sleep hours (the rest of the block is free time), with the
// start of Work hours once over the tired line, and at WakeRest in any
// other block. These gaps keep it from bouncing between bed and work.
func (s *Sleep) done(id entity.EntityID, n needs.Pair, j job.Record) bool {
	if v, ok := s.h.Vital(id); ok && v.Status == health.Downed {
		if v.Cause != health.CauseNoBed || n.Rest < needs.WakeRest {
			return false
		}
		v.Status, v.Cause = health.Alive, health.CauseNone
		s.h.SetVital(id, v)
	}
	if j.Target != 0 {
		if b, ok := s.h.Building(j.Target); ok && care.IsMedicalBed(b) && !s.h.MayUseMedicalBed(id) {
			return true // well again: medical beds are for patients (care/rest.go)
		}
	}
	if n.Hungry() && s.h.SeekFood(id) {
		return true
	}
	if s.h.OnBedRest(id) {
		// care/rest.go: stays put until well; one only waiting for a
		// surgeon who never came gets up after a while
		s.h.SurgeryWait(id, j.WorkLeft)
		return false
	}
	act := s.h.Activity(id)
	var done bool
	switch {
	case needs.SleepScheduled(act):
		done = n.Rest >= needs.Full
		if done {
			s.rested[id] = true
		}
	case needs.SleepAllowed(act):
		done = n.Rest >= needs.WakeRest
	default: // Work hours
		done = n.Rest > needs.CriticalRest && j.WorkLeft <= 0
	}
	return done
}

// RestedFree: id woke fully rested in the current Sleep block (free time
// until it tires).
func (s *Sleep) RestedFree(id entity.EntityID) bool { return s.rested[id] }

// MarkRested marks id as rested for the rest of this Sleep block.
func (s *Sleep) MarkRested(id entity.EntityID) { s.rested[id] = true }

// Unmark ends id's free time (the Sleep block ended, or it died or left).
func (s *Sleep) Unmark(id entity.EntityID) { delete(s.rested, id) }

// Export is the rested marks, for the save to copy (the live map).
func (s *Sleep) Export() map[entity.EntityID]bool { return s.rested }

// Import restores Export's marks; nil (an older save) keeps the current
// ones.
func (s *Sleep) Import(m map[entity.EntityID]bool) {
	if m != nil {
		s.rested = m
	}
}
