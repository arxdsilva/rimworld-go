package game

import (
	"cmp"
	"slices"

	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/sim/entity"
)

// Pawns off the map (ENG-003, M9): a stashed colonist leaves pawnIDs and
// loses its map components (position, job, step timing, reservations,
// bed claim, carried things, draft, orders), so no pawnIDs-driven loop
// sees it; its identity stays (name, needs, health, body, drugs, skills,
// bio, mood memories, social edges, registry record, equipment), so the
// same id comes back with the same relationships. Caravans stash their
// members; caskets (AX-9) also prisoners and tame animals (its state in
// the record).

// stashRec is one pawn off the map: its holder (a caravan, a casket), the
// cell it left from on map Map (MAP-01), a prisoner's mark (with its
// raider record when it travels by caravan, MAP-01) or an animal's state
// (AX-9).
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

// everyoneGone reports no living colonist left under our control (wiki: Ending):
// none on a map, on the road, in a pod or casket, due to land or away on the
// ship. Prisoners, animals and the kidnapped do not count.
func everyoneGone(g *Game) bool {
	for _, m := range g.maps {
		if slices.ContainsFunc(m.pawnIDs, g.alive) {
			return false
		}
	}
	if slices.ContainsFunc(g.ng.pods, func(p podState) bool { return !p.Landed && len(p.Pawns) > 0 }) {
		return false
	}
	if g.worldSt == nil || g.worldSt.ship.Launched() {
		return g.worldSt == nil
	}
	return !slices.ContainsFunc(g.worldSt.stashed, func(r stashRec) bool {
		p := g.personOf(r.ID)
		return !r.Prisoner && r.Beast == nil && g.alive(r.ID) && (p == nil || p.Where != whereKidnapped)
	})
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
		a := *beast
		rec.Beast = &a
		g.removeAnimal(id)
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
		g.pawnIDs = withID(g.pawnIDs, id) // prisoners.go: ascending, as movers needs
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
