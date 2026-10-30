package game

import (
	"fmt"
	"math"
	"slices"

	"rimworld-go/internal/application/game/care"
	"rimworld-go/internal/application/game/raid"
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
// on raiderIDs as non-hostile pawns (raid.Raider.Leaving, as the colony's
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
	// Down: it was down last tick, so a death now is a death while down
	// (care.Hospitality.Died, SIM-063).
	Down bool
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
	g.raiders[id] = raid.Raider{Group: visitorGroup, Arrived: now, Leaving: true, Idle: now, RetryAt: math.MaxUint64}
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
	for i := range w.visitors {
		v := &w.visitors[i] // in place: careGuests.Leave updates the record mid-step
		if g.visitorStep(v, now) {
			kept = append(kept, *v)
			continue
		}
		g.traderVisitorGone(v.ID) // trade_caravan.go (PLAY-024): the trader's goods
		if v.Ally {
			g.allyGone(*v) // allies.go: a letter for one who fell
		}
		if st, _ := g.vital.Get(v.ID); v.Down && st.Status == health.Dead {
			g.care.Guest.Died(v.ID) // care/guest.go: died while down
		}
	}
	clear(w.visitors[len(kept):])
	w.visitors = kept
}

// visitorStep runs visitor v for one tick; false: it is no longer a
// visitor on the map (gone, dead, taken prisoner).
func (g *Game) visitorStep(v *visitorRec, now uint64) bool {
	r, ok := g.raiders[v.ID]
	if !ok || g.prison.Is(v.ID) {
		return false
	}
	if p := g.personOf(v.ID); p == nil || p.Where != whereVisitor {
		return false
	}
	st, _ := g.vital.Get(v.ID)
	if st.Status == health.Dead {
		return false
	}
	// prison.Live walks a Leaving pawn out once RetryAt passes or it idled
	// a day: the visit holds both off.
	r.Leaving, r.Idle, r.RetryAt = true, now, math.MaxUint64
	g.raiders[v.ID] = r
	if v.Down = st.Status != health.Alive; v.Down {
		return true // down: it waits for help (or capture)
	}
	if g.care.Guest.Step(v.ID) {
		return true // care/guest.go: rescued, it heals in the colony's bed
	}
	pos, _ := g.positions.Get(v.ID)
	j, _ := g.jobs.Get(v.ID)
	if !v.Leaving && now >= v.LeaveAt {
		v.Leaving, v.RetryAt = true, 0
		g.reserve.ReleaseOwner(v.ID)
		g.jobs.Set(v.ID, job.Record{})
		j = job.Record{}
	}
	if v.Leaving && (nearBorder(pos, g.grid.Width(), g.grid.Height()) || j.Kind == job.Move && j.State == job.Complete || now >= v.LeaveAt+visitorGiveUp) {
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
	if v.Leaving { // walk to the nearest edge band; the check above sends it home there
		if j.Active() || now < v.RetryAt {
			return true
		}
		ex, ey := nearestEdge(pos, g.grid.Width(), g.grid.Height())
		x, y, ok := g.insetLandNear(ex, ey, nil)
		if !ok {
			x, y, ok = g.combat.Tiles.WalkableNear(ex, ey, visitorSpotR) // a wet edge: any way out
		}
		if !ok || !g.startJob(v.ID, job.Record{Kind: job.Move, TX: x, TY: y}) {
			v.RetryAt = now + visitorRetry
		}
		return true
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

// nearBorder reports p is within visitorInset of the border of a w x h map.
func nearBorder(p Position, w, h int) bool { return min(p.X, p.Y, w-1-p.X, h-1-p.Y) <= visitorInset }

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
	tx, ty, ok := g.combat.Tiles.WalkableNear(x, y, 3)
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
	if tx, ty, ok := g.combat.Tiles.WalkableNear(hx, hy, 4); ok {
		g.cmdMove(id, tx, ty)
	}
	g.letter(letterGood, "Wanderer joins", g.personName(id)+", a wanderer, has asked to join the colony.", id)
	g.addHistory("joined", g.personName(id)+" wandered in and joined the colony.", id)
	return true
}

// careGuests is care.Guests (care/deps.go, SIM-063): the visitors on the
// map (traders and allies too) are guests, one of no faction a stranger.
// A recovered guest leaves by the visit's leave path; a stranger may ask
// to join by letter (accept: command.AcceptLetter; dismiss: decline).
type careGuests struct{ g *Game }

const (
	// joinUnsafeMargin: outdoor air this far (tenths of a degree) beyond a
	// stranger's comfort band makes leaving unsafe (original tuning).
	joinUnsafeMargin = 100
	// joinOfferWait: a stranger who asked waits this long for an answer.
	joinOfferWait = clock.TicksPerDay
)

func (h careGuests) IsGuest(id entity.EntityID) bool { return h.g.visitor(id) } // prisoners.go
func (h careGuests) FactionOf(id entity.EntityID) uint64 {
	if p := h.g.personOf(id); p != nil && p.Faction > 0 {
		return uint64(p.Faction)
	}
	return 0
}

// FactionName is guest id's faction for the read model ("" for a stranger
// or anyone who is no guest).
func (h careGuests) FactionName(id entity.EntityID) string {
	if f := h.g.factionOf(int(h.FactionOf(id))); f != nil && h.IsGuest(id) {
		return f.Name
	}
	return ""
}

func (h careGuests) Goodwill(fac uint64, delta int, reason string) {
	if f, g := h.g.factionOf(int(fac)), h.g; f != nil {
		faction.Adjust(f, delta, fmt.Sprintf("Day %d: %s (%+d goodwill)", g.relDay(uint64(g.clock.Now()))+1, reason, delta))
		g.worldNote("%s: %s (%+d goodwill).", f.Name, reason, delta)
	}
}

// Leave turns recovered guest id for home now, with visitorGiveUp to walk
// out before it slips away.
func (h careGuests) Leave(id entity.EntityID) {
	g, now := h.g, uint64(h.g.clock.Now())
	for i := range g.worldS().visitors {
		if v := &g.worldSt.visitors[i]; v.ID == id {
			v.Leaving, v.RetryAt, v.LeaveAt = true, 0, now
		}
	}
	g.reserve.ReleaseOwner(id)
	g.jobs.Set(id, job.Record{})
	pos, _ := g.positions.Get(id)
	g.message(pos.X, pos.Y, "%s, back on their feet, sets off for home.", g.personName(id))
}

// JoinOffer: recovered stranger id asks to join (care.JoinAskSafe, or
// JoinAskUnsafe when leaving is unsafe, rolled on the world stream): a
// letter carries the offer and it waits joinOfferWait; else it leaves.
func (h careGuests) JoinOffer(id entity.EntityID) {
	g, ask := h.g, care.JoinAskSafe
	if !h.leavingSafe(id) {
		ask = care.JoinAskUnsafe
	}
	if g.stream(streamWorld).IntN(1000) >= ask {
		h.Leave(id)
		return
	}
	h.offerJoin(id)
}

// offerJoin keeps stranger id a day and posts its offer to join.
func (h careGuests) offerJoin(id entity.EntityID) {
	g := h.g
	g.setVisitorStay(id, uint64(g.clock.Now())+joinOfferWait)
	g.letter(letterInfo, "Stranger asks to join", g.personName(id)+", the stranger your colony took in and nursed back to health, asks to join you. "+
		"Accept to welcome them; dismiss this letter and they will go their way.", id)
	g.letters.list[len(g.letters.list)-1].Join = id
}

// leavingSafe: no hostile raider stands on the map and the outdoor air
// where id stands is within joinUnsafeMargin of its comfort band.
func (h careGuests) leavingSafe(id entity.EntityID) bool {
	g := h.g
	if slices.ContainsFunc(g.raiderIDs, g.standingRaider) {
		return false
	}
	pos, _ := g.positions.Get(id)
	t, b := int(g.climate.Outdoor(g.clock.Now())+g.weatherTempDelta(pos.X, pos.Y)), g.bandsOf(id)
	return t >= b.Min-joinUnsafeMargin && t <= b.Max+joinUnsafeMargin
}

// join makes stranger p a colonist (prison_host.go: enlist).
func (h careGuests) join(p entity.EntityID) {
	g := h.g
	g.enlist(p)
	g.invalidateCreatures()
	text := g.personName(p) + ", the stranger you nursed back to health, has joined the colony."
	g.letter(letterGood, "Stranger joins", text, p)
	g.addHistory("joined", text, p)
}
