package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/component"
	"rimworld-go/internal/domain/sim/entity"
)

// TicksPerCellDiag is how long a pawn takes over a diagonal step
// (15 * sqrt 2, rounded), so diagonals are no faster than straights.
const TicksPerCellDiag = 21

// blockedFailTicks: a walker kept from its next cell this long gives up
// its job with job.Blocked (the liveness guarantee of SIM-014).
const blockedFailTicks = 600

// motionRec is one creature's step timing (colonists, raiders, animals).
// A mover may step when now >= NextMoveAt; a step at tick now sets
// StepTick = now, StepTicks = its cost and NextMoveAt = now + cost.
// BlockedTicks counts consecutive ticks the next step was refused (it
// keeps counting across repaths) and BlockedBy names the creature in the
// way; both reset on any successful step.
type motionRec struct {
	NextMoveAt   uint64
	StepTick     uint64
	StepTicks    int
	BlockedTicks int
	BlockedBy    entity.EntityID
}

// motionOf is id's record. A creature without one (new, or from a save
// that predates motion records) is due now and reads a straight step.
func (g *Game) motionOf(id entity.EntityID) motionRec {
	if m, ok := g.motion.Get(id); ok {
		return m
	}
	return motionRec{NextMoveAt: uint64(g.clock.Now()), StepTicks: TicksPerCell}
}

// motionBlocked reports how long id has been blocked and by whom; (0, 0)
// when it has no record.
func (g *Game) motionBlocked(id entity.EntityID) (ticks int, by entity.EntityID) {
	m, ok := g.motion.Get(id)
	if !ok {
		return 0, 0
	}
	return m.BlockedTicks, m.BlockedBy
}

// stepped records a step taken at tick now costing cost ticks.
func (m *motionRec) stepped(now uint64, cost int) {
	m.StepTick, m.StepTicks, m.NextMoveAt = now, cost, now+uint64(cost)
	m.BlockedTicks, m.BlockedBy = 0, 0
}

// isDiagonal reports a step that changes both coordinates.
func isDiagonal(from, to Position) bool { return from.X != to.X && from.Y != to.Y }

// pawnStepCost is a colonist's or raider's ticks for from -> to.
func pawnStepCost(from, to Position) int {
	if isDiagonal(from, to) {
		return TicksPerCellDiag
	}
	return TicksPerCell
}

// mover is one creature in the movement phase; animal marks the kind.
type mover struct {
	id     entity.EntityID
	animal bool
}

// movers fills g.moverBuf with every colonist, raider and animal in
// ascending ID (each list is already ascending: IDs only grow and are
// appended at spawn).
func (g *Game) movers() []mover {
	buf := g.moverBuf[:0]
	a, b, c := g.pawnIDs, g.raiderIDs, g.animalIDs
	for len(a)+len(b)+len(c) > 0 {
		switch {
		case len(a) > 0 && (len(b) == 0 || a[0] < b[0]) && (len(c) == 0 || a[0] < c[0]):
			buf, a = append(buf, mover{id: a[0]}), a[1:]
		case len(b) > 0 && (len(c) == 0 || b[0] < c[0]):
			buf, b = append(buf, mover{id: b[0]}), b[1:]
		default:
			buf, c = append(buf, mover{id: c[0], animal: true}), c[1:]
		}
	}
	g.moverBuf = buf
	return buf
}

// pruneMotion drops records of creatures that left the map. Runs only
// when the store holds more records than there are creatures.
func (g *Game) pruneMotion(movers []mover) {
	if g.motion.Len() <= len(movers) {
		return
	}
	kept := component.NewStore[motionRec]()
	for _, mv := range movers {
		if m, ok := g.motion.Get(mv.id); ok {
			kept.Set(mv.id, m)
		}
	}
	g.motion = kept
}

// exportMotion is the records of creatures on the map, in ID order.
func (g *Game) exportMotion() []entry[motionRec] {
	ids := g.movers()
	out := make([]entry[motionRec], 0, len(ids))
	for _, mv := range ids {
		if m, ok := g.motion.Get(mv.id); ok {
			out = append(out, entry[motionRec]{mv.id, m})
		}
	}
	return out
}

// defaultMotion gives every creature without a record (an older save)
// one that is due now and reads a straight step.
func (g *Game) defaultMotion() {
	now := uint64(g.clock.Now())
	for _, mv := range g.movers() {
		if _, ok := g.motion.Get(mv.id); !ok {
			g.motion.Set(mv.id, motionRec{NextMoveAt: now, StepTicks: TicksPerCell})
		}
	}
}

// fillPawnMotion copies a colonist's or raider's step timing and next
// planned cell into its view.
func (g *Game) fillPawnMotion(pv *PawnView) {
	m := g.motionOf(pv.ID)
	pv.StepTick, pv.StepTicks = m.StepTick, m.StepTicks
	if j, ok := g.jobs.Get(pv.ID); ok && j.State == job.Goto && j.PathI < len(j.Path) {
		pv.NextX, pv.NextY, pv.HasNext = j.Path[j.PathI].X, j.Path[j.PathI].Y, true
		pv.BlockedBy = m.BlockedBy
	}
}

// decide is the occupancy verdict for id stepping from -> to.
func (g *Game) decide(id entity.EntityID, from, to Position) (stepVerdict, Position) {
	if g.enterHook != nil {
		return g.enterHook(id, from, to)
	}
	return g.enter(id, from, to)
}

// walkTarget is where a walk is headed: once loaded (Carry set) every job
// walks to DestX/DestY, the rule the pickups follow (a haul keeps TX/TY
// on the pickup cell); otherwise to TX/TY.
func walkTarget(j job.Record) (int, int) {
	if j.Carry != 0 {
		return j.DestX, j.DestY
	}
	return j.TX, j.TY
}
