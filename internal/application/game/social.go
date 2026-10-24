package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/entity"
)

// phaseSocial runs after phaseJobs (ENG-003, M3): interactions between
// pawns, on a staggered cadence.
func (g *Game) phaseSocial() {
	// TODO(M3)
}

// onHarmed is called from hurt when pawn victim takes damage from by.
func (g *Game) onHarmed(victim, by entity.EntityID) {
	// TODO(M3)
}

// onRescued is called when rescuer lays patient in a bed.
func (g *Game) onRescued(rescuer, patient entity.EntityID) {
	// TODO(M3)
}

// doSocialRelax runs a social-recreation job.
func (g *Game) doSocialRelax(id entity.EntityID, j job.Record) {
	g.endStubJob(id, j) // TODO(M3)
}

// endStubJob ends a job whose milestone has not filled its runner yet,
// as phaseJobs ends an unknown kind.
func (g *Game) endStubJob(id entity.EntityID, j job.Record) {
	j.State = job.Complete
	g.jobs.Set(id, j)
}
