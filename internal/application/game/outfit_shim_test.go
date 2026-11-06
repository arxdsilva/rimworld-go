package game

import (
	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/application/game/outfit"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// The dressing tests' names for game/outfit (apparel_test.go and friends
// predate the move; test files cost the package no budget).

const (
	dressCool  = outfit.Cool
	dressEvery = outfit.Every
)

func (g *Game) dressPlan(id entity.EntityID, full bool) (craft.Plan, bool) {
	return outfitOf(g).Plan(id, full)
}
func (g *Game) dress(id entity.EntityID) bool                     { return outfitOf(g).Dress(id) }
func (g *Game) apparelStock(validate bool) []entity.EntityID      { return outfitOf(g).Stock(validate) }
func (g *Game) apparelKinds() []item.Kind                         { return outfitOf(g).Kinds() }
func (g *Game) dressCooling(id entity.EntityID, s item.Slot) bool { return outfitOf(g).Cooling(id, s) }
