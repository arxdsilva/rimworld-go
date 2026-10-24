package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/entity"
)

// socialDuty is called in phaseAI right after mentalBreakTick: true means
// pawn id is taken up by a gathering or a fight this tick (ENG-003, M5).
func (g *Game) socialDuty(id entity.EntityID, j job.Record) bool {
	return false // TODO(M5)
}

// doGather runs a gathering job (party, wedding).
func (g *Game) doGather(id entity.EntityID, j job.Record) {
	g.endStubJob(id, j) // TODO(M5)
}
