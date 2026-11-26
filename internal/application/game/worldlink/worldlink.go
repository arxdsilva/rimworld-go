// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package worldlink

import (
	"fmt"
	"slices"

	"rimworld-go/internal/application/game/maps"
	"rimworld-go/internal/application/game/setupview"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/world/planet"
)

// Link records the planet of a standalone colony that predates Setup.
// Factions is the durable diplomacy state, which historical saves without
// NG1 otherwise have no place to persist.
type Link struct {
	Params   planet.Params
	Tile     int
	Factions []faction.Faction
}

func (l *Link) Clone() *Link {
	if l == nil {
		return nil
	}
	out := *l
	out.Params.Factions = slices.Clone(l.Params.Factions)
	out.Factions = setupview.CloneFactions(l.Factions)
	return &out
}

// Bind prepares a deterministic landing and merges original faction history
// with generated settlements. It does not change the local map or tick.
func Bind(params planet.Params, existing []faction.Faction) (*Link, []faction.Faction, error) {
	if params.Coverage <= 0 {
		return nil, nil, fmt.Errorf("planet coverage is missing")
	}
	p := planet.Build(params)
	tile := p.RandomLanding(0)
	if tile < 0 || !p.Land(tile) {
		return nil, nil, fmt.Errorf("planet has no valid landing tile")
	}
	if len(existing) > len(p.Factions) {
		return nil, nil, fmt.Errorf("existing factions exceed the generated planet")
	}
	next := setupview.CloneFactions(p.Factions)
	for i, old := range existing {
		if old.ID != i || old.Type != next[i].Type ||
			len(old.Settlements) != 0 || len(old.SettlementNames) != 0 {
			return nil, nil, fmt.Errorf("existing faction %d has incompatible world provenance", i)
		}
		next[i] = setupview.CloneFactions([]faction.Faction{old})[0]
		next[i].Settlements = slices.Clone(p.Factions[i].Settlements)
		next[i].SettlementNames = slices.Clone(p.Factions[i].SettlementNames)
	}
	bound := (&Link{Params: params, Tile: tile, Factions: next}).Clone()
	return bound, next, nil
}

// Planet rebuilds the unsaved globe of a historical colony using the
// existing native market/faction rules, and its durable diplomacy.
func Planet(l *Link, factions []faction.Faction) *planet.Planet {
	if l == nil {
		return nil
	}
	p := maps.BuildPlanet(l.Params, l.Tile, factions)
	if p != nil {
		p.Factions = setupview.CloneFactions(factions)
	}
	return p
}

// Context describes whether an existing colony may receive a planet.
// The game supplies these authoritative facts without exposing its stores.
type Context struct {
	HasWorld, Standalone, Active bool
}

// Attach binds a standalone map only when no prior world activity exists.
// An existing binding is idempotent; rejection never mutates source data.
func (c Context) Attach(params planet.Params, existing []faction.Faction) (*Link, []faction.Faction, bool, error) {
	switch {
	case c.HasWorld:
		return nil, nil, false, nil
	case !c.Standalone:
		return nil, nil, false, fmt.Errorf("world attachment requires one standalone map")
	case c.Active:
		return nil, nil, false, fmt.Errorf("world activity prevents attachment")
	}
	link, factions, err := Bind(params, existing)
	return link, factions, err == nil, err
}

// Snapshot saves a detached copy of the link and current diplomacy.
func (l *Link) Snapshot(factions []faction.Faction) *Link {
	s := l.Clone()
	if s != nil {
		s.Factions = setupview.CloneFactions(factions)
	}
	return s
}

// ParamsCopy keeps the World UI's returned settings detached from save state.
func (l *Link) ParamsCopy() planet.Params {
	p := l.Params
	p.Factions = slices.Clone(p.Factions)
	return p
}

// Restore detaches a saved link and restores its historic faction timeline.
// NG1 setup worlds already restore their factions from the setup record.
func Restore(saved *Link, hasSetup bool, current []faction.Faction) (*Link, []faction.Faction) {
	l := saved.Clone()
	if l == nil || hasSetup {
		return l, current
	}
	return l, setupview.CloneFactions(l.Factions)
}
