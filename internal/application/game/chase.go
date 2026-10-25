package game

import (
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/sim/entity"
)

// PLAY-017: the player is warned (and the game auto-pauses, threat_ui.go)
// when an animal starts chasing a colonist.

// chaseTarget is the colonist animal a is chasing (0 = none).
func (g *Game) chaseTarget(a animal.State) entity.EntityID {
	if a.Mode == animal.Attack && a.Target != 0 && g.isPawn(a.Target) && !g.isRaider(a.Target) {
		return a.Target
	}
	return 0
}

// noteChase is called where animal aid starts chasing colonist target.
func (g *Game) noteChase(aid, target entity.EntityID) {}
