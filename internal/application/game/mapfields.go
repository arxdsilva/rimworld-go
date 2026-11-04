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

// mapFieldCache is the generated (non-live) layers.
type mapFieldCache struct{ layers []MapFieldLayer }

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
	w, h := g.grid.Width(), g.grid.Height()
	if g.mapFields == nil {
		layers, err := mapfields.Generate(g.ng.setup.Seed, w, h, g.ng.setup.Site)
		if err != nil {
			return nil, err
		}
		g.mapFields = &mapFieldCache{layers}
	}
	out := append([]MapFieldLayer(nil), g.mapFields.layers...)
	return append(out, mapfields.Roofs(w, h, g.roofs), mapfields.Fog(w, h, g.fogged),
		mapfields.Regions(w, h, func(x, y int) bool { return g.terr.At(x, y).Walkable() })), nil
}
