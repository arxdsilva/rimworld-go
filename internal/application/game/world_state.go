package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/entity"
)

// worldState is the world beyond the map (ENG-003, M9): caravans and the
// pawns away on them. nil until the first caravan forms; saved as World. The Game field is
// worldSt (g.world is the World read model).
type worldState struct{}

// worldSave is worldState as saved (filled by M9).
type worldSave struct {
	Version uint8 // format of this block; 0 = empty
}

// phaseWorld runs after phaseIncidents: caravans travel the world map.
func (g *Game) phaseWorld() {
	// TODO(M9)
}

// doKidnap carries a downed colonist off the map.
func (g *Game) doKidnap(id entity.EntityID, j job.Record) {
	g.endStubJob(id, j) // TODO(M8)
}

// doExitMap walks a pawn off the map edge.
func (g *Game) doExitMap(id entity.EntityID, j job.Record) {
	g.endStubJob(id, j) // TODO(M9)
}

// doVisit runs a visitor's stay.
func (g *Game) doVisit(id entity.EntityID, j job.Record) {
	g.endStubJob(id, j) // TODO(M8)
}

// exportWorld is the World save field (nil: nothing yet).
func (g *Game) exportWorld() *worldSave {
	if g.worldSt == nil {
		return nil
	}
	return nil // TODO(M9)
}

// importWorld restores exportWorld's state (nil: fresh).
func (g *Game) importWorld(s *worldSave) {
	if s == nil {
		return
	}
	// TODO(M9)
}
