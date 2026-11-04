package dossier

import (
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// Scene is what the selectors need that Reader does not carry: the pawn's
// allowed area, the food policies and the hour of the day. The host fills it
// from World() and the simulation.
type Scene interface {
	// AreaOf is the pawn's or tame animal's allowed-area ref (0 or -1:
	// unrestricted, -2: Home, else an area id).
	AreaOf(id entity.EntityID) int
	// FoodPolicies lists the colony's food policies.
	FoodPolicies() []game.FoodPolicyView
	// Hour is the hour of the day, 0..23.
	Hour() int
}
