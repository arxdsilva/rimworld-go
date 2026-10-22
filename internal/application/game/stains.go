package game

import (
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/stain"
)

// Blood and ichor on the ground (SIM-020): hits, deaths and bleeding
// creatures stain tiles; stains fade over 7 days and wash off open ground
// in rain and snow (stain package). Deterministic and saved.

// StainView is one stained tile for presentation.
type StainView struct {
	X, Y     int
	Kind     string  // "blood" or "ichor"
	Amount   float64 // 0..1 strength
	AgeTicks uint64  // since the last fresh blood (drying)
	Variant  uint8
}

// stainState is the stains on the map and their fade/wash remainders.
type stainState struct {
	tiles map[[2]int]stain.Stain
	frac  map[[2]int]int          // fixed-point remainder per tile (stain.Fade/Wash)
	dripN map[entity.EntityID]int // tiles walked since the last drip, per creature
}

// stainKindOf is what creature id bleeds: hive insects ichor, else blood.
func (g *Game) stainKindOf(id entity.EntityID) stain.Kind {
	if a, ok := g.animals.Get(id); ok && animal.RosterOf(a.Species).Behaviour == animal.Hive {
		return stain.Ichor
	}
	return stain.Blood
}

// stamp adds blood of kind to tile (x, y). SIM-020 BS: implement (merge,
// Born, variant, cap).
func (g *Game) stamp(x, y int, kind stain.Kind, add int) {}

// stainHit stains a hit: the target's tile, and a spray beyond it away from
// the attacker for heavy hits. Called before hurt (the target still
// exists). SIM-020 BS: implement.
func (g *Game) stainHit(target entity.EntityID, from, to Position, dmg int) {}

// stainDeath pools blood where creature id died (pawns; called from die).
// SIM-020 BS: implement.
func (g *Game) stainDeath(id entity.EntityID) {}

// stainAnimalDeath pools blood (or ichor) where an animal of species s
// died at pos (called from killAnimal). SIM-020 BS: implement.
func (g *Game) stainAnimalDeath(s animal.Species, pos Position) {}

// stainStep drips blood behind a bleeding creature every 3rd tile it walks
// (called from stepPawn and stepAnimal with the tile it left). SIM-020 BS.
func (g *Game) stainStep(id entity.EntityID, from Position) {}

// stainDowned grows a pool under a downed, untended colonist once an hour
// (called each tick from phaseNeeds for downed pawns). SIM-020 BS.
func (g *Game) stainDowned(id entity.EntityID, now uint64) {}

// stainDecay fades every stain and washes those under the open sky while
// rain or snow falls (called every climate interval). SIM-020 BS.
func (g *Game) stainDecay() {}

// fillStainView copies the stains into the view, sorted y then x.
func (g *Game) fillStainView(w *WorldView) {} // SIM-020 BS

// exportStains is the stains for a save, sorted y then x.
func (g *Game) exportStains() []stain.Stain { return nil } // SIM-020 BS

// importStains installs saved stains (nil: an older save, no stains).
func (g *Game) importStains(s []stain.Stain) {} // SIM-020 BS
