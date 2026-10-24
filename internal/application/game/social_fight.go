package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/sim/entity"
)

// doBrawl runs a social fight (ENG-003, M6).
func (g *Game) doBrawl(id entity.EntityID, j job.Record) {
	g.endStubJob(id, j) // TODO(M6)
}

// maybeSocialFight rolls whether interaction k that a started with b
// turns into a fight (M6).
func (g *Game) maybeSocialFight(a, b entity.EntityID, k social.Interaction) {
	// TODO(M6)
}

// inSocialFight reports whether id is in a social fight (M6).
func (g *Game) inSocialFight(id entity.EntityID) bool { return false }
