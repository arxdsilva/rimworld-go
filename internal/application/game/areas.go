package game

import (
	"fmt"
	"math/bits"
	"strings"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/area"
)

// Allowed areas (ENG-003, M9): g.areas is the colony's area set (up to
// area.MaxAreas named bitsets, overlapping zones freely); nil until the
// first area is made. Saved as Areas; each pawn's assignment lives in the
// world state (worldState.areaOf, saved with World). A restricted pawn
// plans its paths over its area only (pathQuery) and its jobs must target
// cells in it (startJob turns the rest down, with the postponement or
// food cooldown so the giver moves on). Drafted and breaking pawns and
// the player's own orders ignore areas; so do sleep, warmth, rescue and
// medical care, fights and a caravan's walk out. A restricted pawn caught
// outside its area (painted out, newly assigned) walks back in.

const (
	areaNameMax = 24
	// areaDefaultName names an area made by painting ("Area 1").
	areaDefaultName = "Area %d"
)

// areaSet is the colony's area set, created on first use.
func (g *Game) areaSet() *area.Set {
	if g.areas == nil {
		g.areas = area.New(g.grid.Width(), g.grid.Height())
	}
	return g.areas
}

// areaFor is pawn id's allowed area (-1: unrestricted).
func (g *Game) areaFor(id entity.EntityID) int {
	if g.worldSt == nil || g.areas == nil || len(g.worldSt.areaOf) == 0 {
		return -1
	}
	a, ok := g.worldSt.areaOf[id]
	if !ok || a < 0 || a >= len(g.areas.Areas) {
		return -1
	}
	return a
}

// restrictedArea is the area that binds id right now (-1: none): drafted
// and breaking pawns go anywhere.
func (g *Game) restrictedArea(id entity.EntityID) int {
	a := g.areaFor(id)
	if a < 0 || g.drafted(id) || g.breaking(id) {
		return -1
	}
	return a
}

// inArea reports whether pawn id may go to (x, y) under its allowed area.
func (g *Game) inArea(id entity.EntityID, x, y int) bool {
	a := g.restrictedArea(id)
	return a < 0 || g.areas.Contains(a, x, y)
}

// AreaOf is colonist id's allowed area for the Assign tab (-1:
// unrestricted).
func (g *Game) AreaOf(id entity.EntityID) int { return g.areaFor(id) }

// areaQuery is a path query limited to one area's cells.
type areaQuery struct {
	base pathing.Query
	set  *area.Set
	i    int
}

func (q areaQuery) Width() int  { return q.base.Width() }
func (q areaQuery) Height() int { return q.base.Height() }
func (q areaQuery) Walkable(x, y int) bool {
	return q.set.Contains(q.i, x, y) && q.base.Walkable(x, y)
}

// areaPathQuery is base limited to pawn id's area while id stands inside
// it; a pawn outside its area walks unrestricted (so it can get back), and
// so does one on a forced or exempt job (being started, or walked).
func (g *Game) areaPathQuery(id entity.EntityID, base pathing.Query) pathing.Query {
	a := g.restrictedArea(id)
	if a < 0 || g.worldSt.pathFree == id {
		return base
	}
	if j, _ := g.jobs.Get(id); j.Kind != job.None && (j.Forced || areaExempt(j.Kind)) {
		return base
	}
	if p, ok := g.positions.Get(id); !ok || !g.areas.Contains(a, p.X, p.Y) {
		return base
	}
	return areaQuery{base: base, set: g.areas, i: a}
}

// areaOutOfReach reports that a pawn keeping to its area cannot reach
// (x, y) at all: neither the cell nor any cell beside it is in the area.
// (Anything else a restricted pawn may reach is answered by a search over
// its area, since the regions know no allowed areas.)
func (g *Game) areaOutOfReach(id entity.EntityID, x, y int) bool {
	a := g.restrictedArea(id)
	if a < 0 {
		return false
	}
	if p, ok := g.positions.Get(id); !ok || !g.areas.Contains(a, p.X, p.Y) {
		return false // outside its area it walks free
	}
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if g.areas.Contains(a, x+dx, y+dy) {
				return false
			}
		}
	}
	return true
}

// areaExempt are the jobs an area never stops: the player's moves and
// a caravan's walk out, rest, warmth, rescue and medicine, fights, and
// gatherings and visits.
func areaExempt(k job.Kind) bool {
	switch k {
	case job.Move, job.Sleep, job.Warm, job.Rescue, job.Tend, job.Surgery, job.Hide, job.Attack, job.Brawl,
		job.Kidnap, job.ExitMap, job.Visit, job.Gather, job.Lovin, job.None:
		return true
	}
	return false
}

// areaRejects reports that job rec would take pawn id out of its allowed
// area: its target cell (a walkable one) or every cell beside it (a
// building or wall worked from beside) lies outside.
func (g *Game) areaRejects(id entity.EntityID, rec job.Record) bool {
	if rec.Forced || areaExempt(rec.Kind) {
		return false
	}
	a := g.restrictedArea(id)
	if a < 0 {
		return false
	}
	if g.areas.Contains(a, rec.TX, rec.TY) {
		return false
	}
	if g.grid.Walkable(rec.TX, rec.TY) {
		return true
	}
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if g.areas.Contains(a, rec.TX+dx, rec.TY+dy) {
				return false
			}
		}
	}
	return true
}

// areaBypass reports that job rec of pawn id paths free of its area (a
// forced or exempt job of a restricted pawn) and marks it so for the
// search startJob is about to run; endAreaBypass clears the mark.
func (g *Game) areaBypass(id entity.EntityID, rec job.Record) bool {
	if !rec.Forced && !areaExempt(rec.Kind) || g.restrictedArea(id) < 0 {
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

// areasTick (every worldStep) walks each restricted colonist standing
// outside its area, with nothing pressing to do, back in.
func (g *Game) areasTick(now uint64) {
	for _, id := range g.pawnIDs {
		a := g.restrictedArea(id)
		if a < 0 || !g.standingPawn(id) {
			continue
		}
		p, _ := g.positions.Get(id)
		if g.areas.Contains(a, p.X, p.Y) {
			continue
		}
		j, _ := g.jobs.Get(id)
		if j.Active() && !j.Kind.IsIdle() {
			continue
		}
		x, y, ok := g.nearestAreaCell(a, p)
		if !ok {
			continue
		}
		if j.Active() {
			g.interrupt(id, job.Interrupted)
		}
		g.startJob(id, job.Record{Kind: job.Move, TX: x, TY: y})
	}
}

// nearestAreaCell is area a's walkable cell nearest to p (Chebyshev, then
// row-major), ok false for an empty area.
func (g *Game) nearestAreaCell(a int, p Position) (int, int, bool) {
	s := g.areas
	best, bx, by := 1<<30, 0, 0
	for wi, word := range s.Areas[a].Bits {
		for ; word != 0; word &= word - 1 {
			n := wi*64 + bits.TrailingZeros64(word)
			x, y := n%s.W, n/s.W
			if d := max(abs(x-p.X), abs(y-p.Y)); d < best && g.grid.Walkable(x, y) {
				best, bx, by = d, x, y
			}
		}
	}
	return bx, by, best < 1<<30
}

// areaViews lists the colony's allowed areas.
func (g *Game) areaViews() []AreaView {
	if g.areas == nil {
		return nil
	}
	out := make([]AreaView, 0, len(g.areas.Areas))
	for i, a := range g.areas.Areas {
		out = append(out, AreaView{Index: i, Name: a.Name, Cells: g.areas.Count(i), ID: a.ID, Color: a.Color, Bits: a.Bits})
	}
	return out
}

// ensureArea makes area i when it is the next one (i == the count, below
// the cap) and reports whether area i exists.
func (g *Game) ensureArea(i int, name string) bool {
	s := g.areaSet()
	if i == len(s.Areas) {
		if name == "" {
			name = fmt.Sprintf(areaDefaultName, i+1)
		}
		return s.Add(name) == i
	}
	return i >= 0 && i < len(s.Areas)
}

// cmdPaintArea paints (on) or clears area i over the rectangle; painting
// the next index makes a new area.
func (g *Game) cmdPaintArea(i, x, y, x2, y2 int, on bool) {
	if !g.ensureArea(i, "") {
		return
	}
	g.areas.Paint(i, x, y, x2, y2, on)
}

// cmdRenameArea renames area i (the next index: a new area so named).
func (g *Game) cmdRenameArea(i int, name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	if r := []rune(name); len(r) > areaNameMax {
		name = string(r[:areaNameMax])
	}
	if g.areas != nil && g.areas.Rename(i, name) {
		return
	}
	g.ensureArea(i, name) // the next index: a new area so named
}

// cmdAssignArea restricts colonist pawn to area i (-1: unrestricted); a
// job outside the new area ends.
func (g *Game) cmdAssignArea(pawn entity.EntityID, i int) {
	w := g.worldS()
	if i < 0 {
		delete(w.areaOf, pawn)
		return
	}
	if g.areas == nil || i >= len(g.areas.Areas) || !g.alive(pawn) && !g.isStashed(pawn) {
		return
	}
	if w.areaOf == nil {
		w.areaOf = map[entity.EntityID]int{}
	}
	w.areaOf[pawn] = i
	if j, ok := g.jobs.Get(pawn); ok && j.Active() && g.areaRejects(pawn, j) {
		g.interrupt(pawn, job.Interrupted)
	}
}
