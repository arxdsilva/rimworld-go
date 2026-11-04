package recreate

import (
	"slices"

	"rimworld-go/internal/domain/pawn/recreation"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Pawn is one colonist's recreation record (saved as is).
type Pawn struct {
	Tol recreation.Tolerance
	// Carry is recreation won but not yet whole (milli-points).
	Carry int32
	// Retry: no recreation piece is sought before this tick (a session
	// cut short waits RetryTicks, so a flickering screen or a dawn
	// never has a colonist bounce between it and something else).
	Retry uint64
}

// Save is State as saved (saveState.Recreation; nil in older saves).
type Save struct {
	Pawns map[entity.EntityID]Pawn
	// Rate is the tolerance worn off a day (percent), read from the
	// colony's wealth on day RateDay.
	Rate    int
	RateDay uint64
	// Craving are the colonists craving the outdoors, sorted (CSE-001).
	Craving []entity.EntityID
}

// State is the recreation component: per-colonist tolerances and its
// host.
type State struct {
	h Host
	s Save
}

// New is an empty recreation state on host h.
func New(h Host) *State { return &State{h: h, s: Save{Pawns: map[entity.EntityID]Pawn{}}} }

// Load is the recreation state saved as in (nil: an empty one) on h.
func Load(h Host, in *Save) *State {
	st := New(h)
	if in != nil {
		st.s.Rate, st.s.RateDay = in.Rate, in.RateDay
		st.s.Craving = slices.Clone(in.Craving)
		for id, p := range in.Pawns {
			st.s.Pawns[id] = p
		}
	}
	return st
}

// Export is the state to save (nil when there is nothing to keep).
func (s *State) Export() *Save {
	if s == nil || len(s.s.Pawns) == 0 && len(s.s.Craving) == 0 {
		return nil
	}
	out := Save{Pawns: make(map[entity.EntityID]Pawn, len(s.s.Pawns)), Rate: s.s.Rate, RateDay: s.s.RateDay, Craving: slices.Clone(s.s.Craving)}
	for id, p := range s.s.Pawns {
		out.Pawns[id] = p
	}
	return &out
}

// rate is the tolerance decay (percent a day), refreshed from the
// colony's wealth once a day.
func (s *State) rate(now uint64) int {
	if day := now / clock.TicksPerDay; s.s.Rate == 0 || day != s.s.RateDay {
		s.s.Rate, s.s.RateDay = recreation.DecayPct(s.h.Wealth()), day
	}
	return s.s.Rate
}

// pawn is id's record with its tolerances worn off to now (not stored).
func (s *State) pawn(id entity.EntityID, now uint64) Pawn {
	p := s.s.Pawns[id]
	if p.Tol.At == 0 {
		p.Tol.At = now
	}
	p.Tol.Decay(now, s.rate(now), clock.TicksPerDay)
	return p
}

// Tolerance is id's tolerances now, for the views: worn off at the last
// known rate, nothing stored.
func (s *State) Tolerance(id entity.EntityID) recreation.Tolerance {
	p, ok := s.s.Pawns[id]
	if !ok {
		return recreation.Tolerance{}
	}
	if s.s.Rate > 0 {
		p.Tol.Decay(s.h.Now(), s.s.Rate, clock.TicksPerDay)
	}
	return p.Tol
}

// Gain gives id a step of gain milli-points of joy kind k through its
// tolerance (also for other packages' recreation: reading) and returns
// the whole points granted.
func (s *State) Gain(id entity.EntityID, k recreation.JoyKind, gain int) int {
	now := s.h.Now()
	p := s.pawn(id, now)
	got := int(p.Carry) + p.Tol.Take(k, gain)
	whole := got / 1000
	p.Carry = int32(got % 1000)
	s.s.Pawns[id] = p
	if whole > 0 {
		s.h.AddJoy(id, whole)
	}
	return whole
}

// Forget drops id's record (it left the colony or died).
func (s *State) Forget(id entity.EntityID) {
	delete(s.s.Pawns, id)
	if i, on := slices.BinarySearch(s.s.Craving, id); on {
		s.s.Craving = slices.Delete(s.s.Craving, i, i+1)
	}
}
