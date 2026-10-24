package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/drug"
	"rimworld-go/internal/domain/sim/entity"
)

// Drugs (ENG-003, M2): g.drugs holds each colonist's drug state; nil until
// the first dose. Saved as Drugs.

// assignDrugUse sends colonist id to take a drug its policy allows or its
// addiction craves; called before idle recreation in phaseAI.
func (g *Game) assignDrugUse(id entity.EntityID) bool {
	return false // TODO(M2)
}

// doIngest runs a drug-taking job.
func (g *Game) doIngest(id entity.EntityID, j job.Record) {
	g.endStubJob(id, j) // TODO(M2)
}

// exportDrugs is the Drugs save field in id order (nil: none).
func (g *Game) exportDrugs() []entry[drug.State] { return exportMap(g.drugs) }

// importDrugs restores exportDrugs' state.
func (g *Game) importDrugs(es []entry[drug.State]) { g.drugs = importMap(es) }
