package game

import "rimworld-go/internal/domain/sim/entity"

// orderOpenCasket marks sealed casket id to be opened by a colonist
// (MAPGEN-3). Stub: VAULT-GAME implements the job, the grouped opening and
// the occupants.
func (g *Game) orderOpenCasket(id entity.EntityID) { _ = id }
