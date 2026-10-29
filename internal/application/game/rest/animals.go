package rest

import (
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
)

// Animals is where tame animals sleep (BED-1): an animal sleeping spot or
// animal bed it holds (Building.Claimant is the animal), else the nearest
// free one inside its allowed area. The choice runs only when the animal
// beds down (the bucket decision's T6, game/animal_needs.go), and its own
// bed always comes first, so it never swaps beds between nights. A bed
// held by an animal no longer tame on the map counts as free.
type Animals struct {
	h     Host
	stats *Stats
}

// Bed gives tame animal id at (x, y) its bed: the cell to walk to, false
// when none can be had (it sleeps where it stands).
func (a *Animals) Bed(id entity.EntityID, x, y int) (bx, by int, ok bool) {
	var best entity.EntityID
	bestD := 1 << 30
	for _, bid := range a.h.BuildingIDs() {
		b, ok := a.h.Building(bid)
		if !ok || !b.Kind.AnimalSleeps() || b.Forbidden || !a.h.AreaAllows(id, b.X, b.Y) {
			continue
		}
		d := pathing.Dist(x, y, b.X, b.Y)
		switch {
		case b.Claimant == uint64(id):
			d = -1 // its own bed first
		case b.Claimant != 0 && a.h.TameAnimal(entity.EntityID(b.Claimant)):
			continue // another animal's
		}
		if d < bestD || (d == bestD && bid < best) {
			best, bestD = bid, d
		}
	}
	if best == 0 {
		return 0, 0, false
	}
	b, _ := a.h.Building(best)
	if b.Claimant != uint64(id) {
		a.h.ReleaseBed(id) // moving to a new bed gives up the old one
		b.Claimant = uint64(id)
		a.h.SetBuilding(best, b)
	}
	return b.X, b.Y, true
}

// SleepRate is the rest per hour sleeping animal id regains, from base
// (the ground's rate): a tame one lying in its own animal bed has it
// scaled by the bed's rest effectiveness over the ground's. A wild one
// (tame false) holds no bed: base, with no lookup.
func (a *Animals) SleepRate(id entity.EntityID, tame bool, base int) int {
	if !tame {
		return base
	}
	bid := a.h.ObjectAt(a.h.Pos(id))
	if bid == 0 {
		return base
	}
	if b, ok := a.h.Building(bid); !ok || !b.Kind.AnimalSleeps() || b.Claimant != uint64(id) {
		return base
	}
	return base * a.stats.RestEffect(bid) / construction.GroundRestEffectiveness
}
