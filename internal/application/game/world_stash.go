// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"cmp"
	"slices"
	"strings"

	"rimworld-go/internal/application/game/aging"
	"rimworld-go/internal/application/game/axview"
	"rimworld-go/internal/application/game/evlog"
	"rimworld-go/internal/application/game/letters"
	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/application/game/roster"
	"rimworld-go/internal/application/game/visit"
	"rimworld-go/internal/application/game/worldpawn"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/scenario"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// A stashed pawn loses map-local jobs, positions and reservations while
// preserving identity, health, skills and relationships across returns.
// Caravans and caskets may stash colonists, prisoners and tame animals.

// stashRec preserves the holder, origin, prisoner status or animal state.
type stashRec struct {
	ID       entity.EntityID
	Holder   entity.EntityID
	X, Y     int
	Prisoner bool
	Beast    *animal.State
	Map      uint32
	Raider   *raid.Raider
}

// stashCmp orders stash records by id (binary search).
func stashCmp(r stashRec, id entity.EntityID) int { return cmp.Compare(r.ID, id) }

// isStashed reports pawn id is off the map.
func (g *Game) isStashed(id entity.EntityID) bool {
	if g.worldSt == nil {
		return false
	}
	_, found := slices.BinarySearchFunc(g.worldSt.stashed, id, stashCmp)
	return found
}

// everyoneGone ignores prisoners, pets, kidnapped people and departed ships.
func everyoneGone(g *Game) bool {
	for _, m := range g.maps {
		if slices.ContainsFunc(m.pawnIDs, g.alive) {
			return false
		}
	}
	if slices.ContainsFunc(g.ng.pods, func(p podState) bool { return !p.Landed && len(p.Pawns) > 0 }) {
		return false
	}
	if g.worldSt == nil {
		return true
	}
	return !slices.ContainsFunc(g.worldSt.stashed, func(r stashRec) bool {
		p := g.personOf(r.ID)
		return !r.Prisoner && r.Beast == nil && g.alive(r.ID) && (p == nil || p.Where != whereKidnapped && p.Where != whereEscaped)
	})
}

// wanderersTitle is the letter that announces new wanderers.
const wanderersTitle = "New wanderers"

// wanderersTick starts or clears the last-colonist-gone wait.
func wanderersTick(g *Game, now uint64) {
	s := g.ng.setup
	if s == nil {
		return
	}
	if !everyoneGone(g) {
		s.GoneAt = 0
	} else if s.GoneAt == 0 {
		s.GoneAt = now
	}
}

// wanderersOpen is the option's state now (visit/wanderers.go).
func wanderersOpen(g *Game) (in, left int, ok bool) {
	s := g.ng.setup
	return visit.WanderersOpen(everyoneGone(g), s.GoneAt, uint64(g.clock.Now()), s.WandererGroups)
}

// wanderersMax is how many may set out together: as many as this game's
// scenario started with.
func wanderersMax(g *Game) int { return min(max(g.ng.fx.People, 1), scenario.MaxPeople) }

// fillWanderers fills the option's read model.
func fillWanderers(g *Game, ax *axview.World) {
	ax.WanderersIn, ax.WanderersLeft, ax.WanderersOK = wanderersOpen(g)
	ax.WanderersMax, ax.WanderersRules = wanderersMax(g), g.ng.fx.Rules
	ax.WanderersRules.Backstories = backstoriesOf(g) // the run's book (PLG-024)
	ax.WanderersRules.Traits = traitsOf(g)
}

// WandererPawn is the person seed makes under rules
// (WorldView.AX.WanderersRules): the picker's preview is exactly who
// command.NewWanderers brings for that seed.
func WandererPawn(seed uint64, rules pawngen.Rules) pawngen.Pawn {
	return pawngen.Generate(seed, 0, 0, rules)
}

// WandererBio is WandererPawn's card for the picker.
func WandererBio(seed uint64, rules pawngen.Rules) BioView {
	return roster.BioOf(WandererPawn(seed, rules), rules.Backstories, rules.Traits)
}

// cmdNewWanderers brings a new group in from an edge of the first colony map,
// the same way a wanderer comes, under the player's control (MAP-19; name:
// the chosen seeds, visit.ParseSeeds). That map always exists: abandoning the
// last colony leaves it, emptied (game/maps).
func cmdNewWanderers(g *Game, name string) {
	s := g.ng.setup
	if s == nil {
		g.worldNote("No new wanderers can come to this colony.")
		return
	}
	_, left, open := wanderersOpen(g)
	switch {
	case !everyoneGone(g):
		g.worldNote("New wanderers come only once everyone is gone.")
		return
	case left == 0:
		g.worldNote("No more wanderers will come.")
		return
	case !open:
		g.worldNote("New wanderers can't come until eight hours after the last colonist went.")
		return
	}
	seeds, why := visit.ParseSeeds(name, wanderersMax(g))
	if why != "" {
		g.worldNote("New wanderers can't set out: %s.", why)
		return
	}
	cur := g.mapState
	g.mapState = g.maps[0]
	defer func() { g.mapState = cur }()
	ids, names := walkInWanderers(g, seeds, g.ng.fx.Rules)
	if len(ids) == 0 {
		g.worldNote("New wanderers can't find a way in.")
		return
	}
	s.WandererGroups++
	s.GoneAt = 0
	g.addHistory("joined", letterf(g, letters.Good, wanderersTitle, ids[0], "%s came across what is left and mean to start again. They are yours to lead.", strings.Join(names, ", ")), ids...)
}

// walkInWanderers stands seeds' people on land near a map edge, walking in.
func walkInWanderers(g *Game, seeds []uint64, rules pawngen.Rules) (ids []entity.EntityID, names []string) {
	rules.Backstories = backstoriesOf(g)
	rules.Traits = traitsOf(g)
	ex, ey, ok := g.worldEdge()
	if !ok {
		return nil, nil
	}
	var used [][2]int
	for _, seed := range seeds {
		x, y, ok := g.insetLandNear(ex, ey, used)
		if !ok {
			break
		}
		used = append(used, [2]int{x, y})
		if id, ok := walkInColonist(g, WandererPawn(seed, rules), x, y); ok { // visitors.go
			ids, names = append(ids, id), append(names, g.personName(id))
		}
	}
	return ids, names
}

// stashPawn takes colonist, prisoner or tame animal id off the map for
// holder; false when id is none of those on the map.
func (g *Game) stashPawn(id, holder entity.EntityID) bool {
	pos, ok := g.positions.Get(id)
	prisoner, beast := g.prison.Is(id) && slices.Contains(g.raiderIDs, id), g.animals.Ref(id)
	if !ok || !slices.Contains(g.pawnIDs, id) && !prisoner && (beast == nil || !beast.Tame) || g.isStashed(id) {
		return false
	}
	rec := stashRec{ID: id, Holder: holder, X: pos.X, Y: pos.Y, Prisoner: prisoner, Map: g.id}
	if beast != nil {
		a := beast.State()
		if trainingAt(g, id).Task.Source != 0 {
			a.Path, a.PathI = nil, 0
			a.Errand = false
		}
		rec.Beast = &a
		g.detachAnimal(id)
		return addStash(g, rec)
	}
	g.ensurePerson(id)
	g.interrupt(id, job.Cancelled) // drops what it carries, frees its claims
	g.releaseBed(id)
	g.reserve.ReleaseOwner(id)
	g.releaseDelivery(id)
	g.care.Carry.Forget(id)
	delete(g.draft, id)
	delete(g.orderQueue, id)
	delete(g.calm, id)
	g.rest.Unmark(id)
	g.care.Rest.Clear(id) // care/rest.go: "Rest when hurt" stays
	delete(g.sleptAt, id)
	delete(g.noComfort, id)
	delete(g.noFood, id)
	g.positions.Delete(id)
	g.jobs.Delete(id)
	g.motion.Delete(id)
	g.pawnIDs, g.raiderIDs = withoutID(g.pawnIDs, id), withoutID(g.raiderIDs, id) // prisoners.go
	// a casket or a pod keeps what it is (AX-9); a prisoner on a caravan
	// takes its raider record along (MAP-01)
	if fleetOf(g).At(holder) != nil && prisoner {
		r := g.raiders[id]
		rec.Raider = &r
		delete(g.raiders, id)
	} else if fleetOf(g).At(holder) != nil {
		g.setWhere(id, whereCaravan)
	}
	return addStash(g, rec)
}

// addStash files stash record r (ascending by id).
func addStash(g *Game, r stashRec) bool {
	w := g.worldS()
	i, _ := slices.BinarySearchFunc(w.stashed, r.ID, stashCmp)
	w.stashed = slices.Insert(w.stashed, i, r)
	g.invalidateCreatures()
	return true
}

// unstashPawn brings stashed pawn id back onto the map at (x, y) as a
// colonist (idle, undrafted). It reports false when id is not stashed or
// the tile is not walkable.
func (g *Game) unstashPawn(id entity.EntityID, x, y int) bool {
	i, found := slices.BinarySearchFunc(g.worldS().stashed, id, stashCmp)
	if !found || !g.grid.Walkable(x, y) {
		return false
	}
	r := g.worldSt.stashed[i]
	dropStash(g, id)
	if r.Beast != nil { // AX-9: a tame animal, as it went
		putAnimal(g, id, *r.Beast, x, y)
		return true
	}
	g.positions.Set(id, Position{X: x, Y: y})
	g.jobs.Set(id, job.Record{})
	if r.Prisoner {
		if r.Raider != nil { // MAP-01: in its own group on the map it enters, out of any raid
			g.raidGroups = append(g.raidGroups, raid.Group{Over: true})
			r.Raider.Group = len(g.raidGroups) - 1
			g.raiders[id] = *r.Raider
		}
		g.raiderIDs = withID(g.raiderIDs, id)
	} else {
		addPawnID(g, id) // scripts.go: ascending, as movers needs, and the cadence index (PLG-060)
	}
	if p := g.personOf(id); p != nil && p.Where == whereCaravan {
		p.Where = whereColony
	}
	g.invalidateCreatures()
	return true
}

// dropStash forgets stashed pawn id (back on the map, or dead away).
func dropStash(g *Game, id entity.EntityID) {
	if g.worldSt == nil {
		return
	}
	g.worldSt.stashed = slices.DeleteFunc(g.worldSt.stashed, func(r stashRec) bool { return r.ID == id })
	delete(g.worldSt.worldPawnMothballed, id)
}

// putAnimal sets stashed or carried animal id back down at (x, y) as a.
func putAnimal(g *Game, id entity.EntityID, a animal.State, x, y int) {
	i, _ := slices.BinarySearch(g.animalIDs, id)
	g.animalIDs = slices.Insert(g.animalIDs, i, id)
	a.FeedClaim, a.Target, a.Attacker, a.NextThink = 0, 0, 0, uint64(g.clock.Now())+1
	g.positions.Set(id, Position{X: x, Y: y})
	g.animals.Set(id, a)
	g.invalidateCreatures()
}

// offMapPawns is worldpawn's view of the colony.
type offMapPawns struct{ g *Game }

func (h offMapPawns) Calendar() clock.CalendarPolicy { return h.g.clock.Calendar() }

func (h offMapPawns) Stashed() []entity.EntityID {
	if h.g.worldSt == nil {
		return nil
	}
	ids := make([]entity.EntityID, len(h.g.worldSt.stashed))
	for i, r := range h.g.worldSt.stashed {
		ids[i] = r.ID
	}
	return ids
}
func (h offMapPawns) Owned(id entity.EntityID) bool {
	g := h.g
	i, ok := slices.BinarySearchFunc(g.worldSt.stashed, id, stashCmp)
	if !ok || g.worldSt.stashed[i].Beast != nil {
		return false
	}
	p := g.personOf(id)
	return p == nil || p.Where != whereCaravan && p.Where != whereEscaped
}
func (h offMapPawns) Pod(id entity.EntityID) bool    { return h.g.worldSt.ax9.InTransportPod(id) }
func (h offMapPawns) Asleep(id entity.EntityID) bool { return h.g.worldSt.ax9.Asleep(id) }
func (h offMapPawns) Prepare() {
	if h.g.worldSt.ax9 == nil {
		travelOf(h.g) // tests and direct callers may enter the owner without phaseWorld first
	}
}
func (h offMapPawns) Vital(id entity.EntityID) (health.Record, bool) { return h.g.vital.Get(id) }
func (h offMapPawns) Body(id entity.EntityID) *body.Body             { return h.g.bodies[id] }
func (h offMapPawns) EnsureBody(id entity.EntityID, v *health.Record) *body.Body {
	return h.g.ensureBody(id, v)
}
func (h offMapPawns) Need(id entity.EntityID) (needs.Pair, bool) { return h.g.need.Get(id) }
func (h offMapPawns) SetNeed(id entity.EntityID, n needs.Pair)   { h.g.need.Set(id, n) }
func (h offMapPawns) Bio(id entity.EntityID) (pawngen.Pawn, bool) {
	p, ok := h.g.sk.bios[id]
	return p, ok
}
func (h offMapPawns) SetBio(id entity.EntityID, p pawngen.Pawn) { h.g.sk.bios[id] = p }
func (h offMapPawns) RebuildStats(id entity.EntityID)           { h.g.rebuildStats(id) }
func (h offMapPawns) BodyRoll(n int) int                        { return h.g.stream(streamBody).IntN(n) }
func (h offMapPawns) MedicalRoll(n int) int                     { return h.g.stream(streamMedical).IntN(n) }
func (h offMapPawns) NoMedicine() item.Kind                     { return noMedicine }
func (h offMapPawns) NeedsCare(b *body.Body) bool               { return needsCare(b) }
func (h offMapPawns) HasDisease(b *body.Body) bool              { return hasDisease(b) }
func (h offMapPawns) Poisoned(id entity.EntityID) bool          { return h.g.poisoned(id) }

func (h offMapPawns) DieMedical(id entity.EntityID, c health.Cause) { h.g.dieMedical(id, c) }
func (h offMapPawns) ClearPatient(id entity.EntityID)               { h.g.clearPatient(id) }
func (h offMapPawns) BodyChanged(id entity.EntityID, b *body.Body, v *health.Record) {
	h.g.bodyChanged(id, b, v)
}
func (h offMapPawns) UpdateDowned(id entity.EntityID, b *body.Body, v *health.Record) {
	h.g.updateDowned(id, b, v)
}
func (h offMapPawns) RollInfections(id entity.EntityID, b *body.Body, now uint64) {
	h.g.rollInfections(id, b, now)
}
func (h offMapPawns) StepConds(id entity.EntityID, b *body.Body, now uint64) bool {
	return h.g.stepConds(id, b, now)
}
func (h offMapPawns) StepDisease(id entity.EntityID, b *body.Body, v *health.Record, now uint64) {
	h.g.stepDisease(id, b, v, now)
}
func (h offMapPawns) StepFoodPoison(id entity.EntityID, b *body.Body, v *health.Record, now uint64) {
	h.g.stepFoodPoison(id, b, v, now)
}
func (h offMapPawns) StepAging(id entity.EntityID, now uint64) {
	aging.StepIDs(agingHost(h), now, []entity.EntityID{id})
}
func (h offMapPawns) DevelopAging(id entity.EntityID, k body.CondKind, part int, now uint64) {
	aging.Develop(agingHost(h), id, k, part, now)
}
func (h offMapPawns) LogScar(id entity.EntityID, part string) {
	h.g.logEvent(id, logEntry{Kind: evlog.Scarred, Name: "A wound on the " + part + " healed into a scar"})
}
func (h offMapPawns) Heal(id entity.EntityID, v *health.Record, now uint64) { h.g.heal(id, v, now) }
func (h offMapPawns) WakeFromAnaesthetic(id entity.EntityID, v *health.Record, now uint64) {
	h.g.wakeFromAnaesthetic(id, v, now)
}
func (h offMapPawns) TickDrugs(id entity.EntityID, now uint64)    { doseOf(h.g).Tick(id, now) }
func (h offMapPawns) DownedHunger(id entity.EntityID, now uint64) { h.g.downedHunger(id, now, false) }
func (h offMapPawns) DecayNeeds(id entity.EntityID, n needs.Pair, now uint64) needs.Pair {
	return h.g.decayNeeds(id, n, now)
}
func (h offMapPawns) DrainJoy(id entity.EntityID, now uint64)    { h.g.drainJoy(id, now) }
func (h offMapPawns) SkillsDaily(id entity.EntityID, now uint64) { h.g.skillsDaily(id, now) }
func (h offMapPawns) Down(id entity.EntityID, c health.Cause)    { h.g.down(id, c) }

// worldPawnCandidate reports a pawn the off-map pass ticks (worldpawn).
func worldPawnCandidate(g *Game, id entity.EntityID) bool {
	return g.worldSt != nil && worldpawn.Candidate(offMapPawns{g}, id)
}
