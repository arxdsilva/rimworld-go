// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"rimworld-go/internal/application/game/maps"
	"rimworld-go/internal/application/game/travel"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/sim/entity"
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
	if g.hasSetup() {
		return g.ng.setup.World, true
	}
	return g.worldSt.link.ParamsCopy(), true
}

// WorldSites lists named settlements using the already built planet.
func (g *Game) WorldTerritories() []int { return planet.Territories(g.worldPlanet()) }

func (g *Game) WorldSites() []WorldSiteView {
	if g.colonyTile() < 0 {
		return nil
	}
	var p *planet.Planet
	if g.worldSt != nil {
		p = g.worldSt.planet
	}
	return travel.SettlementViews(g.ng.factions, p)
}

type marketWorldSource struct{ g *Game }

func (s marketWorldSource) MarketAt(t int) (maps.SettlementStock, bool) {
	return fleetOf(s.g).Market().At(t)
}
func (s marketWorldSource) FactionAt(t int) *faction.Faction {
	return s.g.factionOf(s.g.settlementFaction(t))
}
func (s marketWorldSource) Eligible(id entity.EntityID) bool {
	return s.g.standingPawn(id) && s.g.capacity(id, body.Talking) > 0 && !s.g.breaking(id)
}
func (s marketWorldSource) Terms(id entity.EntityID, fa *faction.Faction) trade.Terms {
	return s.g.termsFor(id, fa)
}
func (s marketWorldSource) Items() item.Book { return s.g.sk.content.Items }
func (s marketWorldSource) Cargo(cid uint64) []item.Stack {
	if s.g.worldSt.ax9 == nil {
		return nil
	}
	return travelOf(s.g).Goods(cid)
}
