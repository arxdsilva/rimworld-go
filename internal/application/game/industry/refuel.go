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

// RefuelWork is the work a hauler spends feeding a burner once there.
const RefuelWork = 30

// Room is how many items of fuel k a burner of capacity capTicks holding
// fuel ticks takes before it is full (a part-burnt one rounds up); 0 for an
// item that is no fuel.
func Room(capTicks, fuel int, k item.Kind) int {
	per := FuelTicks(k)
	if per <= 0 {
		return 0
	}
	return max(0, (capTicks-fuel+per-1)/per)
}

// Plan is the refuel job pawn would start on burner bid, or why it can't:
// RefuseFull, RefuseNoFuel plus the fuel, or RefuseUnreachable. It does not
// check the burner's threshold, reservations or the pawn's work types.
func Plan(h RefuelHost, pawn, bid entity.EntityID) (job.Record, string) {
	b, ok := h.Building(bid)
	if !ok {
		return job.Record{}, RefuseUnreachable
	}
	spec, _ := FuelSpec(b.Kind)
	have := h.Fuel(bid)
	fuel := item.Wood
	if b.Kind == construction.ChemfuelGenerator || b.Kind == construction.PodLauncher {
		fuel = item.Chemfuel // AX-4, AX-9: they take chemfuel only
	} else if b.Kind == construction.FishTank {
		fuel = item.Hay // FISH-*: hay first; the tank feed has the rest
	}
	need := Room(spec.CapTicks, have, fuel)
	if need == 0 {
		return job.Record{}, RefuseFull
	}
	sid, st := h.Ingredient(pawn, recipe.Input{Kind: fuel, Qty: 1, Max: need})
	switch {
	case sid != 0:
	case fuel == item.Hay:
		if sid, st = h.TankFeed(pawn, need); sid == 0 {
			return job.Record{}, RefuseNoFuel + fuel.String()
		}
	case fuel == item.Chemfuel:
		return job.Record{}, RefuseNoFuel + fuel.String()
	default:
		fuel = item.EmberOil
		need = Room(spec.CapTicks, have, fuel)
		if sid, st = h.Ingredient(pawn, recipe.Input{Kind: fuel, Qty: 1, Max: need}); sid == 0 {
			return job.Record{}, RefuseNoFuel + item.Wood.String()
		}
	}
	sx, sy, ok := h.Beside(pawn, b.X, b.Y)
	if !ok {
		return job.Record{}, RefuseUnreachable
	}
	rec := job.Record{Kind: job.Refuel, Target: sid, TX: st.X, TY: st.Y, Dest: bid,
		DestX: sx, DestY: sy, WorkLeft: RefuelWork, Count: need}
	if st.Qty > need {
		// HAUL-001: a full load when a stockpile cell takes the rest
		if px, py, ok := h.Spare(st, b.X, b.Y); ok {
			rec.Count = max(need, min(st.Qty, item.CarryLimit))
			rec.Route, rec.SpareX, rec.SpareY, rec.HasSpare = []entity.EntityID{bid}, px, py, true
		}
	}
	return rec, ""
}
