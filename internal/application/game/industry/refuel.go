package industry

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/sim/entity"
)

// Refuelling (PLAY-021, REF-1): which fuel a burner takes, how much it
// has room for, and the job that brings it. The game's haulers ask for
// burners under their threshold (game/fuel.go, assignRefuel); a player's
// order asks for any burner not full (game/orders.go, OrderRefuel). Both
// plan the trip here.

// Why a refuel can't be planned (player-facing, the order menu's reasons).
const (
	RefuseFull        = "already full"
	RefuseNoFuel      = "no free " // followed by the fuel's name
	RefuseUnreachable = "cannot reach"
)

// RefuelHost is what planning a refuel reads from the simulation.
type RefuelHost interface {
	Building(id entity.EntityID) (construction.Building, bool)
	// Fuel is burner id's fuel, in ticks of burning.
	Fuel(id entity.EntityID) int
	// Ingredient is the nearest stack pawn may take for in (forbidden,
	// rotten, carried and claimed stacks are skipped); 0 if none.
	Ingredient(pawn entity.EntityID, in recipe.Input) (entity.EntityID, item.Stack)
	// TankFeed is the nearest fish feed besides hay for a tank taking up
	// to need items; 0 if none.
	TankFeed(pawn entity.EntityID, need int) (entity.EntityID, item.Stack)
	// Beside is the free spot beside (x, y) pawn stands on to work it.
	Beside(pawn entity.EntityID, x, y int) (int, int, bool)
	// Spare is a stockpile cell near (x, y) that takes the rest of st
	// (HAUL-001); ok is false when none does.
	Spare(st item.Stack, x, y int) (int, int, bool)
}

// Plan is the refuel job pawn would start on burner bid, or why it can't:
// RefuseFull, RefuseNoFuel plus the fuel, or RefuseUnreachable. It does not
// check the burner's threshold, reservations or the pawn's work types.
func Plan(h RefuelHost, pawn, bid entity.EntityID) (job.Record, string) {
	return job.Record{}, "" // REF-1 contract stub: filled by the move out of game/fuel.go
}
