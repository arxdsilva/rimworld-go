package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/prisoner"
	"rimworld-go/internal/domain/sim/entity"
)

// Prisoners (ENG-003, M8): g.prisoners holds each prisoner's state; nil
// until the first capture. Saved as Prisoners.

// assignWarden sends warden id to a prisoner who needs a visit or food.
func (g *Game) assignWarden(id entity.EntityID) bool {
	return false // TODO(M8)
}

// doWarden runs a warden's visit.
func (g *Game) doWarden(id entity.EntityID, j job.Record) {
	g.endStubJob(id, j) // TODO(M8)
}

// doCapture carries a downed pawn to a prisoner bed.
func (g *Game) doCapture(id entity.EntityID, j job.Record) {
	g.endStubJob(id, j) // TODO(M8)
}

// doArrest arrests a colonist or visitor.
func (g *Game) doArrest(id entity.EntityID, j job.Record) {
	g.endStubJob(id, j) // TODO(M8)
}

// doDeliverFood carries a meal to a prisoner.
func (g *Game) doDeliverFood(id entity.EntityID, j job.Record) {
	g.endStubJob(id, j) // TODO(M8)
}

// exportPrisoners is the Prisoners save field in id order (nil: none).
func (g *Game) exportPrisoners() []entry[prisoner.State] { return exportMap(g.prisoners) }

// importPrisoners restores exportPrisoners' state.
func (g *Game) importPrisoners(es []entry[prisoner.State]) { g.prisoners = importMap(es) }
