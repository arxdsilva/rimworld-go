package game

import (
	"rimworld-go/internal/application/game/sense"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/sim/entity"
)

// game/sense's roles over the game: small adapters, pointer-shaped so that
// building a component where it is needed allocates nothing.

// senseOf is the habit rules and their components, built once (CSE).
func senseOf(g *Game) *sense.Sense {
	w := g.worldS()
	if w.sense == nil {
		w.sense = sense.New(sense.Roles{Gear: senseGear{g}})
	}
	return w.sense
}

// cleaningOf is game/sense's cleaning (SIM-021).
func cleaningOf(g *Game) sense.Cleaning {
	return sense.NewCleaning(senseFilth{g}, senseGround{g}, senseWalks{g}, senseLabor{g})
}

// wanderOf is game/sense's idle stroll.
func wanderOf(g *Game) sense.Wander { return sense.NewWander(senseWalks{g}, senseStrolls{g}) }

type senseFilth struct{ g *Game }

func (h senseFilth) Queue() []int32            { return h.g.filth.cleanQ }
func (h senseFilth) SetQueue(q []int32)        { h.g.filth.cleanQ = q }
func (h senseFilth) Dirt(i int) int            { return h.g.dirtAtIdx(i) }
func (h senseFilth) Stain(i int) int           { return h.g.stainAtIdx(i) }
func (h senseFilth) AddDirt(i, delta int)      { h.g.addDirt(i, delta) }
func (h senseFilth) SetStain(x, y, amount int) { h.g.setStainAmount(x, y, amount) }
func (h senseFilth) CleanFree(id entity.EntityID, x, y int) bool {
	if _, taken := h.g.reserve.TileOwner(x, y); taken {
		return false
	}
	if h.g.postponedFor(id, job.Clean, 0, x, y) { // SIM-027: out of reach lately
		return false
	}
	return h.g.inArea(id, x, y) // areas.go: the cleaner's allowed area
}

type senseGround struct{ g *Game }

func (h senseGround) Width() int             { return h.g.grid.Width() }
func (h senseGround) InBounds(x, y int) bool { return h.g.grid.InBounds(x, y) }
func (h senseGround) HomeCells() []uint8     { return h.g.homeArea.cells }

type senseWalks struct{ g *Game }

func (h senseWalks) Pos(id entity.EntityID) (int, int) {
	p, _ := h.g.positions.Get(id)
	return p.X, p.Y
}
func (h senseWalks) StartJob(id entity.EntityID, r job.Record) bool { return h.g.startJob(id, r) }
func (h senseWalks) SetJob(id entity.EntityID, j job.Record)        { h.g.jobs.Set(id, j) }
func (h senseWalks) Reachable(id entity.EntityID, x, y int) bool    { return h.g.reachable(id, x, y) }
func (h senseWalks) ReachableSpot(id entity.EntityID, x, y int) (int, int, bool) {
	return h.g.reachableSpot(id, x, y)
}
func (h senseWalks) PostponeTile(id entity.EntityID, k job.Kind, x, y int) {
	h.g.postponeTile(id, k, x, y)
}
func (h senseWalks) ClaimTile(id entity.EntityID, x, y int) { h.g.reserve.TryTile(id, x, y) }
func (h senseWalks) TileTaken(x, y int) bool {
	_, taken := h.g.reserve.TileOwner(x, y)
	return taken
}

type senseLabor struct{ g *Game }

func (h senseLabor) PawnIDs() []entity.EntityID       { return h.g.pawnIDs }
func (h senseLabor) IsRaider(id entity.EntityID) bool { return h.g.isRaider(id) }
func (h senseLabor) HasLabor(id entity.EntityID) bool {
	_, ok := h.g.labor.Get(id)
	return ok
}
func (h senseLabor) Priorities(id entity.EntityID) work.Priorities { return h.g.prioOf(id) }
func (h senseLabor) SetPriority(id entity.EntityID, t work.Type, v int) {
	h.g.setPriority(id, t, v)
}
func (h senseLabor) WorkUnits(id entity.EntityID) int { return h.g.workProgress(id, skNone, false) } // skills.go

type senseStrolls struct{ g *Game }

func (h senseStrolls) Roams(id entity.EntityID) bool { return h.g.temperamentOf(id) == Wanderer }
func (h senseStrolls) Roll(n int) int                { return h.g.rng.IntN(n) }
func (h senseStrolls) OpenTile(id entity.EntityID, x, y int) bool {
	g := h.g
	return g.grid.Walkable(x, y) && !g.bedCovering(x, y) && !g.idleFireBlocked(x, y) && g.inArea(id, x, y) // areas.go
}
