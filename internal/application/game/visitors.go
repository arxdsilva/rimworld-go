// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"fmt"
	"math"
	"slices"

	"rimworld-go/internal/application/game/care"
	"rimworld-go/internal/application/game/combat"
	"rimworld-go/internal/application/game/letters"
	"rimworld-go/internal/application/game/quest"
	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/application/game/visit"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/body"
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
	// shootBackHold is how long after its last wound a turned visitor keeps
	// shooting before it walks on (MAP-18, original tuning).
	shootBackHold = 300
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

// rebuildVisitGroups upgrades old visit records without touching running jobs.
func rebuildVisitGroups(g *Game) {
	if g.worldSt == nil || len(g.worldSt.visitors) == 0 {
		return
	}
	cars := caravansOf(&g.trade)
	legacy := make([]visit.LegacyCaravan, 0, len(cars))
	for _, c := range cars {
		legacy = append(legacy, visit.LegacyCaravan{Trader: c.Trader, Members: c.Members,
			Faction: g.factionRef(c.Faction), LeaveAt: c.LeaveAt})
	}
	groupIDs := visitsOf(g).RebuildLegacy(legacy, uint64(g.clock.Now()))
	for i, c := range cars {
		c.GroupID = groupIDs[i]
		if len(c.Cargo) == 0 {
			c.Cargo = visit.AssignCargo(c.Stock, []entity.EntityID{c.Trader})
		}
	}
}

// visitorsFit permits independent natural visitors while an existing
// party, including a long-lived externally managed guest, is on the map.
func (g *Game) visitorsFit(now uint64) bool {
	if !g.hasSetup() || len(g.pawnIDs) == 0 || len(g.visitingFactions()) == 0 {
		return false
	}
	if g.worldSt == nil {
		return true
	}
	return visit.RoomForNaturalVisitors(g.worldSt.visitGroups)
}

// visitorsArrive brings a visitor group in from a map edge.
func (g *Game) visitorsArrive(now uint64) bool { return visitsOf(g).Arrive(now) }

// A small fraction of ordinary visits bring a trading party. The frequency
// is original tuning; the real trade session is shared with the caravan.
func visitorIncidentArrive(g *Game, now uint64) bool {
	if g.stream(streamWorld).IntN(4) == 0 && smallTraderArrive(g, now) {
		return true
	}
	return g.visitorsArrive(now)
}

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
// is not yet large. With nobody left at all, one restarts it (wiki: Ending).
func (g *Game) wandererFits(now uint64) bool {
	return g.hasSetup() && (everyoneGone(g) || len(g.pawnIDs) > 0 && len(g.pawnIDs) < wandererMaxColony)
}

// wandererJoins posts a wanderer's offer to join (QST-170): accepted within
// a day, they walk in (wandererWalksIn); dismissed, the colony turned them
// away (a short mood thought); lapsed, they went on.
func (g *Game) wandererJoins(now uint64) bool {
	if _, _, ok := g.worldEdge(); !ok {
		return false
	}
	seed := (g.worldSeed() ^ wandererSalt ^ now) | 1
	if everyoneGone(g) { // nobody to answer: they restart the colony at once
		return g.wandererWalksIn(seed)
	}
	letterf(g, letters.Info, "Wanderer asks to join", 0, "%s, a wanderer, asks to join the colony. "+
		"Accept within a day to welcome them; dismiss this letter and they go their way.", quest.PreviewOf(seed, backstoriesOf(g)).Name)
	g.letters.EditLast(func(l *letters.Letter) { l.Wanderer, l.Until = seed, now+clock.TicksPerDay })
	return true
}

// wandererWalksIn brings the wanderer made from seed in from an edge to
// join the colony.
func (g *Game) wandererWalksIn(seed uint64) bool {
	ex, ey, ok := g.worldEdge()
	x, y, in := g.insetLandNear(ex, ey, nil)
	if !ok || !in {
		return false
	}
	id, ok := walkInColonist(g, quest.PawnOf(seed, backstoriesOf(g)), x, y)
	if !ok {
		return false
	}
	letterf(g, letters.Good, "Wanderer joins", id, "%s, a wanderer, has joined the colony.", g.personName(id))
	g.addHistory("joined", g.personName(id)+" wandered in and joined the colony.", id)
	return true
}

// walkInColonist stands generated person p on (x, y) as a colonist, walking
// to the colony's heart.
func walkInColonist(g *Game, p pawngen.Pawn, x, y int) (entity.EntityID, bool) {
	id, err := g.spawnGenerated(p, x, y) // skills.go
	if err != nil {
		return 0, false
	}
	g.ensurePerson(id)
	hx, hy := g.home()
	if tx, ty, ok := g.combat.Tiles.WalkableNear(hx, hy, 4); ok {
		g.cmdMove(id, tx, ty)
	}
	return id, true
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
			v.Leaving, v.RetryAt, v.LeaveAt, v.Healed = true, 0, now, true
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
	letterf(g, letters.Info, "Stranger asks to join", id, "%s, the stranger your colony took in and nursed back to health, asks to join you. "+
		"Accept to welcome them; dismiss this letter and they will go their way.", g.personName(id))
	g.letters.EditLast(func(l *letters.Letter) { l.Join = id })
}

// leavingSafe: no hostile raider stands on the map and the outdoor air
// where id stands is within joinUnsafeMargin of its comfort band.
func (h careGuests) leavingSafe(id entity.EntityID) bool {
	g := h.g
	if g.incidents.Fallout(uint64(g.clock.Now())) || slices.ContainsFunc(g.raiderIDs, g.standingRaider) {
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
	g.addHistory("joined", letterf(g, letters.Good, "Stranger joins", p, "%s, the stranger you nursed back to health, has joined the colony.", g.personName(p)), p)
	questSignal(g, quest.Signal{Kind: quest.SigPawnJoined, Pawn: p}) // a quest's guest joined (QST-058)
}

// visitHost is game/visit's roles (visit/deps.go) over small adapters.
type visitHost struct {
	visitBodies
	visitMovers
	visitGround
	visitArrivals
	visitFighters
	visitDiplomacy
	visitTurning
	visitCarrying
	raidFight // shared walk, strike, and breach paths
}

func newVisitHost(g *Game) visitHost {
	return visitHost{visitBodies{g}, visitMovers{g}, visitGround{g}, visitArrivals{g}, visitFighters{g}, visitDiplomacy{g}, visitTurning{g}, visitCarrying{g}, raidFight{g}}
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
func (h visitBodies) NeedsStep(id entity.EntityID, now uint64) {
	g := h.g
	j, _ := g.jobs.Get(id)
	if j.Kind == job.Sleep && j.State == job.Do {
		return
	}
	n, _ := g.need.Get(id)
	if n.Hunger <= needs.HungryAt && g.worldSt != nil {
		for i := range g.worldSt.visitors {
			r := &g.worldSt.visitors[i]
			if r.ID == id && r.Rations > 0 {
				r.Rations--
				n.Hunger = min(n.Hunger+needs.Full/2, needs.Full)
				break
			}
		}
	}
	g.need.Set(id, n)
}
func (h visitBodies) RestStep(id entity.EntityID, now uint64) bool {
	g := h.g
	j, _ := g.jobs.Get(id)
	n, _ := g.need.Get(id)
	if j.Kind == job.Sleep {
		if j.State == job.Goto {
			return true
		}
		if j.State == job.Do {
			return true // phaseJobs owns the actual sleep tick
		}
	}
	if n.Tired() {
		g.rest.InPlace(id)
		return true
	}
	return false
}
func (h visitBodies) GuestDied(id entity.EntityID)     { h.g.care.Guest.Died(id) }
func (h visitBodies) Standing(id entity.EntityID) bool { return h.g.standingPawn(id) }
func (h visitBodies) Name(id entity.EntityID) string   { return h.g.name(id) }
func (h visitBodies) Depart(id entity.EntityID) { // it stays a known person
	g := h.g
	if g.worldSt != nil && g.alive(id) {
		for _, r := range g.worldSt.visitors {
			if r.ID == id && r.Healed && !r.Turned && r.Faction > 0 {
				careGuests{g}.Goodwill(uint64(r.Faction), care.GoodwillHealed, care.ReasonHealed)
				break
			}
		}
	}
	g.setWhere(id, whereOffMap)
	g.removeRaider(id) // raids.go: off the map
	g.invalidateCreatures()
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
func (h visitMovers) Free(x, y int) bool                               { return h.g.dryWalkable(x, y) && !h.g.pawnOn(x, y) }
func (h visitMovers) Size() (int, int)                                 { return h.g.grid.Width(), h.g.grid.Height() }

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
func (h visitGround) Groups(create bool) *[]visit.Group {
	if h.g.worldSt == nil && !create {
		return nil
	}
	return &h.g.worldS().visitGroups
}
func (h visitGround) GroupID() uint64 {
	w := h.g.worldS()
	w.nextVisitGroup++
	return w.nextVisitGroup
}
func (h visitGround) MemberGone(id entity.EntityID, pack bool) (gone, harmed bool) {
	g := h.g
	if pack {
		a, ok := g.animals.Get(id)
		return !ok, !ok || a.Downed
	}
	p := g.personOf(id)
	if p == nil {
		return true, true
	}
	if p.Where == whereOffMap {
		return true, false
	}
	if p.Where == whereColony {
		return true, false // a quest guest recruited into the colony
	}
	if p.Where != whereVisitor && !g.onMap(id) {
		return true, true
	}
	if g.prison.Is(id) {
		return true, true
	}
	v, ok := g.vital.Get(id)
	if ok && v.Status == health.Dead {
		return true, true
	}
	if ok && v.Status == health.Downed && g.lastHit[id].Tick > 0 {
		return false, true // violent incapacitation counts toward the party's losses
	}
	return false, false
}
func (h visitGround) GroupDanger(id entity.EntityID) bool {
	return h.g.exposureDanger(id)
}

type visitArrivals struct{ g *Game }

func (h visitArrivals) NewPerson(ref int, f *faction.Faction) entity.EntityID {
	g := h.g
	id := g.entities.Next()
	p := pawngen.Generate(g.worldSeed()^visitorSalt, int(id), 0, pawngen.Rules{Tech: techTag(f), Backstories: backstoriesOf(g), Traits: traitsOf(g)})
	g.peopleReg().add(&person{ID: id, Bio: p, HasBio: true, Faction: ref, Where: whereVisitor})
	g.socState().sig[id] = personSig{}
	return id
}
func (h visitArrivals) PackAnimal(x, y int) (entity.EntityID, bool) {
	id, err := h.g.spawnAnimal(animal.Donkey, x, y)
	return id, err == nil
}

func (h visitArrivals) Place(id entity.EntityID, ref, x, y int, now uint64) {
	g := h.g
	g.raiderIDs = withID(g.raiderIDs, id) // prisoners.go
	g.positions.Set(id, Position{X: x, Y: y})
	g.names.Set(id, g.personOf(id).Bio.DisplayName())
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
	h.g.relativeLetter(letters.Info, "Relative arrives", kin, among)
}
func (h visitArrivals) Letter(kind, title string, focus entity.EntityID, format string, args ...any) {
	letterf(h.g, kind, title, focus, format, args...)
}

type visitFighters struct{ g *Game }

func (h visitFighters) RaiderIDs() []entity.EntityID           { return h.g.raiderIDs }
func (h visitFighters) StandingRaider(id entity.EntityID) bool { return h.g.standingRaider(id) }
func (h visitFighters) Arm(id entity.EntityID, k item.Kind)    { h.g.equip[id] = k }
func (h visitFighters) NearestColonist(x, y int) (entity.EntityID, int, int, bool) {
	id, p, ok := h.g.nearestColonist(Position{X: x, Y: y})
	return id, p.X, p.Y, ok
}
func (h visitFighters) HitAt(id entity.EntityID) uint64 { return h.g.lastHit[id].Tick }
func (h visitFighters) Trader(id entity.EntityID) bool {
	return caravanByTrader(&h.g.trade, id) != nil
}

type visitDiplomacy struct{ g *Game }

func (h visitDiplomacy) Factions() []faction.Faction              { return h.g.ng.factions }
func (h visitDiplomacy) Cooldown(id int) faction.Cooldowns        { return h.g.cooldownOf(id) }
func (h visitDiplomacy) SetCooldown(id int, cd faction.Cooldowns) { h.g.setCooldown(id, cd) }
func (h visitDiplomacy) RelDay(now uint64) int                    { return h.g.relDay(now) }
func (h visitDiplomacy) Sided(i int, now uint64)                  { queueRival(h.g, i, now) } // factions.go

// visitCarrying is visit.Carrying (visit/carry.go, MAP-23).
type visitCarrying struct{ g *Game }

func (h visitCarrying) Carrying(id entity.EntityID) entity.EntityID {
	return h.g.care.Carry.Patient(id)
}
func (h visitCarrying) Pick(id, t entity.EntityID) { h.g.care.Carry.Pick(id, t) }
func (h visitCarrying) PutDown(id entity.EntityID) { h.g.care.Carry.Drop(id) }
func (h visitCarrying) Beside(id entity.EntityID, x, y int) (int, int, bool) {
	return h.g.besideSpot(id, x, y)
}
func (h visitCarrying) Message(x, y int, format string, args ...any) {
	h.g.message(x, y, format, args...)
}

// visitTurning is visit.Turning (visit/turn.go, MAP-18): a turned visitor
// becomes a raider already fleeing, the way a prisoner breaking out does
// (prison_host.go), in a group of its own that is no raid.
type visitTurning struct{ g *Game }

func (h visitTurning) Enemy(id entity.EntityID) bool {
	r, ok := h.g.raiders[id]
	if !ok || r.Leaving || h.g.prison.Is(id) {
		return false
	}
	v, ok := h.g.vital.Get(id)
	return ok && v.Status != health.Dead
}

func (h visitTurning) TurnHostile(id entity.EntityID, now uint64) {
	g := h.g
	g.raidGroups = append(g.raidGroups, raid.Group{Size: 1, Over: true})
	v, _ := g.vital.Get(id)
	r := g.raiders[id]
	r.Group, r.Mode, r.Escaping, r.Leaving = len(g.raidGroups)-1, raid.Fleeing, true, false
	r.Idle, r.RetryAt, r.Hurt = 0, 0, v.Injury
	r.KidnapAt = math.MaxUint64 // it is leaving, not raiding
	if v.Status != health.Alive {
		r.Downed = now
	}
	g.raiders[id] = r
	g.setWhere(id, whereRaider)
	g.reserve.ReleaseOwner(id)
	g.jobs.Set(id, job.Record{})
	g.invalidateCreatures()
}

// ShootBack: a new wound makes an armed enemy shoot back at the nearest
// colonist it can hit, for shootBackHold after its last wound; only a wound
// starts this, so it can't flap. It never chases: a fight whose target is out
// of reach ends, and it walks on.
func (h visitTurning) ShootBack(id entity.EntityID, now uint64) {
	g := h.g
	v, _ := g.vital.Get(id)
	if v.Status != health.Alive {
		return
	}
	r, d := g.raiders[id], weaponDefOf(g, id)
	pos, _ := g.positions.Get(id)
	shot := v.Injury > r.Hurt
	if shot {
		r.Engaged = now
	}
	r.Hurt = v.Injury
	g.raiders[id] = r
	if j, _ := g.jobs.Get(id); j.Kind == job.Attack && j.Active() {
		tp, ok := g.positions.Get(j.Target)
		if !ok || !g.combat.Sight.CanHit(pos.X, pos.Y, d, tp.X, tp.Y) ||
			weapon.Distance(pos.X, pos.Y, tp.X, tp.Y) > 1 && now >= r.Engaged+shootBackHold {
			g.interrupt(id, job.Cancelled)
		}
		return
	}
	if !shot || !d.Ranged {
		return
	}
	if t, tp, ok := g.nearestColonist(pos); ok && g.combat.Sight.CanHit(pos.X, pos.Y, d, tp.X, tp.Y) {
		g.reserve.ReleaseOwner(id)
		g.jobs.Set(id, job.Record{Kind: job.Attack, Target: t, TX: tp.X, TY: tp.Y, State: job.Do, WorkLeft: combat.AimTicks(combatSkills{g}, id, d)})
	}
}

func (h visitTurning) Rescuing(id entity.EntityID) bool {
	if h.g.care.Carry.Carrier(id) != 0 {
		return true
	}
	v, _ := h.g.vital.Get(id)
	_, taken := h.g.reserve.EntityOwner(id)
	return taken && v.Status != health.Alive
}

func (h visitTurning) TraderTurned(ref int) bool {
	g := h.g
	changed := false
	for _, c := range caravansOf(&g.trade) {
		if g.factionRef(c.Faction) != ref {
			continue
		}
		c.Turned, changed = true, true
		if s := g.trade.session; s != nil && s.trader == c.Trader {
			g.trade.session = nil
		}
	}
	return changed
}

func (h visitGround) GroupClosed(id uint64) {
	g := h.g
	if c := caravanByGroup(&g.trade, id); c != nil {
		removeCaravan(&g.trade, c)
	}
}

func (h visitGround) GroupGift(gr visit.Group) {
	g := h.g
	if g.stream(streamTrade).IntN(100) >= 30 { // original tuning; wiki: Trade
		return
	}
	qty := 1 + g.stream(streamTrade).IntN(3)
	g.dropGoods(trade.Line{Kind: item.HerbalMedicine, Qty: qty}, gr.CampX, gr.CampY, false)
	g.message(gr.CampX, gr.CampY, "The visitors left a parting gift.")
}

// PackStep runs a caravan animal's actual movement after the normal
// animal survival/flee phase. Animals outside visiting groups use their
// existing AI without new work.
func (h visitGround) PackStep(id entity.EntityID, group visit.Group, now uint64) {
	g := h.g
	a, ok := g.animals.Get(id)
	if !ok || a.Downed {
		return
	}
	pos, _ := g.positions.Get(id)
	if group.Phase == visit.PhaseLeaving {
		if max(pos.X-group.ExitX, group.ExitX-pos.X, pos.Y-group.ExitY, group.ExitY-pos.Y) <= 1 {
			// Debit a normally departing carrier before marking it departed;
			// the removal path must not drop merchandise a second time.
			if c := caravanByGroup(&g.trade, group.ID); c != nil {
				carrierGone(g, c, id, false)
			}
			for i := range g.worldS().visitGroups {
				if gr := &g.worldSt.visitGroups[i]; gr.ID == group.ID {
					for j := range gr.Members {
						if gr.Members[j].ID == id {
							gr.Members[j].Departed = true
						}
					}
				}
			}
			g.leaveMap(id)
			return
		}
		if a.PathI >= len(a.Path) && now >= a.RetryAt {
			g.animalGoTo(id, &a, group.ExitX, group.ExitY)
		}
	} else {
		tx, ty := group.CampX, group.CampY
		if g.onMap(group.Leader) {
			p, _ := g.positions.Get(group.Leader)
			tx, ty = p.X, p.Y
		}
		if max(pos.X-tx, tx-pos.X, pos.Y-ty, ty-pos.Y) > 4 &&
			a.PathI >= len(a.Path) && now >= a.RetryAt {
			g.animalGoTo(id, &a, tx, ty)
		}
	}
	a.Mode = animal.Wander
	a.NextDecide = now + 180
	g.animals.Set(id, a)
}

// dismissVisitorCommand orders an actual conversation with the group's
// leader; walking and interruption use the existing forced-job system.
func dismissVisitorCommand(g *Game, pawn, target entity.EntityID) {
	if !g.isPawn(pawn) || !g.alive(pawn) || g.drafted(pawn) || g.capacity(pawn, body.Talking) <= 0 {
		return
	}
	gr, ok := visitsOf(g).GroupOf(target)
	if !ok || gr.Leader != target || gr.Phase == visit.PhaseLeaving ||
		(gr.Kind != visit.KindVisitor && gr.Kind != visit.KindCaravan && gr.Kind != visit.KindSmallTrader) ||
		!g.alive(target) || !g.visitor(target) {
		return
	}
	pos, ok := g.positions.Get(target)
	if !ok {
		return
	}
	tx, ty, ok := g.besideSpot(pawn, pos.X, pos.Y)
	if !ok {
		return
	}
	g.interrupt(pawn, job.Cancelled)
	g.startJob(pawn, job.Record{Kind: job.DismissVisitorGroup, Target: target,
		TX: tx, TY: ty, WorkLeft: 30, Forced: true})
}

func dismissVisitorJob(g *Game, pawn entity.EntityID, j job.Record) {
	gr, ok := visitsOf(g).GroupOf(j.Target)
	if !ok || gr.Leader != j.Target || gr.Phase == visit.PhaseLeaving || !g.alive(j.Target) {
		j.State = job.Fail
		g.jobs.Set(pawn, j)
		return
	}
	p, ok := g.positions.Get(j.Target)
	me, _ := g.positions.Get(pawn)
	if !ok {
		j.State = job.Fail
		g.jobs.Set(pawn, j)
		return
	}
	if max(p.X-me.X, me.X-p.X, p.Y-me.Y, me.Y-p.Y) > 1 {
		if tx, ty, ok := g.besideSpot(pawn, p.X, p.Y); ok {
			if tx != j.TX || ty != j.TY {
				g.startJob(pawn, job.Record{Kind: j.Kind, Target: j.Target,
					TX: tx, TY: ty, WorkLeft: j.WorkLeft, Forced: true})
				return
			}
		}
		j.State = job.Fail
		g.jobs.Set(pawn, j)
		return
	}
	if j.WorkLeft > 1 {
		j.WorkLeft--
		g.jobs.Set(pawn, j)
		return
	}
	visitsOf(g).LeaveGroup(gr.ID, visit.LeaveDismissed, uint64(g.clock.Now()))
	j.State = job.Complete
	g.jobs.Set(pawn, j)
}
