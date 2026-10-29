package rest

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/entity"
)

// Needs is every pawn's comfort, beauty and outdoors needs (BED-3, wiki
// rates in domain/pawn/needs). Tick runs each tick for each pawn awake
// on its feet (the game's needs phase) and each tick of sleep (Sleep.Do),
// but does its work only on the pawn's own staggered beat: comfort and
// outdoors every needs.StepTicks, a look around for beauty every
// needs.BeautySampleTicks while awake. Each need's thought is a memory
// refreshed on every beat while its stage holds, so it fades an hour
// after the pawn stops being updated (downed, gone).
type Needs struct {
	h     Host
	stats *Stats
	seats *Seats
	m     map[entity.EntityID]NeedRecord
}

// NeedRecord is one pawn's comfort, beauty and outdoors levels (millionths,
// needs.LevelFull), the stage each thought stands at (-1: none yet) and
// SeatAt, the tick it may next look for a seat after finding none. Saved
// (the game's RestNeeds).
type NeedRecord struct {
	Comfort, Beauty, Outdoors                int32
	ComfortStage, BeautyStage, OutdoorsStage int8
	SeatAt                                   uint64
}

// newRecord is a pawn's needs before its first beat.
func newRecord() NeedRecord {
	return NeedRecord{Comfort: needs.ComfortStart, Beauty: needs.BeautyStart, Outdoors: needs.OutdoorsStart,
		ComfortStage: -1, BeautyStage: -1, OutdoorsStage: -1}
}

// Of is pawn id's needs (a new pawn's when it has no record yet).
func (n *Needs) Of(id entity.EntityID) NeedRecord {
	if r, ok := n.m[id]; ok {
		return r
	}
	return newRecord()
}

// Percents is pawn id's comfort, beauty and outdoors needs in percent,
// for the read model.
func (n *Needs) Percents(id entity.EntityID) (comfort, beauty, outdoors int) {
	r := n.Of(id)
	return needs.LevelPercent(r.Comfort), needs.LevelPercent(r.Beauty), needs.LevelPercent(r.Outdoors)
}

// Tick is one tick of pawn id's needs at tick now.
func (n *Needs) Tick(id entity.EntityID, now uint64) {
	beat := now + uint64(id)
	step, look := beat%needs.StepTicks == 0, beat%needs.BeautySampleTicks == 0
	if !step && !look {
		return
	}
	r := n.Of(id)
	j := n.h.Job(id)
	asleep := j.Kind == job.Sleep && j.State == job.Do
	x, y := n.h.Pos(id)
	if step {
		seat, using := n.comfortSource(j, asleep, x, y)
		r.Comfort = needs.ComfortStep(r.Comfort, seat, using)
		roofed, thick := n.h.Roof(x, y)
		r.Outdoors = needs.OutdoorsStep(r.Outdoors, needs.PlaceOf(n.h.RoomAt(x, y) >= 0, roofed, thick), asleep && j.Target != 0)
		r.ComfortStage = n.feel(id, mood.ComfortMood, &needs.ComfortLadder, r.ComfortStage, r.Comfort)
		r.OutdoorsStage = n.feel(id, mood.OutdoorsMood, &needs.OutdoorsLadder, r.OutdoorsStage, r.Outdoors)
		if j.Kind == job.SitDown && j.State == job.Do {
			n.seats.sitting(id, j, r.Comfort, seat, using)
		}
	}
	if look {
		if !asleep { // a sleeper sees nothing: its beauty holds
			r.Beauty = needs.BeautyStep(r.Beauty, needs.BeautyTarget(n.Perceived(x, y)))
		}
		r.BeautyStage = n.feel(id, mood.BeautyMood, &needs.BeautyLadder, r.BeautyStage, r.Beauty)
	}
	n.m[id] = r
}

// comfortSource is the comfort (permille) of the furniture a pawn on job j
// at (x, y) uses: the bed it sleeps in (with its end table and dresser),
// or the seat it sits on while at work there (eating at a table, resting,
// working at a bench). using is false when it uses none.
func (n *Needs) comfortSource(j job.Record, asleep bool, x, y int) (comfort int, using bool) {
	if asleep {
		if j.Target == 0 {
			return 0, false // on the ground
		}
		return n.stats.Of(j.Target).Comfort, true
	}
	if j.State != job.Do {
		return 0, false // walking or idle on its feet
	}
	return n.seats.ComfortAt(x, y)
}

// feel moves a need's thought to its stage for level (with the ladder's
// hysteresis) and refreshes it; a stage with no thought forgets the old
// one once.
func (n *Needs) feel(id entity.EntityID, k mood.Kind, l *needs.Ladder, cur int8, level int32) int8 {
	s := l.Stage(int(cur), level)
	if v := l.Value[s]; v != 0 {
		n.h.RememberValue(id, k, v, l.Label[s])
	} else if int(cur) != s {
		n.h.ForgetGroup(id, k)
	}
	return int8(s)
}

// Export is the live records, for the save to copy.
func (n *Needs) Export() map[entity.EntityID]NeedRecord { return n.m }

// Import restores Export's records; nil (an older save) starts every pawn
// afresh.
func (n *Needs) Import(m map[entity.EntityID]NeedRecord) {
	if m == nil {
		m = make(map[entity.EntityID]NeedRecord)
	}
	n.m = m
}
