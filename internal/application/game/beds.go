package game

import "rimworld-go/internal/domain/sim/entity"

// Couples' beds, bed ownership and lovin' live in the rest package
// (rest/beds.go, rest/lovin.go); the lovin' cooldown stays saved with the
// gatherings and its roll on the social stream (rest_host.go).

// BedForPrisoners reports bed is marked for prisoners (the inspector's
// toggle state; M8 owns the mark).
func (g *Game) BedForPrisoners(bed entity.EntityID) bool {
	b, ok := g.bldgs.Get(bed)
	return ok && b.ForPrisoners
}
