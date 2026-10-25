package game

import (
	"cmp"
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// Area destinations (PLAY-015): a restricted pawn's job must work, fetch
// and deliver inside its area: the work target (the cell worked, not the
// spot stood on), a haul's destination (stockpile cell, frame, route
// stops, spare cell), a bill's bench, a fuel user, a grave, food, beds,
// recreation and wander spots, and the prey of a hunt (one that stays
// outside for huntOutTicks is given up).

// huntOutTicks is how long a hunt's prey may stay outside the hunter's
// area before the hunt is abandoned.
const huntOutTicks = 600

// areaExempt are the job kinds an area never stops: the player's moves
// (and a flight), a caravan's walk out, warmth, rescue and medicine,
// fights, and gatherings and visits.
func areaExempt(k job.Kind) bool {
	switch k {
	case job.Move, job.Warm, job.Rescue, job.Tend, job.Surgery, job.Hide, job.Attack, job.Brawl,
		job.Kidnap, job.ExitMap, job.Visit, job.Gather, job.Lovin, job.None:
		return true
	}
	return false
}

// areaExemptJob is areaExempt for pawn id's job rec, which also lets a
// patient rest in a hospital bed wherever it stands.
func (g *Game) areaExemptJob(id entity.EntityID, rec job.Record) bool {
	if areaExempt(rec.Kind) {
		return true
	}
	if rec.Kind == job.Sleep && rec.Target != 0 {
		b, ok := g.bldgs.Get(rec.Target)
		return ok && b.Kind == construction.HospitalBed
	}
	return false
}

// areaRejects reports that job rec would take pawn id out of its allowed
// area: its work target or any destination lies outside (refAllows).
func (g *Game) areaRejects(id entity.EntityID, rec job.Record) bool {
	if rec.Forced || g.areaExemptJob(id, rec) {
		return false
	}
	r := g.restrictedRef(id)
	if r == 0 {
		return false
	}
	tx, ty := rec.TX, rec.TY
	switch rec.Kind {
	case job.Mine, job.Smooth, job.Clean, job.Roof, job.Unroof:
		tx, ty = rec.DestX, rec.DestY // the cell worked, from beside
	case job.Cut:
		if rec.Target == 0 {
			tx, ty = rec.DestX, rec.DestY // grass
		}
	}
	if !g.refAllows(r, tx, ty) {
		return true
	}
	switch rec.Kind {
	case job.Haul, job.Bill, job.Refuel, job.Bury:
		if !g.refAllows(r, rec.DestX, rec.DestY) {
			return true
		}
	}
	if rec.HasSpare && !g.refAllows(r, rec.SpareX, rec.SpareY) {
		return true
	}
	if rec.Kind == job.Haul {
		for _, key := range rec.Route {
			fid, _ := slotFrame(key)
			if fr, ok := g.frames.Get(fid); ok && !g.refAllows(r, fr.X, fr.Y) {
				return true
			}
		}
	}
	return false
}

// areaBypass reports that job rec of pawn id paths free of its area (a
// forced or exempt job of a restricted pawn) and marks it so for the
// search startJob is about to run; endAreaBypass clears the mark.
func (g *Game) areaBypass(id entity.EntityID, rec job.Record) bool {
	if !rec.Forced && !g.areaExemptJob(id, rec) || g.restrictedRef(id) == 0 {
		return false
	}
	g.worldSt.pathFree = id
	return true
}

func (g *Game) endAreaBypass() { g.worldSt.pathFree = 0 }

// rejectOutOfArea fails job rec of pawn id for leaving its area, with the
// cooldown its giver honours: no meal search for a while, the work put
// off as out of reach.
func (g *Game) rejectOutOfArea(id entity.EntityID, rec job.Record) {
	g.reserve.ReleaseOwner(id)
	rec.State, rec.Fail, rec.Path = job.Fail, job.Unreachable, nil
	g.jobs.Set(id, rec)
	if rec.Kind == job.Eat {
		g.noFood[id] = uint64(g.clock.Now()) + foodRetry
		return
	}
	g.postpone(postKeyOf(id, rec), job.Unreachable) // postpone.go
}

// areaStockpileCell is stockpileCell for hauler id: the nearest stockpile
// cell taking k inside its area.
func (g *Game) areaStockpileCell(id entity.EntityID, k item.Kind, nearX, nearY int) (int, int, bool) {
	r := g.restrictedRef(id)
	if r == 0 {
		return g.stockpileCell(k, nearX, nearY)
	}
	return g.stockpileCellWhere(k, nearX, nearY, func(x, y int) bool { return g.refContains(r, x, y) })
}

// sleepFloorSpot is where pawn id at pos lies down with no bed: where it
// stands, or, standing outside its area, the nearest cell inside.
func (g *Game) sleepFloorSpot(id entity.EntityID, pos Position) (int, int) {
	r := g.restrictedRef(id)
	if r == 0 || g.refContains(r, pos.X, pos.Y) {
		return pos.X, pos.Y
	}
	if x, y, ok := g.nearestRefCell(r, pos); ok {
		return x, y
	}
	return pos.X, pos.Y
}

// huntGivenUp reports that hunter id's prey, at tp, has stayed outside
// the hunter's area for huntOutTicks (a forced hunt never gives up). It
// keeps the clock: the first tick seen outside, cleared once back in.
func (g *Game) huntGivenUp(id entity.EntityID, j job.Record, tp Position, now uint64) bool {
	if j.Forced || g.inArea(id, tp.X, tp.Y) {
		g.clearHuntOut(id)
		return false
	}
	w := g.worldS()
	h, ok := w.huntOut[id]
	if !ok || h.Prey != j.Target {
		if w.huntOut == nil {
			w.huntOut = map[entity.EntityID]huntOutRec{}
		}
		w.huntOut[id] = huntOutRec{Pawn: id, Prey: j.Target, Since: now}
		return false
	}
	if now-h.Since < huntOutTicks {
		return false
	}
	delete(w.huntOut, id)
	return true
}

func (g *Game) clearHuntOut(id entity.EntityID) {
	if g.worldSt != nil && len(g.worldSt.huntOut) > 0 {
		delete(g.worldSt.huntOut, id)
	}
}

// abandonHunt ends hunter id's hunt j for the prey staying out of its
// area, putting the prey off so the giver picks another.
func (g *Game) abandonHunt(id entity.EntityID, j job.Record) {
	j.State, j.Fail, j.Path = job.Fail, job.Unreachable, nil
	g.jobs.Set(id, j)
	g.postponeJob(id, j)
}

// huntOutRec is a hunt's out-of-area clock: hunter Pawn's prey, seen
// outside the hunter's area since tick Since.
type huntOutRec struct {
	Pawn, Prey entity.EntityID
	Since      uint64
}

func exportHuntOut(m map[entity.EntityID]huntOutRec) []huntOutRec {
	if len(m) == 0 {
		return nil
	}
	out := make([]huntOutRec, 0, len(m))
	for _, h := range m {
		out = append(out, h)
	}
	slices.SortFunc(out, func(a, b huntOutRec) int { return cmp.Compare(a.Pawn, b.Pawn) })
	return out
}

func importHuntOut(s []huntOutRec) map[entity.EntityID]huntOutRec {
	if len(s) == 0 {
		return nil
	}
	m := make(map[entity.EntityID]huntOutRec, len(s))
	for _, r := range s {
		m[r.Pawn] = r
	}
	return m
}
