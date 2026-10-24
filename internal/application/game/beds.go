package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/entity"
)

// partnerBed is the bed and slot pawn id shares with its partner, consulted
// first in assignSleep (ENG-003, M5).
func (g *Game) partnerBed(id entity.EntityID) (entity.EntityID, int, bool) {
	return 0, 0, false // TODO(M5)
}

// sleepInPartnerBed sends pawn id to slot of bed bid; false leaves
// assignSleep to choose a bed as usual.
func (g *Game) sleepInPartnerBed(id, bid entity.EntityID, slot int) bool {
	return false // TODO(M5)
}

// doLovin runs a lovin' job.
func (g *Game) doLovin(id entity.EntityID, j job.Record) {
	g.endStubJob(id, j) // TODO(M5)
}
