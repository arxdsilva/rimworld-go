package game

import "rimworld-go/internal/domain/sim/entity"

// Bio is a colonist's identity for the inspector's Bio tab (NG-1 §2.12,
// §5.4): bioOf over its bio with the live skills (levels, XP progress,
// passions). Pawns without a bio report false. Weekly learning is
// LearnedThisWeek. Owned by PAWN-G.
func (g *Game) Bio(id entity.EntityID) (BioView, bool) { return g.bioView(id) }
