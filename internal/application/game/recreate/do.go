package recreate

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/quality"
	"rimworld-go/internal/domain/pawn/recreation"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// XPEvery: a skill-training piece teaches 1 experience point every
// XPEvery ticks (wiki: Telescope, 0.004 a tick; the other games' rate is
// not published, so they take the telescope's, original tuning).
const XPEvery = 250

// stepGain is the recreation (milli-points) one recreation.GainPeriod at
// a piece of power permille gives before tolerance: recreation.HourGain
// an hour at power 1.0.
func stepGain(power int) int {
	return recreation.HourGain * recreation.GainPeriod * power / clock.TicksPerHour
}

// PowerOf is piece b's recreation power now (permille): its kind's,
// scaled by its quality where quality counts.
func PowerOf(b construction.Building) int {
	sp, ok := SpecOf(b.Kind)
	if !ok {
		return 0
	}
	if l, has := quality.FromStored(b.Quality); has && sp.Quality {
		return quality.Apply(sp.Power, l, quality.Recreation)
	}
	return sp.Power
}

// Do runs one tick of colonist id's session j at its piece (j.Dest): the
// recreation and skill it gains, until its time is up or its recreation
// full. A piece gone, unpowered, or a sky grown light cuts the session
// short, and the colonist then seeks no piece for RetryTicks.
func (s *State) Do(id entity.EntityID, j job.Record) {
	h := s.h
	now := h.Now()
	j.WorkLeft--
	b, ok := h.Building(j.Dest)
	sp, rec := SpecOf(b.Kind)
	switch {
	case !ok || !rec || !s.usable(id, j.Dest, b, sp, true):
		j.WorkLeft = 0
		p := s.s.Pawns[id]
		p.Retry = now + RetryTicks
		s.s.Pawns[id] = p
		j.State = job.Complete
		h.SetJob(id, j)
		return
	case h.Joy(id) >= recreation.Full:
		j.WorkLeft = 0 // entertained: nothing more to gain
	case now%recreation.GainPeriod == 0:
		s.Gain(id, sp.Joy, stepGain(PowerOf(b)))
	}
	if sp.Learns && now%XPEvery == 0 {
		h.Learn(id, sp.Trains, 1)
	}
	if j.WorkLeft <= 0 {
		j.State = job.Complete
		h.Enjoyed(id)
	}
	h.SetJob(id, j)
}
