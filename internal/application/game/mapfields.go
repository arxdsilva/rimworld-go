// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"errors"

	"rimworld-go/internal/application/game/mapfields"
	"rimworld-go/internal/domain/world/mapgen"
)

// Map field layers (game/mapfields, MAPGEN-2 §6.2): a developer read model,
// regenerated from the saved Setup on demand, cached on the Game, never saved
// or hashed.

// The layer's read models.
type (
	LegendEntry   = mapfields.LegendEntry
	MapFieldLayer = mapfields.Layer
)

// ErrMapFieldsOld is returned for a map the current generator cannot rebuild.
var ErrMapFieldsOld = errors.New("map made by an older generator")

// MapFieldLayers returns every debug layer. The generated ones are cached;
// roofs, fog and regions are rebuilt from the live map on each call.
func (g *Game) MapFieldLayers() ([]MapFieldLayer, error) {
	if !g.hasSetup() {
		return nil, errors.New("map fields need a landing-site colony")
	}
	if g.ng.setup.GenVersion != mapgen.Version {
		return nil, ErrMapFieldsOld
	}
	if g.mapFields == nil {
		g.mapFields = &mapfields.Cache{}
	}
	return g.mapFields.Layers(g.ng.setup.Seed, g.grid.Width(), g.grid.Height(),
		g.ng.setup.Site, g.roofs, g.fogged,
		func(x, y int) bool { return g.terr.At(x, y).Walkable() })
}
