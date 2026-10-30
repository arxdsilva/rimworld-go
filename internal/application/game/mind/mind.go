package mind

import (
	"maps"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/mentalbreak"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Tuning of the breaks aimed at others (original values: the wiki gives
// no numbers for them).
const (
	// RetargetTicks: a berserker or an insulting spree picks a new victim
	// at most this often (no flapping between two of them).
	RetargetTicks = 250
	// InsultGap: an insulting colonist waits this long between insults.
	InsultGap = clock.TicksPerHour / 2
	// InsultRange: insults land from this close (tiles).
	InsultRange = 2
	// BerserkRadius: a berserker goes for creatures this close.
	BerserkRadius = 12
	// repathTicks: a pawn chasing someone re-paths this often.
	repathTicks = 60
	// checkStagger spreads the hourly break checks over the hour.
	checkStagger = 977
)

// Track is what mind keeps about one break: whom it is aimed at (the
// fixed target of a targeted insult or a murderous rage, or the current
// victim of a spree or a berserker) and the tick before which it picks no
// new victim and lands no new insult.
type Track struct {
	Target entity.EntityID
	Next   uint64
	// X, Y (CBT-4, absent in older saves) is a tantrum's target's tile.
	X, Y int
}

// Save is mind's saved state: the tracks of breaks in progress, and the
// tick each colonist that broke is calm until.
type Save struct {
	Track map[entity.EntityID]Track
	Calm  map[entity.EntityID]uint64
	// PRD-2 (inspiration.go): inspirations and the calm after one.
	Inspire     map[entity.EntityID]Inspiration
	InspireCalm map[entity.EntityID]uint64
}

// Mind runs every colonist's mental breaks.
type Mind struct {
	h     Host
	track map[entity.EntityID]Track
	// calm: no new break before this tick (the catharsis after one).
	calm map[entity.EntityID]uint64
	buf  []entity.EntityID // victim candidates, reused
	// Inspire is the inspiration component (inspiration.go, PRD-2).
	Inspire *Inspirations
}

// New builds a Mind over h.
func New(h Host) *Mind {
	return &Mind{h: h, track: map[entity.EntityID]Track{}, calm: map[entity.EntityID]uint64{}, Inspire: newInspirations(h)}
}

// Hostile reports a break that makes a colonist a fair target (colonists
// may attack and arrest it).
func Hostile(k mentalbreak.Kind) bool {
	return k == mentalbreak.Berserk || k == mentalbreak.MurderousRage
}

// Target is whom id's break is aimed at (0: no break, or none aimed).
func (m *Mind) Target(id entity.EntityID) entity.EntityID {
	if k, _, ok := m.h.Break(id); ok && mentalbreak.TargetsOthers(k) {
		return m.track[id].Target
	}
	return 0
}

// Export is mind's state for the save.
func (m *Mind) Export() Save {
	s := Save{Track: maps.Clone(m.track), Calm: maps.Clone(m.calm)}
	s.Inspire, s.InspireCalm = m.Inspire.export()
	return s
}

// Import restores Export's state (empty: none, or an older save).
func (m *Mind) Import(s Save) {
	m.track, m.calm = map[entity.EntityID]Track{}, map[entity.EntityID]uint64{}
	maps.Copy(m.track, s.Track)
	maps.Copy(m.calm, s.Calm)
	m.Inspire.load(s.Inspire, s.InspireCalm)
}

// Force starts break k on id now for its minimum duration (dev tools and
// fixtures); nothing happens to the dead.
func (m *Mind) Force(id entity.EntityID, k mentalbreak.Kind) {
	if !m.h.Alive(id) {
		return
	}
	m.start(id, k, mentalbreak.DefOf(k).Duration)
}

// Tick handles one standing colonist in the AI phase: it may start a
// break, runs one in progress and ends it. j is the colonist's job. It
// reports whether the colonist is breaking (the AI leaves it alone).
func (m *Mind) Tick(id entity.EntityID, j job.Record) bool {
	h := m.h
	now := h.Now()
	k, until, ok := h.Break(id)
	if !ok {
		m.forget(id)
		if (now+uint64(id)*checkStagger)%clock.TicksPerHour != 0 {
			return false
		}
		m.Inspire.Check(id) // inspiration.go (PRD-2)
		return m.maybeBreak(id)
	}
	if k == mentalbreak.SocialFight {
		return m.fight(id, j, now, until) // fight.go
	}
	if now >= until {
		m.end(id, k)
		return false
	}
	if j.Active() && j.Kind == job.Eat && k != mentalbreak.Catatonic {
		return true // a break never starves anyone
	}
	m.act(id, k, j, now, until)
	return true
}

// Downed handles a colonist that is down or dead (the AI phase's downed
// branch, and every blow that lands): a social fight ends; a catatonic
// one recovers when its time is up; any other break ends, since a downed
// pawn can't act out; the dead keep no break.
func (m *Mind) Downed(id entity.EntityID) {
	h := m.h
	k, until, ok := h.Break(id)
	if !ok {
		return
	}
	switch {
	case k == mentalbreak.SocialFight:
		if !h.Standing(id) {
			m.EndFight(id, m.partner(id)) // a downed brawler ends the fight
		}
	case !h.Alive(id):
		m.forget(id)
		h.ClearBreak(id)
	case k == mentalbreak.Catatonic:
		if h.Now() >= until {
			m.end(id, k)
		}
	case !h.Standing(id):
		m.end(id, k)
	}
}

// End ends id's break now (an arrest): it calms down as after any break.
func (m *Mind) End(id entity.EntityID) {
	if k, _, ok := m.h.Break(id); ok && k != mentalbreak.SocialFight {
		m.end(id, k)
	}
}

// roll is a uniform 31-bit draw for the mentalbreak tables.
func (m *Mind) roll() uint32 { return uint32(m.h.Roll(1 << 31)) }

func (m *Mind) forget(id entity.EntityID) {
	if len(m.track) > 0 {
		delete(m.track, id)
	}
}

// end finishes break k: the record and the track go, a catatonic pawn
// gets up, and the colonist calms down with catharsis.
func (m *Mind) end(id entity.EntityID, k mentalbreak.Kind) {
	m.forget(id)
	m.h.ClearBreak(id)
	m.calm[id] = m.h.Now() + mood.DefOf(mood.Catharsis).Duration
	if k == mentalbreak.Catatonic {
		m.h.StandUp(id)
	}
	m.h.Recovered(id, k)
}
