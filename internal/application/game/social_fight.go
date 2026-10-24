package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/entity"
)

// doBrawl runs a social fight (ENG-003, M6).
func (g *Game) doBrawl(id entity.EntityID, j job.Record) {
	g.endStubJob(id, j) // TODO(M6)
}
