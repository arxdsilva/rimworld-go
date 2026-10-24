package game

import (
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/sim/entity"
)

// romanceInteraction handles a romance, breakup or proposal interaction
// a started with b; false lets the pawn chitchat instead (M5).
func (g *Game) romanceInteraction(k social.Interaction, a, b entity.EntityID) bool { return false }
