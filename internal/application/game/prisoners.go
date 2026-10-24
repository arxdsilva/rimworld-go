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

// prisonerView builds prisoner id's tab (M8).
func (g *Game) prisonerView(id entity.EntityID) (PrisonerView, bool) { return PrisonerView{}, false }

// cmdCapture orders captor to take the downed target prisoner (M8).
func (g *Game) cmdCapture(captor, target entity.EntityID) {}

// cmdArrest orders arrester to arrest target (M8).
func (g *Game) cmdArrest(arrester, target entity.EntityID) {}

// cmdSetPrisonerMode sets prisoner's interaction mode (M8).
func (g *Game) cmdSetPrisonerMode(prisoner entity.EntityID, mode uint8) {}

// cmdSetBedForPrisoners marks bed for prisoners (M8).
func (g *Game) cmdSetBedForPrisoners(bed entity.EntityID, on bool) {}

// cmdReleasePrisoner releases prisoner (M8).
func (g *Game) cmdReleasePrisoner(prisoner entity.EntityID) {}

// isPrisoner reports whether id is the colony's prisoner (M8).
func (g *Game) isPrisoner(id entity.EntityID) bool { return false }
