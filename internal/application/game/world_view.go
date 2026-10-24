package game

import (
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/world/planet"
)

// World tab read models beyond WorldMapView (ENG-003, M9). The UI reads
// them through a small consumer-owned interface; a simulation without
// them shows the caravan list only.

// WorldSiteView is one settlement a caravan may head for.
type WorldSiteView struct {
	Tile     int
	Name     string
	Faction  string
	Relation string // "friendly", "neutral" or "hostile"
	// Visitable: a caravan is received there (a human people, not hostile).
	Visitable bool
}

// WorldParams are the colony's world params, to rebuild its globe (ok
// false: this colony has no world map).
func (g *Game) WorldParams() (planet.Params, bool) {
	if g.colonyTile() < 0 {
		return planet.Params{}, false
	}
	return g.ng.setup.World, true
}

// WorldSites lists every settlement of the colony's peoples, faction by
// faction, capital first.
func (g *Game) WorldSites() []WorldSiteView {
	if g.colonyTile() < 0 {
		return nil
	}
	var out []WorldSiteView
	for i := range g.ng.factions {
		f := &g.ng.factions[i]
		if f.Defeated {
			continue
		}
		rel := "neutral"
		switch f.Relation {
		case faction.Hostile:
			rel = "hostile"
		case faction.Friendly:
			rel = "friendly"
		}
		for k, t := range f.Settlements {
			name := ""
			if k < len(f.SettlementNames) {
				name = f.SettlementNames[k]
			}
			if name == "" && g.worldSt != nil && g.worldSt.planet != nil {
				name = g.worldSt.planet.SettlementName(t) // never built from a read model
			}
			if name == "" {
				name = "a settlement of " + f.Name
			}
			out = append(out, WorldSiteView{Tile: t, Name: name, Faction: f.Name, Relation: rel,
				Visitable: f.Type.Human() && f.Relation != faction.Hostile})
		}
	}
	return out
}
