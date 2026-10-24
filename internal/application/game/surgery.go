package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/entity"
)

// assignSurgery sends doctor id to a queued operation (ENG-003, M1).
func (g *Game) assignSurgery(id entity.EntityID) bool {
	return false // TODO(M1)
}

// doSurgery runs an operation.
func (g *Game) doSurgery(id entity.EntityID, j job.Record) {
	g.endStubJob(id, j) // TODO(M1)
}
