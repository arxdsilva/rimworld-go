package game

import (
	"fmt"
	"math"
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Visitors and wanderers (ENG-003, M9). A friendly or neutral faction
// sends two to four of its people: they walk in from an edge, stroll near
// the colony for half a day (job.Visit: walk to a spot, linger) and walk
// out again (a trader caravan, trade_caravan.go, stays longer and
// closer). They are registered people (Where Visitor, their faction)
// on raiderIDs as non-hostile pawns (raiderState.Leaving, as the colony's
// released prisoners are), so colonists can chat with them and capture or
// arrest them (M8); the visit itself is driven from phaseWorld, out of
// the raid and prisoner AI. Leaving is one-way: once a visitor turns for
// the edge it never strolls again. Off the map they stay in the registry
// (OffMap) with their relations.
//
// A wanderer is one generated human who walks in and joins the colony.

const (
	visitorSalt  = 0x715170
	wandererSalt = 0x3A4D3E
	// visitorStay is how long a group strolls before it leaves.
	visitorStay = clock.TicksPerDay / 2
	// visitorLinger is how long a visitor stands at a spot it walked to.
	visitorLinger = 600
	// visitorRange is how far from the colony's heart they stroll.
	visitorRange = 10
	// visitorRetry: a walk that failed is tried again this much later.
	visitorRetry = 250
	// visitorGiveUp: a leaving visitor still on the map this long after its
	// leave time slips away where it stands.
	visitorGiveUp = clock.TicksPerDay / 2
	// visitorInset keeps visitors this far in from the border (the
	// released-prisoner rule takes anyone standing on it off the map).
	visitorInset = 2
	// visitorGroup is the raid group a visitor belongs to: none.
	visitorGroup = math.MaxInt32
	// visitorSpotR is how far from an edge point a spot is looked for.
	visitorSpotR = 12
	// wandererMaxColony: no wanderer joins a colony this large.
	wandererMaxColony = 16
)

// visitorRec is one visitor on the map.
type visitorRec struct {
	ID      entity.EntityID
	Faction int // 1 + faction index
	LeaveAt uint64
	Leaving bool
	RetryAt uint64
	// Ally: a fighter sent as military aid (allies.go, PLAY-024); false
	// for every ordinary visitor and in older saves.
	Ally bool
}

// visitingFactions are the factions (1 + index) that may send visitors:
// human, not hostile, not defeated.
func (g *Game) visitingFactions() []int {
	var out []int
	for i, f := range g.ng.factions {
		if f.Type.Human() && f.Relation != faction.Hostile && !f.Defeated {
			out = append(out, i+1)
		}
	}
	return out
}

// visitorsFit reports whether a friendly visitor group can arrive: a
// colony with a friendly or neutral people and no visitors already here.
func (g *Game) visitorsFit(now uint64) bool {
	if !g.hasSetup() || len(g.pawnIDs) == 0 || len(g.visitingFactions()) == 0 {
		return false
	}
	return g.worldSt == nil || len(g.worldSt.visitors) == 0
}

// visitorsArrive brings a visitor group in from a map edge.
func (g *Game) visitorsArrive(now uint64) bool {
	facs := g.visitingFactions()
	if len(facs) == 0 {
		return false
	}
	r := g.stream(streamWorld)
	fac := facs[r.IntN(len(facs))]
	f := g.factionOf(fac)
	ex, ey, ok := g.worldEdge()
	if !ok {
		return false
	}
	n := 2 + r.IntN(3)
	wantKin := g.relativeRoll(relativeVisitPermille) // relative_world.go
	var kin entity.EntityID
	var used [][2]int
	var ids []entity.EntityID
	for k := 0; k < n; k++ {
		x, y, ok := g.insetLandNear(ex, ey, used)
		if !ok {
			break
		}
		used = append(used, [2]int{x, y})
		if wantKin {
			wantKin = false
			if kin = g.relativeFor(fac, whereVisitor, techTag(f)); kin != 0 {
				g.placeVisitor(kin, fac, x, y, now)
				ids = append(ids, kin)
				continue
			}
		}
		ids = append(ids, g.spawnVisitor(fac, f, x, y, now))
	}
	if len(ids) == 0 {
		return false
	}
	g.letter(letterInfo, "Visitors", fmt.Sprintf("%d people of %s have come to visit. They will stay about half a day.", len(ids), f.Name), ids[0])
	if kin != 0 {
		g.relativeLetter(letterInfo, "Relative arrives", kin, "visitors from "+f.Name)
	}
	return true
}

// insetLandNear is a free dry tile near (x, y) at least visitorInset from
// the border.
func (g *Game) insetLandNear(x, y int, used [][2]int) (int, int, bool) {
	w, h := g.grid.Width(), g.grid.Height()
	x = min(max(x, visitorInset), w-1-visitorInset)
	y = min(max(y, visitorInset), h-1-visitorInset)
	for d := 0; d <= visitorSpotR; d++ {
		for dy := -d; dy <= d; dy++ {
			for dx := -d; dx <= d; dx++ {
				if max(abs(dx), abs(dy)) != d {
					continue
				}
				nx, ny := x+dx, y+dy
				if !g.inset(nx, ny) || !g.dryWalkable(nx, ny) || slices.Contains(used, [2]int{nx, ny}) || g.pawnOn(nx, ny) {
					continue
				}
				return nx, ny, true
			}
		}
	}
	return 0, 0, false
}

// inset reports (x, y) is at least visitorInset in from the border.
func (g *Game) inset(x, y int) bool {
	w, h := g.grid.Width(), g.grid.Height()
	return x >= visitorInset && y >= visitorInset && x < w-visitorInset && y < h-visitorInset
}

// spawnVisitor puts one new visitor of faction fac on (x, y).
func (g *Game) spawnVisitor(fac int, f *faction.Faction, x, y int, now uint64) entity.EntityID {
	id := g.entities.Next()
	p := pawngen.Generate(g.worldSeed()^visitorSalt, int(id), 0, pawngen.Rules{Tech: techTag(f)})
	g.peopleReg().add(&person{ID: id, Bio: p, HasBio: true, Faction: fac, Where: whereVisitor})
	g.socState().sig[id] = personSig{}
	g.placeVisitor(id, fac, x, y, now)
	return id
}

// placeVisitor puts registered person id (Where Visitor) on (x, y) as a
// visitor of faction fac.
func (g *Game) placeVisitor(id entity.EntityID, fac, x, y int, now uint64) {
	g.raiderIDs = withID(g.raiderIDs, id) // prisoners.go
	g.positions.Set(id, Position{X: x, Y: y})
	g.names.Set(id, bioName(g.personOf(id).Bio))
	g.jobs.Set(id, job.Record{})
	g.need.Set(id, needs.NewFull())
	g.vital.Set(id, health.Record{Status: health.Alive})
	g.raiders[id] = raiderState{Group: visitorGroup, Arrived: now, Leaving: true, Idle: now, RetryAt: math.MaxUint64}
	w := g.worldS()
	w.visitors = append(w.visitors, visitorRec{ID: id, Faction: fac, LeaveAt: now + visitorStay})
	g.invalidateCreatures()
}

// setVisitorStay makes visitor id leave at leaveAt (a trader caravan's
// longer stay).
func (g *Game) setVisitorStay(id entity.EntityID, leaveAt uint64) {
	w := g.worldS()
	for i := range w.visitors {
		if w.visitors[i].ID == id {
			w.visitors[i].LeaveAt = leaveAt
		}
	}
}

// visitorsTick drives every visitor: keep it out of the released-prisoner
// walk-out, stroll near the colony, and leave when the stay is over.
func (g *Game) visitorsTick(now uint64) {
	w := g.worldSt
	kept := w.visitors[:0]
	for _, v := range w.visitors {
		if g.visitorStep(&v, now) {
			kept = append(kept, v)
		} else {
			g.traderVisitorGone(v.ID) // trade_caravan.go (PLAY-024): the trader's goods
			if v.Ally {
				g.allyGone(v) // allies.go: a letter for one who fell
			}
		}
	}
	clear(w.visitors[len(kept):])
	w.visitors = kept
}

// visitorStep runs visitor v for one tick; false: it is no longer a
// visitor on the map (gone, dead, taken prisoner).
func (g *Game) visitorStep(v *visitorRec, now uint64) bool {
	r, ok := g.raiders[v.ID]
	if !ok || g.prisoners[v.ID] != nil {
		return false
	}
	if p := g.personOf(v.ID); p == nil || p.Where != whereVisitor {
		return false
	}
	st, _ := g.vital.Get(v.ID)
	if st.Status == health.Dead {
		return false
	}
	// thinkPrisoner walks a Leaving pawn out once RetryAt passes or it idled
	// a day: the visit holds both off.
	r.Leaving, r.Idle, r.RetryAt = true, now, math.MaxUint64
	g.raiders[v.ID] = r
	if st.Status != health.Alive {
		return true // down: it waits for help (or capture)
	}
	pos, _ := g.positions.Get(v.ID)
	j, _ := g.jobs.Get(v.ID)
	if !v.Leaving && now >= v.LeaveAt {
		v.Leaving, v.RetryAt = true, 0
		g.reserve.ReleaseOwner(v.ID)
		g.jobs.Set(v.ID, job.Record{})
		j = job.Record{}
	}
	if v.Leaving && (g.nearBorder(pos) || j.Kind == job.Move && j.State == job.Complete || now >= v.LeaveAt+visitorGiveUp) {
		g.departVisitor(v.ID)
		return false
	}
	if v.Ally && !v.Leaving {
		return g.allyStep(v, pos, j, now) // allies.go: fight, else stroll
	}
	if j.State == job.Complete || j.State == job.Fail || j.State == job.Do && j.Kind != job.Visit {
		// a finished walk, or another system's job left standing (a step
		// aside): nothing runs a raiderIDs pawn's job but this
		g.reserve.ReleaseOwner(v.ID)
		j = job.Record{}
		g.jobs.Set(v.ID, j)
	}
	if v.Leaving {
		return g.visitorLeave(v, pos, j, now)
	}
	switch {
	case j.Kind == job.Visit && j.State == job.Do:
		g.lingerVisit(v.ID, j)
	case j.Active():
	case g.traderHeld(v.ID): // trade_caravan.go: a trader stays put while traded with
	case now >= v.RetryAt:
		if !g.startStroll(v.ID) {
			v.RetryAt = now + visitorRetry
		}
	}
	return true
}

// visitorLeave walks a leaving visitor to the nearest edge band
// (visitorStep sends it home once it is there).
func (g *Game) visitorLeave(v *visitorRec, pos Position, j job.Record, now uint64) bool {
	if j.Active() || now < v.RetryAt {
		return true
	}
	ex, ey := nearestEdge(pos, g.grid.Width(), g.grid.Height())
	x, y, ok := g.insetLandNear(ex, ey, nil)
	if !ok {
		x, y, ok = g.walkableNear(ex, ey, visitorSpotR) // a wet edge: any way out
	}
	if ok && g.startJob(v.ID, job.Record{Kind: job.Move, TX: x, TY: y}) {
		return true
	}
	v.RetryAt = now + visitorRetry
	return true
}

// nearBorder reports p is within visitorInset of the map border.
func (g *Game) nearBorder(p Position) bool {
	w, h := g.grid.Width(), g.grid.Height()
	return min(p.X, p.Y, w-1-p.X, h-1-p.Y) <= visitorInset
}

// departVisitor: visitor id walks off the map; it stays a known person.
func (g *Game) departVisitor(id entity.EntityID) {
	g.setWhere(id, whereOffMap)
	g.removeRaider(id) // raids.go: off the map
	g.invalidateCreatures()
}

// startStroll sends visitor id to a spot near the colony to linger.
func (g *Game) startStroll(id entity.EntityID) bool {
	hx, hy := g.home()
	r := g.stream(streamWorld)
	rg := g.caravanStrollRange(id) // trade_caravan.go: a trader caravan camps closer
	x := hx + r.IntN(2*rg+1) - rg
	y := hy + r.IntN(2*rg+1) - rg
	tx, ty, ok := g.walkableNear(x, y, 3)
	if !ok || !g.inset(tx, ty) {
		return false
	}
	return g.startJob(id, job.Record{Kind: job.Visit, TX: tx, TY: ty, WorkLeft: visitorLinger})
}

// lingerVisit counts a visitor's stay at its spot down; the visit ends
// when it is over.
func (g *Game) lingerVisit(id entity.EntityID, j job.Record) {
	if j.WorkLeft > 0 {
		j.WorkLeft--
	}
	if j.WorkLeft == 0 {
		j.State = job.Complete
	}
	g.jobs.Set(id, j)
}

// doVisit runs a visit job for a pawn on pawnIDs (phaseJobs): the same
// linger.
func (g *Game) doVisit(id entity.EntityID, j job.Record) { g.lingerVisit(id, j) }

// visitorIDs lists the visitors on the map, ascending.
func (g *Game) visitorIDs() []entity.EntityID {
	if g.worldSt == nil {
		return nil
	}
	out := make([]entity.EntityID, 0, len(g.worldSt.visitors))
	for _, v := range g.worldSt.visitors {
		out = append(out, v.ID)
	}
	slices.Sort(out)
	return out
}

// wandererFits reports whether a wanderer can join: a set-up colony that
// is not yet large.
func (g *Game) wandererFits(now uint64) bool {
	return g.hasSetup() && len(g.pawnIDs) > 0 && len(g.pawnIDs) < wandererMaxColony
}

// wandererJoins brings in a generated human who walks to the colony and
// joins it.
func (g *Game) wandererJoins(now uint64) bool {
	ex, ey, ok := g.worldEdge()
	if !ok {
		return false
	}
	x, y, ok := g.insetLandNear(ex, ey, nil)
	if !ok {
		return false
	}
	p := pawngen.Generate(g.worldSeed()^wandererSalt, int(g.entities.Peek()), 0, pawngen.Rules{})
	id, err := g.spawnGenerated(p, x, y) // skills.go
	if err != nil {
		return false
	}
	g.ensurePerson(id)
	hx, hy := g.home()
	if tx, ty, ok := g.walkableNear(hx, hy, 4); ok {
		g.cmdMove(id, tx, ty)
	}
	g.letter(letterGood, "Wanderer joins", g.personName(id)+", a wanderer, has asked to join the colony.", id)
	g.addHistory("joined", g.personName(id)+" wandered in and joined the colony.", id)
	return true
}
