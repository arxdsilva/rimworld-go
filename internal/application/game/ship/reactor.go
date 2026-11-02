package ship

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Countdown raids. The wiki says only that a waking reactor draws
// constant raids, people and mechanoids (wiki: Ship, Ship reactor); the
// spacing is original tuning: the first half a day after the start, then
// one every RaidGap plus up to RaidJitter (about ten over the fifteen
// days), people and machines by turns, each worth the storyteller's
// threat points.
const (
	FirstRaid  = clock.TicksPerDay / 2
	RaidGap    = clock.TicksPerDay
	RaidJitter = clock.TicksPerDay / 2
	raidSalt   = 0x5A1F_7E4C
)

// Start starts reactor id's countdown; the reason it can't ("": started).
// A reactor needs no one to start it and may wake before the rest of the
// ship is built (wiki: Ship reactor).
func (s *State) Start(h Host, id entity.EntityID) string {
	b, ok := h.Building(id)
	switch {
	case !ok || b.Kind != construction.ShipReactor:
		return "not a ship reactor"
	case s.launched:
		return "the ship has already left"
	case s.reactor(id) != nil:
		return "already started"
	}
	now := h.Now()
	s.reactors = append(s.reactors, Reactor{ID: id, X: b.X, Y: b.Y, Start: now})
	sortReactors(s.reactors)
	if s.nextRaid == 0 {
		s.nextRaid = now + FirstRaid
	}
	h.Letter("Ship reactor waking",
		"The reactor hums awake. In "+Duration(CountdownTicks)+" it will be ready to lift the ship, and until then its call carries far: "+
			"raiders and machines will come for it again and again. Defend it, for if it falls the waking is lost.", b.X, b.Y, true)
	h.Changed()
	return ""
}

// Tick runs every few ticks while Busy: sleepers of a fallen casket
// wake, started reactors that are gone lose their waking, an awake one is announced once, and a countdown
// raid comes when due while any reactor still wakes.
func (s *State) Tick(h Host) {
	now := h.Now()
	if len(s.sleepers) > 0 {
		s.Prune(h)
	}
	keep := s.reactors[:0]
	waking := false
	for _, r := range s.reactors {
		b, ok := h.Building(r.ID)
		if !ok || b.Kind != construction.ShipReactor {
			h.Message(r.X, r.Y, "The ship reactor is gone, and its waking with it.")
			h.Changed()
			continue
		}
		if r.left(now) == 0 {
			if !r.Told {
				r.Told = true
				h.Letter("Ship reactor ready", "The reactor is fully awake. Once every part is in place and someone sleeps in a casket, the ship can launch.", b.X, b.Y, false)
				h.Changed()
			}
		} else {
			waking = true
		}
		keep = append(keep, r)
	}
	s.reactors = keep
	if !waking {
		s.nextRaid = 0 // the raids stop with the countdown (or the reactor)
		return
	}
	if s.nextRaid == 0 || now < s.nextRaid {
		return
	}
	h.Raid(s.raids%2 == 1) // people first, then machines, by turns
	s.raids++
	s.nextRaid = now + RaidGap + hash(h.Seed()^raidSalt, uint64(s.raids))%RaidJitter
}

// NextRaid is the next countdown raid's tick (0: none due).
func (s *State) NextRaid() uint64 { return s.nextRaid }

func sortReactors(rs []Reactor) {
	for i := 1; i < len(rs); i++ {
		for j := i; j > 0 && rs[j].ID < rs[j-1].ID; j-- {
			rs[j], rs[j-1] = rs[j-1], rs[j]
		}
	}
}

// hash mixes seed and n (splitmix64's finaliser).
func hash(seed, n uint64) uint64 {
	z := seed + n*0x9E3779B97F4A7C15
	z = (z ^ z>>30) * 0xBF58476D1CE4E5B9
	z = (z ^ z>>27) * 0x94D049BB133111EB
	return z ^ z>>31
}
