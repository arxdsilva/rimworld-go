package game

import "rimworld-go/internal/domain/sim/entity"

// Bio is a colonist's identity for the inspector's Bio tab (NG-1 §2.12,
// §5.4). Owned by PAWN-G.
func (g *Game) Bio(id entity.EntityID) (BioView, bool) { return BioView{}, false } // NG-1 stub (PAWN-G)
