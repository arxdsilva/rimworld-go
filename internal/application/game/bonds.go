package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/entity"
)

// onTamed is called when tamer tames animal (ENG-003, M10).
func (g *Game) onTamed(tamer, animal entity.EntityID) {
	// TODO(M10)
}

// onAnimalDied is called from killAnimal before the animal is removed;
// by is its killer (0: none).
func (g *Game) onAnimalDied(animal, by entity.EntityID) {
	// TODO(M10)
}

// assignTrain sends handler id to train a tame animal.
func (g *Game) assignTrain(id entity.EntityID) bool {
	return false // TODO(M10)
}

// assignTendAnimal sends doctor id to tend a hurt tame animal.
func (g *Game) assignTendAnimal(id entity.EntityID) bool {
	return false // TODO(M10)
}

// doTrain runs a training session.
func (g *Game) doTrain(id entity.EntityID, j job.Record) {
	g.endStubJob(id, j) // TODO(M10)
}

// doTendAnimal tends a hurt animal.
func (g *Game) doTendAnimal(id entity.EntityID, j job.Record) {
	g.endStubJob(id, j) // TODO(M10)
}
