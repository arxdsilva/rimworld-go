package haul

import (
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
)

// WantsHarvest is true for a ripe food plant that carries a harvest mark.
// Zone crops get theirs when they ripen or when a zone is drawn over them
// (game.markRipeHarvest), so "Cancel harvest" holds until the next ripening.
func WantsHarvest(p plant.Plant) bool {
	return p.HarvestableNow() && p.Designated // FOOD-2: from 65 % growth (wiki: Plants)
}

// Harvester is what starting a harvest needs from the simulation (a
// consumer-owned role, .cursor/SOLID_GO.md §7).
type Harvester interface {
	// NearestRipe is the nearest plant WantsHarvest picks that pawn id may
	// take (claims, put-off plants and reach respected) and its cell; ok
	// false when there is none.
	NearestRipe(id entity.EntityID) (target entity.EntityID, x, y int, ok bool)
	// StartHarvest starts pawn id's harvest of plant target.
	StartHarvest(id, target entity.EntityID) bool
}

// Harvest starts pawn id's harvest of the nearest ripe marked plant (the
// Grow work type's first job); false when there is none or it can't start.
func Harvest(h Harvester, id entity.EntityID) bool {
	t, _, _, ok := h.NearestRipe(id)
	return ok && h.StartHarvest(id, t)
}
