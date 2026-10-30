package craft

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/recipe"
)

// What a pawn working a bill is doing, by the bench (game.Action*
// carries the same words).
const (
	ActionCraft   = "craft"
	ActionCook    = "cook"
	ActionTailor  = "tailor"
	ActionButcher = "butcher"
	ActionCremate = "cremate"
)

// Action names bill work at a bench of kind k: an electric twin works as
// its base bench (recipe.Base).
func Action(k construction.Kind) string {
	switch recipe.Base(k) {
	case construction.Stove, construction.Campfire, construction.DryingRack, construction.FermentingBarrel:
		return ActionCook
	case construction.TailoringBench:
		return ActionTailor
	case construction.ButcherTable, construction.ButcherSpot:
		return ActionButcher
	case construction.Pyre:
		return ActionCremate
	}
	return ActionCraft
}
