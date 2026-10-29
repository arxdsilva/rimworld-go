package cover

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/world/plant"
)

// OfBuilding is the cover a building of kind k gives (None for the rest:
// floors, spots, traps, benches the wiki gives no value for). wiki: Cover
func OfBuilding(k construction.Kind) Kind {
	switch k {
	case construction.Sandbags:
		return Sandbags
	case construction.Barricade:
		return Barricade
	case construction.Wall:
		return Wall
	case construction.Door, construction.Autodoor:
		return Door
	case construction.Fence, construction.FenceGate:
		return Fence
	case construction.Stool:
		return Stool
	case construction.Table, construction.Bed, construction.DoubleBed, construction.HospitalBed:
		return Table
	case construction.Chair:
		return Chair
	case construction.SentryTurret:
		return Turret
	case construction.Autocannon, construction.UraniumSlugTurret:
		return HeavyTurret
	}
	return None
}

// OfPlant is the cover a plant of species s gives: a tree (any plant with
// a trunk), a saguaro-like cactus or a bush. grown false (a sapling) gives
// none. wiki: Cover
func OfPlant(s plant.Species, grown bool) Kind {
	switch {
	case !grown:
		return None
	case s == plant.Cactus:
		return Cactus
	case plant.DefOf(s).Trunk:
		return Tree
	case s == plant.BerryBush || s == plant.Shrub:
		return Bush
	}
	return None
}
