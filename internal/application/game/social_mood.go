package game

import (
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/sim/entity"
)

// socialThoughts are the social and host-system thoughts of pawn id
// (partner, grief, drugs, bonds, body), appended in thoughts (ENG-003).
func (g *Game) socialThoughts(id entity.EntityID) []mood.Thought {
	return nil // TODO(M3)
}

// smoothedMood is the mood moodOf reports: target, approached gradually
// (ENG-003, M3). The stub returns target.
func (g *Game) smoothedMood(id entity.EntityID, target int) int {
	return target // TODO(M3)
}
