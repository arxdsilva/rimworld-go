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

// Ripe is which plants a harvest search takes (HRV-003): zone crops are Grow
// work, wild plants Plant cut work.
type Ripe uint8

const (
	RipeCrops Ripe = 1 << iota
	RipeWild
)

// RipeFor is the plants of a worker with Grow and Plant cut on or off.
func RipeFor(grow, cut bool) Ripe {
	var r Ripe
	if grow {
		r |= RipeCrops
	}
	if cut {
		r |= RipeWild
	}
	return r
}

// Takes reports whether r includes a wild plant (wild) or a crop.
func (r Ripe) Takes(wild bool) bool {
	if wild {
		return r&RipeWild != 0
	}
	return r&RipeCrops != 0
}

// Harvester is what starting a harvest needs from the simulation (a
// consumer-owned role, .agents/specs/SOLID_GO.md §7).
type Harvester interface {
	// NearestRipe is the nearest plant of the kinds in takes that
	// WantsHarvest picks and pawn id may take (allowed area, claims,
	// put-off plants and reach respected) and its cell; ok false when there
	// is none.
	NearestRipe(id entity.EntityID, takes Ripe) (target entity.EntityID, x, y int, ok bool)
	// StartHarvest starts pawn id's harvest of plant target.
	StartHarvest(id, target entity.EntityID) bool
}

// Harvest starts pawn id's harvest of the nearest ripe marked plant of the
// kinds in takes (the first job of the Grow and Plant cut work types); false
// when there is none or it can't start.
func Harvest(h Harvester, id entity.EntityID, takes Ripe) bool {
	t, _, _, ok := h.NearestRipe(id, takes)
	return ok && h.StartHarvest(id, t)
}
