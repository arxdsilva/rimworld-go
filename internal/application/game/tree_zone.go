package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
)

// wantsZoneChop: a fully grown tree sown by a growing zone. Wild trees
// still need a Cut designation.
func wantsZoneChop(p plant.Plant) bool {
	return p.Zone && plant.DefOf(p.Species).Sowable && p.Ripe()
}

// zoneTreeChoppable: grown zone tree p that growers may fell: it still
// stands in a growing zone, and that zone's cutting is not turned off
// (PLAY-020). A tree left outside any zone (the zone was deleted or shrunk)
// is an ordinary tree again.
func (g *Game) zoneTreeChoppable(p plant.Plant) bool {
	if !wantsZoneChop(p) {
		return false
	}
	z, in := g.zoneAt(p.X, p.Y)
	return in && !z.NoCut
}

// assignZoneChop sends grower id to fell the nearest grown zone tree with
// the ordinary cut job (wood drops as for a wild tree); the emptied cell is
// then resown by assignSow.
func (g *Game) assignZoneChop(id entity.EntityID) bool {
	best := g.nearestPlant(id, job.Cut, g.zoneTreeChoppable)
	if best == 0 {
		return false
	}
	p, _ := g.plants.Get(best)
	return g.startJob(id, job.Record{Kind: job.Cut, Target: best, TX: p.X, TY: p.Y, WorkLeft: plant.DefOf(p.Species).CutWork})
}
