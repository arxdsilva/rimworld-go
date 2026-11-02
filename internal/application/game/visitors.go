package game

import (
	"fmt"
	"math"
	"slices"

	"rimworld-go/internal/application/game/care"
	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/application/game/visit"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Visitors and wanderers (ENG-003, M9). The visit itself (groups walking
// in, strolling, leaving; military aid among them) lives in game/visit
// over the adapters below; the records are worldSt.visitors. A wanderer
// is one generated human who walks in and joins the colony.

const (
	visitorSalt  = 0x715170
	wandererSalt = 0x3A4D3E
	// visitorRange is how far from the colony's heart visitors stroll.
	visitorRange = visit.StrollRange
	// visitorGroup is the raid group a visitor belongs to: none.
	visitorGroup = math.MaxInt32
	// wandererMaxColony: no wanderer joins a colony this large.
	wandererMaxColony = 16
)

// visitorRec is one visitor on the map (game/visit).
type visitorRec = visit.Rec

// visitsOf is game/visit over the game (its host built once per world
// state).
func visitsOf(g *Game) visit.Visits {
	if w := g.worldSt; w != nil {
		if w.visitH == nil {
			w.visitH = newVisitHost(g)
		}
		return visit.Of(w.visitH)
	}
	return visit.Of(newVisitHost(g))
}

// visitingFactions are the factions (1 + index) that may send visitors.
func (g *Game) visitingFactions() []int { return visitsOf(g).Factions() }

// visitorsFit reports whether a friendly visitor group can arrive: a
// colony with a friendly or neutral people and no visitors already here.
func (g *Game) visitorsFit(now uint64) bool {
	if !g.hasSetup() || len(g.pawnIDs) == 0 || len(g.visitingFactions()) == 0 {
		return false
	}
	return g.worldSt == nil || len(g.worldSt.visitors) == 0
}

// visitorsArrive brings a visitor group in from a map edge.
func (g *Game) visitorsArrive(now uint64) bool { return visitsOf(g).Arrive(now) }

// insetLandNear is a free dry tile near (x, y) in from the border.
func (g *Game) insetLandNear(x, y int, used [][2]int) (int, int, bool) {
	return visitsOf(g).InsetLand(x, y, used)
}

// spawnVisitor puts one new visitor of faction fac on (x, y).
func (g *Game) spawnVisitor(fac int, f *faction.Faction, x, y int, now uint64) entity.EntityID {
	return visitsOf(g).Spawn(fac, f, x, y, now)
}

// setVisitorStay makes visitor id leave at leaveAt.
func (g *Game) setVisitorStay(id entity.EntityID, leaveAt uint64) { visitsOf(g).SetStay(id, leaveAt) }

// doVisit runs a visit job for a pawn on pawnIDs (phaseJobs): the same
// linger as a visitor's.
func (g *Game) doVisit(id entity.EntityID, j job.Record) { visitsOf(g).Linger(id, j) }

// visitorIDs lists the visitors on the map, ascending.
func (g *Game) visitorIDs() []entity.EntityID {
	if g.worldSt == nil {
		return nil
	}
	return visitsOf(g).IDs()
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

// Leave turns recovered guest id for home now, with its give-up time to walk
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

// visitHost is game/visit's roles (visit/deps.go) over small adapters.
type visitHost struct {
	visitBodies
	visitMovers
	visitGround
	visitArrivals
	visitFighters
	visitDiplomacy
}

func newVisitHost(g *Game) visitHost {
	return visitHost{visitBodies{g}, visitMovers{g}, visitGround{g}, visitArrivals{g}, visitFighters{g}, visitDiplomacy{g}}
}

type visitBodies struct{ g *Game }

func (h visitBodies) Present(id entity.EntityID) bool {
	_, ok := h.g.raiders[id]
	p := h.g.personOf(id)
	return ok && !h.g.prison.Is(id) && p != nil && p.Where == whereVisitor
}
func (h visitBodies) Status(id entity.EntityID) health.Status {
	st, _ := h.g.vital.Get(id)
	return st.Status
}
func (h visitBodies) Hold(id entity.EntityID, now uint64) { // prison.Live walks a Leaving pawn out once RetryAt passes or it idled a day
	r := h.g.raiders[id]
	r.Leaving, r.Idle, r.RetryAt = true, now, math.MaxUint64
	h.g.raiders[id] = r
}
func (h visitBodies) GuestStep(id entity.EntityID) bool { return h.g.care.Guest.Step(id) }
func (h visitBodies) GuestDied(id entity.EntityID)      { h.g.care.Guest.Died(id) }
func (h visitBodies) Standing(id entity.EntityID) bool  { return h.g.standingPawn(id) }
func (h visitBodies) Name(id entity.EntityID) string    { return h.g.name(id) }
func (h visitBodies) Depart(id entity.EntityID) { // it stays a known person
	h.g.setWhere(id, whereOffMap)
	h.g.removeRaider(id) // raids.go: off the map
	h.g.invalidateCreatures()
}

type visitMovers struct{ g *Game }

func (h visitMovers) Pos(id entity.EntityID) (int, int) {
	p, _ := h.g.positions.Get(id)
	return p.X, p.Y
}
func (h visitMovers) Job(id entity.EntityID) job.Record                { j, _ := h.g.jobs.Get(id); return j }
func (h visitMovers) SetJob(id entity.EntityID, rec job.Record)        { h.g.jobs.Set(id, rec) }
func (h visitMovers) Release(id entity.EntityID)                       { h.g.reserve.ReleaseOwner(id) }
func (h visitMovers) StartJob(id entity.EntityID, rec job.Record) bool { return h.g.startJob(id, rec) }
func (h visitMovers) WalkableNear(x, y, r int) (int, int, bool) {
	return h.g.combat.Tiles.WalkableNear(x, y, r)
}
func (h visitMovers) Free(x, y int) bool { return h.g.dryWalkable(x, y) && !h.g.pawnOn(x, y) }
func (h visitMovers) Size() (int, int)   { return h.g.grid.Width(), h.g.grid.Height() }

type visitGround struct{ g *Game }

func (h visitGround) Home() (int, int)                   { return h.g.home() }
func (h visitGround) Edge() (int, int, bool)             { return h.g.worldEdge() }
func (h visitGround) TraderHeld(id entity.EntityID) bool { return h.g.traderHeld(id) }
func (h visitGround) StrollRange(id entity.EntityID) int { return h.g.caravanStrollRange(id) }
func (h visitGround) TraderGone(id entity.EntityID)      { h.g.traderVisitorGone(id) }
func (h visitGround) WorldRoll(n int) int                { return h.g.stream(streamWorld).IntN(n) }
func (h visitGround) TradeRoll(n int) int                { return h.g.stream(streamTrade).IntN(n) }
func (h visitGround) Records(create bool) *[]visit.Rec {
	if h.g.worldSt == nil && !create {
		return nil
	}
	return &h.g.worldS().visitors
}

type visitArrivals struct{ g *Game }

func (h visitArrivals) NewPerson(ref int, f *faction.Faction) entity.EntityID {
	g := h.g
	id := g.entities.Next()
	p := pawngen.Generate(g.worldSeed()^visitorSalt, int(id), 0, pawngen.Rules{Tech: techTag(f)})
	g.peopleReg().add(&person{ID: id, Bio: p, HasBio: true, Faction: ref, Where: whereVisitor})
	g.socState().sig[id] = personSig{}
	return id
}
func (h visitArrivals) Place(id entity.EntityID, ref, x, y int, now uint64) {
	g := h.g
	g.raiderIDs = withID(g.raiderIDs, id) // prisoners.go
	g.positions.Set(id, Position{X: x, Y: y})
	g.names.Set(id, bioName(g.personOf(id).Bio))
	g.jobs.Set(id, job.Record{})
	g.need.Set(id, needs.NewFull())
	g.vital.Set(id, health.Record{Status: health.Alive})
	g.raiders[id] = raid.Raider{Group: visitorGroup, Arrived: now, Leaving: true, Idle: now, RetryAt: math.MaxUint64}
	g.invalidateCreatures()
}
func (h visitArrivals) RelativeRoll() bool { return h.g.relativeRoll(relativeVisitPermille) } // relative_world.go
func (h visitArrivals) RelativeFor(ref int, f *faction.Faction) entity.EntityID {
	return h.g.relativeFor(ref, whereVisitor, techTag(f))
}
func (h visitArrivals) RelativeLetter(kin entity.EntityID, among string) {
	h.g.relativeLetter(letterInfo, "Relative arrives", kin, among)
}
func (h visitArrivals) Letter(kind, title, text string, focus entity.EntityID) {
	h.g.letter(kind, title, text, focus)
}

type visitFighters struct{ g *Game }

func (h visitFighters) RaiderIDs() []entity.EntityID           { return h.g.raiderIDs }
func (h visitFighters) StandingRaider(id entity.EntityID) bool { return h.g.standingRaider(id) }
func (h visitFighters) Arm(id entity.EntityID, k item.Kind)    { h.g.equip[id] = k }
func (h visitFighters) Strike(id entity.EntityID, j job.Record, now uint64) {
	h.g.combat.Strikes.Do(id, j, now)
}
func (h visitFighters) NearestColonist(x, y int) (entity.EntityID, int, int, bool) {
	id, p, ok := h.g.nearestColonist(Position{X: x, Y: y})
	return id, p.X, p.Y, ok
}

type visitDiplomacy struct{ g *Game }

func (h visitDiplomacy) Factions() []faction.Faction              { return h.g.ng.factions }
func (h visitDiplomacy) Cooldown(id int) faction.Cooldowns        { return h.g.cooldownOf(id) }
func (h visitDiplomacy) SetCooldown(id int, cd faction.Cooldowns) { h.g.setCooldown(id, cd) }
func (h visitDiplomacy) RelDay(now uint64) int                    { return h.g.relDay(now) }
