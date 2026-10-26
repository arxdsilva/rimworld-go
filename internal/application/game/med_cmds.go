package game

import "rimworld-go/internal/domain/sim/entity"

// PLAY-020 medical beds and bed rest (contract stubs: MEDSIM implements).

// cmdSetBedMedical keeps bed id for the sick and injured (on) or not; it
// changes that bed only.
func (g *Game) cmdSetBedMedical(id entity.EntityID, on bool) {}

// cmdSetPawnBedRest turns colonist id's rest when hurt on or off.
func (g *Game) cmdSetPawnBedRest(id entity.EntityID, on bool) {}
