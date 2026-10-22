package game

import "rimworld-go/internal/domain/sim/entity"

// Frame inspector read model and blueprint cancelling (UI-015).

// cancelFrame removes blueprint/frame id; its delivered materials go back on
// the map. (SIM-028 work: refund, reservations, haul shares.)
func (g *Game) cancelFrame(id entity.EntityID) {
	if _, ok := g.frames.Get(id); !ok {
		return
	}
	g.frames.Delete(id)
	kept := g.frameIDs[:0]
	for _, fid := range g.frameIDs {
		if fid != id {
			kept = append(kept, fid)
		}
	}
	g.frameIDs = kept
}
