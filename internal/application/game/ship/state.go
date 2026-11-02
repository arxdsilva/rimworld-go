package ship

import (
	"slices"

	"rimworld-go/internal/domain/sim/entity"
)

// State is the escape ship's state. The zero State is a colony with no
// reactor started, nobody aboard and no launch; Save is what a save
// keeps of it, the ships themselves are derived from the buildings.
type State struct {
	reactors []Reactor // started reactors, ascending ID
	sleepers []Sleeper // occupied caskets, ascending Casket
	nextRaid uint64    // the next countdown raid's tick (0: none due)
	raids    int       // countdown raids sent so far (the spacing hash, the turn of people and machines)
	launched bool
	launchAt uint64
	escaped  []string // who left aboard, in casket order

	g graph // the ships, per structure version
}

// Reactor is one started reactor: where it stands, when its countdown
// began, the countdown ticks dev mode skipped, and whether the colony was
// told it is awake.
type Reactor struct {
	ID    entity.EntityID
	X, Y  int
	Start uint64
	Skip  uint64
	Told  bool
}

// left is the reactor's countdown ticks left at now.
func (r Reactor) left(now uint64) int { return Left(r.Start, now+r.Skip) }

// Rewind moves every started reactor's countdown, and the next countdown
// raid, on by ticks (dev mode and journeys).
func (s *State) Rewind(ticks uint64) {
	for i := range s.reactors {
		s.reactors[i].Skip += ticks
	}
	if s.nextRaid != 0 {
		s.nextRaid -= min(ticks, s.nextRaid-1)
	}
}

// Sleeper is one occupied ship casket and who sleeps in it.
type Sleeper struct {
	Casket, Pawn entity.EntityID
}

// Save is State's saved form (game save field AX8; absent in older
// saves: nothing started).
type Save struct {
	Reactors []Reactor
	Sleepers []Sleeper
	NextRaid uint64
	Raids    int
	Launched bool
	LaunchAt uint64
	Escaped  []string
}

// Export is the saved form (nil when there is nothing to keep).
func (s *State) Export() *Save {
	if s == nil || len(s.reactors) == 0 && len(s.sleepers) == 0 && !s.launched && s.raids == 0 {
		return nil
	}
	return &Save{Reactors: slices.Clone(s.reactors), Sleepers: slices.Clone(s.sleepers), NextRaid: s.nextRaid,
		Raids: s.raids, Launched: s.launched, LaunchAt: s.launchAt, Escaped: slices.Clone(s.escaped)}
}

// Import restores a saved state (nil: the zero State).
func (s *State) Import(v *Save) {
	*s = State{}
	if v == nil {
		return
	}
	s.reactors, s.sleepers = slices.Clone(v.Reactors), slices.Clone(v.Sleepers)
	s.nextRaid, s.raids, s.launched, s.launchAt = v.NextRaid, v.Raids, v.Launched, v.LaunchAt
	s.escaped = slices.Clone(v.Escaped)
	slices.SortFunc(s.reactors, func(a, b Reactor) int { return int(a.ID) - int(b.ID) })
	slices.SortFunc(s.sleepers, func(a, b Sleeper) int { return int(a.Casket) - int(b.Casket) })
}

// Launched reports the ship has left (the colony's story is complete).
func (s *State) Launched() bool { return s != nil && s.launched }

// Sleeper is who sleeps in ship casket id (0: nobody).
func (s *State) Sleeper(casket entity.EntityID) entity.EntityID {
	if i, ok := slices.BinarySearchFunc(s.sleepers, casket, func(e Sleeper, id entity.EntityID) int { return int(e.Casket) - int(id) }); ok {
		return s.sleepers[i].Pawn
	}
	return 0
}

// CasketOf is the ship casket pawn sleeps in (0: none).
func (s *State) CasketOf(pawn entity.EntityID) entity.EntityID {
	for _, e := range s.sleepers {
		if e.Pawn == pawn {
			return e.Casket
		}
	}
	return 0
}

// setSleeper puts pawn in casket (0 empties it).
func (s *State) setSleeper(casket, pawn entity.EntityID) {
	i, ok := slices.BinarySearchFunc(s.sleepers, casket, func(e Sleeper, id entity.EntityID) int { return int(e.Casket) - int(id) })
	switch {
	case ok && pawn == 0:
		s.sleepers = slices.Delete(s.sleepers, i, i+1)
	case ok:
		s.sleepers[i].Pawn = pawn
	case pawn != 0:
		s.sleepers = slices.Insert(s.sleepers, i, Sleeper{casket, pawn})
	}
}

// reactor is started reactor id's record (nil: not started).
func (s *State) reactor(id entity.EntityID) *Reactor {
	for i := range s.reactors {
		if s.reactors[i].ID == id {
			return &s.reactors[i]
		}
	}
	return nil
}

// Busy reports anything for Tick to do: a started reactor, a raid due or
// a sleeper whose casket may fall.
func (s *State) Busy() bool {
	return s != nil && (len(s.reactors) > 0 || s.nextRaid != 0 || len(s.sleepers) > 0)
}
