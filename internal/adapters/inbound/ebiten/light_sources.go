package ebiten

import (
	"rimworld-go/internal/application/game"
)

// Light sources (PLAY-020 §3b): every building the renderer lights the
// world with comes from appendLightSources, and nothing else in the
// package tests a building's kind for light. The light grid (light.go)
// and the creature-shadow point lights (shadow_cast.go) both read its
// list, so PLAY-021 (power, torches) gates lights in this one place.

// lightKind is how a source lights: its falloff and its shadow height.
type lightKind uint8

const (
	lightLamp     lightKind = iota + 1 // a point lamp: smooth falloff, walls and rooms stop it, sprites shade toward it
	lightCampfire                      // a burning campfire: 1 − d/3.5 within 3 tiles, flat
)

// lightSource is one light: the building's id (shadow flicker and tie
// breaks), its tile and its reach in tiles.
type lightSource struct {
	ID     uint64
	X, Y   int
	Radius float64
	Kind   lightKind
}

// lampDefaultRadius is a lamp's reach when its sprite metadata has no
// light_radius.
const lampDefaultRadius = 6.0

// appendLightSources appends w's light sources to dst, in building order:
// every lamp (lamps are always on) with reach lampRadius, and every
// burning campfire. It reuses dst, so it allocates only while dst grows.
func appendLightSources(dst []lightSource, w *game.WorldView, lampRadius float64) []lightSource {
	for i := range w.Buildings {
		b := &w.Buildings[i]
		switch {
		case b.Kind == "lamp":
			dst = append(dst, lightSource{ID: uint64(b.ID), X: b.X, Y: b.Y, Radius: lampRadius, Kind: lightLamp})
		case b.Kind == "campfire" && b.Burning:
			dst = append(dst, lightSource{ID: uint64(b.ID), X: b.X, Y: b.Y, Radius: campfireReach, Kind: lightCampfire})
		}
	}
	return dst
}

// lampRadiusOf is the lamp sprite's light_radius from c's metadata (read
// without decoding the sprite), lampDefaultRadius without one.
func lampRadiusOf(c *artCatalog) float64 {
	if c != nil {
		if sp := c.sprites[buildSprite("lamp", "", 0, false, false)]; sp != nil && sp.meta.LightRadius > 0 {
			return sp.meta.LightRadius
		}
	}
	return lampDefaultRadius
}
