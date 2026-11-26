// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package worldview

import (
	"slices"

	"rimworld-go/internal/adapters/inbound/ebiten/globe"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/planet"
)

// TerritoryID is the toolbar toggle that tints the land each faction holds.
const TerritoryID = "world.territory"

// territoryState mirrors the territory of the view's own planet from the
// simulation's live settlements (the planet is a read-only copy, so a base
// the game has lost is still on it). It is rebuilt only when the live set or
// the planet changes, never per frame, and pushed to the globe once per
// version while the tint is on.
type territoryState struct {
	t       faction.Territory
	built   bool           // t was built at least once
	forP    *planet.Planet // the planet t was built for
	forLive []int          // the live tiles t was built for (sorted)
	on      bool           // the toolbar toggle
	to      *globe.View    // the globe last given the tint
	ver     uint64         // the version last pushed to it
	tinted  bool           // what was last pushed: a tint (true) or none
	pushes  int            // SetTerritory calls made (the once-per-version proof)
	types   []faction.Type
}

// sync rebuilds the territory when the live settlements or the planet
// changed since the last build, then pushes it to the globe if due.
func (s *territoryState) sync(p *planet.Planet, live []int, standing func(int) bool, g *globe.View) {
	if !s.built || s.forP != p || !slices.Equal(s.forLive, live) {
		planet.BuildTerritory(p, &s.t, standing)
		s.built, s.forP = true, p
		s.forLive = append(s.forLive[:0], live...)
		s.types = s.types[:0]
		for i := range p.Factions {
			s.types = append(s.types, p.Factions[i].Type)
		}
	}
	s.push(g)
}

// push gives the globe the tint (or takes it off) when what it holds is out
// of date: another globe (a rebuild), another version or the toggle.
func (s *territoryState) push(g *globe.View) {
	if g == nil || !s.built {
		return
	}
	ver := s.pushVersion()
	if s.to == g && s.tinted == s.on && (!s.on || s.ver == ver) {
		return
	}
	if s.on {
		g.SetTerritory(s.t.Owners(), ver, s.types)
	} else {
		g.SetTerritory(nil, ver, s.types)
	}
	s.to, s.ver, s.tinted = g, ver, s.on
	s.pushes++
}

// pushVersion is the version given to the globe: the territory's, with a hash
// of the faction types folded in, because the globe ignores a changed type
// list under an unchanged version.
func (s *territoryState) pushVersion() uint64 {
	h := uint64(14695981039346656037)
	for _, t := range s.types {
		h = (h ^ uint64(t)) * 1099511628211
	}
	return s.t.Version() + h<<20
}

// owner is the faction index owning tile t, -1 when unclaimed or unbuilt.
func (s *territoryState) owner(t int) int {
	if !s.built {
		return -1
	}
	return s.t.Owner(t)
}

// ownerName is the name of faction f among the game's factions, the
// simulation's own answer rather than the view planet's.
func ownerName(fs []game.FactionView, f int) string {
	for _, r := range fs {
		if r.ID == f {
			return r.Name
		}
	}
	return ""
}

// territoryRow is the tile card's Territory row ("Unclaimed" off every
// settlement's reach).
func (v inspector) territoryRow(t int) line {
	o := v.terr.owner(t)
	if o < 0 {
		return line{v.sh.T("Territory"), v.sh.T("Unclaimed")}
	}
	if g := v.sh.Env().Game; g != nil {
		if n := ownerName(g.Factions(), o); n != "" {
			return line{v.sh.T("Territory"), n}
		}
	}
	return line{v.sh.T("Territory"), v.sh.T("Unknown people")}
}

// toggleTerritory flips the tint and pushes the change at once.
func (v chromeBar) toggleTerritory() {
	v.terr.on = !v.terr.on
	v.terr.push(v.g)
}

// tradeGizmo is the Trade action of caravan c (the game's reason when it
// cannot): it sends TradeSettlement for the caravan. With no caravan
// (id 0 and why set by the caller) it is disabled.
func (v inspector) tradeGizmo(id uint64, why string) gizmo {
	g := gizmo{name: "trade", label: v.sh.T("Trade"), why: why,
		tip: v.sh.T("Trade\nTrade with the settlement this caravan stands at.")}
	if why == "" {
		g.run = func() {
			v.send(command.Command{Kind: command.TradeSettlement, Target: id})
		}
	}
	return g
}

// settlementTrade is the Trade gizmo of the settlement on tile: for our
// lowest-id caravan standing there, else disabled with a reason.
func (v inspector) settlementTrade(tile int) gizmo {
	var best *Object
	for i := range v.objs {
		o := &v.objs[i]
		if o.Kind == KindCaravan && o.Caravan.Tile == tile && (best == nil || o.Caravan.ID < best.Caravan.ID) {
			best = o
		}
	}
	if best == nil {
		return v.tradeGizmo(0, v.sh.T("Send a caravan here to trade."))
	}
	return v.tradeGizmo(best.Caravan.ID, best.Caravan.TradeWhy)
}
