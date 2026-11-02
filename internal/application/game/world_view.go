package game

import (
	"rimworld-go/internal/application/game/travel"
	"rimworld-go/internal/domain/world/planet"
)

// World tab read models beyond WorldMapView (ENG-003, M9). The UI reads
// them through a small consumer-owned interface; a simulation without
// them shows the caravan list only.

// WorldSiteView is one settlement a caravan may head for (game/travel).
type WorldSiteView = travel.SiteView

// WorldParams are the colony's world params, to rebuild its globe (ok
// false: this colony has no world map).
func (g *Game) WorldParams() (planet.Params, bool) {
	if g.colonyTile() < 0 {
		return planet.Params{}, false
	}
	return g.ng.setup.World, true
}

// WorldSites lists every settlement of the colony's peoples, faction by
// faction, capital first (game/travel).
func (g *Game) WorldSites() []WorldSiteView {
	if g.colonyTile() < 0 {
		return nil
	}
	return travel.SettlementViews(g.ng.factions, func(t int) string {
		if g.worldSt != nil && g.worldSt.planet != nil {
			return g.worldSt.planet.SettlementName(t) // never built from a read model
		}
		return ""
	})
}
