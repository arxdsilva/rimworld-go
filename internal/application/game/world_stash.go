package game

import (
	"cmp"
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/entity"
)

// Pawns off the map (ENG-003, M9): a stashed colonist leaves pawnIDs and
// loses its map components (position, job, step timing, reservations,
// bed claim, carried things, draft, orders), so no pawnIDs-driven loop
// sees it; its identity stays (name, needs, health, body, drugs, skills,
// bio, mood memories, social edges, registry record, equipment), so the
// same id comes back with the same relationships. Caravans stash their
// members; M9b's kidnapping can reuse it.

// stashRec is one pawn off the map: who holds it (a caravan id, or
// another owner) and the cell it left from.
type stashRec struct {
	ID     entity.EntityID
	Holder entity.EntityID
	X, Y   int
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

// stashPawn takes colonist id off the map for holder (a caravan). It
// reports false when id is not a colonist on the map.
func (g *Game) stashPawn(id, holder entity.EntityID) bool {
	pos, ok := g.positions.Get(id)
	if !ok || !slices.Contains(g.pawnIDs, id) || g.isStashed(id) {
		return false
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
	delete(g.restedFree, id)
	g.care.Rest.Clear(id) // care/rest.go: "Rest when hurt" stays
	delete(g.sleptAt, id)
	delete(g.noComfort, id)
	delete(g.noFood, id)
	g.positions.Delete(id)
	g.jobs.Delete(id)
	g.motion.Delete(id)
	g.pawnIDs = withoutID(g.pawnIDs, id) // prisoners.go
	w := g.worldS()
	i, _ := slices.BinarySearchFunc(w.stashed, id, stashCmp)
	w.stashed = slices.Insert(w.stashed, i, stashRec{ID: id, Holder: holder, X: pos.X, Y: pos.Y})
	g.setWhere(id, whereCaravan)
	g.invalidateCreatures()
	return true
}

// unstashPawn brings stashed pawn id back onto the map at (x, y) as a
// colonist (idle, undrafted). It reports false when id is not stashed or
// the tile is not walkable.
func (g *Game) unstashPawn(id entity.EntityID, x, y int) bool {
	if !g.isStashed(id) || !g.grid.Walkable(x, y) {
		return false
	}
	g.dropStash(id)
	g.positions.Set(id, Position{X: x, Y: y})
	g.jobs.Set(id, job.Record{})
	g.pawnIDs = withID(g.pawnIDs, id) // prisoners.go: ascending, as movers needs
	g.setWhere(id, whereColony)
	g.invalidateCreatures()
	return true
}

// dropStash forgets stashed pawn id (back on the map, or dead away).
func (g *Game) dropStash(id entity.EntityID) {
	if g.worldSt == nil {
		return
	}
	g.worldSt.stashed = slices.DeleteFunc(g.worldSt.stashed, func(r stashRec) bool { return r.ID == id })
}
