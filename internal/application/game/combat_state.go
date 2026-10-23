package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/entity"
)

// inCombat reports a colonist in a fight: drafted, attacking, or shaken
// after fleeing (a live calm window, combat.go defend). Movement keeps
// such colonists off each other's tiles (PLAY-010); outside combat they
// may share one.
func (g *Game) inCombat(id entity.EntityID) bool {
	if g.drafted(id) {
		return true
	}
	if j, ok := g.jobs.Get(id); ok && j.Kind == job.Attack && j.State != job.Fail {
		return true
	}
	until, ok := g.calm[id]
	return ok && uint64(g.clock.Now()) < until
}
